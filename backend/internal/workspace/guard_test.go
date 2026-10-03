package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestSafePath_InsideProject(t *testing.T) {
	root := t.TempDir()

	cases := []struct {
		candidate string
		wantRel   string
	}{
		{"main.go", "main.go"},
		{"src/app.ts", filepath.Join("src", "app.ts")},
		{"src\\app.ts", filepath.Join("src", "app.ts")},
		{"internal/service/auth.go", filepath.Join("internal", "service", "auth.go")},
	}

	for _, tc := range cases {
		got, err := SafePath(root, tc.candidate)
		if err != nil {
			t.Fatalf("SafePath(%q) unexpected error: %v", tc.candidate, err)
		}
		expected := filepath.Join(root, tc.wantRel)
		if got != expected {
			t.Errorf("SafePath(%q) = %q, want %q", tc.candidate, got, expected)
		}
	}
}

func TestSafePath_RejectsEscapeAttempts(t *testing.T) {
	root := t.TempDir()

	escapes := []string{
		"../secret.txt",
		"../../../../etc/passwd",
		"src/../../outside.go",
		"..\\..\\Windows\\System32\\config",
		"C:\\Windows\\System32",
		"D:\\other\\project",
	}

	for _, esc := range escapes {
		_, err := SafePath(root, esc)
		if err == nil {
			t.Errorf("SafePath(%q) expected error for path escape, got nil", esc)
			continue
		}
		if !errors.Is(err, ErrOutsideProject) && !strings.Contains(err.Error(), "outside") {
			t.Errorf("SafePath(%q) error %v did not match ErrOutsideProject", esc, err)
		}
	}
}

func TestSafePath_EmptyAndInvalid(t *testing.T) {
	root := t.TempDir()

	if _, err := SafePath(root, ""); err == nil {
		t.Error("expected error for empty candidate")
	}
	if _, err := SafePath("", "main.go"); err == nil {
		t.Error("expected error for empty root")
	}
}

func TestSafeReadPath(t *testing.T) {
	root := t.TempDir()

	filePath := filepath.Join(root, "config.json")
	if err := os.WriteFile(filePath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create config.json: %v", err)
	}

	subDir := filepath.Join(root, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// 1. Valid file
	got, err := SafeReadPath(root, "config.json")
	if err != nil {
		t.Fatalf("SafeReadPath(config.json) error: %v", err)
	}
	if got != filePath {
		t.Errorf("SafeReadPath = %q, want %q", got, filePath)
	}

	// 2. Missing file
	if _, err := SafeReadPath(root, "nonexistent.go"); err == nil {
		t.Error("expected error for missing file")
	}

	// 3. Directory (not a file)
	if _, err := SafeReadPath(root, "subdir"); err == nil {
		t.Error("expected error when target is a directory")
	}

	// 4. Traversal
	if _, err := SafeReadPath(root, "../outside"); err == nil {
		t.Error("expected error for traversal attempt")
	}
}

func TestRelPath(t *testing.T) {
	root := t.TempDir()

	fileAbs := filepath.Join(root, "internal", "auth", "token.go")

	rel, err := RelPath(root, fileAbs)
	if err != nil {
		t.Fatalf("RelPath error: %v", err)
	}
	if rel != "internal/auth/token.go" {
		t.Errorf("RelPath = %q, want %q", rel, "internal/auth/token.go")
	}

	// Root itself
	relRoot, err := RelPath(root, root)
	if err != nil {
		t.Fatalf("RelPath(root, root) error: %v", err)
	}
	if relRoot != "" {
		t.Errorf("RelPath(root, root) = %q, want empty string", relRoot)
	}

	// Outside path
	outsideAbs := filepath.Join(t.TempDir(), "other.go")
	if _, err := RelPath(root, outsideAbs); err == nil {
		t.Error("expected error for outside path")
	}
}

// The unit tests above cover the path maths. These prove the guard is actually
// wired into the write path, which is what stops an agent from touching files
// it was not given.
func TestWorkspace_SyncFilesRejectsEscape(t *testing.T) {
	base := t.TempDir()
	proj := filepath.Join(base, "project")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(base, "secret.txt")
	if err := os.WriteFile(secret, []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := NewWorkspace(filepath.Join(base, "data"), proj)
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}

	out, err := ws.SyncFiles([]domain.CodeFile{{
		Path:     "../escaped.go",
		Content:  "package main",
		Language: "go",
	}})
	if err != nil {
		t.Fatalf("SyncFiles should reject rather than crash: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(base, "escaped.go")); statErr == nil {
		t.Fatal("a file was created OUTSIDE the project folder")
	}
	if data, _ := os.ReadFile(secret); string(data) != "SECRET" {
		t.Error("an unrelated file outside the project was modified")
	}
	if len(out) != 1 || !strings.Contains(out[0].Diff, "REJECTED") {
		t.Errorf("the caller must be told the file was rejected, got %+v", out)
	}
}

func TestWorkspace_SaveFileRejectsEscape(t *testing.T) {
	base := t.TempDir()
	proj := filepath.Join(base, "project")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}

	ws, err := NewWorkspace(filepath.Join(base, "data"), proj)
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}

	if err := ws.SaveFile("../escaped.go", "package main"); err == nil {
		t.Error("SaveFile must refuse to write outside the project")
	}
	if _, statErr := os.Stat(filepath.Join(base, "escaped.go")); statErr == nil {
		t.Error("SaveFile created a file outside the project")
	}

	// A normal nested path must still work.
	if err := ws.SaveFile("backend/api/router.go", "package api\n\nfunc Route() {}"); err != nil {
		t.Errorf("SaveFile should allow normal nested paths: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(proj, "backend", "api", "router.go")); statErr != nil {
		t.Errorf("expected the nested file to be created: %v", statErr)
	}
}

func TestWorkspace_SyncFilesAcceptsNestedPaths(t *testing.T) {
	base := t.TempDir()
	proj := filepath.Join(base, "project")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}

	ws, err := NewWorkspace(filepath.Join(base, "data"), proj)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ws.SyncFiles([]domain.CodeFile{{
		Path:     "backend/internal/agent/custom.go",
		Content:  "package agent\n\nfunc Run() {}",
		Language: "go",
	}}); err != nil {
		t.Fatalf("SyncFiles: %v", err)
	}

	want := filepath.Join(proj, "backend", "internal", "agent", "custom.go")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected a nested file at %s: %v", want, err)
	}
}

