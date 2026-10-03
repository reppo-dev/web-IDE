package agent

import (
	"context"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

func TestPlanner_NormalizePlan(t *testing.T) {
	raw := domain.ProjectPlan{
		Goal: "  Build User Service  ",
		Tasks: []domain.ProjectTask{
			{ID: "", Title: "   Setup DB   ", State: domain.TaskDone},
			{ID: "", Title: "   ", State: domain.TaskTodo}, // empty title should be skipped
			{ID: "T-001", Title: "Duplicate ID 1"},
			{ID: "T-001", Title: "Duplicate ID 2"}, // should collide and get suffix
		},
	}

	norm := normalizePlan(raw)
	if norm.Goal != "Build User Service" {
		t.Errorf("expected trimmed goal, got %q", norm.Goal)
	}
	if len(norm.Tasks) != 3 {
		t.Fatalf("expected 3 tasks (empty title skipped), got %d", len(norm.Tasks))
	}

	// Task 1 got generated ID
	if norm.Tasks[0].ID != "T-001" || norm.Tasks[0].Title != "Setup DB" {
		t.Errorf("task 1 unexpected: %+v", norm.Tasks[0])
	}
	if norm.Tasks[0].State != domain.TaskTodo {
		t.Errorf("task state should be normalized to TaskTodo, got %s", norm.Tasks[0].State)
	}

	// Collision handling
	if norm.Tasks[1].ID == norm.Tasks[2].ID {
		t.Errorf("task IDs must be unique, got collision %s and %s", norm.Tasks[1].ID, norm.Tasks[2].ID)
	}
}

func TestPlanner_ValidatePlan(t *testing.T) {
	// Empty tasks
	err := validatePlan(domain.ProjectPlan{Goal: "Valid Goal", Tasks: nil})
	if err == nil {
		t.Errorf("expected error for empty tasks")
	}

	// Empty goal
	err = validatePlan(domain.ProjectPlan{Goal: "", Tasks: []domain.ProjectTask{{ID: "1", Title: "Do X"}}})
	if err == nil {
		t.Errorf("expected error for empty goal")
	}

	// Valid
	err = validatePlan(domain.ProjectPlan{Goal: "Valid Goal", Tasks: []domain.ProjectTask{{ID: "1", Title: "Do X"}}})
	if err != nil {
		t.Errorf("unexpected error for valid plan: %v", err)
	}
}

func TestPlanner_DerivePlanFromFiles(t *testing.T) {
	fileTree := []string{"main.go", "handlers/user.go", "models/user.go"}
	existing := []domain.CodeFile{
		{Path: "main.go", Content: "package main"},
	}

	plan := DerivePlanFromFiles("Create Backend API", fileTree, existing, 1)
	if plan == nil {
		t.Fatalf("expected derived plan, got nil")
	}
	if plan.Goal != "Create Backend API" {
		t.Errorf("expected goal, got %q", plan.Goal)
	}
	// main.go already exists, so only 2 tasks should be created
	if len(plan.Tasks) != 2 {
		t.Fatalf("expected 2 tasks for non-existing files, got %d", len(plan.Tasks))
	}
	if plan.Tasks[0].Title != "Create handlers/user.go" {
		t.Errorf("expected first task for handlers/user.go, got %q", plan.Tasks[0].Title)
	}
	if plan.Tasks[1].Title != "Create models/user.go" {
		t.Errorf("expected second task for models/user.go, got %q", plan.Tasks[1].Title)
	}
}

type plannerMockLLM struct {
	response string
}

func (m *plannerMockLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	return &llm.Response{
		Content: m.response,
	}, nil
}

func TestPlanner_BuildPlan(t *testing.T) {
	jsonPlan := `
	{
		"goal": "Build Currency Converter",
		"tasks": [
			{"id": "T-001", "title": "Create Rates Service", "acceptance": "rates.go compiles"},
			{"id": "T-002", "title": "Create CLI Interface", "acceptance": "main.go parses args"}
		]
	}`

	mock := &plannerMockLLM{response: "```json\n" + jsonPlan + "\n```"}
	arch := NewArchitectAgent(mock)

	sCtx := &SwarmContext{
		UserPrompt:  "Build a currency converter CLI",
		AgentModels: map[domain.AgentRole]string{domain.RoleArchitect: "openrouter/free"},
		APIKeys:     map[string]string{"openrouter": "dummy-key"},
	}

	plan, err := arch.BuildPlan(context.Background(), sCtx)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if plan == nil {
		t.Fatalf("expected plan, got nil")
	}
	if plan.Goal != "Build Currency Converter" {
		t.Errorf("expected goal 'Build Currency Converter', got %q", plan.Goal)
	}
	if len(plan.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(plan.Tasks))
	}
}
