package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveProjectAcceptsExistingFolder(t *testing.T) {
	dir := t.TempDir()
	ref, err := ResolveProject(dir)
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if ref.Name != filepath.Base(dir) {
		t.Errorf("expected name %q, got %q", filepath.Base(dir), ref.Name)
	}
	if !filepath.IsAbs(ref.Root) {
		t.Errorf("root must be absolute, got %q", ref.Root)
	}
}

func TestResolveProjectRejectsMissingFolder(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := ResolveProject(missing); err == nil {
		t.Error("expected an error for a folder that does not exist")
	}
}

func TestResolveProjectRejectsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveProject(file); err == nil {
		t.Error("expected an error when the path is a file, not a folder")
	}
}

func TestResolveProjectRejectsEmptyPath(t *testing.T) {
	if _, err := ResolveProject("   "); err == nil {
		t.Error("expected an error for an empty path")
	}
}

// The id becomes a folder name under data/, so a Windows path or a traversal
// sequence must never survive into it.
func TestProjectIDIsFilesystemSafe(t *testing.T) {
	dangerous := []string{
		`C:\Users\me\my project`,
		`..\..\etc\passwd`,
		`/home/user/../root/.ssh`,
		`relative/path`,
	}

	for _, p := range dangerous {
		id := ProjectIDFor(p)
		if strings.ContainsAny(id, `/\:`) {
			t.Errorf("id for %q contains a path separator: %q", p, id)
		}
		if strings.Contains(id, "..") {
			t.Errorf("id for %q contains traversal: %q", p, id)
		}
		if id == "" {
			t.Errorf("id for %q is empty", p)
		}
	}
}

func TestProjectIDIsStable(t *testing.T) {
	p := `/home/user/project`
	first := ProjectIDFor(p)
	second := ProjectIDFor(p)
	if first != second {
		t.Errorf("id must be stable for the same path: %q vs %q", first, second)
	}
}

func TestProjectIDDiffersPerPath(t *testing.T) {
	a := ProjectIDFor(`/home/user/alpha`)
	b := ProjectIDFor(`/home/user/beta`)
	if a == b {
		t.Errorf("different paths must produce different ids, both were %q", a)
	}
}

// Windows paths are case-insensitive: C:\Code and c:\code are the same folder
// and must not end up with two separate memories.
func TestProjectIDNormalisesWindowsCase(t *testing.T) {
	upper := ProjectIDFor(`C:\Users\Me\MyProject`)
	lower := ProjectIDFor(`c:\users\me\myproject`)
	if upper != lower {
		t.Errorf("case-differing Windows paths must share an id: %q vs %q", upper, lower)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"My Project":     "my-project",
		"Go_Microservice": "go-microservice",
		"  spaces  ":     "spaces",
		"!!!":            "",
		"مشروع":           "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// A project id must never be mistaken for an absolute path by ProjectPath,
// or the agents would write into the data folder instead of the real project.
func TestResolvedIDDoesNotLookLikeAPath(t *testing.T) {
	dir := t.TempDir()
	ref, err := ResolveProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.IsAbs(ref.ID) {
		t.Errorf("id %q must not be an absolute path", ref.ID)
	}
	if strings.HasPrefix(ref.ID, "..") {
		t.Errorf("id %q must not start with traversal", ref.ID)
	}
}