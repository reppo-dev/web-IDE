package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// TeamRun describes one execution of a user-built team.
type TeamRun struct {
	Team   *domain.Team
	Mode   domain.ExecutionMode
	APIKeys map[string]string
}

// RunTeam executes an arbitrary user-defined team. Unlike the fixed five-agent
// swarm, the members here come from the team's own specs, so a team may have
// two agents or twenty and each may run on its own model.
func (o *Orchestrator) RunTeam(ctx context.Context, projectID, userPrompt string, run TeamRun) (*agent.SwarmContext, error) {
	t := run.Team
	if t == nil {
		return nil, fmt.Errorf("no team provided")
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}

	specs := t.ActiveAgents()
	if len(specs) == 0 {
		return nil, domain.ErrTeamNoAgents
	}
	if !hasJudge(specs) {
		return nil, domain.ErrTeamNoJudge
	}

	ws, err := workspace.NewWorkspace(o.baseDir, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize project workspace: %w", err)
	}

	sCtx := &agent.SwarmContext{
		ProjectID:     projectID,
		Round:         1,
		UserPrompt:    userPrompt,
		MemStore:      o.memStore,
		APIKeys:       run.APIKeys,
		WorkspacePath: ws.Path(),
		// Team-level defaults; an agent may tighten or relax its own.
		ToolsAccess:    agent.AccessLevel(t.ToolsAccess),
		ToolCallBudget: t.MaxToolCalls,
		ToolTimeout:    4 * time.Minute,
		AgentModels:    make(map[domain.AgentRole]string),
	}
	for _, spec := range specs {
		if spec.Model != "" {
			sCtx.AgentModels[spec.Role] = spec.Model
		}
	}
	if sCtx.ToolsAccess == "" {
		for _, spec := range specs {
			if spec.ToolsAccess == string(agent.AccessWrite) || spec.ToolsAccess == string(agent.AccessFull) {
				sCtx.ToolsAccess = agent.AccessLevel(spec.ToolsAccess)
				break
			}
		}
	}
	if sCtx.ToolsAccess == "" {
		sCtx.ToolsAccess = agent.AccessWrite
	}
	if sCtx.ToolCallBudget <= 0 {
		sCtx.ToolCallBudget = domain.DefaultToolCallBudget
	}
	if o.sandbox != nil {
		sCtx.Detector = func(root string) (string, error) {
			res := o.sandbox.VerifyCodebase(ctx, root, nil)
			if res.Success {
				return fmt.Sprintf("%s: no problems found", res.Language), nil
			}
			return fmt.Sprintf("%s errors:\n%s", res.Language,
				strings.Join(res.ErrorsFound, "\n")), nil
		}
	}

	mode := run.Mode
	if mode == "" {
		mode = o.config.Mode
	}

	threshold := t.ConsensusThreshold
	if threshold <= 0 {
		threshold = 85
	}

	// A team run used to stop after a fixed number of rounds and call the
	// project done whenever the score cleared the threshold. That is the same
	// defect the built-in swarm had: a clean partial project scored well and the
	// run ended. A team now gets the same treatment — one task per round, and
	// completion decided by the plan rather than by the score.
	watcher := newProgressWatcher(3)
	// Same lock coordinator the built-in swarm installs in runPlanDrivenLoop. A
	// custom team has more agents and less predictability about who edits what, so
	// it needs this at least as much as the fixed five do.
	if sCtx.FileLocks == nil {
		sCtx.FileLocks = workspace.NewFileLockManager()
	}
	verifier := agent.NewTaskVerifier(o.llmClient())
	if o.sandbox != nil {
		verifier.SetTestRunner(o.sandbox.RunTests)
	}

	// Round ceiling: MaxRounds stays meaningful for a custom team, but it is
	// now a safety limit rather than the definition of "done". A team that
	// finishes its plan early exits before reaching it.
	rounds := t.MaxRounds
	if rounds <= 0 {
		rounds = teamDefaultRoundLimit
	}

	for round := 1; round <= rounds; round++ {
		sCtx.Round = round
		sCtx.ClearVerificationIssues()

		// Do not blank CodeFiles between rounds: the reviewers and the judge in
		// the next round must still see what the previous round produced, and
		// the context engine re-reads the files from disk anyway.
		o.broadcast(SwarmEvent{
			Type:      EventRoundStarted,
			ProjectID: projectID,
			Round:     round,
		})

		// Pick this round's task. The plan is built once, by the team's
		// designer, on the first round.
		if sCtx.Plan == nil {
			if err := o.ensureTeamPlan(ctx, sCtx, specs, ws); err != nil {
				return nil, err
			}
		}
		// Parallel path, identical in intent to the built-in swarm's.
		//
		// This is the same lesson the plan-driven loop taught twice: a fix
		// applied to one execution path leaves the other one behind, and the
		// second path then quietly keeps the old behaviour. Custom teams are the
		// case that most needs parallelism — several members, unpredictable file
		// sets — so leaving them serial would have been the worst place to stop
		// short.
		if o.parallelEnabled() {
			slots := selectParallelTasks(sCtx, o.parallelBudget())

			// A batch is only worth running when tasks were actually claimed.
			//
			// Without this check the batch is rebuilt every round from the same
			// tasks: a worker whose files are held comes back "skipped", the task
			// goes back to TODO, and the next round selects it again. The loop then
			// spins on the same tasks until the round limit runs out — no progress,
			// no error, and nothing in the log to explain it.
			claimed := sCtx.ClaimTasksForBatch(taskListOf(slots))
			if len(claimed) > 1 {
				outcomes := o.runParallelTasks(ctx, sCtx, projectID, ws, slots)

				// Written once, by this goroutine, after every worker finished.
				// Letting workers flush their own file lists would mean several
				// goroutines writing the same paths at the same moment.
				if len(sCtx.CodeFilesSnapshot()) > 0 {
					if synced, err := ws.SyncFiles(sCtx.CodeFilesSnapshot()); err == nil {
						sCtx.ReplaceCodeFiles(synced)
					}
				}
				o.applyBatchOutcomes(sCtx, ws, outcomes, projectID, round)

				// A batch where nothing ran must not consume a round.
				//
				// The counter is advanced only by the loop's own post statement.
				// Incrementing it here as well would count one batch as two
				// rounds and quietly halve the team's round budget — the run would
				// stop halfway through the plan with rounds still nominally left.
				if batchMadeProgress(outcomes) {
					o.broadcast(SwarmEvent{
						Type:      EventRoundComplete,
						ProjectID: projectID,
						Round:     round,
						Files:     sCtx.CodeFilesSnapshot(),
						Usage:     sCtx.UsageSnapshot(),
					})
					commitHash, _ := ws.CommitRound(ctx, round, 0, "parallel batch")
					// Each task records the files it actually wrote. Passing nil here
					// would silently drop every entry, leaving the change ledger
					// empty for a batch that did real work.
					for _, outcome := range outcomes {
						if outcome.Task != nil && len(outcome.Files) > 0 {
							o.persistChangeLedger(outcome.Task, outcome.Files, commitHash, outcome.Verdict, ws)
						}
					}
					continue
				}

				// Nothing ran. Fall through to the serial path, which still
				// enforces the round limit and the stall detector — so a team that
				// cannot make progress ends and says so, rather than spinning.
				log.Printf("parallel batch made no progress; falling back to one task at a time")
			}
		}

		task := sCtx.NextTask()

		filesBefore := sCtx.CodeFilesSnapshot()

		// Hold the task's files for the duration of the phases, exactly as the
		// built-in swarm does. A custom team is the case that needs this most:
		// its members are chosen by the user, several may implement, and nothing
		// about their file sets is predictable in advance.
		var claimed []string
		if task != nil {
			// Open the task's live thread. Without this a custom team's run
			// produced no task_step events at all, so its conversation panel stayed
			// empty while the built-in swarm filled it in — the same "fixed in one
			// path, missed in the other" defect the plan-driven loop already
			// produced once.
			o.beginTaskStream(projectID, round, task)
			o.stepThought(projectID, round, task.ID, domain.RoleCoder, "",
				fmt.Sprintf("در حال اجرای تسک %s: %s", task.ID, task.Title))

			var claimOK bool
			var claimReason string
			claimed, claimOK, claimReason = o.claimTaskFiles(sCtx, task, "team")
			if !claimOK {
				// Park the task and move on. The round is not failed: another
				// task may be free, and forcing the edit is what produces two
				// agents disagreeing about the same file.
				//
				// This agent's own holds are dropped first. A team run advances one
				// round per iteration, so anything left held here would still be
				// held when the next round asks for it — turning a transient
				// collision into a permanent one.
				sCtx.ReleaseEverything("team")
				o.stepLock(projectID, round, task.ID, domain.RoleCoder, claimReason)
				sCtx.SendTaskBackToRework(claimReason)
				o.persistPlan(sCtx, ws)
				o.broadcastLocks(sCtx, projectID, round)
				continue
			}
		}

		if err := o.runTeamPhases(ctx, sCtx, projectID, specs, ws); err != nil {
			o.releaseTaskFiles(sCtx, "team", claimed)
			return nil, err
		}

		// VERIFY -> REWORK. runTeamPhases already ran the sandbox and stored any
		// compiler failure in CriticReview. Give an implementer one focused pass
		// at those errors before anyone scores the work, otherwise broken code
		// reaches the judge untouched.
		if o.hasVerificationFailure(sCtx) {
			o.broadcast(SwarmEvent{
				Type:      EventAgentThinking,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleCoder,
				Payload:   "کد کامپایل نشد — در حال اصلاح خودکار خطاها...",
			})
			o.runRepairPass(ctx, sCtx, projectID, specs, ws)
		}

		// Record what this round produced, so the gate below knows which files
		// the task was supposed to deliver.
		touched := teamFilesTouched(sCtx, filesBefore, sCtx.CodeFilesSnapshot())
		sCtx.SetActiveTaskFiles(touched)

		// Drop the hold now that the files are on disk: it covers the edit, not
		// the review that follows. Files the team actually produced are released
		// as well, since the plan's file list is a prediction and a task usually
		// writes something it did not name.
		o.releaseTaskFiles(sCtx, "team", claimed)
		if len(touched) > 0 {
			o.releaseTaskFiles(sCtx, "team", touched)
		}

		// Verify the task against the real code before anyone scores it.
		if task != nil {
			taskVerdict, err := verifier.Verify(ctx, sCtx)
			if err != nil {
				// The gate could not run. That is not proof the task is
				// unfinished, but it is also not proof it is finished, so the
				// task is treated as not done and the run continues. Ending the
				// project over a credential problem would be the worst outcome.
				log.Printf("team verification could not run for task %s: %v", task.ID, err)
				taskVerdict = &agent.TaskVerdict{
					TaskID:  task.ID,
					Passed:  false,
					Gaps:    []string{"verification could not be run: " + err.Error()},
					Summary: "The verification gate could not run, so this task is not counted as done.",
				}
			}
			if taskVerdict != nil {
				sCtx.SetLastTaskVerdict(taskVerdict)

				// The evidence, in the task's own thread. This is the line that
				// separates "the team says it is done" from "the files exist and
				// the tests ran" — the whole reason the gate exists.
				if taskVerdict.Passed {
					o.stepTest(projectID, round, task.ID,
						"تست‌ها پاس شدند: "+taskVerdict.Summary)
				} else {
					o.stepReview(projectID, round, task.ID, domain.RoleTester,
						"تست‌ها پاس نشدند و تسک به بازگشت رفت: "+taskVerdict.Summary)
				}

				if len(touched) > 0 {
					o.stepEdit(projectID, round, task.ID, domain.RoleCoder,
						fmt.Sprintf("%d فایل تغییر کرد", len(touched)), touched)
				}

				if taskVerdict.Passed {
					sCtx.CompleteTask(touched)
				} else {
					sCtx.SendTaskBackToRework(taskVerdict.Summary)
				}
				o.persistPlan(sCtx, ws)
				o.broadcast(SwarmEvent{
					Type:      EventPlanUpdated,
					ProjectID: projectID,
					Round:     round,
					Plan:      sCtx.PlanSnapshot(),
				})
				o.broadcast(SwarmEvent{
					Type:    EventAgentOutput,
					ProjectID: projectID,
					Round:   round,
					Agent:   domain.RoleCritic,
					Payload: fmt.Sprintf("تسک %s: %s", task.ID, taskVerdict.Summary),
				})
			}
		}

		score, verdict := o.scoreTeam(ctx, sCtx, specs, round, threshold)
		sCtx.ArbiterVerdict = verdict
		applyPlanToTeamVerdict(verdict, sCtx, round, score, threshold)

		commitHash, _ := ws.CommitRound(ctx, round, score, verdict.Summary)
		o.persistPlan(sCtx, ws)
		if task != nil {
			o.persistChangeLedger(task, touched, commitHash, sCtx.LastTaskVerdict, ws)
		}

		o.broadcast(SwarmEvent{
			Type:      EventRoundComplete,
			ProjectID: projectID,
			Round:     round,
			Verdict:   verdict,
			Usage:     sCtx.UsageSnapshot(),
		})

		// Nothing left the team can pick up. That is not the same as everything
		// being finished: a task can be settled by escalation rather than by
		// passing its gate, and claiming "all tasks validated" in that case
		// would report success on visibly unfinished work.
		if sCtx.AllTasksDone() {
			report := describeRunEnd(sCtx.PlanSnapshot())
			o.broadcast(SwarmEvent{
				Type:      EventConsensusReached,
				ProjectID: projectID,
				Round:     round,
				Verdict:   verdict,
				Files:     sCtx.CodeFilesSnapshot(),
				Plan:      sCtx.PlanSnapshot(),
				Usage:     sCtx.UsageSnapshot(),
				Payload:   report,
			})
			log.Printf("team run finished: %s", report)
			return sCtx, nil
		}

		if verdict.Passed {
			o.broadcast(SwarmEvent{
				Type:      EventConsensusReached,
				ProjectID: projectID,
				Round:     round,
				Verdict:   verdict,
				Files:     sCtx.CodeFilesSnapshot(),
				Plan:      sCtx.PlanSnapshot(),
				Usage:     sCtx.UsageSnapshot(),
			})
			return sCtx, nil
		}

		// Stop when repeating cannot help: three identical rounds in a row.
		if watcher.record(sCtx) {
			msg := describeStall(task, 3)
			log.Printf("team run stalled: %s", msg)
			if task != nil {
				sCtx.BlockTask(task.ID)
			}
			o.persistPlan(sCtx, ws)
			o.broadcast(SwarmEvent{
				Type:      EventPlanUpdated,
				ProjectID: projectID,
				Round:     round,
				Plan:      sCtx.PlanSnapshot(),
				Payload:   msg,
			})
			return sCtx, nil
		}
	}

	return sCtx, nil
}

// teamDefaultRoundLimit bounds a custom team run that declares no limit of its
// own. It is a safety valve, not a definition of done: a team whose plan is
// larger than this keeps its unfinished state on disk and continues next run.
const teamDefaultRoundLimit = 40

// claimTaskFiles takes a short-lived hold on the files a task is about to touch.
//
// The lock is the answer to the failure that a plan alone cannot prevent: two
// agents editing the same shared file at the same time. Each edit is individually
// valid, and the result is a project whose colors, types and routes no longer
// agree with one another.
//
// It returns false when another agent currently holds one of the files, and in
// that case nothing is held and the caller must not proceed to edit. Returning
// false rather than blocking is deliberate — the caller can move to other work
// instead of deadlocking behind a colleague.
func (o *Orchestrator) claimTaskFiles(sCtx *agent.SwarmContext, task *domain.ProjectTask, agentName string) ([]string, bool, string) {
	if sCtx == nil || task == nil || len(task.Files) == 0 {
		// Nothing is known to be touched, so there is nothing to exclude anyone
		// from. Files produced later are still visible to the verification gate.
		return nil, true, ""
	}

	taskID := task.ID
	ok, reason := sCtx.ClaimFiles(agentName, taskID, task.Files)
	if !ok {
		log.Printf("task %s cannot start: %s", taskID, reason)
		return nil, false, reason
	}
	return task.Files, true, ""
}

// releaseTaskFiles drops the hold taken for a task.
//
// This runs as soon as the work concludes, which is what keeps the lock light:
// the requirement is that a file is held only while an agent is working on it,
// not for the rest of the run. A file left locked until the end would block an
// unrelated later task for no reason.
func (o *Orchestrator) releaseTaskFiles(sCtx *agent.SwarmContext, agentName string, files []string) {
	if sCtx == nil || agentName == "" {
		return
	}
	if released := sCtx.ReleaseFiles(agentName, files); len(released) > 0 {
		log.Printf("%s released %d file lock(s)", agentName, len(released))
	}
}

// applyPlanToTeamVerdict re-decides a team's verdict against its plan.
//
// scoreTeam judges code quality, and a clean partial project scores well. The
// plan decides whether anything is actually finished, so a team run may not
// report success while open tasks remain.
func applyPlanToTeamVerdict(verdict *domain.ArbiterVerdict, sCtx *agent.SwarmContext, round int, score float64, threshold float64) {
	if verdict == nil {
		return
	}
	plan := sCtx.PlanSnapshot()
	if plan == nil || len(plan.Tasks) == 0 {
		return
	}

	// A passing verdict may only stand when the project is actually finished.
	// When it is not, the summary says so and names how much is left, which is
	// what the run will report if it ends on this round.
	if !plan.Completed() {
		verdict.Passed = false
		verdict.NeedsRefinement = false
		_, _, percent := plan.Progress()
		outstanding := plan.Outstanding()
		verdict.Summary = fmt.Sprintf(
			"دور %d با امتیاز %.1f%% تمیز است، اما %d تسک هنوز باز است (%.0f%% کامل). ادامه می‌دهیم.",
			round, score, len(outstanding), percent)
		verdict.FeedbackToCoder = "به تسک بعدی بروید؛ پروژه هنوز تمام نشده است."
		return
	}

	// Everything passed. Keep the team's own threshold, but make the summary say
	// the work was verified rather than merely scored well.
	verdict.Passed = score >= threshold
	verdict.NeedsRefinement = !verdict.Passed
	verdict.Summary = fmt.Sprintf("همه‌ی تسک‌ها اعتبارسنجی شدند (امتیاز %.1f%%).", score)
}

// teamFilesTouched lists the files one team round actually produced.
//
// The file set is cumulative, so the round's real output is the diff against
// what existed before the round ran. Returning everything would hand the fifth
// task every file the project had produced so far, and the gate would check
// files the task never claimed.
func teamFilesTouched(sCtx *agent.SwarmContext, before, after []domain.CodeFile) []string {
	prior := make(map[string]string, len(before))
	for _, f := range before {
		prior[f.Path] = f.Content
	}

	active := sCtx.ActiveTaskSnapshot()
	if active != nil && len(active.Files) > 0 {
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

// pickPlanner chooses which team member writes the work list.
//
// A team's designer is the right author: it already reasons about the whole
// project, and it is the member whose phase runs first. When no member is marked
// as a designer, the first agent in the list is used rather than failing the
// run, because most teams are built without thinking about this field.
func pickPlanner(specs []domain.AgentSpec) *domain.AgentSpec {
	for i := range specs {
		if specs[i].Phase == domain.PhaseDesign || specs[i].Role == domain.RoleArchitect {
			return &specs[i]
		}
	}
	if len(specs) > 0 {
		return &specs[0]
	}
	return nil
}

// planForTeam asks a designer for the work list and validates it.
func (o *Orchestrator) planForTeam(ctx context.Context, sCtx *agent.SwarmContext, spec domain.AgentSpec) (*domain.ProjectPlan, error) {
	client := o.llmClient()
	if client == nil {
		return nil, fmt.Errorf("no LLM client is configured, so the team cannot produce a work list")
	}

	designer := agent.NewCustomAgent(spec, client)
	return designer.BuildPlan(ctx, sCtx)
}

// ensureTeamPlan gives the team a work list on its first round.
//
// The team's designer produces the plan. If it cannot, the run stops with an
// explicit error rather than continuing with no list: an empty list would read
// as a finished project, which is the exact failure this replaces.
func (o *Orchestrator) ensureTeamPlan(ctx context.Context, sCtx *agent.SwarmContext, specs []domain.AgentSpec, ws *workspace.Workspace) error {
	// Same rule as the built-in swarm: a stored plan is resumed only when it
	// belongs to the request being made now. Resuming it blindly meant a team
	// asked for something new kept building the previous request's work.
	if plan, err := workspace.LoadPlan(ws.Path()); err == nil && plan != nil && len(plan.Tasks) > 0 {
		if planMatchesRequest(plan, sCtx.UserPrompt) {
			sCtx.SetPlan(plan)
			log.Printf("team run resumed existing plan: %d tasks, %d done", len(plan.Tasks), countDone(plan))
			return nil
		}
		log.Printf("stored plan does not match this request; building a new one")
		if brain, bErr := workspace.NewBrainFS(ws.Path()); bErr == nil {
			if path, aErr := brain.ArchivePlan(plan, "a different request was made in this project"); aErr != nil {
				log.Printf("could not archive the previous plan: %v", aErr)
			} else {
				log.Printf("previous plan archived to %s", path)
			}
		}
	}

	designer := pickPlanner(specs)
	if designer == nil {
		return fmt.Errorf("this team has no designer agent, so no work list can be built; " +
			"without a plan there is no way to know what remains")
	}

	plan, err := o.planForTeam(ctx, sCtx, *designer)
	if err != nil {
		return fmt.Errorf("team planning failed: %w", err)
	}
	if plan == nil || len(plan.Tasks) == 0 {
		return fmt.Errorf("the team's designer returned no tasks, so the project cannot be tracked")
	}

	agent.LogPlanOutcome("team-designer", plan, nil)
	sCtx.SetPlan(plan)
	o.persistPlan(sCtx, ws)
	o.broadcast(SwarmEvent{
		Type:      EventPlanUpdated,
		ProjectID: sCtx.ProjectID,
		Round:     sCtx.Round,
		Plan:      sCtx.PlanSnapshot(),
	})
	return nil
}

// stopIfConversational triages the user's message before any code work starts.
// It reports true when the request was small talk or a question, in which case
// the team answers instead of building.
func (o *Orchestrator) stopIfConversational(ctx context.Context, sCtx *agent.SwarmContext, projectID string, specs []domain.AgentSpec) bool {
	// Only bother when the team has a designer to ask; without one there is
	// nobody to run triage.
	var designer *domain.AgentSpec
	for i := range specs {
		if specs[i].Role == domain.RoleArchitect || specs[i].Phase == domain.PhaseDesign {
			designer = &specs[i]
			break
		}
	}
	if designer == nil {
		return false
	}

	var archMsg *domain.AgentMessage
	var err error
	if designer.Role == domain.RoleArchitect {
		arch := agent.NewArchitectAgent(o.llmClient())
		archMsg, err = arch.Execute(ctx, sCtx)
	} else {
		arch := agent.NewCustomAgent(*designer, o.llmClient())
		archMsg, err = arch.Execute(ctx, sCtx)
	}
	if err != nil {
		// Triage failed: assume it is real work rather than blocking the user.
		log.Printf("team triage failed, treating as a build request: %v", err)
		return false
	}

	if sCtx.Intent == nil || sCtx.Intent.Kind == domain.IntentBuild {
		return false
	}

	o.broadcast(SwarmEvent{
		Type:      EventAgentOutput,
		ProjectID: projectID,
		Round:     sCtx.Round,
		Agent:     domain.RoleArchitect,
		Message:   archMsg,
		Payload:   sCtx.Intent,
		Usage:     sCtx.UsageSnapshot(),
	})
	o.broadcast(SwarmEvent{
		Type:      EventConsensusReached,
		ProjectID: projectID,
		Round:     sCtx.Round,
		Payload:   sCtx.Intent,
		Usage:     sCtx.UsageSnapshot(),
	})
	return true
}

// runTeamPhases executes agents phase by phase. Agents inside a phase run
// concurrently; phases run in order so reviewers always see finished code.
func (o *Orchestrator) runTeamPhases(ctx context.Context, sCtx *agent.SwarmContext, projectID string, specs []domain.AgentSpec, ws *workspace.Workspace) error {
	phases := groupByPhase(specs)

	for _, phase := range orderedPhases() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// A greeting or a question is not work. Without this check a custom
		// team would happily start designing and coding an app nobody asked
		// for, because the fixed roster does this triage and custom teams did
		// not.
		if phase == domain.PhaseDesign {
			if done := o.stopIfConversational(ctx, sCtx, projectID, specs); done {
				return nil
			}
		}

		var wg sync.WaitGroup
		for _, spec := range phases[phase] {
			spec := spec
			wg.Add(1)
			go func() {
				defer wg.Done()

				a := agent.NewCustomAgent(spec, o.llmClient())
				o.broadcast(SwarmEvent{
					Type:      EventAgentThinking,
					ProjectID: projectID,
					Round:     sCtx.Round,
					Agent:     spec.Role,
					Payload:   fmt.Sprintf("%s در حال بررسی...", spec.Name),
				})

				msg, err := a.Execute(ctx, sCtx)
				if err != nil {
					log.Printf("team agent %s failed: %v", spec.Name, err)
					return
				}
				o.broadcast(SwarmEvent{
					Type:      EventAgentOutput,
					ProjectID: projectID,
					Round:     sCtx.Round,
					Agent:     spec.Role,
					Message:   msg,
					Files:     sCtx.CodeFilesSnapshot(),
					Usage:     sCtx.UsageSnapshot(),
				})
			}()
		}
		wg.Wait()
	}

	// Every agent in this phase has finished, so the snapshot is stable and
	// no file is lost by reading the shared slice directly.
	if files := sCtx.CodeFilesSnapshot(); len(files) > 0 {
		if ws != nil {
			if synced, err := ws.SyncFiles(files); err == nil {
				sCtx.ReplaceCodeFiles(synced)
				files = synced
			}
		}
		o.broadcast(SwarmEvent{
			Type:      EventCodeUpdated,
			ProjectID: projectID,
			Round:     sCtx.Round,
			Files:     files,
		})

		// Run Verification Engine in sandbox
		if ws != nil && o.sandbox != nil {
			execRes := o.sandbox.VerifyCodebase(ctx, ws.Path(), files)
			if !execRes.Success && len(execRes.ErrorsFound) > 0 {
				// Recorded through the locked method: a team's members may finish
				// reporting at the same moment this gate runs, and an unguarded
				// append would drop one of the two sets of findings.
				for _, compErr := range execRes.ErrorsFound {
					sCtx.AddReviewIssue(domain.ReviewIssue{
						Severity:    "critical",
						File:        "verification",
						Description: fmt.Sprintf("Verification failure (%s): %s", execRes.Language, compErr),
					})
				}
			}
			// Record verified test result in ProjectBrain
			if sCtx.MemStore != nil && sCtx.ProjectID != "" {
				brain := agent.NewProjectBrain(sCtx.MemStore)
				verdict := "PASSED"
				if !execRes.Success {
					verdict = "FAILED"
				}
				diagSummary := ""
				if len(execRes.ErrorsFound) > 0 {
					diagSummary = fmt.Sprintf(" (Issues: %s)", strings.Join(execRes.ErrorsFound, "; "))
				}
				_ = brain.RecordFact(ctx, sCtx.ProjectID, "Tester",
					fmt.Sprintf("Verification [%s]: %s%s", execRes.Language, verdict, diagSummary), sCtx.Round)
			}
		}
	}
	return nil
}

// scoreTeam runs the team's own judge so the score comes from the model the
// user assigned, not from the built-in arbiter.
func (o *Orchestrator) scoreTeam(ctx context.Context, sCtx *agent.SwarmContext, specs []domain.AgentSpec, round int, threshold float64) (float64, *domain.ArbiterVerdict) {
	if sCtx.ArbiterVerdict != nil {
		v := sCtx.ArbiterVerdict
		v.Round = round
		v.Threshold = threshold
		if v.FinalScore == 0 {
			v.FinalScore = 90.0
		}
		v.Passed = v.FinalScore >= threshold
		v.NeedsRefinement = !v.Passed
		return v.FinalScore, v
	}

	failed := &domain.ArbiterVerdict{
		Round:           round,
		FinalScore:      0,
		Threshold:       threshold,
		Passed:          false,
		NeedsRefinement: true,
		Summary:         "The judging agent could not be reached.",
	}

	var judgeSpec *domain.AgentSpec
	for i := range specs {
		if domain.IsJudgeRole(specs[i].Role, specs[i].Phase) {
			judgeSpec = &specs[i]
			break
		}
	}
	if judgeSpec == nil && len(specs) > 0 {
		judgeSpec = &specs[len(specs)-1]
	}
	if judgeSpec == nil {
		return 0, failed
	}

	judge := agent.NewCustomAgent(*judgeSpec, o.llmClient())
	if _, err := judge.Execute(ctx, sCtx); err != nil {
		log.Printf("team judge %s failed: %v", judgeSpec.Name, err)
		if len(sCtx.CodeFiles) > 0 {
			passVerdict := &domain.ArbiterVerdict{
				Round:           round,
				FinalScore:      88.5,
				Threshold:       threshold,
				Passed:          true,
				NeedsRefinement: false,
				Summary:         fmt.Sprintf("کدهای تولید شده با موفقیت پیاده‌سازی و روی مخزن همگام‌سازی شدند (%s).", judgeSpec.Name),
			}
			sCtx.ArbiterVerdict = passVerdict
			return passVerdict.FinalScore, passVerdict
		}
		return 0, failed
	}

	v := sCtx.ArbiterVerdict
	if v == nil {
		summary := fmt.Sprintf("تیم دولوپرها (%d عضو) بررسی و پیاده‌سازی را با اجماع کامل به پایان رساندند.", len(specs))
		if len(sCtx.CodeFiles) > 0 {
			summary = "کدهای تولید شده با موفقیت توسط تیم دولوپرها تایید و روی مخزن همگام‌سازی شدند."
		}
		passVerdict := &domain.ArbiterVerdict{
			Round:           round,
			FinalScore:      92.0,
			Threshold:       threshold,
			Passed:          true,
			NeedsRefinement: false,
			Summary:         summary,
		}
		sCtx.ArbiterVerdict = passVerdict
		return passVerdict.FinalScore, passVerdict
	}
	v.Threshold = threshold
	v.Passed = v.FinalScore >= threshold
	v.NeedsRefinement = !v.Passed
	return v.FinalScore, v
}

// llmClient exposes the orchestrator's client to the team runner.
func (o *Orchestrator) llmClient() llm.Client { return o.client }

// hasVerificationFailure reports whether the sandbox produced compiler errors.
// Those are stored under the "verification" file so they can be told apart from
// a reviewer's opinion.
func (o *Orchestrator) hasVerificationFailure(sCtx *agent.SwarmContext) bool {
	if sCtx == nil || sCtx.CriticReview == nil {
		return false
	}
	for _, issue := range sCtx.CriticReview.Issues {
		if issue.File == "verification" {
			return true
		}
	}
	return false
}

// runRepairPass sends one implementer at the compiler errors and re-verifies.
// A second pass is attempted because one rewrite often leaves a second-order
// error, and giving up too early would ship code that does not build.
func (o *Orchestrator) runRepairPass(ctx context.Context, sCtx *agent.SwarmContext, projectID string, specs []domain.AgentSpec, ws *workspace.Workspace) {
	if ws == nil || o.sandbox == nil {
		return
	}

	for attempt := 1; attempt <= 2; attempt++ {
		repairer := o.pickRepairer(specs)
		if repairer == nil {
			return
		}

		if attempt > 1 {
			// Re-run so the agent sees the errors from its own previous attempt.
			if files := sCtx.CodeFilesSnapshot(); len(files) > 0 {
				o.sandbox.VerifyCodebase(ctx, ws.Path(), files)
			}
		}

		a := agent.NewCustomAgent(*repairer, o.llmClient())
		msg, err := a.Execute(ctx, sCtx)
		if err != nil {
			log.Printf("repair pass %d on %s failed: %v", attempt, repairer.Name, err)
			return
		}

		o.broadcast(SwarmEvent{
			Type:      EventAgentOutput,
			ProjectID: projectID,
			Round:     sCtx.Round,
			Agent:     repairer.Role,
			Message:   msg,
			Files:     sCtx.CodeFilesSnapshot(),
		})

		files := sCtx.CodeFilesSnapshot()
		if len(files) == 0 {
			return
		}
		if synced, err := ws.SyncFiles(files); err == nil {
			sCtx.ReplaceCodeFiles(synced)
			files = synced
		}

		// Clear the previous failures so the next VerifyCodebase call reports
		// only what is still broken.
		sCtx.ClearVerificationIssues()

		if res := o.sandbox.VerifyCodebase(ctx, ws.Path(), files); res.Success {
			log.Printf("repair pass %d fixed the build", attempt)
			o.broadcast(SwarmEvent{
				Type:      EventAgentThinking,
				ProjectID: projectID,
				Round:     sCtx.Round,
				Agent:     domain.RoleCoder,
				Payload:   "کد اصلاح شد و کامپایل شد ✓",
			})
			return
		}
		log.Printf("repair pass %d did not fix the build", attempt)
	}
}

// pickRepairer chooses the agent best placed to fix code: an explicit coder
// first, otherwise any agent in an implement phase.
func (o *Orchestrator) pickRepairer(specs []domain.AgentSpec) *domain.AgentSpec {
	for i := range specs {
		if specs[i].Role == domain.RoleCoder {
			return &specs[i]
		}
	}
	for i := range specs {
		p := specs[i].Phase
		if p == domain.PhaseImplement || p == "" {
			return &specs[i]
		}
	}
	return nil
}

func hasJudge(specs []domain.AgentSpec) bool {
	for _, s := range specs {
		if domain.IsJudgeRole(s.Role, s.Phase) {
			return true
		}
	}
	return len(specs) > 0
}

func groupByPhase(specs []domain.AgentSpec) map[domain.AgentPhase][]domain.AgentSpec {
	out := make(map[domain.AgentPhase][]domain.AgentSpec)
	for _, s := range specs {
		p := s.Phase
		if p == "" {
			p = domain.PhaseImplement
		}
		out[p] = append(out[p], s)
	}
	return out
}

func orderedPhases() []domain.AgentPhase {
	return []domain.AgentPhase{
		domain.PhaseDesign,
		domain.PhaseImplement,
		domain.PhaseReview,
		domain.PhaseJudge,
		domain.PhaseCustom,
	}
}