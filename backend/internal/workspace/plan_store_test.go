package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestPlanStore_WriteAndReadPlan(t *testing.T) {
	tmpDir := t.TempDir()
	brain, err := NewBrainFS(tmpDir)
	if err != nil {
		t.Fatalf("failed to init brain: %v", err)
	}

	// 1. Missing plan returns nil without error
	p, err := brain.ReadPlan()
	if err != nil {
		t.Fatalf("reading missing plan returned error: %v", err)
	}
	if p != nil {
		t.Fatalf("expected nil plan for fresh brain, got %+v", p)
	}

	// 2. Write plan atomically
	plan := &domain.ProjectPlan{
		Goal: "Build Authentication Microservice",
		Tasks: []domain.ProjectTask{
			{
				ID:         "T-001",
				Title:      "Create User Model",
				Acceptance: "user.go exists with PasswordHash",
				State:      domain.TaskDone,
				Files:      []string{"user.go"},
				Round:      1,
			},
			{
				ID:         "T-002",
				Title:      "Implement JWT Generator",
				Acceptance: "jwt.go has GenerateToken and validates claims",
				State:      domain.TaskInProgress,
				Files:      []string{"jwt.go"},
				Round:      2,
			},
			{
				ID:         "T-003",
				Title:      "Add Auth Middleware",
				Acceptance: "middleware.go intercepts requests",
				State:      domain.TaskTodo,
			},
		},
	}

	if err := brain.WritePlan(plan); err != nil {
		t.Fatalf("failed to write plan: %v", err)
	}

	// 3. Read it back
	loaded, err := brain.ReadPlan()
	if err != nil {
		t.Fatalf("failed to read back plan: %v", err)
	}
	if loaded == nil {
		t.Fatalf("expected plan, got nil")
	}
	if loaded.Goal != plan.Goal {
		t.Errorf("expected goal %q, got %q", plan.Goal, loaded.Goal)
	}
	if len(loaded.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(loaded.Tasks))
	}

	done, total, percent := loaded.Progress()
	if done != 1 || total != 3 {
		t.Errorf("expected 1 of 3 tasks done, got %d of %d", done, total)
	}
	if percent < 33.0 || percent > 34.0 {
		t.Errorf("expected ~33.3%% progress, got %.2f%%", percent)
	}
}

func TestPlanStore_RenderTasks(t *testing.T) {
	tmpDir := t.TempDir()
	brain, err := NewBrainFS(tmpDir)
	if err != nil {
		t.Fatalf("failed to init brain: %v", err)
	}

	plan := &domain.ProjectPlan{
		Goal: "Build Go Ping API",
		Tasks: []domain.ProjectTask{
			{
				ID:         "T-001",
				Title:      "Setup Router",
				Acceptance: "router.go compiles",
				State:      domain.TaskDone,
				Files:      []string{"router.go"},
				Round:      1,
			},
			{
				ID:         "T-002",
				Title:      "Write Ping Handler",
				Acceptance: "handler.go returns 200 OK",
				State:      domain.TaskTodo,
			},
		},
	}

	if err := brain.RenderTasks(plan); err != nil {
		t.Fatalf("failed to render tasks: %v", err)
	}

	tasksPath := filepath.Join(tmpDir, ProjectBrainDir, FileTasks)
	contentBytes, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read rendered tasks.md: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, "# Project Tasks") {
		t.Errorf("expected header '# Project Tasks' in rendered markdown")
	}
	if !strings.Contains(content, "Build Go Ping API") {
		t.Errorf("expected goal in rendered markdown")
	}
	if !strings.Contains(content, "[DONE]") || !strings.Contains(content, "**T-001** Setup Router") {
		t.Errorf("expected task T-001 with state [DONE] in rendered markdown")
	}
	if !strings.Contains(content, "[TODO]") || !strings.Contains(content, "**T-002** Write Ping Handler") {
		t.Errorf("expected task T-002 with state [TODO] in rendered markdown")
	}
}

func TestPlanStore_ConvenienceWrappers(t *testing.T) {
	tmpDir := t.TempDir()
	plan := &domain.ProjectPlan{
		Goal: "Test Convenience",
		Tasks: []domain.ProjectTask{
			{ID: "T-001", Title: "Unit Test", State: domain.TaskDone},
		},
	}

	if err := SavePlan(tmpDir, plan); err != nil {
		t.Fatalf("SavePlan failed: %v", err)
	}

	loaded, err := LoadPlan(tmpDir)
	if err != nil {
		t.Fatalf("LoadPlan failed: %v", err)
	}
	if loaded == nil || len(loaded.Tasks) != 1 {
		t.Fatalf("expected 1 loaded task, got %+v", loaded)
	}
	if loaded.Tasks[0].Title != "Unit Test" {
		t.Errorf("expected 'Unit Test', got %q", loaded.Tasks[0].Title)
	}
}

// TestArchivePlanKeepsTheOldBoard proves archiving really preserves the previous
// plan. Writing the superseded plan back to plan.json destroyed it within one
// round, because the new plan is persisted to that same file immediately after.
func TestArchivePlanKeepsTheOldBoard(t *testing.T) {
	tmpDir := t.TempDir()
	brain, err := NewBrainFS(tmpDir)
	if err != nil {
		t.Fatalf("NewBrainFS: %v", err)
	}

	old := &domain.ProjectPlan{
		Goal: "Build the original thing",
		Tasks: []domain.ProjectTask{
			{ID: "T-001", Title: "old work", State: domain.TaskDone},
		},
	}
	path, err := brain.ArchivePlan(old, "a different request was made")
	if err != nil {
		t.Fatalf("ArchivePlan: %v", err)
	}
	if path == "" {
		t.Fatal("ArchivePlan returned no path")
	}

	// The archive is a distinct file, so the live plan is untouched by it.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("archive file missing: %v", err)
	}
	live, err := brain.ReadPlan()
	if err != nil {
		t.Fatalf("ReadPlan: %v", err)
	}
	if live != nil {
		t.Errorf("archiving must not touch the live plan, got %+v", live)
	}

	// And the content really is the old plan.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if !strings.Contains(string(data), "Build the original thing") {
		t.Errorf("archive does not contain the old goal:\n%s", data)
	}
	if !strings.Contains(string(data), "a different request was made") {
		t.Errorf("archive does not record why it was superseded:\n%s", data)
	}
}
