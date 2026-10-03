package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

type verifierMockLLM struct {
	response string
	err      error
	called   bool
}

func (m *verifierMockLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return &llm.Response{Content: m.response}, nil
}

func TestTaskVerifier_NilActiveTask(t *testing.T) {
	mock := &verifierMockLLM{}
	v := NewTaskVerifier(mock)
	sCtx := &SwarmContext{}

	verdict, err := v.Verify(context.Background(), sCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != nil {
		t.Fatalf("expected nil verdict for empty task, got %+v", verdict)
	}
	if mock.called {
		t.Errorf("LLM should not be called when no active task")
	}
}

func TestTaskVerifier_MissingFilesFailsMechanicalGate(t *testing.T) {
	mock := &verifierMockLLM{}
	v := NewTaskVerifier(mock)

	sCtx := &SwarmContext{}
	task := domain.ProjectTask{
		ID:         "T-001",
		Title:      "Create main.go and router.go",
		Acceptance: "Entry point exists",
		Files:      []string{"main.go", "router.go"},
	}
	sCtx.SetActiveTask(&task)

	verdict, err := v.Verify(context.Background(), sCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict == nil {
		t.Fatalf("expected non-nil verdict")
	}
	if verdict.Passed {
		t.Errorf("verdict should not pass when files are missing")
	}
	if len(verdict.Gaps) == 0 {
		t.Errorf("expected gaps for missing files, got none")
	}
	if mock.called {
		t.Errorf("mechanical gate should fail BEFORE calling LLM")
	}
}

func TestTaskVerifier_TestRunnerFailure(t *testing.T) {
	mock := &verifierMockLLM{}
	v := NewTaskVerifier(mock)
	v.SetTestRunner(func(root string) (string, error) {
		return "--- FAIL: TestSomething (0.01s)", nil
	})

	sCtx := &SwarmContext{
		WorkspacePath: t.TempDir(),
	}
	task := domain.ProjectTask{
		ID:         "T-001",
		Title:      "Pass tests",
		Acceptance: "All unit tests pass",
		Files:      []string{"main.go"},
	}
	sCtx.SetActiveTask(&task)
	sCtx.SetCodeFiles([]domain.CodeFile{
		{Path: "main.go", Content: "package main\nfunc main() {}"},
	})

	verdict, err := v.Verify(context.Background(), sCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict == nil || verdict.Passed {
		t.Errorf("expected failed verdict due to test suite failures")
	}
	if len(verdict.Gaps) == 0 {
		t.Errorf("expected gaps for failing tests")
	}
	if mock.called {
		t.Errorf("test failure should abort before LLM call")
	}
}

func TestTaskVerifier_TestRunnerError(t *testing.T) {
	mock := &verifierMockLLM{}
	v := NewTaskVerifier(mock)
	v.SetTestRunner(func(root string) (string, error) {
		return "", errors.New("command not found: go")
	})

	sCtx := &SwarmContext{
		WorkspacePath: t.TempDir(),
	}
	task := domain.ProjectTask{
		ID:    "T-001",
		Files: []string{"main.go"},
	}
	sCtx.SetActiveTask(&task)
	sCtx.SetCodeFiles([]domain.CodeFile{
		{Path: "main.go", Content: "package main\nfunc main() {}"},
	})

	verdict, err := v.Verify(context.Background(), sCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict == nil || verdict.Passed {
		t.Errorf("expected failed verdict when test runner errors")
	}
}

func TestTaskVerifier_AcceptanceGaps(t *testing.T) {
	llmJSON := `{
		"gaps": ["main.go contains panic(\"not implemented\")"],
		"summary": "Task is incomplete because required handler is a stub."
	}`
	mock := &verifierMockLLM{response: llmJSON}
	v := NewTaskVerifier(mock)

	sCtx := &SwarmContext{
		AgentModels: map[domain.AgentRole]string{domain.RoleCritic: "openrouter/free"},
		APIKeys:     map[string]string{"openrouter": "test-key"},
	}
	task := domain.ProjectTask{
		ID:         "T-002",
		Title:      "Implement Login Handler",
		Acceptance: "Login handler returns 200 OK",
		Files:      []string{"login.go"},
	}
	sCtx.SetActiveTask(&task)
	sCtx.SetCodeFiles([]domain.CodeFile{
		{Path: "login.go", Content: "package main\nfunc login() { panic(\"not implemented\") }"},
	})

	verdict, err := v.Verify(context.Background(), sCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict == nil {
		t.Fatalf("expected verdict")
	}
	if verdict.Passed {
		t.Errorf("verdict should fail when model returns gaps")
	}
	if len(verdict.Gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(verdict.Gaps))
	}
	if verdict.Gaps[0] != "main.go contains panic(\"not implemented\")" {
		t.Errorf("unexpected gap: %s", verdict.Gaps[0])
	}
}

func TestTaskVerifier_AcceptancePassed(t *testing.T) {
	llmJSON := `{
		"gaps": [],
		"summary": "Task T-003 is complete; full validation logic is implemented."
	}`
	mock := &verifierMockLLM{response: llmJSON}
	v := NewTaskVerifier(mock)

	sCtx := &SwarmContext{
		AgentModels: map[domain.AgentRole]string{domain.RoleCritic: "openrouter/free"},
		APIKeys:     map[string]string{"openrouter": "test-key"},
	}
	task := domain.ProjectTask{
		ID:         "T-003",
		Title:      "Implement Validator",
		Acceptance: "Validate function checks email regex",
		Files:      []string{"validator.go"},
	}
	sCtx.SetActiveTask(&task)
	sCtx.SetCodeFiles([]domain.CodeFile{
		{Path: "validator.go", Content: "package main\nfunc Validate(email string) bool { return true }"},
	})

	verdict, err := v.Verify(context.Background(), sCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict == nil {
		t.Fatalf("expected verdict")
	}
	if !verdict.Passed {
		t.Errorf("verdict should pass when gaps are empty")
	}
	if len(verdict.Gaps) != 0 {
		t.Errorf("expected 0 gaps, got %d", len(verdict.Gaps))
	}
}

func TestTaskVerifier_NilClientFallback(t *testing.T) {
	v := NewTaskVerifier(nil) // nil client

	sCtx := &SwarmContext{}
	task := domain.ProjectTask{
		ID:         "T-004",
		Title:      "Offline check",
		Acceptance: "File exists",
		Files:      []string{"README.md"},
	}
	sCtx.SetActiveTask(&task)
	sCtx.SetCodeFiles([]domain.CodeFile{
		{Path: "README.md", Content: "# Project"},
	})

	verdict, err := v.Verify(context.Background(), sCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict == nil {
		t.Fatalf("expected verdict")
	}
	// With nil client, mechanical checks pass and fallback summary is set
	if !verdict.Passed {
		t.Errorf("mechanical checks passed, so nil client fallback should pass")
	}
}
