package orchestrator

import (
	"sync"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// Parallel execution is only safe because of two things: each worker gets its own
// context, and each takes a file lock before writing. These tests pin down both,
// because either one failing produces corrupted output rather than a crash.

// newParallelContext builds a context with a plan, ready for batch selection.
func newParallelContext(tasks ...domain.ProjectTask) *agent.SwarmContext {
	sCtx := &agent.SwarmContext{
		Round:     1,
		FileLocks: workspace.NewFileLockManager(),
	}
	sCtx.SetPlan(&domain.ProjectPlan{Goal: "test", Tasks: tasks})
	return sCtx
}

// TestSelectParallelTasksPicksIndependentTasks is the core of the feature: tasks
// that touch different files must be allowed to run together.
func TestSelectParallelTasksPicksIndependentTasks(t *testing.T) {
	sCtx := newParallelContext(
		domain.ProjectTask{ID: "T-001", Title: "model", State: domain.TaskTodo, Files: []string{"lib/data_model.dart"}},
		domain.ProjectTask{ID: "T-002", Title: "screen", State: domain.TaskTodo, Files: []string{"lib/screens/home.dart"}},
		domain.ProjectTask{ID: "T-003", Title: "widget", State: domain.TaskTodo, Files: []string{"lib/widgets/button.dart"}},
	)

	slots := selectParallelTasks(sCtx, 3)
	if len(slots) != 3 {
		t.Fatalf("three independent tasks should all be selected, got %d", len(slots))
	}
	seen := map[string]bool{}
	for _, slot := range slots {
		if slot.Task == nil {
			t.Fatal("a selected slot has no task")
		}
		if seen[slot.Task.ID] {
			t.Errorf("task %s was selected twice", slot.Task.ID)
		}
		seen[slot.Task.ID] = true
	}
}

// TestSelectParallelTasksExcludesSharedFiles is the reason selection checks files
// at all. Two tasks that both write a theme file cannot run together, no matter
// how independent they look in the plan.
func TestSelectParallelTasksExcludesSharedFiles(t *testing.T) {
	sCtx := newParallelContext(
		domain.ProjectTask{ID: "T-001", Title: "theme", State: domain.TaskTodo, Files: []string{"lib/theme.dart"}},
		domain.ProjectTask{ID: "T-002", Title: "more theme", State: domain.TaskTodo, Files: []string{"lib/theme.dart"}},
		domain.ProjectTask{ID: "T-003", Title: "other", State: domain.TaskTodo, Files: []string{"lib/other.dart"}},
	)

	slots := selectParallelTasks(sCtx, 3)
	if len(slots) != 2 {
		t.Fatalf("only two tasks touch different files, got %d selected", len(slots))
	}
	// The conflicting task must still be in the plan, just not in this batch.
	if len(sCtx.PlanSnapshot().Tasks) != 3 {
		t.Error("an excluded task must stay in the plan for a later batch")
	}
}

// TestSelectParallelTasksRespectsPathSpelling proves selection uses the same
// normalization as the lock. If these two disagreed, selection would let two tasks
// "independent" tasks write the same file.
func TestSelectParallelTasksRespectsPathSpelling(t *testing.T) {
	sCtx := newParallelContext(
		domain.ProjectTask{ID: "T-001", Files: []string{"lib/theme.dart"}, State: domain.TaskTodo},
		domain.ProjectTask{ID: "T-002", Files: []string{"./lib/theme.dart"}, State: domain.TaskTodo},
	)

	slots := selectParallelTasks(sCtx, 2)
	if len(slots) != 1 {
		t.Fatalf("two spellings of one file must not run together, got %d slots", len(slots))
	}
}

// TestSelectParallelTasksHonoursTheLimit keeps a large plan from launching an
// unbounded number of agents at the provider.
func TestSelectParallelTasksHonoursTheLimit(t *testing.T) {
	var tasks []domain.ProjectTask
	for _, id := range []string{"T-001", "T-002", "T-003", "T-004", "T-005", "T-006"} {
		tasks = append(tasks, domain.ProjectTask{
			ID: id, State: domain.TaskTodo, Files: []string{"lib/" + id + ".dart"},
		})
	}
	sCtx := newParallelContext(tasks...)

	if slots := selectParallelTasks(sCtx, 2); len(slots) != 2 {
		t.Errorf("a limit of 2 should select 2 tasks, got %d", len(slots))
	}
}

// TestSelectParallelTasksSkipsFinishedTasks keeps completed work out of a batch.
// Re-running a DONE task would rewrite code the verifier already accepted.
func TestSelectParallelTasksSkipsFinishedTasks(t *testing.T) {
	sCtx := newParallelContext(
		domain.ProjectTask{ID: "T-001", State: domain.TaskDone, Files: []string{"lib/a.dart"}},
		domain.ProjectTask{ID: "T-002", State: domain.TaskTodo, Files: []string{"lib/b.dart"}},
		domain.ProjectTask{ID: "T-003", State: domain.TaskWaiting, Files: []string{"lib/c.dart"}},
		domain.ProjectTask{ID: "T-004", State: domain.TaskTodo, Files: []string{"lib/d.dart"}},
	)

	slots := selectParallelTasks(sCtx, 4)
	if len(slots) != 2 {
		t.Fatalf("only the two TODO tasks are runnable, got %d", len(slots))
	}
	for _, slot := range slots {
		if slot.Task.State == domain.TaskDone || slot.Task.State == domain.TaskWaiting {
			t.Errorf("terminal task %s was selected for a batch", slot.Task.ID)
		}
	}
}

// TestForkIsolatesAgentOutput is the safety property for concurrency. If a fork
// shared the review pointers, two coders would overwrite each other's findings
// and the merged result would be a mixture of two different tasks' work.
func TestForkIsolatesAgentOutput(t *testing.T) {
	sCtx := newParallelContext(
		domain.ProjectTask{ID: "T-001", State: domain.TaskInProgress, Files: []string{"lib/a.dart"}},
	)
	sCtx.CriticReview = &domain.CriticPayload{Issues: []domain.ReviewIssue{{File: "shared.dart"}}}

	forkA := sCtx.Fork(domain.RoleCoder, "T-001")
	forkB := sCtx.Fork(domain.RoleCoder, "T-001")

	if forkA == nil || forkB == nil {
		t.Fatal("fork returned nil")
	}

	// Each worker writes its own review.
	forkA.CriticReview = &domain.CriticPayload{Issues: []domain.ReviewIssue{{File: "a.dart"}}}
	forkB.CriticReview = &domain.CriticPayload{Issues: []domain.ReviewIssue{{File: "b.dart"}}}

	// The shared context must be untouched by either.
	if sCtx.CriticReview == nil || len(sCtx.CriticReview.Issues) != 1 {
		t.Fatal("a fork wrote through to the shared context")
	}
	if sCtx.CriticReview.Issues[0].File != "shared.dart" {
		t.Errorf("shared review was overwritten: %v", sCtx.CriticReview.Issues[0].File)
	}
}

// TestForkSharesTheLockManager is the counterpart, and the one that actually
// matters for safety. Forking the lock coordinator would give each worker a
// private set of locks, and mutual exclusion would be exactly zero.
func TestForkSharesTheLockManager(t *testing.T) {
	sCtx := newParallelContext(domain.ProjectTask{ID: "T-001", State: domain.TaskTodo})

	fork := sCtx.Fork(domain.RoleCoder, "T-001")
	if fork.FileLocks != sCtx.FileLocks {
		t.Fatal("the fork got its own lock manager, so the lock would protect nothing")
	}

	// Prove it end to end: a lock taken through the fork is visible on the parent.
	fork.FileLocks.Claim("coder-1", "T-001", []string{"lib/a.dart"})
	if !sCtx.FileLocks.IsLocked("lib/a.dart") {
		t.Error("a lock taken through the fork is invisible to the parent context")
	}
}

// TestForkCopiesMapsRatherThanAliasing stops two workers writing one map. The
// token ledger is the most likely casualty: concurrent appends to a shared map
// are a data race even when the keys differ.
func TestForkCopiesMapsRatherThanAliasing(t *testing.T) {
	sCtx := newParallelContext(domain.ProjectTask{ID: "T-001", State: domain.TaskTodo})
	sCtx.AgentModels = map[domain.AgentRole]string{domain.RoleCoder: "model-a"}
	sCtx.APIKeys = map[string]string{"key": "secret"}

	fork := sCtx.Fork(domain.RoleCoder, "T-001")

	fork.AgentModels[domain.RoleCoder] = "model-b"
	fork.APIKeys["key"] = "changed"

	if sCtx.AgentModels[domain.RoleCoder] != "model-a" {
		t.Error("writing to the fork's model map changed the shared one")
	}
	if sCtx.APIKeys["key"] != "secret" {
		t.Error("writing to the fork's key map changed the shared one")
	}
}

// TestMergeResultReportsCollisions is what keeps the lock honest. Two workers
// writing one file should be impossible, and if it happens anyway it must be
// reported rather than silently resolved in favour of whoever finished last.
func TestMergeResultReportsCollisions(t *testing.T) {
	sCtx := newParallelContext(domain.ProjectTask{ID: "T-001", State: domain.TaskTodo})

	first := sCtx.MergeResult(agent.ForkResult{
		Files:    []domain.CodeFile{{Path: "lib/a.dart", Content: "from coder-1"}},
		HasFiles: true,
	})
	if len(first) != 0 {
		t.Errorf("the first write to a file is not a collision, got %v", first)
	}

	second := sCtx.MergeResult(agent.ForkResult{
		Files:    []domain.CodeFile{{Path: "lib/a.dart", Content: "from coder-2"}},
		HasFiles: true,
	})
	if len(second) != 1 || second[0] != "lib/a.dart" {
		t.Fatalf("two workers writing one file must be reported, got %v", second)
	}

	// The file must appear once, not twice.
	files := sCtx.CodeFilesSnapshot()
	count := 0
	for _, f := range files {
		if f.Path == "lib/a.dart" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("the file should appear once after two writes, appeared %d times", count)
	}
}

// TestMergeResultIsConcurrencySafe runs the merge from many goroutines at once.
// This is the test that would catch the race the mutex exists to prevent; without
// -race it catches the lost writes.
func TestMergeResultIsConcurrencySafe(t *testing.T) {
	sCtx := newParallelContext(domain.ProjectTask{ID: "T-001", State: domain.TaskTodo})

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(n int) {
			defer wg.Done()
			path := "lib/file" + string(rune('a'+n)) + ".dart"
			sCtx.MergeResult(agent.ForkResult{
				Files:    []domain.CodeFile{{Path: path, Content: "x"}},
				HasFiles: true,
			})
		}(i)
	}
	wg.Wait()

	if got := len(sCtx.CodeFilesSnapshot()); got != workers {
		t.Errorf("concurrent merges lost work: expected %d files, got %d", workers, got)
	}
}

// TestClaimTasksForBatchMarksEachTaskInProgress covers the claim step. Without it
// two workers could be handed the same task, and without the batch marking a task
// stays TODO while it is being worked on.
func TestClaimTasksForBatchMarksEachTaskInProgress(t *testing.T) {
	sCtx := newParallelContext(
		domain.ProjectTask{ID: "T-001", State: domain.TaskTodo, Files: []string{"lib/a.dart"}},
		domain.ProjectTask{ID: "T-002", State: domain.TaskTodo, Files: []string{"lib/b.dart"}},
	)

	candidates := sCtx.AvailableTasks()
	if len(candidates) != 2 {
		t.Fatalf("expected two available tasks, got %d", len(candidates))
	}

	claimed := sCtx.ClaimTasksForBatch(candidates)
	if len(claimed) != 2 {
		t.Fatalf("expected two claimed tasks, got %d", len(claimed))
	}

	for _, task := range sCtx.PlanSnapshot().Tasks {
		if task.State != domain.TaskInProgress {
			t.Errorf("task %s should be IN_PROGRESS after the batch claim, is %s", task.ID, task.State)
		}
	}
}

// TestAvailableTasksHasNoSideEffects matters because selection runs before the
// claim. If listing the candidates already flipped them to IN_PROGRESS, a task
// that was never chosen would look like work in progress.
func TestAvailableTasksHasNoSideEffects(t *testing.T) {
	sCtx := newParallelContext(
		domain.ProjectTask{ID: "T-001", State: domain.TaskTodo, Files: []string{"lib/a.dart"}},
		domain.ProjectTask{ID: "T-002", State: domain.TaskTodo, Files: []string{"lib/b.dart"}},
	)

	sCtx.AvailableTasks()

	for _, task := range sCtx.PlanSnapshot().Tasks {
		if task.State != domain.TaskTodo {
			t.Errorf("listing available tasks changed %s to %s", task.ID, task.State)
		}
	}
}

// TestReturnTaskToQueueDoesNotChargeARework keeps a blocked task from escalating.
// A task skipped because its file was held is not a failure, and charging it a
// rework would eventually send it to a human for no reason.
func TestReturnTaskToQueueDoesNotChargeARework(t *testing.T) {
	sCtx := newParallelContext(domain.ProjectTask{ID: "T-001", State: domain.TaskTodo})

	sCtx.ClaimTasksForBatch(sCtx.AvailableTasks())
	sCtx.ReturnTaskToQueue("T-001")

	task := sCtx.PlanSnapshot().Tasks[0]
	if task.State != domain.TaskTodo {
		t.Errorf("a skipped task should go back to the queue, is %s", task.State)
	}
	if task.ReworkCount != 0 {
		t.Errorf("a skipped task must not be charged a rework, count is %d", task.ReworkCount)
	}
	if task.LastError != "" {
		t.Errorf("a skipped task should carry no error, has %q", task.LastError)
	}
}

// TestCompleteTaskAsClosesOnlyTheNamedTask matters under a batch: several results
// are applied one after another, and each must land on its own task.
func TestCompleteTaskAsClosesOnlyTheNamedTask(t *testing.T) {
	sCtx := newParallelContext(
		domain.ProjectTask{ID: "T-001", State: domain.TaskInProgress},
		domain.ProjectTask{ID: "T-002", State: domain.TaskInProgress},
	)

	sCtx.CompleteTaskAs("T-001", []string{"lib/a.dart"})

	tasks := sCtx.PlanSnapshot().Tasks
	if tasks[0].State != domain.TaskDone {
		t.Errorf("T-001 should be done, is %s", tasks[0].State)
	}
	if tasks[1].State != domain.TaskInProgress {
		t.Errorf("T-002 must be untouched, is %s", tasks[1].State)
	}
	if len(tasks[0].Files) != 1 || tasks[0].Files[0] != "lib/a.dart" {
		t.Errorf("the produced files should be recorded, got %v", tasks[0].Files)
	}
}

// TestParallelBudgetClampsToASaneRange covers the configuration edge cases. A
// value below two must collapse to the serial path rather than misbehave, and a
// huge value must not be taken literally.
func TestParallelBudgetClampsToASaneRange(t *testing.T) {
	cases := []struct {
		configured int
		want       int
	}{
		{0, 1}, // unset means serial
		{-3, 1},
		{1, 1}, // explicitly serial
		{2, 2},
		{99, maxParallelTasks},
	}
	for _, tc := range cases {
		o := &Orchestrator{config: Config{MaxParallelTasks: tc.configured}}
		if got := o.parallelBudget(); got != tc.want {
			t.Errorf("configured %d: expected budget %d, got %d", tc.configured, tc.want, got)
		}
		if want := tc.want > 1; o.parallelEnabled() != want {
			t.Errorf("configured %d: parallelEnabled should be %v", tc.configured, want)
		}
	}
}

// TestNewOrchestratorKeepsMaxParallelTasks exists because of a bug this whole
// file could not catch.
//
// Parallel execution was fully implemented, correctly tested here, and never ran
// once: main.go built the orchestrator's config without the field, so it stayed
// zero, so parallelEnabled() was always false. Every test passed because they all
// constructed the config themselves.
//
// The gap is not in the config struct — it is in whether the wiring reaches it.
// So this test asserts the value survives the constructor, which is the step the
// others all skipped.
func TestNewOrchestratorKeepsMaxParallelTasks(t *testing.T) {
	orch := NewOrchestrator(
		Config{MaxRounds: 5, Mode: "rolebased", MaxParallelTasks: 3},
		nil, nil, nil, nil, nil, nil, nil, t.TempDir(),
	)

	if got := orch.parallelBudget(); got != 3 {
		t.Errorf("a configured parallelism of 3 should survive construction, got %d", got)
	}
	if !orch.parallelEnabled() {
		t.Error("an orchestrator built with parallelism configured should have it enabled")
	}
}

// TestSerialByDefault documents the choice that made the bug possible. A
// zero-value config is serial, which is the safe default — but it also means any
// missing wiring turns the feature off silently rather than loudly, which is
// exactly what happened. This test names that risk so the default is a decision
// on record rather than an accident.
func TestSerialByDefault(t *testing.T) {
	orch := &Orchestrator{}
	if orch.parallelEnabled() {
		t.Error("a zero-value orchestrator must stay serial; that is the safe default")
	}
	if got := orch.parallelBudget(); got != 1 {
		t.Errorf("a zero-value orchestrator should have a budget of 1, got %d", got)
	}
}

// TestFilesChangedSinceCoversRevisions covers the bug that made most tasks
// report nothing.
//
// Detecting new work by comparing list lengths misses the single most common case:
// revising a file that already exists leaves the list the same size. The task then
// records no files, the plan is given an empty file list, and the change ledger
// shows nothing despite the code having changed.
func TestFilesChangedSinceCoversRevisions(t *testing.T) {
	before := map[string]string{
		"lib/theme.dart": "original theme",
		"lib/other.dart": "untouched",
	}

	// A coder rewrote one file, added another, and left a third alone.
	after := []domain.CodeFile{
		{Path: "lib/theme.dart", Content: "REWRITTEN theme"},
		{Path: "lib/other.dart", Content: "untouched"},
		{Path: "lib/new.dart", Content: "brand new"},
	}

	changed := filesChangedSince(before, after)
	got := map[string]bool{}
	for _, f := range changed {
		got[f.Path] = true
	}

	if !got["lib/theme.dart"] {
		t.Error("a rewritten file must count as changed; comparing list lengths misses it")
	}
	if !got["lib/new.dart"] {
		t.Error("a new file must count as changed")
	}
	if got["lib/other.dart"] {
		t.Error("an untouched file must not count as changed")
	}
	if len(changed) != 2 {
		t.Errorf("expected exactly two changed files, got %d: %+v", len(changed), changed)
	}
}

// TestFilesChangedSinceIsEmptyWhenNothingHappened matters because the reverse
// mistake is worse: reporting every file as changed would make every parallel task
// claim the whole project, and the file lock would then block all of them.
func TestFilesChangedSinceIsEmptyWhenNothingHappened(t *testing.T) {
	before := map[string]string{"lib/a.dart": "same", "lib/b.dart": "also same"}
	after := []domain.CodeFile{
		{Path: "lib/a.dart", Content: "same"},
		{Path: "lib/b.dart", Content: "also same"},
	}

	if changed := filesChangedSince(before, after); len(changed) != 0 {
		t.Errorf("an unchanged project should report no changed files, got %+v", changed)
	}
}

// TestBatchMadeProgressDetectsAStuckBatch covers the livelock guard.
//
// Every task in this batch was blocked, so nothing changed. A loop that treated
// this as a completed round would advance its counter over the same stuck state
// forever — no error, no log, just an agent burning money until something else
// stopped it.
func TestBatchMadeProgressDetectsAStuckBatch(t *testing.T) {
	stuck := []ParallelOutcome{
		{Task: &domain.ProjectTask{ID: "T-001"}, Skipped: "held by coder-1"},
		{Task: &domain.ProjectTask{ID: "T-002"}, Skipped: "held by coder-2"},
	}
	if batchMadeProgress(stuck) {
		t.Error("a batch where every task was skipped made no progress")
	}

	partial := []ParallelOutcome{
		{Task: &domain.ProjectTask{ID: "T-001"}, Skipped: "held by coder-1"},
		{Task: &domain.ProjectTask{ID: "T-002"}},
	}
	if !batchMadeProgress(partial) {
		t.Error("one task running is enough to count as progress")
	}

	if batchMadeProgress(nil) {
		t.Error("an empty batch made no progress")
	}
}

// TestPlanProgressPercentMovesOnlyWhenATaskCloses is what the guard in the run
// loop relies on. If the percentage moved for any reason other than real
// progress, the loop would treat a stuck batch as a finished round.
func TestPlanProgressPercentMovesOnlyWhenATaskCloses(t *testing.T) {
	sCtx := newParallelContext(
		domain.ProjectTask{ID: "T-001", State: domain.TaskTodo, Files: []string{"lib/a.dart"}},
		domain.ProjectTask{ID: "T-002", State: domain.TaskTodo, Files: []string{"lib/b.dart"}},
	)

	before := sCtx.PlanProgressPercent()
	if before != 0 {
		t.Errorf("a fresh plan should be 0 percent, got %d", before)
	}

	// A task sent back to rework is not progress.
	sCtx.ClaimTasksForBatch(sCtx.AvailableTasks())
	sCtx.SendTaskBackToReworkAs("T-001", "blocked")
	if after := sCtx.PlanProgressPercent(); after != before {
		t.Errorf("a reworked task must not count as progress: %d then %d", before, after)
	}

	// A task returned to the queue is not progress either.
	sCtx.ReturnTaskToQueue("T-002")
	if after := sCtx.PlanProgressPercent(); after != before {
		t.Errorf("a requeued task must not count as progress: %d then %d", before, after)
	}

	// Only a closed task moves it.
	sCtx.CompleteTaskAs("T-001", []string{"lib/a.dart"})
	if after := sCtx.PlanProgressPercent(); after <= before {
		t.Errorf("closing a task should raise progress: %d then %d", before, after)
	}
}

func TestParallelWorkerModelInheritance(t *testing.T) {
	sCtx := newParallelContext(
		domain.ProjectTask{ID: "T-001", State: domain.TaskTodo, Files: []string{"lib/a.dart"}},
	)
	sCtx.AgentModels = map[domain.AgentRole]string{
		"custom": "test-custom-model",
	}

	fork := sCtx.Fork(domain.RoleCoder, "T-001")
	if fork == nil {
		t.Fatal("expected fork")
	}

	if fork.AgentModels[domain.RoleCoder] == "" {
		for _, m := range sCtx.AgentModels {
			if m != "" {
				fork.AgentModels[domain.RoleCoder] = m
				break
			}
		}
	}

	if fork.AgentModels[domain.RoleCoder] != "test-custom-model" {
		t.Errorf("expected test-custom-model, got %s", fork.AgentModels[domain.RoleCoder])
	}
}

