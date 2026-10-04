package workspace

import (
	"strings"
	"testing"
	"time"
)

// These tests cover the behaviour the lock exists for: two agents must not edit
// the same file at once, and a lock must never outlive the work that took it.

// newTestLockManager returns a manager with an injectable clock, so lease expiry
// can be exercised without sleeping for the real TTL.
func newTestLockManager() (*FileLockManager, *time.Time) {
	m := NewFileLockManager()
	clock := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return clock }
	return m, &clock
}

// TestClaimPreventsConcurrentEdit is the reason this file exists: a second agent
// must be refused a file that is already being written.
func TestClaimPreventsConcurrentEdit(t *testing.T) {
	m, _ := newTestLockManager()

	first := m.Claim("coder", "T-001", []string{"src/theme.css"})
	if !first.Acquired {
		t.Fatalf("first agent should acquire a free file: %+v", first.Conflicts)
	}

	second := m.Claim("reviewer", "T-002", []string{"src/theme.css"})
	if second.Acquired {
		t.Fatal("second agent acquired a file that is already held")
	}
	if len(second.Conflicts) != 1 {
		t.Fatalf("expected one conflict, got %d: %+v", len(second.Conflicts), second.Conflicts)
	}
	if second.Conflicts[0].Holder != "coder" {
		t.Errorf("conflict should name the holder, got %q", second.Conflicts[0].Holder)
	}
	if second.Conflicts[0].TaskID != "T-001" {
		t.Errorf("conflict should name the task, got %q", second.Conflicts[0].TaskID)
	}
}

// TestClaimIsAllOrNothing guards the worst outcome. An agent that took half its
// files has excluded everyone else from them while still being unable to work.
func TestClaimIsAllOrNothing(t *testing.T) {
	m, _ := newTestLockManager()

	m.Claim("coder", "T-001", []string{"src/theme.css"})

	result := m.Claim("reviewer", "T-002", []string{"src/other.css", "src/theme.css"})
	if result.Acquired {
		t.Fatal("claim should fail when one file in the set is busy")
	}
	// The free file must NOT have been taken, or the caller would be holding a
	// lock it is about to abandon.
	if m.IsLocked("src/other.css") {
		t.Error("a failed claim must not leave locks behind")
	}
	if len(result.Conflicts) != 1 {
		t.Errorf("expected exactly the busy file as a conflict, got %+v", result.Conflicts)
	}
}

// TestReleaseMakesFileAvailableAgain is what keeps the lock short-lived: work
// that has finished must not keep other agents out.
func TestReleaseMakesFileAvailableAgain(t *testing.T) {
	m, _ := newTestLockManager()

	m.Claim("coder", "T-001", []string{"src/theme.css"})
	released := m.Release("coder", []string{"src/theme.css"})
	if len(released) != 1 {
		t.Fatalf("expected the release to report one path, got %v", released)
	}

	next := m.Claim("reviewer", "T-002", []string{"src/theme.css"})
	if !next.Acquired {
		t.Fatal("file should be free once the previous holder released it")
	}
}

// TestReleaseAllFreesEverythingAnAgentHeld keeps a finishing agent from leaving
// locks scattered across the project.
func TestReleaseAllFreesEverythingAnAgentHeld(t *testing.T) {
	m, _ := newTestLockManager()

	m.Claim("coder", "T-001", []string{"a.css", "b.css", "c.css"})
	m.Claim("reviewer", "T-002", []string{"d.css"})

	released := m.ReleaseAll("coder")
	if len(released) != 3 {
		t.Fatalf("expected three releases, got %v", released)
	}
	if m.IsLocked("a.css") || m.IsLocked("b.css") || m.IsLocked("c.css") {
		t.Error("coder's files should all be free")
	}
	// Another agent's hold must survive: ReleaseAll is scoped to its caller.
	if !m.IsLocked("d.css") {
		t.Error("release must not free files held by a different agent")
	}
}

// TestReleaseCannotStealAnotherAgentsLock stops a bug in one agent from letting
// two agents believe they own the same file.
func TestReleaseCannotStealAnotherAgentsLock(t *testing.T) {
	m, _ := newTestLockManager()

	m.Claim("coder", "T-001", []string{"src/theme.css"})
	m.Release("reviewer", []string{"src/theme.css"})

	if !m.IsLocked("src/theme.css") {
		t.Fatal("an agent must not be able to release a lock it does not hold")
	}
	holder, _ := m.Holder("src/theme.css")
	if holder.Agent != "coder" {
		t.Errorf("lock should still belong to coder, got %q", holder.Agent)
	}
}

// TestExpiredLeaseFreesAbandonedLock covers the crash case. An agent that dies
// mid-edit leaves a lock nobody will release; the lease has to expire it or the
// run deadlocks on a file that no living agent owns.
func TestExpiredLeaseFreesAbandonedLock(t *testing.T) {
	m, clock := newTestLockManager()
	m.SetLeaseTTL(10 * time.Minute)

	m.Claim("coder", "T-001", []string{"src/theme.css"})

	// Still inside the lease: the lock must hold.
	*clock = clock.Add(9 * time.Minute)
	if m.Claim("reviewer", "T-002", []string{"src/theme.css"}).Acquired {
		t.Fatal("lock should still be live inside its lease")
	}

	// Past the lease: the abandoned hold is gone and the file is usable.
	*clock = clock.Add(2 * time.Minute)
	if !m.Claim("reviewer", "T-002", []string{"src/theme.css"}).Acquired {
		t.Fatal("an expired lease should free the file")
	}
}

// TestRenewKeepsLongWorkAlive stops a legitimately slow task from losing its
// files mid-edit.
func TestRenewKeepsLongWorkAlive(t *testing.T) {
	m, clock := newTestLockManager()
	m.SetLeaseTTL(10 * time.Minute)

	m.Claim("coder", "T-001", []string{"src/theme.css"})

	for i := 0; i < 5; i++ {
		*clock = clock.Add(8 * time.Minute)
		renewed := m.Renew("coder", []string{"src/theme.css"})
		if len(renewed) != 1 {
			t.Fatalf("renewal should succeed while the agent still holds the file, got %v", renewed)
		}
	}

	// The lease never lapsed despite 40 minutes of work.
	if m.Claim("reviewer", "T-002", []string{"src/theme.css"}).Acquired {
		t.Fatal("a renewed lock must stay held")
	}
}

// TestRenewCannotStealAnExpiredLock is subtle but important. After expiry the
// file belongs to nobody, so silently renewing would let a second agent and the
// original one both believe they hold it.
func TestRenewCannotStealAnExpiredLock(t *testing.T) {
	m, clock := newTestLockManager()
	m.SetLeaseTTL(10 * time.Minute)

	m.Claim("coder", "T-001", []string{"src/theme.css"})
	*clock = clock.Add(11 * time.Minute)

	if renewed := m.Renew("coder", []string{"src/theme.css"}); len(renewed) != 0 {
		t.Fatal("renewal must not resurrect an expired lock")
	}
	if m.IsLocked("src/theme.css") {
		t.Error("an expired lock should be gone")
	}
}

// TestPathSpellingsResolveToOneLock closes the loophole that would defeat the
// whole mechanism. If "./src/theme.css" and "src/theme.css" were different keys,
// two agents could hold the same file and both believe they were safe.
func TestPathSpellingsResolveToOneLock(t *testing.T) {
	m, _ := newTestLockManager()

	spellings := [][]string{
		{"src/theme.css"},
		{"./src/theme.css"},
		{"src/../src/theme.css"},
		{"src//theme.css"},
	}
	for i, spelling := range spellings {
		result := m.Claim("agent", "T", spelling)
		if i == 0 {
			if !result.Acquired {
				t.Fatalf("first spelling should be acquired: %+v", result.Conflicts)
			}
		} else {
			if result.Acquired {
				t.Fatalf("spelling %d should resolve to the same file and be refused", i)
			}
		}
	}
}

// TestEmptyAndDuplicatePathsAreIgnored keeps a malformed entry from producing a
// bogus lock on the empty path that every other caller would collide with.
func TestEmptyAndDuplicatePathsAreIgnored(t *testing.T) {
	m, _ := newTestLockManager()

	result := m.Claim("coder", "T-001", []string{"", "  ", "a.css", "a.css", "./a.css"})
	if !result.Acquired {
		t.Fatalf("valid files should still be claimable: %+v", result.Conflicts)
	}
	if len(result.Locks) != 1 {
		t.Errorf("duplicates and blanks should collapse to one lock, got %d: %+v", len(result.Locks), result.Locks)
	}
}

// TestActiveExcludesExpiredLocks keeps the UI from showing a lock that no longer
// blocks anything.
func TestActiveExcludesExpiredLocks(t *testing.T) {
	m, clock := newTestLockManager()
	m.SetLeaseTTL(time.Minute)

	m.Claim("coder", "T-001", []string{"live.css"})
	m.Claim("coder", "T-001", []string{"doomed.css"})

	if len(m.Active()) != 2 {
		t.Fatalf("expected two live locks, got %d", len(m.Active()))
	}

	*clock = clock.Add(2 * time.Minute)
	if len(m.Active()) != 0 {
		t.Errorf("expired locks should not be reported as active, got %+v", m.Active())
	}
}

// TestDescribeConflictsNamesTheHolder matters because this string is what an
// agent reads when it cannot proceed. "File is locked" is not actionable.
func TestDescribeConflictsNamesTheHolder(t *testing.T) {
	msg := DescribeConflicts([]LockConflict{
		{Path: "src/theme.css", Holder: "coder", TaskID: "T-007", HeldFor: 90 * time.Second},
	})

	if msg == "" {
		t.Fatal("a conflict should produce a message")
	}
	for _, want := range []string{"src/theme.css", "coder", "T-007"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message should mention %q, got: %s", want, msg)
		}
	}
	if DescribeConflicts(nil) != "" {
		t.Error("no conflicts should produce no message")
	}
}
