package orchestrator

import (
	"testing"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// These tests cover the wiring, not the lock itself. The lock's own behaviour is
// pinned down in workspace/file_lock_test.go; what can still be wrong is the
// orchestrator forgetting to use it, or letting a hold outlive the work that
// took it. Both failures are silent, so they are pinned down here.

// TestClaimTaskFilesRefusesABusyFile proves the loop actually consults the lock.
// A lock that is correct but never called protects nothing, and nothing in the
// suite would notice.
func TestClaimTaskFilesRefusesABusyFile(t *testing.T) {
	sCtx := &agent.SwarmContext{FileLocks: workspace.NewFileLockManager()}
	o := &Orchestrator{}

	task := &domain.ProjectTask{ID: "T-001", Title: "theme", Files: []string{"src/theme.css"}}

	// Another agent is already writing this file.
	sCtx.FileLocks.Claim("reviewer", "T-000", []string{"src/theme.css"})

	files, ok, reason := o.claimTaskFiles(sCtx, task, "coder")
	if ok {
		t.Fatal("the orchestrator claimed a file another agent holds")
	}
	if files != nil {
		t.Errorf("a refused claim must hold nothing, got %v", files)
	}
	// The reason is what an agent reads when it cannot proceed, so it has to name
	// the holder rather than just reporting a failure.
	if reason == "" {
		t.Error("a refused claim should explain which file is held and by whom")
	}
}

// TestClaimTaskFilesAllowsAFreeFile is the control for the test above: without
// it, a broken implementation that always refuses would pass.
func TestClaimTaskFilesAllowsAFreeFile(t *testing.T) {
	sCtx := &agent.SwarmContext{FileLocks: workspace.NewFileLockManager()}
	o := &Orchestrator{}

	task := &domain.ProjectTask{ID: "T-001", Title: "theme", Files: []string{"src/theme.css"}}

	files, ok, _ := o.claimTaskFiles(sCtx, task, "coder")
	if !ok {
		t.Fatal("a free file should be claimable")
	}
	if len(files) != 1 {
		t.Errorf("expected the claimed file back, got %v", files)
	}
	if !sCtx.FileLocks.IsLocked("src/theme.css") {
		t.Error("the claim did not actually take the lock")
	}
}

// TestReleaseTaskFilesFreesTheFile covers the requirement that a lock is held
// only while the work is happening. A hold that outlives its task would keep an
// unrelated later task out of a file nobody is writing.
func TestReleaseTaskFilesFreesTheFile(t *testing.T) {
	sCtx := &agent.SwarmContext{FileLocks: workspace.NewFileLockManager()}
	o := &Orchestrator{}

	task := &domain.ProjectTask{ID: "T-001", Title: "theme", Files: []string{"src/theme.css"}}

	_, _, _ = o.claimTaskFiles(sCtx, task, "coder")
	o.releaseTaskFiles(sCtx, "coder", []string{"src/theme.css"})

	if sCtx.FileLocks.IsLocked("src/theme.css") {
		t.Error("releasing should free the file for the next agent")
	}
	if _, ok, _ := o.claimTaskFiles(sCtx, task, "reviewer"); !ok {
		t.Error("the next agent should be able to take the file once released")
	}

}

// TestReleaseCannotFreeAnotherAgentsHold is the safety property. If any agent
// could release any lock, a bug in one agent would silently unlock a file another
// agent is actively writing — reintroducing the exact collision the lock prevents.
func TestReleaseCannotFreeAnotherAgentsHold(t *testing.T) {
	sCtx := &agent.SwarmContext{FileLocks: workspace.NewFileLockManager()}
	o := &Orchestrator{}

	sCtx.FileLocks.Claim("coder", "T-001", []string{"src/theme.css"})
	o.releaseTaskFiles(sCtx, "reviewer", []string{"src/theme.css"})

	if !sCtx.FileLocks.IsLocked("src/theme.css") {
		t.Fatal("an agent released a lock it does not hold")
	}
}

// TestReleaseEverythingClearsAnAbandonedTask covers the case that would strand a
// file. When a task escalates to WAITING_FOR_HUMAN it is never handed out again,
// so anything still held under its name would stay locked for the whole run.
func TestReleaseEverythingClearsAnAbandonedTask(t *testing.T) {
	sCtx := &agent.SwarmContext{FileLocks: workspace.NewFileLockManager()}

	// The coder picks up several files across rounds, as it would in a real run.
	sCtx.FileLocks.Claim("coder", "T-001", []string{"a.dart"})
	sCtx.FileLocks.Claim("coder", "T-002", []string{"b.dart", "c.dart"})
	// Another agent's work must survive.
	sCtx.FileLocks.Claim("reviewer", "T-003", []string{"d.dart"})

	sCtx.ReleaseEverything("coder")

	for _, p := range []string{"a.dart", "b.dart", "c.dart"} {
		if sCtx.FileLocks.IsLocked(p) {
			t.Errorf("%s should be free after the agent walked away", p)
		}
	}
	if !sCtx.FileLocks.IsLocked("d.dart") {
		t.Error("another agent's hold must not be released")
	}
}

// TestTaskWithNoFilesIsAlwaysClaimable keeps the lock out of the way of the
// common case. The planner does not always know the paths in advance, and a task
// with no file list must not be blocked by a mechanism that has nothing to check.
func TestTaskWithNoFilesIsAlwaysClaimable(t *testing.T) {
	sCtx := &agent.SwarmContext{FileLocks: workspace.NewFileLockManager()}
	o := &Orchestrator{}

	// Even with every file held by someone else.
	sCtx.FileLocks.Claim("other", "T-000", []string{"anything.css"})

	task := &domain.ProjectTask{ID: "T-001", Title: "write docs"}
	files, ok, _ := o.claimTaskFiles(sCtx, task, "coder")
	if !ok {
		t.Fatal("a task with no files should never be blocked")
	}
	if len(files) != 0 {
		t.Errorf("expected no files to hold, got %v", files)
	}
}

// TestNilLockManagerDoesNotBlock guards the single-writer path. Swarms created
// without a coordinator must keep working exactly as before rather than failing
// every claim.
func TestNilLockManagerDoesNotBlock(t *testing.T) {
	sCtx := &agent.SwarmContext{} // no FileLocks
	o := &Orchestrator{}

	task := &domain.ProjectTask{ID: "T-001", Files: []string{"a.css"}}
	if _, ok, _ := o.claimTaskFiles(sCtx, task, "coder"); !ok {
		t.Fatal("a run without a lock manager must still be able to work")
	}
	// Releasing and querying must be safe no-ops rather than nil-pointer panics.
	o.releaseTaskFiles(sCtx, "coder", []string{"a.css"})
	sCtx.ReleaseEverything("coder")
	if sCtx.LockedFiles() != nil {
		t.Error("a run without a lock manager should report no locks")
	}
}
