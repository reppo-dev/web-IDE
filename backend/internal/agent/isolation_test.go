package agent

import (
	"sync"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

func TestForkCreatesIndependentContext(t *testing.T) {
	locks := workspace.NewFileLockManager()
	parent := &SwarmContext{
		ProjectID:  "p1",
		Round:      1,
		UserPrompt: "build something",
		FileLocks:  locks,
		AgentModels: map[domain.AgentRole]string{
			domain.RoleCoder: "model-a",
		},
		APIKeys: map[string]string{
			"model-a": "key-123",
		},
		UsageByAgent: map[string]*domain.AgentUsage{
			"coder": {Agent: "coder", TotalTokens: 100},
		},
		CodeFiles: []domain.CodeFile{
			{Path: "main.go", Content: "package main"},
		},
		Plan: &domain.ProjectPlan{
			Tasks: []domain.ProjectTask{
				{ID: "T-001", Title: "Setup", Files: []string{"main.go"}},
				{ID: "T-002", Title: "Styles", Files: []string{"style.css"}},
			},
		},
	}

	fork := parent.Fork(domain.RoleCoder, "T-001")
	if fork == nil {
		t.Fatal("Fork returned nil")
	}

	// 1. FileLocks must be shared
	if fork.FileLocks != parent.FileLocks {
		t.Error("FileLocks must be shared across forks for mutual exclusion")
	}

	// 2. ActiveTask must be populated for T-001
	if fork.ActiveTask == nil || fork.ActiveTask.ID != "T-001" {
		t.Fatalf("expected ActiveTask T-001, got %v", fork.ActiveTask)
	}

	// 3. Mutating fork's maps must not affect parent
	fork.AgentModels[domain.RoleCoder] = "model-b"
	if parent.AgentModels[domain.RoleCoder] != "model-a" {
		t.Error("mutating fork AgentModels affected parent")
	}

	fork.APIKeys["model-a"] = "mutated-key"
	if parent.APIKeys["model-a"] != "key-123" {
		t.Error("mutating fork APIKeys affected parent")
	}

	fork.UsageByAgent["coder"].TotalTokens = 999
	if parent.UsageByAgent["coder"].TotalTokens != 100 {
		t.Error("mutating fork UsageByAgent affected parent")
	}

	// 4. Mutating fork's files must not affect parent
	fork.CodeFiles = append(fork.CodeFiles, domain.CodeFile{Path: "fork.go", Content: "test"})
	if len(parent.CodeFiles) != 1 {
		t.Error("appending to fork CodeFiles affected parent")
	}
}

func TestFindTask(t *testing.T) {
	ctx := &SwarmContext{
		Plan: &domain.ProjectPlan{
			Tasks: []domain.ProjectTask{
				{ID: "T-001", Title: "Task 1"},
				{ID: "T-002", Title: "Task 2"},
			},
		},
	}

	found := ctx.FindTask("T-002")
	if found == nil || found.Title != "Task 2" {
		t.Fatalf("expected Task 2, got %v", found)
	}

	missing := ctx.FindTask("T-999")
	if missing != nil {
		t.Fatalf("expected nil for nonexistent task, got %v", missing)
	}

	var nilCtx *SwarmContext
	if nilCtx.FindTask("T-001") != nil {
		t.Fatal("nil SwarmContext should return nil")
	}
}

func TestMergeResult(t *testing.T) {
	parent := &SwarmContext{
		CodeFiles: []domain.CodeFile{
			{Path: "main.go", Content: "original main"},
			{Path: "shared.css", Content: "original css"},
		},
		UsageByAgent: map[string]*domain.AgentUsage{
			"architect": {Agent: "architect", TotalTokens: 50},
		},
	}

	res := ForkResult{
		Role:   domain.RoleCoder,
		TaskID: "T-001",
		HasFiles: true,
		Files: []domain.CodeFile{
			{Path: "shared.css", Content: "updated css"},
			{Path: "new.go", Content: "package new"},
		},
		Usage: map[string]*domain.AgentUsage{
			"coder": {Agent: "coder", TotalTokens: 120},
		},
		CriticReview: &domain.CriticPayload{
			GeneralReview: "looks solid",
		},
	}

	collisions := parent.MergeResult(res)
	if len(collisions) != 1 || collisions[0] != "shared.css" {
		t.Errorf("expected collision on shared.css, got %v", collisions)
	}

	// Verify files replaced / appended
	var mainContent, cssContent, newContent string
	for _, f := range parent.CodeFiles {
		switch f.Path {
		case "main.go":
			mainContent = f.Content
		case "shared.css":
			cssContent = f.Content
		case "new.go":
			newContent = f.Content
		}
	}

	if mainContent != "original main" {
		t.Errorf("main.go modified unexpectedly: %s", mainContent)
	}
	if cssContent != "updated css" {
		t.Errorf("shared.css was not updated: %s", cssContent)
	}
	if newContent != "package new" {
		t.Errorf("new.go was not appended: %s", newContent)
	}

	// Verify CriticReview and Usage merged
	if parent.CriticReview == nil || parent.CriticReview.GeneralReview != "looks solid" {
		t.Error("CriticReview not merged")
	}
	if parent.UsageByAgent["coder"] == nil || parent.UsageByAgent["coder"].TotalTokens != 120 {
		t.Error("UsageByAgent for coder not merged")
	}
}

func TestTakeUsage(t *testing.T) {
	fork := &SwarmContext{
		UsageByAgent: map[string]*domain.AgentUsage{
			"coder": {Agent: "coder", TotalTokens: 300},
		},
	}

	taken := fork.TakeUsage()
	if taken == nil || taken["coder"].TotalTokens != 300 {
		t.Fatal("TakeUsage did not return expected usage")
	}
	if fork.UsageByAgent != nil {
		t.Error("TakeUsage must clear UsageByAgent from fork")
	}
}

func TestConcurrentForksAndMerges(t *testing.T) {
	parent := &SwarmContext{
		FileLocks: workspace.NewFileLockManager(),
		CodeFiles: []domain.CodeFile{
			{Path: "base.go", Content: "package base"},
		},
		Plan: &domain.ProjectPlan{
			Tasks: []domain.ProjectTask{
				{ID: "T-001", Title: "Task 1", Files: []string{"a.go"}},
				{ID: "T-002", Title: "Task 2", Files: []string{"b.go"}},
				{ID: "T-003", Title: "Task 3", Files: []string{"c.go"}},
			},
		},
		UsageByAgent: make(map[string]*domain.AgentUsage),
	}

	var wg sync.WaitGroup
	results := make([]ForkResult, 3)

	taskIDs := []string{"T-001", "T-002", "T-003"}
	fileNames := []string{"a.go", "b.go", "c.go"}

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			fork := parent.Fork(domain.RoleCoder, taskIDs[idx])
			if fork == nil {
				return
			}
			// simulate work
			results[idx] = ForkResult{
				Role:   domain.RoleCoder,
				TaskID: taskIDs[idx],
				HasFiles: true,
				Files: []domain.CodeFile{
					{Path: fileNames[idx], Content: "package " + fileNames[idx]},
				},
				Usage: map[string]*domain.AgentUsage{
					taskIDs[idx]: {Agent: taskIDs[idx], TotalTokens: 50 * (idx + 1)},
				},
			}
		}(i)
	}

	wg.Wait()

	// Sequentially merge back
	for _, res := range results {
		parent.MergeResult(res)
	}

	if len(parent.CodeFiles) != 4 { // base.go + a.go + b.go + c.go
		t.Fatalf("expected 4 files, got %d", len(parent.CodeFiles))
	}
	if len(parent.UsageByAgent) != 3 {
		t.Fatalf("expected 3 usage entries, got %d", len(parent.UsageByAgent))
	}
}
