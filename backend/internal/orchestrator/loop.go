package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"unicode"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// runArchitect executes the architect phase once and returns its message so the
// relay discussion can seed its history.
func (o *Orchestrator) runArchitect(ctx context.Context, sCtx *agent.SwarmContext, projectID string) (*domain.AgentMessage, error) {
	o.broadcast(SwarmEvent{
		Type: EventAgentThinking, ProjectID: projectID, Round: sCtx.Round,
		Agent: domain.RoleArchitect,
		Payload: "در حال تحلیل نیازمندی‌ها، طراحی معماری و تجزیه پروژه به تسک‌های کاری...",
	})
	msg, err := o.architect.Execute(ctx, sCtx)
	if err != nil {
		return nil, fmt.Errorf("architect failed: %w", err)
	}
	o.broadcast(SwarmEvent{
		Type: EventAgentOutput, ProjectID: projectID, Round: sCtx.Round,
		Agent: domain.RoleArchitect, Message: msg,
		Usage: sCtx.UsageSnapshot(),
	})
	return msg, nil
}

// runCompilationGate turns real build errors into review issues, so a broken
// build can never be waved through by a high code-quality score.
func (o *Orchestrator) runCompilationGate(ctx context.Context, sCtx *agent.SwarmContext, ws *workspace.Workspace) {
	if len(sCtx.CodeFiles) == 0 || o.sandbox == nil {
		return
	}
	res := o.sandbox.VerifyCodebase(ctx, ws.Path(), sCtx.CodeFiles)
	if res.Success || len(res.ErrorsFound) == 0 {
		return
	}
	if sCtx.CriticReview == nil {
		sCtx.CriticReview = &domain.CriticPayload{}
	}
	for _, e := range res.ErrorsFound {
		sCtx.CriticReview.Issues = append(sCtx.CriticReview.Issues, domain.ReviewIssue{
			Severity:    "critical",
			File:        "verification",
			Description: fmt.Sprintf("Compiler/Syntax failure: %s", e),
		})
	}
}

// runReview runs the critic and the security auditor concurrently and returns
// their messages.
//
// Both must succeed: a review that errors is not a clean review, and treating it
// as one would let a task close on the strength of a failed check.
func (o *Orchestrator) runReview(ctx context.Context, sCtx *agent.SwarmContext, projectID string) (criticMsg, secMsg *domain.AgentMessage) {
	review := func(role domain.AgentRole, payload string, a agent.Agent) (*domain.AgentMessage, error) {
		o.broadcast(SwarmEvent{
			Type: EventAgentThinking, ProjectID: projectID, Round: sCtx.Round,
			Agent: role, Payload: payload,
		})
		msg, err := a.Execute(ctx, sCtx)
		if err != nil {
			return nil, err
		}
		o.broadcast(SwarmEvent{
			Type: EventAgentOutput, ProjectID: projectID, Round: sCtx.Round,
			Agent: role, Message: msg,
			Usage: sCtx.UsageSnapshot(),
		})
		return msg, nil
	}

	var criticErr, secErr error

	if sCtx.ReviewSequentially {
		criticMsg, criticErr = review(domain.RoleCritic,
			"در حال بازبینی موشکافانه، کشف باگ‌ها و لبه‌های تست...", o.critic)
		secMsg, secErr = review(domain.RoleSecurity,
			"در حال ممیزی امنیتی کد و جلوگیری از آسیب‌پذیری‌ها...", o.security)
	} else {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			criticMsg, criticErr = review(domain.RoleCritic,
				"در حال بازبینی موشکافانه، کشف باگ‌ها و لبه‌های تست...", o.critic)
		}()
		go func() {
			defer wg.Done()
			secMsg, secErr = review(domain.RoleSecurity,
				"در حال ممیزی امنیتی کد و جلوگیری از آسیب‌پذیری‌ها...", o.security)
		}()
		wg.Wait()
	}

	// A review failure is recorded as an issue rather than swallowed, so the
	// task goes back to rework instead of closing on an unperformed check.
	if criticErr != nil {
		recordReviewFailure(sCtx, "critic review could not be completed", criticErr)
	}
	if secErr != nil {
		recordReviewFailure(sCtx, "security audit could not be completed", secErr)
	}
	return criticMsg, secMsg
}

func recordReviewFailure(sCtx *agent.SwarmContext, what string, err error) {
	if sCtx.CriticReview == nil {
		sCtx.CriticReview = &domain.CriticPayload{}
	}
	sCtx.CriticReview.Issues = append(sCtx.CriticReview.Issues, domain.ReviewIssue{
		Severity:    "major",
		File:        "verification",
		Description: fmt.Sprintf("%s: %v", what, err),
	})
}

// runPlanDrivenLoop is the corrected round loop.
//
// It differs from the old loop in three ways, each fixing a specific failure:
//
//  1. It builds a plan first and takes one task per round, so the team advances
//     through the project instead of re-scoring whatever it just did.
//  2. It runs a verification gate before the arbiter, so a task closes only on
//     evidence from the real code, never on an agent saying "done".
//  3. It has no round ceiling. It stops when the plan is complete, or when the
//     swarm is provably stuck, and it reports which of those happened.
func (o *Orchestrator) runPlanDrivenLoop(ctx context.Context, sCtx *agent.SwarmContext, projectID string, ws *workspace.Workspace) (*agent.SwarmContext, error) {
	// Every execution path reaches the project through here, so this is where
	// file locking is switched on. Installing it in each caller instead would
	// mean a mode added later silently runs unprotected — the same mistake that
	// left one path behind when the plan-driven loop was adopted.
	//
	// The manager is per-run and lives in memory. It coordinates the agents of
	// this run only; a lock must never outlive the process that took it, so
	// nothing is written to disk that would need recovering later.
	if sCtx.FileLocks == nil {
		sCtx.FileLocks = workspace.NewFileLockManager()
	}

	watcher := newProgressWatcher(3)
	verifier := agent.NewTaskVerifier(o.client)
	if o.sandbox != nil {
		verifier.SetTestRunner(o.sandbox.RunTests)
	}

	// Every lock this run takes is dropped when it ends, including on an early
	// return or a panic. A hold left behind would keep the next run's agents out
	// of files for no reason.
	defer func() {
		for _, lock := range sCtx.LockedFiles() {
			sCtx.ReleaseFiles(lock.Agent, []string{lock.Path})
		}
	}()

	for {
		// A cancelled context is a user request, not a failure of the project.
		select {
		case <-ctx.Done():
			return sCtx, ctx.Err()
		default:
		}

		sCtx.ClearVerificationIssues()

		// 1. Choose this round's task. On the first round the architect both
		// designs the solution and decomposes it.
		if sCtx.Round == 1 {
			archMsg, err := o.runArchitect(ctx, sCtx, projectID)
			if err != nil {
				return nil, err
			}
			if sCtx.Intent != nil && sCtx.Intent.Kind != domain.IntentBuild {
				return o.runRelayDiscussion(ctx, sCtx, projectID, archMsg)
			}
			if err := o.ensurePlan(ctx, sCtx, ws); err != nil {
				return nil, err
			}
			o.broadcast(SwarmEvent{
				Type:      EventPlanUpdated,
				ProjectID: projectID,
				Round:     sCtx.Round,
				Plan:      sCtx.PlanSnapshot(),
			})
		}

		task := sCtx.NextTask()

		// No task can be selected, so the swarm has nothing left it can do on
		// its own. The plan distinguishes two endings that used to look
		// identical from the outside.
		if task == nil {
			plan := sCtx.PlanSnapshot()
			o.persistPlan(sCtx, ws)

			if sCtx.AllTasksDone() {
				// Everything either passed verification or was escalated, so the
				// run is over. Which of those happened is stated plainly: a run
				// that quietly mixed the two would report success on a project
				// that is visibly unfinished.
				report := describeRunEnd(plan)
				o.broadcast(SwarmEvent{
					Type:      EventConsensusReached,
					ProjectID: projectID,
					Round:     sCtx.Round,
					Verdict:   sCtx.ArbiterVerdict,
					Files:     sCtx.CodeFiles,
					// Plan travels with the event so a client reading the plan
					// rather than the prose can still tell a finished project
					// from one that stopped with work outstanding.
					Plan:    plan,
					Payload: report,
				})
				log.Printf("run finished: %s", report)
				return sCtx, nil
			}

			// Still open tasks that NextTask will not hand out, which should not
			// be reachable, but reporting beats spinning.
			log.Printf("run ended with %d unselectable tasks remaining", len(plan.OpenTasks()))
			return sCtx, nil
		}

		o.broadcast(SwarmEvent{
			Type:      EventPlanUpdated,
			ProjectID: projectID,
			Round:     sCtx.Round,
			Plan:      sCtx.PlanSnapshot(),
		})

		o.broadcast(SwarmEvent{
			Type:      EventRoundStarted,
			ProjectID: projectID,
			Round:     sCtx.Round,
			Payload:   fmt.Sprintf("Task %s: %s", task.ID, task.Title),
		})

		// Open this task's live card. Everything that follows — what the coder
		// decided, which files changed, what the tester found — appends to this
		// one card instead of arriving as a separate chat bubble.
		o.beginTaskStream(projectID, sCtx.Round, task)

		// 2. Implement the task. The file set is snapshotted first so this
		// round's real output can be told apart from everything built earlier.
		filesBefore := sCtx.CodeFilesSnapshot()

		// Take a short-lived hold on the files this task will touch.
		//
		// A plan alone cannot prevent two agents from rewriting the same shared
		// file: no planner sees the other agent's later edits, so both plans are
		// individually reasonable and the project ends up inconsistent. Only
		// mutual exclusion stops that.
		//
		// A refused claim does not end the round. The task is parked so the swarm
		// picks up different work and returns when the file is free, which beats
		// both forcing the edit and deadlocking behind a colleague.
		claimedFiles, claimOK, claimReason := o.claimTaskFiles(sCtx, task, string(domain.RoleCoder))
		if !claimOK {
			// Shown in the task's own card, not as a stray bubble: a blocked
			// agent is the most confusing thing a user can watch happen, so it
			// says which file, who holds it, and on what.
			//
			// Holds from earlier rounds are dropped before giving up on the task.
			// If the task escalates to WAITING_FOR_HUMAN it will never be handed
			// out again, so anything still held under its name would stay locked
			// for the rest of the run — a temporary collision turned permanent by
			// nothing more than the order these statements are written in.
			sCtx.ReleaseEverything(string(domain.RoleCoder))
			o.stepLock(projectID, sCtx.Round, task.ID, domain.RoleCoder, claimReason)
			sCtx.SendTaskBackToRework(claimReason)
			o.persistPlan(sCtx, ws)
			o.broadcastLocks(sCtx, projectID, sCtx.Round)
			continue
		}
		o.broadcastLocks(sCtx, projectID, sCtx.Round)
		// Released explicitly once this round's work concludes, NOT deferred.
		// A defer inside this loop would not run until the whole run ended, so a
		// file locked in round 2 would still be held in round 20 — the opposite
		// of the requirement that a file is locked only while it is being worked
		// on.
		releaseClaim := func() { o.releaseTaskFiles(sCtx, string(domain.RoleCoder), claimedFiles) }

		// 2. Implement the task.
		o.broadcast(SwarmEvent{
			Type:      EventAgentThinking,
			ProjectID: projectID,
			Round:     sCtx.Round,
			Agent:     domain.RoleCoder,
			Payload:   fmt.Sprintf("در حال پیاده‌سازی تسک %s: %s", task.ID, task.Title),
		})
		// What the coder is about to do, in its own words. This is the line that
		// makes the card read as an agent working rather than a file changing.
		o.stepThought(projectID, sCtx.Round, task.ID, domain.RoleCoder, "",
			fmt.Sprintf("در حال پیاده‌سازی تسک %s: %s", task.ID, task.Title))

		coderMsg, err := o.coder.Execute(ctx, sCtx)
		if err != nil {
			// The coder failed, so nothing is being written to these files any
			// more. Drop the hold on both exits below, or the next run would find
			// them locked by an agent that no longer exists.
			releaseClaim()
			if errors.Is(err, llm.ErrBudgetExhausted) {
				log.Printf("token budget exhausted on task %s, transitioning to WAITING_FOR_HUMAN", task.ID)
				sCtx.SetTaskState(task.ID, domain.TaskWaiting)
				o.persistPlan(sCtx, ws)
				o.broadcast(SwarmEvent{
					Type:      EventPlanUpdated,
					ProjectID: projectID,
					Round:     sCtx.Round,
					Plan:      sCtx.PlanSnapshot(),
				})
				o.broadcast(SwarmEvent{
					Type:      EventError,
					ProjectID: projectID,
					Round:     sCtx.Round,
					Payload:   "Token budget exhausted. Task is waiting for human intervention.",
				})
				return sCtx, nil
			}
			return nil, fmt.Errorf("coder failed on task %s: %w", task.ID, err)
		}
		if len(sCtx.CodeFiles) > 0 {
			if synced, err := ws.SyncFiles(sCtx.CodeFiles); err == nil {
				sCtx.ReplaceCodeFiles(synced)
			}
		}
		o.broadcast(SwarmEvent{
			Type: EventAgentOutput, ProjectID: projectID, Round: sCtx.Round,
			Agent: domain.RoleCoder, Message: coderMsg, Files: sCtx.CodeFiles,
			Usage: sCtx.UsageSnapshot(),
		})
		o.broadcast(SwarmEvent{
			Type: EventCodeUpdated, ProjectID: projectID, Round: sCtx.Round,
			Files: sCtx.CodeFiles,
		})

		// 3. Record what the coder produced, so the gate below knows which files
		// this task was supposed to deliver and can check they really exist.
		touched := filesTouchedThisRound(sCtx, filesBefore, sCtx.CodeFilesSnapshot())
		sCtx.SetActiveTaskFiles(touched)

		// Show what actually landed on disk. The file list is the concrete part
		// of "the work happened", and it is what the user would otherwise have to
		// infer from the diff view.
		if len(touched) > 0 {
			o.stepEdit(projectID, sCtx.Round, task.ID, domain.RoleCoder,
				fmt.Sprintf("%d فایل تغییر کرد", len(touched)), touched)
		}

		// The hold covers the edit and nothing more, so it is dropped here — the
		// moment the files are safely on disk. Review and verification only read,
		// and holding the lock through them would keep other agents out of a file
		// nobody is writing any more.
		//
		// Files the coder actually produced are released too. The planner's list
		// is a guess, and a task commonly writes a file nobody predicted; leaving
		// that one locked would strand it for the rest of the run.
		releaseClaim()
		if extra := touched; len(extra) > 0 {
			o.releaseTaskFiles(sCtx, string(domain.RoleCoder), extra)
		}
		o.broadcastLocks(sCtx, projectID, sCtx.Round)

		// 4. Review. Compilation failures are injected as real issues so a
		// broken build cannot be talked past.
		o.runCompilationGate(ctx, sCtx, ws)
		_, _ = o.runReview(ctx, sCtx, projectID)

		// 4. Verify against the real code. This runs before the arbiter so the
		// arbiter's verdict is informed by evidence rather than by opinion.
		verdict, err := verifier.Verify(ctx, sCtx)
		if err != nil {
			// A verification error means the gate could not run — a missing key,
			// an unreachable provider. It is not evidence that the task is
			// unfinished, and it is certainly not a reason to abandon the whole
			// project. Ending the run here was how one credential problem turned
			// into zero work done.
			log.Printf("verification could not run for task %s: %v", task.ID, err)
			o.broadcast(SwarmEvent{
				Type:      EventAgentOutput,
				ProjectID: projectID,
				Round:     sCtx.Round,
				Agent:     domain.RoleCritic,
				Payload: fmt.Sprintf(
					"تسک %s اعتبارسنجی نشد (%v). به‌عنوان انجام‌نشده علامت می‌خورد.", task.ID, err),
			})
			verdict = &agent.TaskVerdict{
				TaskID:  task.ID,
				Passed:  false,
				Gaps:    []string{"verification could not be run: " + err.Error()},
				Summary: "The verification gate could not run, so this task is not counted as done.",
			}
		}
		sCtx.SetLastTaskVerdict(verdict)

		// The evidence a task was judged on, in the task's own card. This is the
		// line that separates "the agent says it is done" from "the files exist and
		// the tests ran" — the distinction the whole verification gate exists for.
		if verdict != nil {
			if verdict.Passed {
				o.stepTest(projectID, sCtx.Round, task.ID, "تست‌ها پاس شدند: "+verdict.Summary)
			} else {
				o.stepReview(projectID, sCtx.Round, task.ID, domain.RoleTester,
					"تست‌ها پاس نشدند و تسک به بازگشت رفت: "+verdict.Summary)
			}
		}

		if verdict != nil {
			if verdict.Passed {
				sCtx.CompleteTask(touched)
			} else {
				sCtx.SendTaskBackToRework(verdict.Summary)
			}
			o.broadcast(SwarmEvent{
				Type:      EventPlanUpdated,
				ProjectID: projectID,
				Round:     sCtx.Round,
				Plan:      sCtx.PlanSnapshot(),
			})
			o.broadcast(SwarmEvent{
				Type:      EventAgentOutput,
				ProjectID: projectID,
				Round:     sCtx.Round,
				Agent:     domain.RoleCritic,
				Payload:   fmt.Sprintf("Task %s verification: %s", task.ID, verdict.Summary),
			})
		}

		// 5. Arbiter judges code quality for this round.
		o.broadcast(SwarmEvent{
			Type: EventAgentThinking, ProjectID: projectID, Round: sCtx.Round,
			Agent: domain.RoleArbiter,
			Payload: "در حال ارزیابی کیفیت این دور و ثبت رأی داور...",
		})
		arbMsg, err := o.arbiter.Execute(ctx, sCtx)
		if err != nil {
			return nil, fmt.Errorf("arbiter failed: %w", err)
		}
		o.broadcast(SwarmEvent{
			Type: EventAgentOutput, ProjectID: projectID, Round: sCtx.Round,
			Agent: domain.RoleArbiter, Message: arbMsg, Verdict: sCtx.ArbiterVerdict,
			Usage: sCtx.UsageSnapshot(),
		})
		o.broadcast(SwarmEvent{
			Type: EventRoundComplete, ProjectID: projectID, Round: sCtx.Round,
			Verdict: sCtx.ArbiterVerdict,
			Usage:   sCtx.UsageSnapshot(),
		})

		// A git checkpoint per task, so a bad change can be rolled back without
		// losing the work already accepted.
		var commitHash string
		if sCtx.ArbiterVerdict != nil {
			commitHash, _ = ws.CommitRound(ctx, sCtx.Round, sCtx.ArbiterVerdict.FinalScore, sCtx.ArbiterVerdict.Summary)
		}
		o.persistPlan(sCtx, ws)
		o.persistChangeLedger(task, touched, commitHash, verdict, ws)

		// 6. Finish only on a real completion signal.
		if sCtx.ArbiterVerdict != nil && sCtx.ArbiterVerdict.Passed {
			o.broadcast(SwarmEvent{
				Type: EventConsensusReached, ProjectID: projectID, Round: sCtx.Round,
				Verdict: sCtx.ArbiterVerdict, Files: sCtx.CodeFiles,
				Plan:    sCtx.PlanSnapshot(),
				Usage:   sCtx.UsageSnapshot(),
			})
			return sCtx, nil
		}

		// 7. No round ceiling. Stop only when the swarm has stopped changing
		// anything, which means repeating it cannot help.
		if watcher.record(sCtx) {
			// The active task was already cleared by CompleteTask or
			// SendTaskBackToRework, so the round's own task is used for the
			// report. Reading it from the context here would always report a
			// stall with no task named, which tells the user nothing.
			msg := describeStall(task, 3)
			log.Printf("run stalled: %s", msg)
			sCtx.BlockTask(task.ID)
			o.persistPlan(sCtx, ws)
			o.broadcast(SwarmEvent{
				Type:      EventPlanUpdated,
				ProjectID: projectID,
				Round:     sCtx.Round,
				Plan:      sCtx.PlanSnapshot(),
			})
			o.broadcast(SwarmEvent{
				Type: EventError, ProjectID: projectID, Round: sCtx.Round, Payload: msg,
			})
			return sCtx, nil
		}

		sCtx.Round++
	}
}

// ensurePlan loads the project's work list or builds one.
//
// The plan is loaded from disk when it exists so a resumed run continues where
// it stopped instead of re-planning from scratch and losing finished work.
func (o *Orchestrator) ensurePlan(ctx context.Context, sCtx *agent.SwarmContext, ws *workspace.Workspace) error {
	// A plan already on disk is only resumed when it belongs to the request being
	// made now. Loading it unconditionally meant a second, different request in
	// the same project silently continued the first one's work: the user asked
	// for something new and the swarm kept building the old thing. That is the
	// most confusing failure this loop can have, because the output looks
	// plausible and nothing reports an error.
	if plan, err := workspace.LoadPlan(ws.Path()); err == nil && plan != nil && len(plan.Tasks) > 0 {
		if !planMatchesRequest(plan, sCtx.UserPrompt) {
			log.Printf("existing plan does not match this request; building a new one")
			// The old board is written to its own file so a user who meant to
			// resume can still find it. Overwriting plan.json here would destroy
			// it within one round, since persistPlan writes the new plan there.
			if brain, bErr := workspace.NewBrainFS(ws.Path()); bErr == nil {
				if path, aErr := brain.ArchivePlan(plan, "a different request was made in this project"); aErr != nil {
					log.Printf("could not archive the previous plan: %v", aErr)
				} else {
					log.Printf("previous plan archived to %s", path)
				}
			}
		} else {
			sCtx.SetPlan(plan)
			log.Printf("resumed existing plan: %d tasks, %d done", len(plan.Tasks), countDone(plan))
			return nil
		}
	}

	architect, ok := o.architect.(interface {
		BuildPlan(context.Context, *agent.SwarmContext) (*domain.ProjectPlan, error)
	})
	if !ok {
		return fmt.Errorf("no planner available, so the project cannot be tracked; " +
			"without a task list there is no way to know what remains")
	}

	plan, err := architect.BuildPlan(ctx, sCtx)
	if err != nil {
		// A planning failure must not be swallowed into a silently empty
		// backlog, because an empty backlog reads as a finished project.
		if fallback := agent.DerivePlanFromFiles(sCtx.UserPrompt, blueprintFileTree(sCtx), sCtx.CodeFilesSnapshot(), sCtx.Round); fallback != nil {
			agent.LogPlanOutcome("derived-from-blueprint", fallback, nil)
			sCtx.SetPlan(fallback)
			return nil
		}
		return fmt.Errorf("project planning failed and no fallback work list could be built: %w", err)
	}

	agent.LogPlanOutcome("architect", plan, nil)
	sCtx.SetPlan(plan)
	return nil
}

// filesTouchedThisRound lists the files this round's work actually produced.
//
// It diffs against the state from before the coder ran, rather than returning
// every file in the workspace. The file set is cumulative across rounds, so the
// undiffed version handed the fifth task every file the project had produced so
// far: the gate then checked files the task never claimed, and a task whose real
// deliverable was missing could still pass because the listed paths happened to
// exist already.
func filesTouchedThisRound(sCtx *agent.SwarmContext, before, after []domain.CodeFile) []string {
	prior := make(map[string]string, len(before))
	for _, f := range before {
		prior[f.Path] = f.Content
	}

	active := sCtx.ActiveTaskSnapshot()
	if active != nil && len(active.Files) > 0 {
		// The task named its own files, which is a stronger statement than any
		// diff: the coder was told to produce exactly these.
		return active.Files
	}

	var out []string
	for _, f := range after {
		if prev, existed := prior[f.Path]; !existed || prev != f.Content {
			out = append(out, f.Path)
		}
	}
	return out
}

// planMatchesRequest reports whether a stored plan was made for this request.
//
// The stored goal is the planner's one-line summary of the original request, so
// it cannot be compared literally. What is compared is the vocabulary both texts
// actually use: a request about a Duolingo clone and a stored plan about a
// different app share almost no significant words, while a re-run of the same
// request overlaps heavily. The threshold is deliberately strict — wrongly
// rebuilding a plan loses a little time, while wrongly resuming an unrelated
// one silently builds the wrong project.
func planMatchesRequest(plan *domain.ProjectPlan, prompt string) bool {
	if plan == nil || plan.Goal == "" || strings.TrimSpace(prompt) == "" {
		return false
	}
	// A superseded plan is never resumed.
	if strings.HasPrefix(plan.Goal, "(superseded)") {
		return false
	}

	// Compare task titles too, not just the goal: they carry the concrete nouns.
	existing := strings.ToLower(plan.Goal)
	for _, t := range plan.Tasks {
		existing += " " + strings.ToLower(t.Title)
	}
	wanted := strings.ToLower(prompt)

	// Every meaningful word of the request should appear in the stored plan. Stop
	// words are excluded because they carry no signal and would make unrelated
	// requests look similar.
	requestWords := significantWords(wanted)
	if len(requestWords) == 0 {
		return false
	}
	covered := 0
	for _, w := range requestWords {
		if strings.Contains(existing, w) {
			covered++
		}
	}
	return float64(covered)/float64(len(requestWords)) >= 0.7
}

// significantWords splits text into lowercase words longer than three letters,
// dropping the common filler that would otherwise match almost anything.
func significantWords(text string) []string {
	drop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "that": true,
		"this": true, "from": true, "your": true, "have": true, "make": true,
		"build": true, "create": true, "want": true, "need": true, "please": true,
		"یک": true, "برای": true, "بسازید": true, "کنید": true, "من": true,
		"یا": true, "تا": true, "که": true, "را": true, "در": true, "با": true,
		"است": true, "های": true, "هایی": true, "می": true, "شود": true,
	}

	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	var out []string
	seen := map[string]bool{}
	for _, f := range fields {
		w := strings.TrimSpace(f)
		if len([]rune(w)) <= 3 || drop[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

func blueprintFileTree(sCtx *agent.SwarmContext) []string {
	if sCtx.Blueprint == nil {
		return nil
	}
	if len(sCtx.Blueprint.FileTree) > 0 {
		return sCtx.Blueprint.FileTree
	}
	var out []string
	for _, f := range sCtx.Blueprint.Files {
		out = append(out, f.Path)
	}
	return out
}

func countDone(plan *domain.ProjectPlan) int {
	if plan == nil {
		return 0
	}
	n := 0
	for _, t := range plan.Tasks {
		if t.State == domain.TaskDone {
			n++
		}
	}
	return n
}

// persistPlan writes the task board to disk after every round, so a crashed run
// resumes from the real state rather than guessing.
func (o *Orchestrator) persistPlan(sCtx *agent.SwarmContext, ws *workspace.Workspace) {
	plan := sCtx.PlanSnapshot()
	if plan == nil || ws == nil {
		return
	}
	brain, err := workspace.NewBrainFS(ws.Path())
	if err != nil {
		return
	}
	if err := brain.WritePlan(plan); err != nil {
		log.Printf("failed to persist plan: %v", err)
		return
	}
	if err := brain.RenderTasks(plan); err != nil {
		log.Printf("failed to render task board: %v", err)
	}
}

// persistChangeLedger writes an immutable change record to .project/ledger.jsonl
// after each round according to PROJECT_VISION §30.
func (o *Orchestrator) persistChangeLedger(task *domain.ProjectTask, touched []string, commitHash string, verdict *agent.TaskVerdict, ws *workspace.Workspace) {
	if ws == nil || len(touched) == 0 {
		return
	}
	brain, err := workspace.NewBrainFS(ws.Path())
	if err != nil {
		return
	}
	taskID := ""
	reason := ""
	if task != nil {
		taskID = task.ID
		reason = task.Title
	}
	for _, f := range touched {
		var testRec *workspace.TestLedgerRecord
		if verdict != nil {
			testRec = &workspace.TestLedgerRecord{
				Result: verdict.Summary,
			}
		}
		_ = brain.RecordChange(workspace.ChangeLedgerEntry{
			TaskID: taskID,
			File:   f,
			Agent:  string(domain.RoleCoder),
			Commit: commitHash,
			Reason: reason,
			Tests:  testRec,
		})
	}
}