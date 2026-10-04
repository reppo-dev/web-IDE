package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// These tests cover the behaviour that keeps one hard task from holding the
// whole project hostage: the team carries on with the rest of the plan, and the
// run ends by naming what is left instead of spinning or claiming success.

// TestWaitingTaskDoesNotBlockTheRestOfThePlan proves the swarm keeps working
// after a task is escalated. A single task the agents cannot close must not stop
// the other tasks from being finished.
func TestWaitingTaskDoesNotBlockTheRestOfThePlan(t *testing.T) {
	script := &scriptedLLM{tasks: testPlan(
		domain.ProjectTask{Title: "Impossible task"},
		domain.ProjectTask{Title: "Doable task A"},
		domain.ProjectTask{Title: "Doable task B"},
	)}

	orch, _ := newScriptedOrchestrator(t, script)

	sCtx, err := orch.RunWithOptions(context.Background(), "escalate", "Build it", domain.ModeRoleBased, testModels(), testAPIKeys())
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	plan := sCtx.PlanSnapshot()
	if plan == nil {
		t.Fatal("no plan")
	}

	// The two doable tasks must be marked done. If the impossible one had
	// stopped the run, they would still be TODO.
	doneTitles := map[string]bool{}
	for _, task := range plan.Tasks {
		if task.State == domain.TaskDone {
			doneTitles[task.Title] = true
		}
	}
	for _, want := range []string{"Doable task A", "Doable task B"} {
		if !doneTitles[want] {
			t.Errorf("task %q was never finished; states were %v", want, taskStates(plan))
		}
	}
}

// TestEscalatedTaskStopsTheRun proves the loop terminates once nothing is left
// it can pick up. Without a settled state for escalated tasks the loop would
// keep asking for the next task forever.
func TestEscalatedTaskStopsTheRun(t *testing.T) {
	plan := &domain.ProjectPlan{
		Goal: "g",
		Tasks: []domain.ProjectTask{
			{ID: "T-001", Title: "stuck", State: domain.TaskWaiting},
			{ID: "T-002", Title: "fine", State: domain.TaskDone},
		},
	}

	// A settled plan stops the loop: NextTask hands out nothing, so the run
	// ends instead of asking for work that does not exist.
	sCtx := &agent.SwarmContext{Round: 1}
	sCtx.SetPlan(plan)
	if next := sCtx.NextTask(); next != nil {
		t.Errorf("expected no selectable task once everything is settled, got %+v", next)
	}
	if !sCtx.AllTasksDone() {
		t.Error("expected a settled plan to end the run")
	}
	if sCtx.PlanSnapshot().Completed() {
		t.Error("a plan with a waiting task is finished, not complete")
	}
}

// TestRunEndReportNamesOutstandingWork proves the user is told what is left.
// Reporting "not done" without saying which tasks would leave the next step to
// guesswork.
func TestRunEndReportNamesOutstandingWork(t *testing.T) {
	plan := &domain.ProjectPlan{
		Goal: "g",
		Tasks: []domain.ProjectTask{
			{ID: "T-001", Title: "Build the parser", State: domain.TaskDone},
			{ID: "T-002", Title: "Wire the CLI", State: domain.TaskWaiting,
				Acceptance: "the binary responds to --help", LastError: "circular import"},
			{ID: "T-003", Title: "Write the README", State: domain.TaskTodo},
		},
	}

	report := describeRunEnd(plan)

	for _, want := range []string{"T-002", "Wire the CLI", "T-003", "Write the README"} {
		if !strings.Contains(report, want) {
			t.Errorf("report does not mention %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "Build the parser") {
		t.Errorf("report should not list a finished task as outstanding:\n%s", report)
	}
	if !strings.Contains(report, "circular import") {
		t.Errorf("report should carry the reason the task stopped:\n%s", report)
	}
	if !strings.Contains(report, "done when") {
		t.Errorf("report should state what the unfinished task required:\n%s", report)
	}
}

// TestFullyCompletePlanReportsCompletion is the other half: a run where
// everything passed must say so plainly, without hedging.
func TestFullyCompletePlanReportsCompletion(t *testing.T) {
	plan := &domain.ProjectPlan{
		Goal: "g",
		Tasks: []domain.ProjectTask{
			{ID: "T-001", Title: "a", State: domain.TaskDone},
			{ID: "T-002", Title: "b", State: domain.TaskDone},
		},
	}
	report := describeRunEnd(plan)
	if !strings.Contains(report, "complete") {
		t.Errorf("expected a clear completion statement, got:\n%s", report)
	}
	if strings.Contains(report, "T-001") || strings.Contains(report, "still open") {
		t.Errorf("a finished project should not list open work:\n%s", report)
	}
}

// TestEscalatedTasksRemainOnTheBoard proves unfinished work survives the run, so
// the next session continues rather than restarting.
func TestEscalatedTasksRemainOnTheBoard(t *testing.T) {
	script := &scriptedLLM{
		tasks: testPlan(
			domain.ProjectTask{Title: "Kept task"},
		),
		verifierGapsBefore: 0,
	}
	orch, tempDir := newScriptedOrchestrator(t, script)

	if _, err := orch.RunWithOptions(context.Background(), "board", "Build it", domain.ModeRoleBased, testModels(), testAPIKeys()); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	ws, err := workspace.NewWorkspace(tempDir, "board")
	if err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	plan, err := workspace.LoadPlan(ws.Path())
	if err != nil || plan == nil {
		t.Fatalf("plan not persisted: %v", err)
	}
	if len(plan.Tasks) == 0 {
		t.Fatal("plan lost its tasks")
	}
	if plan.Tasks[0].State == "" {
		t.Error("persisted task has no state, so a resumed run cannot tell what is left")
	}
}

// TestSettledStatesDoNotReportComplete guards the distinction the whole report
// depends on: DONE means verified, the others mean handed back.
func TestSettledStatesDoNotReportComplete(t *testing.T) {
	cases := []domain.TaskState{
		domain.TaskTodo,
		domain.TaskInProgress,
		domain.TaskRework,
	}
	for _, state := range cases {
		if state.Settled() {
			t.Errorf("%s should not count as settled", state)
		}
	}
	for _, state := range []domain.TaskState{
		domain.TaskDone, domain.TaskWaiting, domain.TaskBlocked,
		domain.TaskCancelled, domain.TaskFailed,
	} {
		if !state.Settled() {
			t.Errorf("%s should count as settled", state)
		}
	}

	plan := &domain.ProjectPlan{
		Tasks: []domain.ProjectTask{{ID: "T-001", State: domain.TaskCancelled}},
	}
	if !plan.AllDone() {
		t.Error("a plan of cancelled tasks should end the run")
	}
	if plan.Completed() {
		t.Error("a cancelled task is not completed work")
	}
	// Cancelled work is still work the user did not get, so it stays visible in
	// the outstanding list. Dropping it there would let the end-of-run report
	// claim nothing was left while its own tally showed an unfinished task.
	if got := plan.Outstanding(); len(got) != 1 {
		t.Errorf("a cancelled task should remain listed as outstanding, got %d entries", len(got))
	}
}

// TestRunEndCountsAlwaysAddUp guards the report against contradicting itself.
// Each task must land in exactly one bucket, or the headline number disagrees
// with the detail beneath it.
func TestRunEndCountsAlwaysAddUp(t *testing.T) {
	plan := &domain.ProjectPlan{
		Goal: "g",
		Tasks: []domain.ProjectTask{
			{ID: "T-001", Title: "done", State: domain.TaskDone},
			{ID: "T-002", Title: "blocked", State: domain.TaskBlocked},
			{ID: "T-003", Title: "failed", State: domain.TaskFailed},
			{ID: "T-004", Title: "cancelled", State: domain.TaskCancelled},
		},
	}

	report := describeRunEnd(plan)
	if !strings.Contains(report, "3 of 4 tasks are still open") {
		t.Errorf("headline count is wrong for a plan of done/blocked/failed/cancelled:\n%s", report)
	}
	for _, want := range []string{"T-002", "T-003", "T-004"} {
		if !strings.Contains(report, want) {
			t.Errorf("report should list %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "T-001 ") {
		t.Errorf("a done task should not be listed as open:\n%s", report)
	}
}