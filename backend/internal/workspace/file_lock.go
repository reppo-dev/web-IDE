package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileLockManager keeps two agents from editing the same file at the same time.
//
// The problem it solves is not theoretical. Several agents working on one
// project at once will each rewrite the same shared file — a theme, a design
// system, a shared type — and the result is the "one button blue, one black, one
// pink" outcome: every file is internally valid and the whole no longer hangs
// together. A plan cannot prevent that, because no single planner sees the other
// agent's later edits. Only mutual exclusion can.
//
// The lock is deliberately short-lived. It exists while an agent is actually
// working, and it is dropped the moment that work ends — so an agent is never
// blocked by an unrelated task that has nothing to do with the file. The lease
// is the safety net for the other case: an agent that crashes mid-edit leaves a
// lock nobody will ever release, and the lease expires it on its own so the run
// cannot deadlock.

// FileLock is one agent's hold on one file.
type FileLock struct {
	// Path is the project-relative path that is held. It is normalized, so two
	// spellings of the same file resolve to the same lock.
	Path string
	// Agent is the role or agent name holding the file, used so a blocked agent
	// can be told who it is waiting for instead of failing anonymously.
	Agent string
	// TaskID is the task the holder is working on, so a stale lock can be traced
	// back to the work that abandoned it.
	TaskID string
	// AcquiredAt is when the hold began.
	AcquiredAt time.Time
}

// lockEntry is the stored state for one held path, including the lease deadline
// that makes an abandoned lock expire on its own.
type lockEntry struct {
	lock       FileLock
	renewedAt  time.Time
	expiresAt  time.Time
}

// DefaultLeaseTTL is how long a claim survives without being renewed.
//
// An agent that writes a large file, runs a slow tool, or waits on a test suite
// can legitimately hold a file for a while, so the TTL is generous. It only has
// to outlive a real unit of work, not to limit how long that work may take.
const DefaultLeaseTTL = 10 * time.Minute

// FileLockManager tracks which files are currently held.
//
// Paths are normalized before they are used as keys. Two agents naming the same
// file differently — `src/app.css` and `./src/app.css`, or `src/app.css` and
// `src\app.css` on Windows — must resolve to one lock, or mutual exclusion fails
// exactly when it is needed most.
type FileLockManager struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
	ttl   time.Duration
	// now is injectable so expiry can be tested without sleeping.
	now func() time.Time
}

// NewFileLockManager returns a manager with the default lease.
func NewFileLockManager() *FileLockManager {
	return &FileLockManager{
		locks: make(map[string]*lockEntry),
		ttl:   DefaultLeaseTTL,
		now:   time.Now,
	}
}

// SetLeaseTTL changes how long a claim survives without renewal. A non-positive
// value restores the default, so a bad configuration cannot disable expiry
// entirely and turn every crash into a permanent deadlock.
func (m *FileLockManager) SetLeaseTTL(ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ttl <= 0 {
		ttl = DefaultLeaseTTL
	}
	m.ttl = ttl
}

// NormalizePath reduces a path to the form used as a lock key.
func NormalizePath(p string) string {
	cleaned := strings.TrimSpace(p)
	if cleaned == "" {
		return ""
	}
	// filepath.Clean collapses "a/./b", "a/b/../b" and duplicate separators. On
	// Windows it also normalizes "/" to "\", which matters because agents may
	// report paths in either style.
	cleaned = filepath.Clean(cleaned)
	// Drop a leading "./" that Clean leaves in place for some inputs, so "./x"
	// and "x" are one key rather than two.
	cleaned = strings.TrimPrefix(cleaned, "."+string(filepath.Separator))
	return cleaned
}

// ClaimResult reports the outcome of a claim attempt.
type ClaimResult struct {
	// Acquired is true when the caller now holds every requested file.
	Acquired bool
	// Locks are the holds the caller owns after the call, which is every file on
	// success and the subset it could take otherwise.
	Locks []FileLock
	// Conflicts names the files that are held by someone else, and by whom. An
	// agent blocked here is told exactly what it is waiting for, so it can pick
	// different work rather than silently producing a conflicting edit.
	Conflicts []LockConflict
}

// LockConflict describes one file the caller could not take.
type LockConflict struct {
	Path    string
	Holder  string
	TaskID  string
	HeldFor time.Duration
}

// Claim takes a hold on every listed file, or on none of them.
//
// The all-or-nothing rule is deliberate. An agent holding half of a set of files
// it needs is in the worst position: it has excluded other agents from those
// files while still being unable to do its own work. Failing cleanly lets it move
// on and come back.
func (m *FileLockManager) Claim(agent, taskID string, paths []string) ClaimResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	m.expireLocked(now)

	// Dedupe first, preserving order, so a caller that lists the same file twice
	// takes one lock rather than a count it cannot reason about.
	unique := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, raw := range paths {
		key := NormalizePath(raw)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, key)
	}

	// Check every path before taking any of them. Without this pre-check a
	// partially successful claim would leave the caller holding locks it is
	// about to discard, having already excluded everyone else from those files.
	var conflicts []LockConflict
	for _, key := range unique {
		if entry, held := m.locks[key]; held {
			conflicts = append(conflicts, LockConflict{
				Path:    key,
				Holder:  entry.lock.Agent,
				TaskID:  entry.lock.TaskID,
				HeldFor: now.Sub(entry.renewedAt),
			})
		}
	}

	if len(conflicts) > 0 {
		return ClaimResult{Acquired: false, Conflicts: conflicts}
	}

	claimed := make([]FileLock, 0, len(unique))
	for _, key := range unique {
		lock := FileLock{Path: key, Agent: agent, TaskID: taskID, AcquiredAt: now}
		m.locks[key] = &lockEntry{
			lock:      lock,
			renewedAt: now,
			expiresAt: now.Add(m.ttl),
		}
		claimed = append(claimed, lock)
	}
	return ClaimResult{Acquired: true, Locks: claimed}
}

// Renew extends the lease on files the caller already holds.
//
// A long task that keeps writing the same file should not have its lock expire
// underneath it. Renewal only succeeds for files this agent actually holds: an
// expired lock belongs to nobody, and quietly re-taking it would let two agents
// believe they own the same file.
func (m *FileLockManager) Renew(agent string, paths []string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	m.expireLocked(now)

	var renewed []string
	for _, raw := range paths {
		key := NormalizePath(raw)
		entry, held := m.locks[key]
		if !held || entry.lock.Agent != agent {
			continue
		}
		entry.renewedAt = now
		entry.expiresAt = now.Add(m.ttl)
		renewed = append(renewed, key)
	}
	return renewed
}

// Release drops the caller's hold on the listed files.
//
// Releasing is scoped to the agent: a caller may only release what it holds, so a
// bug in one agent cannot unlock a file another agent is actively writing.
func (m *FileLockManager) Release(agent string, paths []string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	m.expireLocked(now)

	var released []string
	for _, raw := range paths {
		key := NormalizePath(raw)
		entry, held := m.locks[key]
		if !held || entry.lock.Agent != agent {
			continue
		}
		delete(m.locks, key)
		released = append(released, key)
	}
	return released
}

// ReleaseAll drops every hold belonging to an agent.
//
// This is the call that makes the lock short-lived in practice: an agent drops
// everything it holds as soon as it finishes, so an unrelated task is never kept
// out of a file by work that has already concluded.
func (m *FileLockManager) ReleaseAll(agent string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	m.expireLocked(now)

	var released []string
	for key, entry := range m.locks {
		if entry.lock.Agent != agent {
			continue
		}
		released = append(released, key)
		delete(m.locks, key)
	}
	return released
}

// Holder reports who currently holds a file, if anyone.
func (m *FileLockManager) Holder(path string) (FileLock, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.expireLocked(m.now())
	entry, held := m.locks[NormalizePath(path)]
	if !held {
		return FileLock{}, false
	}
	return entry.lock, true
}

// IsLocked reports whether a file is currently held.
func (m *FileLockManager) IsLocked(path string) bool {
	_, held := m.Holder(path)
	return held
}

// Active returns every live hold, for display and for the conflict report.
func (m *FileLockManager) Active() []FileLock {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.expireLocked(m.now())
	active := make([]FileLock, 0, len(m.locks))
	for _, entry := range m.locks {
		active = append(active, entry.lock)
	}
	return active
}

// expireLocked removes holds whose lease has run out.
//
// It is called from every entry point rather than from a background timer, so
// there is no goroutine to leak and no cleanup to forget when the daemon stops.
// An abandoned lock is therefore noticed the next time anything looks at it.
func (m *FileLockManager) expireLocked(now time.Time) {
	for key, entry := range m.locks {
		if now.After(entry.expiresAt) {
			delete(m.locks, key)
		}
	}
}

// DescribeConflicts renders a conflict set as a message an agent can act on.
//
// The point of naming the holder is that "file is locked" is not actionable,
// while "theme.css is held by coder on T-007" tells the agent that this work is
// temporarily spoken for and should be scheduled after it rather than forced.
func DescribeConflicts(conflicts []LockConflict) string {
	if len(conflicts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		if c.TaskID != "" {
			parts = append(parts, fmt.Sprintf("%s (held by %s on %s for %s)",
				c.Path, c.Holder, c.TaskID, c.HeldFor.Round(1e9)))
		} else {
			parts = append(parts, fmt.Sprintf("%s (held by %s for %s)",
				c.Path, c.Holder, c.HeldFor.Round(1e9)))
		}
	}
	return "these files are currently being edited by another agent: " +
		strings.Join(parts, "; ")
}
