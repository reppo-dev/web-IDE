package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

func setupTestProject(t *testing.T) (string, *workspace.Registry) {
	t.Helper()
	dataDir := t.TempDir()
	reg, err := workspace.NewRegistry(dataDir)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	projDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("failed to create main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "README.md"), []byte("# Test Project\n"), 0o644); err != nil {
		t.Fatalf("failed to create README.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "app.bin"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatalf("failed to create app.bin: %v", err)
	}

	return projDir, reg
}

func TestProjectsHandler_ListAndOpen(t *testing.T) {
	projDir, reg := setupTestProject(t)
	handler := NewProjectHandler(reg)

	// 1. Initial list should be empty
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handler.HandleProjects(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var list []workspace.ProjectRef
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("failed to decode list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 projects initially, got %d", len(list))
	}

	// 2. Open project
	payload, _ := json.Marshal(map[string]string{"path": projDir})
	req = httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(payload))
	rec = httptest.NewRecorder()
	handler.HandleProjects(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on open, got %d: %s", rec.Code, rec.Body.String())
	}
	var opened workspace.ProjectRef
	if err := json.NewDecoder(rec.Body).Decode(&opened); err != nil {
		t.Fatalf("failed to decode opened project: %v", err)
	}
	if opened.ID == "" {
		t.Fatal("expected non-empty project ID")
	}

	// 3. Tree endpoint
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+opened.ID, nil)
	rec = httptest.NewRecorder()
	handler.HandleProjects(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tree, got %d", rec.Code)
	}
	var tree struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Files []struct {
			Path string `json:"path"`
			Size int    `json:"size"`
		} `json:"files"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&tree); err != nil {
		t.Fatalf("failed to decode tree: %v", err)
	}
	if len(tree.Files) == 0 {
		t.Fatal("expected files in tree")
	}

	// 4. Read source file
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+opened.ID+"/file?path=main.go", nil)
	rec = httptest.NewRecorder()
	handler.HandleProjects(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for file read, got %d", rec.Code)
	}
	var fileResp struct {
		Path     string `json:"path"`
		Language string `json:"language"`
		Content  string `json:"content"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&fileResp); err != nil {
		t.Fatalf("failed to decode file response: %v", err)
	}
	if fileResp.Language != "go" {
		t.Errorf("expected language 'go', got %s", fileResp.Language)
	}

	// 5. Binary file should be forbidden
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+opened.ID+"/file?path=app.bin", nil)
	rec = httptest.NewRecorder()
	handler.HandleProjects(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for binary file, got %d", rec.Code)
	}

	// 6. Path traversal attempt should fail
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+opened.ID+"/file?path=../../etc/passwd", nil)
	rec = httptest.NewRecorder()
	handler.HandleProjects(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Errorf("expected 400 or 404 for path traversal, got %d", rec.Code)
	}
}

// A Windows-style traversal must be rejected just like a POSIX one, otherwise
// a project opened from C:\ could reach the rest of the drive.
func TestProjectsHandler_WindowsTraversal(t *testing.T) {
	projDir, reg := setupTestProject(t)
	handler := NewProjectHandler(reg)

	payload, _ := json.Marshal(map[string]string{"path": projDir})
	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.HandleProjects(rec, req)

	var opened workspace.ProjectRef
	if err := json.NewDecoder(rec.Body).Decode(&opened); err != nil {
		t.Fatal(err)
	}

	for _, attempt := range []string{
		`..\..\Windows\System32\drivers\etc\hosts`,
		`..%5C..%5Csecret.txt`,
	} {
		req = httptest.NewRequest(http.MethodGet,
			"/api/projects/"+opened.ID+"/file?path="+attempt, nil)
		rec = httptest.NewRecorder()
		handler.HandleProjects(rec, req)

		if rec.Code == http.StatusOK {
			t.Errorf("windows traversal %q was allowed", attempt)
		}
	}
}

func TestProjectsHandler_RejectsFileAsProject(t *testing.T) {
	projDir, reg := setupTestProject(t)
	handler := NewProjectHandler(reg)

	// Pointing the project at a file rather than a folder must fail.
	payload, _ := json.Marshal(map[string]string{
		"path": filepath.Join(projDir, "main.go"),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.HandleProjects(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when opening a file as a project, got %d", rec.Code)
	}
}

func TestProjectsHandler_RejectsMissingFolder(t *testing.T) {
	_, reg := setupTestProject(t)
	handler := NewProjectHandler(reg)

	payload, _ := json.Marshal(map[string]string{
		"path": filepath.Join(t.TempDir(), "does-not-exist"),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.HandleProjects(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a missing folder, got %d", rec.Code)
	}
}

// Reopening the same folder must reuse its id, otherwise the project would
// lose its memory on every open.
func TestProjectsHandler_SameFolderKeepsID(t *testing.T) {
	projDir, reg := setupTestProject(t)
	handler := NewProjectHandler(reg)

	var ids []string
	for i := 0; i < 2; i++ {
		payload, _ := json.Marshal(map[string]string{"path": projDir})
		req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		handler.HandleProjects(rec, req)

		var ref workspace.ProjectRef
		if err := json.NewDecoder(rec.Body).Decode(&ref); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, ref.ID)
	}

	if ids[0] != ids[1] {
		t.Errorf("same folder produced different ids: %q vs %q", ids[0], ids[1])
	}
}

func TestProjectsHandler_NilRegistry(t *testing.T) {
	handler := NewProjectHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handler.HandleProjects(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 without a registry, got %d", rec.Code)
	}
}

func TestProjectsHandler_PlanEndpoint(t *testing.T) {
	projDir, reg := setupTestProject(t)
	handler := NewProjectHandler(reg)

	// Register project
	ref, err := reg.Register(projDir)
	if err != nil {
		t.Fatalf("failed to register project: %v", err)
	}

	// 1. Plan when none exists should return 200 with empty structure
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+ref.ID+"/plan", nil)
	rec := httptest.NewRecorder()
	handler.HandleProjects(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty plan, got %d: %s", rec.Code, rec.Body.String())
	}
	var emptyPlan domain.ProjectPlan
	if err := json.NewDecoder(rec.Body).Decode(&emptyPlan); err != nil {
		t.Fatalf("failed to decode empty plan: %v", err)
	}
	if len(emptyPlan.Tasks) != 0 {
		t.Errorf("expected 0 tasks in empty plan, got %d", len(emptyPlan.Tasks))
	}

	// 2. Write a real plan via workspace.SavePlan
	saved := &domain.ProjectPlan{
		Goal: "Build Authentication System",
		Tasks: []domain.ProjectTask{
			{ID: "T-001", Title: "Setup JWT", State: domain.TaskDone},
			{ID: "T-002", Title: "Login Endpoint", State: domain.TaskInProgress},
		},
	}
	if err := workspace.SavePlan(ref.Root, saved); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// 3. Query plan endpoint again
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+ref.ID+"/plan", nil)
	rec = httptest.NewRecorder()
	handler.HandleProjects(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for saved plan, got %d", rec.Code)
	}
	var loadedPlan domain.ProjectPlan
	if err := json.NewDecoder(rec.Body).Decode(&loadedPlan); err != nil {
		t.Fatalf("failed to decode saved plan: %v", err)
	}
	if loadedPlan.Goal != "Build Authentication System" {
		t.Errorf("expected goal 'Build Authentication System', got %q", loadedPlan.Goal)
	}
	if len(loadedPlan.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(loadedPlan.Tasks))
	}
	if loadedPlan.Tasks[0].State != domain.TaskDone || loadedPlan.Tasks[1].State != domain.TaskInProgress {
		t.Errorf("unexpected task states: %+v", loadedPlan.Tasks)
	}
}

