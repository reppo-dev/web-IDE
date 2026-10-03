package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
)

// progressWatcher replaces the old fixed round cap.
//
// The run used to stop after MaxRounds, which meant a ten-task project with the
// default cap of five could never finish. The requirement is to keep working
// until the project is genuinely complete, with no round ceiling at all.
//
// But "no ceiling" without a stuck-detector is a different bug: a swarm that
// cannot fix one file will keep re-paying for the same answer forever. So the
// loop runs unbounded while it is making progress, and stops only when three
// consecutive rounds are provably identical work.
//
// The distinction that matters is trying versus stuck. A round that keeps
// changing files, keeps closing tasks, or keeps changing its mind is slow but
// moving. A round that produces byte-identical output three times over is not
// making an attempt at all.

// progressSnapshot is the fingerprint of what one round actually produced.
type progressSnapshot struct {
	// fingerprint identifies the code and the review findings together, so two
	// rounds with the same fingerprint did the same work and learned nothing.
	fingerprint string
	// completed is how many tasks were verified done at the end of the round.
	completed int
}

// progressWatcher tracks whether the swarm is still advancing.
type progressWatcher struct {
	// stuckRounds is how many consecutive identical rounds to tolerate before
	// declaring the swarm stuck. Three matches the circuit breaker in the
	// project's own limits section.
	stuckRounds int

	history []progressSnapshot
	// lastProgressRound is the most recent round that actually changed something.
	lastProgressRound int
	// lastCompleted mirrors the highest task count reached, used to detect a
	// round that churns without ever closing a task.
	lastCompleted int
}

func newProgressWatcher(stuckRounds int) *progressWatcher {
	if stuckRounds <= 0 {
		stuckRounds = 3
	}
	return &progressWatcher{stuckRounds: stuckRounds}
}

// record folds one round's result into the watcher's state and reports whether
// the swarm is stuck.
func (w *progressWatcher) record(sCtx *agent.SwarmContext) bool {
	snap := snapshotOf(sCtx)
	w.history = append(w.history, snap)

	if len(w.history) < 2 {
		return false
	}

	prev := w.history[len(w.history)-2]

	// Progress is any of: the code changed, the review findings changed, or a
	// task moved to done. Only the exact absence of all three means no work.
	changed := snap.fingerprint != prev.fingerprint
	advanced := snap.completed > w.lastCompleted
	if advanced {
		w.lastCompleted = snap.completed
		w.lastProgressRound = len(w.history)
	}

	if changed || advanced {
		return false
	}

	// Count back how many consecutive rounds have produced the identical
	// fingerprint without closing anything.
	identical := 0
	for i := len(w.history) - 1; i >= 0; i-- {
		if w.history[i].fingerprint != snap.fingerprint {
			break
		}
		identical++
	}
	return identical >= w.stuckRounds
}

// snapshotOf fingerprints the round's real output.
func snapshotOf(sCtx *agent.SwarmContext) progressSnapshot {
	h := sha256.New()

	// File paths and contents, sorted so a reordering does not read as change.
	files := sCtx.CodeFilesSnapshot()
	paths := make([]string, 0, len(files))
	byPath := make(map[string]string, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
		byPath[f.Path] = f.Content
	}
	sort.Strings(paths)
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write([]byte(byPath[p]))
		h.Write([]byte{0})
	}

	// Open findings, so a round that changes nothing but finds a new bug still
	// counts as progress rather than as a stall.
	if sCtx.CriticReview != nil {
		for _, issue := range sCtx.CriticReview.Issues {
			h.Write([]byte(issue.Description))
			h.Write([]byte{0})
		}
	}
	if sCtx.SecurityReport != nil {
		for _, v := range sCtx.SecurityReport.Vulnerabilities {
			h.Write([]byte(v.Description))
			h.Write([]byte{0})
		}
	}

	// Completed tasks are counted directly rather than derived from a
	// percentage, so a long plan does not lose a task to rounding.
	completed := 0
	if plan := sCtx.PlanSnapshot(); plan != nil {
		for _, t := range plan.Tasks {
			if t.State == domain.TaskDone {
				completed++
			}
		}
	}

	return progressSnapshot{
		fingerprint: hex.EncodeToString(h.Sum(nil)),
		completed:   completed,
	}
}

// describeStall renders the reason a run ended, so the user is told what to fix
// rather than just seeing the loop stop.
func describeStall(task *domain.ProjectTask, rounds int) string {
	var sb strings.Builder
	sb.WriteString("The swarm stopped making progress")
	if task != nil {
		sb.WriteString(" on task " + task.ID + " (" + task.Title + ")")
	}
	sb.WriteString(" after " + strconv.Itoa(rounds) + " consecutive rounds with identical results.\n")
	sb.WriteString("This is different from running out of attempts: the last rounds produced the same code and the same findings, so repeating them would not change anything.")
	if task != nil && task.Acceptance != "" {
		sb.WriteString("\nThe task's condition is: " + task.Acceptance)
	}
	return sb.String()
}