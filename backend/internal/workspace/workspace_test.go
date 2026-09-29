package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestWorkspace_SyncAndCommit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nexus-ws-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ws, err := NewWorkspace(tempDir, "test-project")
	if err != nil {
		t.Fatalf("failed to init workspace: %v", err)
	}

	files := []domain.CodeFile{
		{
			Path:     "main.go",
			Language: "go",
			Content:  "package main\n\nfunc main() {}\n",
		},
		{
			Path:     "pkg/calc.go",
			Language: "go",
			Content:  "package pkg\n\nfunc Add(a, b int) int { return a + b }\n",
		},
	}

	// 1. Sync files to disk
	synced, err := ws.SyncFiles(files)
	if err != nil {
		t.Fatalf("sync files failed: %v", err)
	}
	if len(synced) != 2 {
		t.Fatalf("expected 2 synced files, got %d", len(synced))
	}

	// Verify file exists on disk
	mainFile := filepath.Join(ws.Path(), "main.go")
	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		t.Errorf("main.go not created on disk: %s", mainFile)
	}

	// 2. Commit round to Git
	commitHash, err := ws.CommitRound(context.Background(), 1, 88.5, "Initial generation")
	if err != nil {
		t.Fatalf("commit round failed: %v", err)
	}
	if commitHash == "" {
		t.Errorf("expected non-empty commit hash")
	}

	// 3. Update file and verify diff computation
	updatedFiles := []domain.CodeFile{
		{
			Path:     "main.go",
			Language: "go",
			Content:  "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n",
		},
	}
	synced2, err := ws.SyncFiles(updatedFiles)
	if err != nil {
		t.Fatalf("sync updated files failed: %v", err)
	}
	if synced2[0].Diff == "" {
		t.Errorf("expected diff to be generated for modified file")
	}
}
