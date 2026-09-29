package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestSandboxRunner_GoValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nexus-sandbox-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create valid Go file
	validFile := filepath.Join(tempDir, "main.go")
	_ = os.WriteFile(validFile, []byte("package main\n\nfunc main() {}\n"), 0644)

	runner := NewRunner(5 * time.Second)
	res := runner.VerifyCodebase(context.Background(), tempDir, []domain.CodeFile{
		{Path: "main.go", Language: "go"},
	})

	if res.Language != "go" {
		t.Errorf("expected go language, got %s", res.Language)
	}
}
