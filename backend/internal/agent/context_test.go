package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAgentContextTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func TestAgentContextEngineReExport(t *testing.T) {
	root := t.TempDir()
	writeAgentContextTestFile(t, root, "main.go", "package main")
	writeAgentContextTestFile(t, root, "auth/service.go", "package auth")

	e := NewContextEngine(root)
	files, err := e.ListSourceFiles()
	if err != nil {
		t.Fatalf("ListSourceFiles: %v", err)
	}
	if len(files) < 2 {
		t.Fatalf("expected at least 2 files, got %d", len(files))
	}

	code, err := e.BuildCodeContext("service", 5)
	if err != nil {
		t.Fatalf("BuildCodeContext: %v", err)
	}
	if !strings.Contains(code, "auth/service.go") {
		t.Errorf("expected auth/service.go in code context, got: %s", code)
	}

	structure, err := e.BuildStructure()
	if err != nil {
		t.Fatalf("BuildStructure: %v", err)
	}
	if !strings.Contains(structure, "main.go") {
		t.Errorf("expected main.go in structure, got: %s", structure)
	}

	if !IsViewableFile("main.go") {
		t.Errorf("expected main.go to be viewable")
	}
	if IsViewableFile("app.exe") {
		t.Errorf("expected app.exe to not be viewable")
	}
}