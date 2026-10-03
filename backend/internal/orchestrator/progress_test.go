package orchestrator

import (
	"testing"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestProgressWatcher_AdvancesWithChanges(t *testing.T) {
	watcher := newProgressWatcher(3)
	sCtx := &agent.SwarmContext{
		Round: 1,
	}

	// Round 1: first file
	sCtx.SetCodeFiles([]domain.CodeFile{
		{Path: "main.go", Content: "package main\n"},
	})
	if watcher.record(sCtx) {
		t.Errorf("round 1 should not be stuck")
	}

	// Round 2: code changes
	sCtx.Round = 2
	sCtx.SetCodeFiles([]domain.CodeFile{
		{Path: "main.go", Content: "package main\nfunc main() {}\n"},
	})
	if watcher.record(sCtx) {
		t.Errorf("round 2 with changed code should not be stuck")
	}

	// Round 3: add test file
	sCtx.Round = 3
	sCtx.SetCodeFiles([]domain.CodeFile{
		{Path: "main.go", Content: "package main\nfunc main() {}\n"},
		{Path: "main_test.go", Content: "package main\n"},
	})
	if watcher.record(sCtx) {
		t.Errorf("round 3 with new file should not be stuck")
	}
}

func TestProgressWatcher_DetectsStall(t *testing.T) {
	watcher := newProgressWatcher(3)
	sCtx := &agent.SwarmContext{
		Round: 1,
	}
	sCtx.SetCodeFiles([]domain.CodeFile{
		{Path: "main.go", Content: "package main\nfunc main() {}\n"},
	})

	// Round 1: baseline
	if watcher.record(sCtx) {
		t.Errorf("round 1 cannot be stuck")
	}

	// Round 2: 2nd identical
	sCtx.Round = 2
	if watcher.record(sCtx) {
		t.Errorf("round 2 should not trigger stall yet (2 of 3)")
	}

	// Round 3: 3rd identical (threshold 3 reached)
	sCtx.Round = 3
	if !watcher.record(sCtx) {
		t.Errorf("round 3 with 3 consecutive identical rounds MUST be flagged as stuck")
	}
}

func TestProgressWatcher_NewFindingsPreventStall(t *testing.T) {
	watcher := newProgressWatcher(3)
	sCtx := &agent.SwarmContext{
		Round: 1,
	}
	sCtx.SetCodeFiles([]domain.CodeFile{
		{Path: "main.go", Content: "package main\n"},
	})

	_ = watcher.record(sCtx)

	// Round 2: same code, but critic finds new issue
	sCtx.Round = 2
	sCtx.CriticReview = &domain.CriticPayload{
		Issues: []domain.ReviewIssue{
			{Severity: "major", Description: "missing error check"},
		},
	}
	if watcher.record(sCtx) {
		t.Errorf("new critic findings must count as progress")
	}

	// Round 3: same code and findings
	sCtx.Round = 3
	if watcher.record(sCtx) {
		t.Errorf("1 duplicate round should not be stuck")
	}
}

func TestProgressWatcher_TaskCompletionPreventsStall(t *testing.T) {
	watcher := newProgressWatcher(3)
	sCtx := &agent.SwarmContext{
		Round: 1,
	}
	sCtx.SetCodeFiles([]domain.CodeFile{
		{Path: "main.go", Content: "package main\n"},
	})
	sCtx.SetPlan(&domain.ProjectPlan{
		Goal: "Test Task Progress",
		Tasks: []domain.ProjectTask{
			{ID: "T-001", Title: "Setup", State: domain.TaskInProgress},
			{ID: "T-002", Title: "Build", State: domain.TaskTodo},
		},
	})

	_ = watcher.record(sCtx)

	// Round 2: code is the same, but T-001 moved to DONE
	sCtx.Round = 2
	sCtx.SetPlan(&domain.ProjectPlan{
		Goal: "Test Task Progress",
		Tasks: []domain.ProjectTask{
			{ID: "T-001", Title: "Setup", State: domain.TaskDone},
			{ID: "T-002", Title: "Build", State: domain.TaskInProgress},
		},
	})

	if watcher.record(sCtx) {
		t.Errorf("task completion must count as advancement even if code didn't change")
	}
}
