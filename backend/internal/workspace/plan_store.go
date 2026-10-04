package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

// ReadPlan loads the persisted project plan.
//
// A missing file is not an error: it means this project has never been planned,
// which is different from a plan that exists but is complete. Callers get a nil
// plan for the first case so they can tell "no work list yet" from "empty work
// list" and build one instead of declaring the project finished.
func (b *BrainFS) ReadPlan() (*domain.ProjectPlan, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(b.dir, FilePlan))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", FilePlan, err)
	}

	var plan domain.ProjectPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", FilePlan, err)
	}
	return &plan, nil
}

// WritePlan persists the project's work list.
//
// The write is atomic: a half-written plan.json would make the next round think
// the project has no plan at all, which is exactly the failure this file exists
// to prevent.
func (b *BrainFS) WritePlan(plan *domain.ProjectPlan) error {
	if plan == nil {
		return fmt.Errorf("cannot write a nil plan")
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode plan: %w", err)
	}

	target := filepath.Join(b.dir, FilePlan)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", FilePlan, err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to commit %s: %w", FilePlan, err)
	}
	return nil
}

// SavePlan is a convenience wrapper used by agents that hold their own lock.
func SavePlan(root string, plan *domain.ProjectPlan) error {
	brain, err := NewBrainFS(root)
	if err != nil {
		return err
	}
	return brain.WritePlan(plan)
}

// LoadPlan is a convenience wrapper for reading a project's work list.
func LoadPlan(root string) (*domain.ProjectPlan, error) {
	brain, err := NewBrainFS(root)
	if err != nil {
		return nil, err
	}
	return brain.ReadPlan()
}

// RenderTasks writes the human-readable task board into tasks.md so the state a
// developer sees on disk matches the plan the agents are working from.
func (b *BrainFS) RenderTasks(plan *domain.ProjectPlan) error {
	if plan == nil {
		return fmt.Errorf("cannot render a nil plan")
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# Project Tasks\n")
	sb.WriteString("> The work list that must be finished for the project to be complete.\n")
	sb.WriteString("> States: TODO | IN_PROGRESS | DONE | BLOCKED | REWORK | WAITING_FOR_HUMAN\n\n")
	sb.WriteString(fmt.Sprintf("**Goal:** %s\n\n", plan.Goal))

	done, total, percent := plan.Progress()
	sb.WriteString(fmt.Sprintf("**Progress:** %d/%d tasks done (%.0f%%)\n\n",
		done, total, percent))

	for _, t := range plan.Tasks {
		sb.WriteString(fmt.Sprintf("- [%s] **%s** %s", t.State, t.ID, t.Title))
		if t.Acceptance != "" {
			sb.WriteString(fmt.Sprintf("\n  - Done when: %s", t.Acceptance))
		}
		if len(t.Files) > 0 {
			sb.WriteString(fmt.Sprintf("\n  - Files: %s", strings.Join(t.Files, ", ")))
		}
		// A task handed back to a human has to say why, or the next person has
		// no way to tell which of the failures were the same one.
		if t.LastError != "" {
			sb.WriteString(fmt.Sprintf("\n  - Needs attention: %s", t.LastError))
		}
		if t.ReworkCount > 0 {
			sb.WriteString(fmt.Sprintf("\n  - Attempts: %d", t.ReworkCount+1))
		}
		sb.WriteString("\n")
	}

	// Anything still open is listed separately, because that list is the whole
	// point of stopping: it is what the user has to decide about.
	if open := plan.Outstanding(); len(open) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Still open (%d)\n\n", len(open)))
		sb.WriteString("The team finished everything it could on its own. These need a decision:\n\n")
		for _, t := range open {
			sb.WriteString(fmt.Sprintf("- **%s** %s — %s\n", t.ID, t.Title, t.State))
		}
	}

	return os.WriteFile(filepath.Join(b.dir, FileTasks), []byte(sb.String()), 0644)
}