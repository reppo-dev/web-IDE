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

	// 4. Test RollbackUncommitted (discards uncommitted modifications)
	brokenFiles := []domain.CodeFile{
		{
			Path:     "broken.go",
			Language: "go",
			Content:  "package main\n\nfunc broken() {}\n",
		},
	}
	_, err = ws.SyncFiles(brokenFiles)
	if err != nil {
		t.Fatalf("sync broken files failed: %v", err)
	}
	if err := ws.RollbackUncommitted(context.Background()); err != nil {
		t.Fatalf("rollback uncommitted failed: %v", err)
	}

	// 5. Test RollbackToCheckpoint (resets to initial commit)
	if err := ws.RollbackToCheckpoint(context.Background(), commitHash); err != nil {
		t.Fatalf("rollback to checkpoint failed: %v", err)
	}
}

func TestWorkspace_CheckpointDedicatedRefAndAcceptChanges(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nexus-ws-chk-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ws, err := NewWorkspace(tempDir, "checkpoint-proj")
	if err != nil {
		t.Fatalf("failed to init workspace: %v", err)
	}

	ctx := context.Background()

	// 1. Initial file
	files := []domain.CodeFile{
		{
			Path:     "app.go",
			Language: "go",
			Content:  "package app\n\nconst Version = \"1.0.0\"\n",
		},
	}
	if _, err := ws.SyncFiles(files); err != nil {
		t.Fatalf("sync files failed: %v", err)
	}

	// 2. Commit round creates checkpoint on refs/nexusai/checkpoints
	hash1, err := ws.CommitRound(ctx, 1, 88.0, "Round 1 checkpoint")
	if err != nil {
		t.Fatalf("commit round 1 failed: %v", err)
	}
	if hash1 == "" {
		t.Fatal("expected non-empty checkpoint hash")
	}

	// 3. Update file and commit round 2
	files[0].Content = "package app\n\nconst Version = \"2.0.0\"\n"
	if _, err := ws.SyncFiles(files); err != nil {
		t.Fatalf("sync round 2 files failed: %v", err)
	}
	hash2, err := ws.CommitRound(ctx, 2, 95.0, "Round 2 checkpoint")
	if err != nil {
		t.Fatalf("commit round 2 failed: %v", err)
	}
	if hash2 == "" || hash2 == hash1 {
		t.Fatalf("expected distinct checkpoint hash, got %q", hash2)
	}

	// 4. Accept Changes merges checkpoint to HEAD
	finalHash, err := ws.AcceptChanges(ctx)
	if err != nil {
		t.Fatalf("accept changes failed: %v", err)
	}
	if finalHash == "" {
		t.Fatal("expected non-empty final hash after accepting changes")
	}

	// Verify ledger recorded changes_accepted
	ledger, err := ws.Brain().ReadLedger()
	if err != nil {
		t.Fatalf("failed to read ledger: %v", err)
	}
	foundAccepted := false
	for _, l := range ledger {
		if l.Event == "changes_accepted" {
			foundAccepted = true
			break
		}
	}
	if !foundAccepted {
		t.Errorf("ledger did not record changes_accepted event")
	}
}


