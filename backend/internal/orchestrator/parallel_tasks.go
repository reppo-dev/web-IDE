package orchestrator

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// Parallel task execution.
//
// The swarm used to advance one task per round: pick a task, run the coder,
// review it, judge it, close it. That is correct but strictly serial, so a
// twenty-five task project took as long as the sum of its tasks rather than the
// longest chain among them.
//
// Running tasks at the same time is only safe because two things now exist that
// did not before. Each agent gets a forked context, so two coders cannot corrupt
// one another's review fields. And each takes a file lock first, so two coders
// cannot silently disagree about the same file. Removing either one turns this
// from a speedup into a source of corrupt output.

// maxParallelTasks bounds how many agents run at once.
//
// The limit is on concurrency, not on ambition: a larger number would not finish
// the project faster, because every agent in the batch is competing for the same
// provider rate limit. Past a handful the wall time is dominated by queuing, and
// each additional agent mostly adds cost.
const maxParallelTasks = 3

// ParallelOutcome is what one task produced when it ran alongside others.
type ParallelOutcome struct {
	Task *domain.ProjectTask
	// Skipped explains why a task was not run. An empty value means it ran.
	Skipped string
	// Verdict is the verification result for a task that ran.
	Verdict *agent.TaskVerdict
	// Files are the paths this task actually wrote.
	Files []string
	// AgentName identifies which worker did the work, so the UI can attribute it.
	AgentName string
}

// selectParallelTasks picks tasks that can safely run at the same time.
//
// Independence is decided by file overlap, not by index. Two tasks listed next to
// each other in the plan may well write the same file, and two tasks far apart may
// share nothing at all — order in a plan is not evidence of independence, and
// treating it as such is how parallel execution produces two versions of the same
// component.
func selectParallelTasks(sCtx *agent.SwarmContext, limit int) []ParallelSlot {
	plan := sCtx.PlanSnapshot()
	if plan == nil || limit <= 0 {
		return nil
	}

	candidates := sCtx.AvailableTasks()
	if len(candidates) == 0 {
		return nil
	}

	var slots []ParallelSlot
	claimed := make(map[string]string) // path -> task that holds it

	for _, task := range candidates {
		if len(slots) >= limit {
			break
		}
		if conflicts := overlappingFiles(task.Files, claimed); len(conflicts) > 0 {
			// Left for a later batch, not dropped: once the earlier task finishes
			// and releases its files, this one becomes runnable.
			continue
		}
		for _, f := range task.Files {
			norm := normalizeTaskPath(f)
			if norm != "" {
				claimed[norm] = task.ID
			}
		}
		slots = append(slots, ParallelSlot{Task: task})
	}
	return slots
}

// ParallelSlot is one task's place in a parallel batch.
type ParallelSlot struct {
	Task *domain.ProjectTask
	// SkipTask is set when the task could not be started, so the batch does not
	// silently shrink without saying why.
	SkipTask string
}

// overlappingFiles reports which of a task's files another task already has.
func overlappingFiles(files []string, claimed map[string]string) []string {
	var conflicts []string
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		norm := normalizeTaskPath(f)
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		if holder, taken := claimed[norm]; taken && holder != "" {
			conflicts = append(conflicts, f)
		}
	}
	sort.Strings(conflicts)
	return conflicts
}

// runParallelTasks implements one batch of concurrently executed tasks.
//
// Every worker follows the same shape: fork a context, claim the files, do the
// work, release the files, verify. The differences are all in what each worker is
// *allowed* to touch, which is what keeps the concurrency safe.
func (o *Orchestrator) runParallelTasks(
	ctx context.Context,
	sCtx *agent.SwarmContext,
	projectID string,
	ws *workspace.Workspace,
	slots []ParallelSlot,
) []ParallelOutcome {
	if len(slots) == 0 {
		return nil
	}

	// One verifier per worker, not one shared. The verifier is stateful and each
	// worker verifying its own task is the whole point of the split.
	results := make([]ParallelOutcome, len(slots))
	var wg sync.WaitGroup

	for i, slot := range slots {
		if slot.Task == nil {
			results[i] = ParallelOutcome{Skipped: "no task was selected for this slot"}
			continue
		}

		task := slot.Task
		agentName := fmt.Sprintf("coder-%d", i+1)

		// Claim before forking: a worker that cannot get its files must not spend
		// a model call discovering that.
		if _, ok, reason := o.claimTaskFiles(sCtx, task, agentName); !ok {
			results[i] = ParallelOutcome{Task: task, Skipped: reason, AgentName: agentName}
			continue
		}

		wg.Add(1)
		go func(idx int, task *domain.ProjectTask, agentName string) {
			defer wg.Done()
			results[idx] = o.runOneParallelTask(ctx, sCtx, projectID, task, agentName)
		}(i, task, agentName)
	}

	wg.Wait()
	return results
}

// runOneParallelTask is a single worker's whole life cycle.
//
// It writes nothing to disk itself. Every worker runs concurrently, and having
// each one flush its own file list would mean two goroutines writing the same
// files at the same moment. The batch writes once, after all workers finish.
func (o *Orchestrator) runOneParallelTask(
	ctx context.Context,
	sCtx *agent.SwarmContext,
	projectID string,
	task *domain.ProjectTask,
	agentName string,
) ParallelOutcome {
	outcome := ParallelOutcome{Task: task, AgentName: agentName}

	// Released on every path out of this function. A worker that returns early —
	// cancelled, errored, budget exhausted — must not leave files held, because
	// the next batch would then skip a task for no reason the user can see.
	defer sCtx.ReleaseEverything(agentName)

	o.beginTaskStream(projectID, sCtx.Round, task)

	fork := sCtx.Fork(domain.RoleCoder, task.ID)
	if fork == nil {
		outcome.Skipped = "the project context could not be prepared for this task"
		return outcome
	}

	// The lock is shared with every other worker, so it must be held through the
	// fork as well.
	fork.FileLocks = sCtx.FileLocks

	o.broadcastLocks(sCtx, projectID, sCtx.Round)

	filesBefore := map[string]string{}
	for _, f := range fork.CodeFiles {
		filesBefore[f.Path] = f.Content
	}

	o.stepThoughtAs(projectID, sCtx.Round, task.ID, agentName, "",
		fmt.Sprintf("%s در حال پیاده‌سازی تسک %s: %s", agentName, task.ID, task.Title))

	// A cancelled run must not keep calling models.
	select {
	case <-ctx.Done():
		outcome.Skipped = "the run was cancelled before this task started"
		return outcome
	default:
	}

	// The coder instance is shared across workers. That is only safe because the
	// agent objects themselves are stateless between calls — all per-task state
	// lives in the fork — so two workers never touch the same mutable field.
	msg, err := o.coder.Execute(ctx, fork)
	if err != nil {
		log.Printf("parallel coder failed on %s: %v", task.ID, err)
		outcome.Skipped = fmt.Sprintf("the coder could not finish this task: %v", err)
		return outcome
	}

	// Only the files this worker actually added or rewrote.
	//
	// Comparing against the fork's starting contents, not against a slice
	// length. A coder that revises an existing file does not grow the list, so
	// slicing by count reported nothing for the most common case of all — and
	// merging the whole list instead would let one worker write back its stale
	// snapshot over another worker's finished work.
	touched := filesChangedSince(filesBefore, fork.CodeFiles)
	if len(touched) > 0 {
		outcome.Files = relativePaths(touched)
	}

	// Verify this task against the real code, in this worker, before anything is
	// merged. Merging first would let one task's output be judged as part of
	// another's.
	verifier := agent.NewTaskVerifier(o.llmClient())
	if o.sandbox != nil {
		verifier.SetTestRunner(o.sandbox.RunTests)
	}
	verdict, err := verifier.Verify(ctx, fork)
	if err != nil {
		verdict = &agent.TaskVerdict{
			TaskID:  task.ID,
			Passed:  false,
			Gaps:    []string{"verification could not be run: " + err.Error()},
			Summary: "The verification gate could not run, so this task is not counted as done.",
		}
	}
	outcome.Verdict = verdict

	if verdict != nil && verdict.Passed {
		o.stepTestAs(projectID, sCtx.Round, task.ID, agentName,
			"تست‌ها پاس شدند: "+verdict.Summary)
	} else if verdict != nil {
		o.taskStepAs(projectID, sCtx.Round, task.ID, domain.RoleTester, agentName, StepReview,
			"تست‌ها پاس نشدند: "+verdict.Summary, nil)
	}

	// Everything the worker produced goes back in one merge, on this goroutine.
	collisions := sCtx.MergeResult(agent.ForkResult{
		Role:           domain.RoleCoder,
		TaskID:         task.ID,
		Message:        msg,
		CriticReview:   fork.CriticReview,
		SecurityReport: fork.SecurityReport,
		Blueprint:      fork.Blueprint,
		Files:          touched,
		HasFiles:       len(touched) > 0,
		Usage:          fork.TakeUsage(),
	})

	if len(collisions) > 0 {
		// The file lock should have made this impossible. If it happened anyway,
		// say so loudly rather than shipping one agent's version over another's.
		log.Printf("parallel workers both wrote %v despite the lock", collisions)
		o.taskStepAs(projectID, sCtx.Round, task.ID, domain.RoleSecurity, "manager", StepReview,
			fmt.Sprintf("هشدار: چند کدنویس فایل‌های یکسان را نوشتند (%s)", collisions[0]), nil)
	}

	if len(touched) > 0 {
		o.stepEditAs(projectID, sCtx.Round, task.ID, agentName,
			fmt.Sprintf("%d فایل تغییر کرد", len(touched)), outcome.Files)
	}

	o.broadcastLocks(sCtx, projectID, sCtx.Round)
	return outcome
}

// filesChangedSince reports which files a worker actually wrote.
//
// The original files are keyed by path so the comparison is on content, not on
// list length. That distinction is the whole point: a revision to an existing
// file leaves the list the same size while being the single most common thing a
// task does, and a length-based check misses every one of them.
func filesChangedSince(before map[string]string, after []domain.CodeFile) []domain.CodeFile {
	var changed []domain.CodeFile
	for _, f := range after {
		previous, existed := before[f.Path]
		if !existed || previous != f.Content {
			changed = append(changed, f)
		}
	}
	return changed
}

// relativePaths reduces absolute file records to project-relative paths.
//
// The absolute form is what the disk layer uses; what a human reads in a task
// card, and what the plan stores, is the path relative to the project root.
func relativePaths(files []domain.CodeFile) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		if f.Path != "" {
			out = append(out, f.Path)
		}
	}
	return out
}

// normalizeTaskPath reduces a planned path to a comparable form, matching the
// lock manager's own normalization so task selection and file locking agree on
// what counts as the same file.
func normalizeTaskPath(p string) string {
	if p == "" {
		return ""
	}
	return workspace.NormalizePath(p)
}

// runParallelBatch advances one batch of independent tasks and reports whether it
// did any work.
//
// It returns false when there is no batch to run, which is the signal for the
// caller to fall back to the single-task path. That distinction is what keeps the
// serial path meaningful: it still runs whenever parallelism cannot help, rather
// than a parallel path that quietly does nothing and leaves the run stalled.
func (o *Orchestrator) runParallelBatch(
	ctx context.Context,
	sCtx *agent.SwarmContext,
	projectID string,
	ws *workspace.Workspace,
) (bool, error) {
	slots := selectParallelTasks(sCtx, o.parallelBudget())
	if len(slots) == 0 {
		// Nothing independent is available right now. That is not a failure: it
		// may mean every open task shares a file, in which case the serial path
		// will make progress on one of them.
		return false, nil
	}

	// Claim the batch before starting, so two workers can never be handed the
	// same task and the plan reflects reality from the moment work begins.
	chosen := make([]*domain.ProjectTask, 0, len(slots))
	for _, slot := range slots {
		if slot.Task != nil {
			chosen = append(chosen, slot.Task)
		}
	}
	claimed := sCtx.ClaimTasksForBatch(chosen)
	if len(claimed) == 0 {
		return false, nil
	}
	// Pair the claimed tasks back to their slots, preserving order, so a worker
	// reports against the task it actually claimed.
	for i := range slots {
		if slots[i].Task == nil {
			continue
		}
		for _, task := range claimed {
			if task.ID == slots[i].Task.ID {
				slots[i].Task = task
				break
			}
		}
	}

	o.broadcast(SwarmEvent{
		Type:      EventRoundStarted,
		ProjectID: projectID,
		Round:     sCtx.Round,
		Payload: fmt.Sprintf("%d تسک مستقل به‌صورت موازی اجرا می‌شود",
			len(slots)),
	})

	outcomes := o.runParallelTasks(ctx, sCtx, projectID, ws, slots)

	// Everything the workers produced is written to disk in one pass, by one
	// goroutine. Writing from the workers themselves would let two agents flush
	// the same file list at the same moment.
	if len(sCtx.CodeFilesSnapshot()) > 0 {
		if synced, err := ws.SyncFiles(sCtx.CodeFilesSnapshot()); err == nil {
			sCtx.ReplaceCodeFiles(synced)
		}
	}

	o.applyBatchOutcomes(sCtx, ws, outcomes, projectID, sCtx.Round)

	// Review and judge the batch as a whole. Each task was already verified on its
	// own evidence above, so this is judging the assembled result, not deciding
	// whether individual tasks counted.
	o.runCompilationGate(ctx, sCtx, ws)
	o.runReview(ctx, sCtx, projectID)

	o.broadcast(SwarmEvent{
		Type:      EventAgentThinking,
		ProjectID: projectID,
		Round:     sCtx.Round,
		Agent:     domain.RoleArbiter,
		Payload:   "در حال ارزیابی کیفیت خروجی این دسته و ثبت رأی داور...",
	})
	if _, err := o.arbiter.Execute(ctx, sCtx); err != nil {
		// The arbiter is the last step of a batch, so a failure here means the
		// work is done but unjudged. That is worth reporting, not worth discarding:
		// the next round will judge whatever comes after it.
		log.Printf("arbiter failed after a parallel batch: %v", err)
	}

	verdict := sCtx.ArbiterVerdict
	// A passing verdict may only stand when the project is actually finished.
	// Judging the batch as a whole is exactly the judgement that once let a clean
	// partial project report itself finished, so the plan gets the final say.
	if verdict != nil {
		plan := sCtx.PlanSnapshot()
		if plan != nil && !plan.Completed() {
			verdict.Passed = false
			_, _, percent := plan.Progress()
			verdict.Summary = fmt.Sprintf(
				"امتیاز این دسته %.0f%% است، اما پروژه هنوز %.0f%% کامل است. ادامه می‌دهیم.",
				verdict.FinalScore, percent)
		}
	}

	if verdict != nil {
		ws.CommitRound(ctx, sCtx.Round, verdict.FinalScore, verdict.Summary)
	}

	o.broadcast(SwarmEvent{
		Type:      EventRoundComplete,
		ProjectID: projectID,
		Round:     sCtx.Round,
		Verdict:   verdict,
		Files:     sCtx.CodeFilesSnapshot(),
		Usage:     sCtx.UsageSnapshot(),
	})

	// A batch where every task was skipped made no progress at all.
	//
	// It still reports handled=true, because the batch did run and did write its
	// outcomes back to the plan. What it must not do is let the caller believe a
	// round's worth of work happened — so the distinction is in the plan's state,
	// not in this return value, and the caller re-checks progress itself.
	if !batchMadeProgress(outcomes) {
		log.Printf("parallel batch made no progress: every task was skipped")
	}

	return true, nil
}

// batchMadeProgress reports whether at least one worker actually did something.
//
// It is the guard against a livelock: when every task in a batch comes back
// skipped, nothing has changed, and treating that as a completed round only
// advances a counter over the same stuck state.
func batchMadeProgress(outcomes []ParallelOutcome) bool {
	for _, outcome := range outcomes {
		if outcome.Skipped == "" {
			return true
		}
	}
	return false
}

// taskListOf pulls the tasks out of a batch of slots.
//
// Claiming needs a flat list, while the slots also carry the reason a task was
// skipped. Keeping the two apart means a skipped task is never handed to a
// worker just because it was part of the batch.
func taskListOf(slots []ParallelSlot) []*domain.ProjectTask {
	tasks := make([]*domain.ProjectTask, 0, len(slots))
	for _, slot := range slots {
		if slot.Task != nil {
			tasks = append(tasks, slot.Task)
		}
	}
	return tasks
}

// parallelBudget reports how many tasks may run at once.
//
// A configuration that disables parallelism must still behave correctly rather
// than fall over, so anything below two collapses to the serial path. That is why
// this returns a minimum of one instead of erroring.
func (o *Orchestrator) parallelBudget() int {
	n := o.config.MaxParallelTasks
	if n <= 0 {
		return 1
	}
	if n > maxParallelTasks {
		return maxParallelTasks
	}
	return n
}

// parallelEnabled reports whether a batch may be built at all.
func (o *Orchestrator) parallelEnabled() bool {
	return o.parallelBudget() > 1
}

// applyBatchOutcomes folds each parallel worker's result back into the plan.
//
// The ordering here matters. A task is closed only if its own verification
// passed, and a task that was skipped is returned to the queue rather than
// counted. Letting the batch score carry a task would reintroduce the original
// defect in a new place: a project where two tasks ran cleanly would report
// itself finished while a third was never touched.
func (o *Orchestrator) applyBatchOutcomes(sCtx *agent.SwarmContext, ws *workspace.Workspace, outcomes []ParallelOutcome, projectID string, round int) {
	for _, outcome := range outcomes {
		if outcome.Task == nil {
			continue
		}

		if outcome.Skipped != "" {
			// Not an error: a blocked task simply waits. Returning it to the queue
			// is what lets a later batch pick it up once the file frees.
			sCtx.ReturnTaskToQueue(outcome.Task.ID)
			o.taskStep(projectID, round, outcome.Task.ID, domain.RoleManager, StepLock,
				outcome.Skipped, nil)
			continue
		}

		sCtx.SetLastTaskVerdict(outcome.Verdict)

		if outcome.Verdict != nil && outcome.Verdict.Passed {
			sCtx.CompleteTaskAs(outcome.Task.ID, outcome.Files)
			continue
		}

		reason := "the verification gate rejected this task"
		if outcome.Verdict != nil && outcome.Verdict.Summary != "" {
			reason = outcome.Verdict.Summary
		}
		sCtx.SendTaskBackToReworkAs(outcome.Task.ID, reason)
	}

	o.persistPlan(sCtx, ws)
	o.broadcast(SwarmEvent{
		Type:      EventPlanUpdated,
		ProjectID: projectID,
		Round:     round,
		Plan:      sCtx.PlanSnapshot(),
	})
}
