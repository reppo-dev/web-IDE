package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeContextTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func TestContextEngineListSourceFiles(t *testing.T) {
	root := t.TempDir()
	writeContextTestFile(t, root, "main.go", "package main")
	writeContextTestFile(t, root, "internal/auth/service.go", "package auth")
	writeContextTestFile(t, root, "README.md", "# hi")
	writeContextTestFile(t, root, "node_modules/pkg/index.js", "module.exports = 1")
	writeContextTestFile(t, root, ".git/config", "secret")
	writeContextTestFile(t, root, "image.png", "\x89PNG")

	e := NewContextEngine(root)
	files, err := e.ListSourceFiles()
	if err != nil {
		t.Fatalf("ListSourceFiles: %v", err)
	}

	got := map[string]bool{}
	for _, f := range files {
		got[f.RelPath] = true
	}

	if !got["main.go"] || !got["internal/auth/service.go"] {
		t.Errorf("expected source files to be found, got %v", got)
	}
	for _, skipped := range []string{"node_modules/pkg/index.js", ".git/config", "image.png"} {
		if got[skipped] {
			t.Errorf("expected %s to be skipped", skipped)
		}
	}
}

func TestContextEngineNoRoot(t *testing.T) {
	e := NewContextEngine("")
	if _, err := e.ListSourceFiles(); err == nil {
		t.Error("expected an error when no workspace path is configured")
	}
}

func TestContextEngineSelectsByKeyword(t *testing.T) {
	root := t.TempDir()
	writeContextTestFile(t, root, "auth/login.go", "package auth")
	writeContextTestFile(t, root, "billing/invoice.go", "package billing")
	writeContextTestFile(t, root, "misc/notes.go", "package misc")

	e := NewContextEngine(root)
	files, err := e.SelectRelevantFiles("fix the login authentication flow", 2)
	if err != nil {
		t.Fatalf("SelectRelevantFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one file")
	}
	if !strings.Contains(strings.ToLower(files[0].RelPath), "auth") {
		t.Errorf("expected an auth file to rank first, got %s", files[0].RelPath)
	}
}

func TestContextEnginePersianKeywords(t *testing.T) {
	root := t.TempDir()
	// The Persian word for login appears in the file's content, not its path,
	// which is exactly the case that path-only matching would miss.
	writeContextTestFile(t, root, "internal/session.go", "package auth\n\n// ورود کاربر را مدیریت می‌کند\nfunc Enter() {}\n")
	writeContextTestFile(t, root, "billing/invoice.go", "package billing\n\nfunc Charge() {}\n")

	e := NewContextEngine(root)
	files, err := e.SelectRelevantFiles("ورود کاربر", 1)
	if err != nil {
		t.Fatalf("SelectRelevantFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !strings.Contains(files[0].RelPath, "session") {
		t.Errorf("expected the file mentioning login, got %s", files[0].RelPath)
	}
}

func TestBuildCodeContextIncludesContent(t *testing.T) {
	root := t.TempDir()
	writeContextTestFile(t, root, "auth/login.go", "package auth\n\nfunc Login() {}\n")

	e := NewContextEngine(root)
	out, err := e.BuildCodeContext("improve login", 5)
	if err != nil {
		t.Fatalf("BuildCodeContext: %v", err)
	}
	if !strings.Contains(out, "auth/login.go") {
		t.Errorf("expected the file name in the context, got:\n%s", out)
	}
	if !strings.Contains(out, "func Login()") {
		t.Errorf("expected file content in the context, got:\n%s", out)
	}
}

func TestBuildCodeContextRespectsBudget(t *testing.T) {
	root := t.TempDir()
	big := strings.Repeat("x", 40*1024)
	for _, name := range []string{"a/one.go", "b/two.go", "c/three.go", "d/four.go"} {
		writeContextTestFile(t, root, name, big)
	}

	e := NewContextEngine(root)
	e.maxFileBytes = 8 * 1024
	e.maxTotalBytes = 16 * 1024

	out, err := e.BuildCodeContext("relevant", 10)
	if err != nil {
		t.Fatalf("BuildCodeContext: %v", err)
	}
	if len(out) > 30*1024 {
		t.Errorf("context grew to %d bytes, expected the budget to be enforced", len(out))
	}
	// With a 16K budget and 8K per file, later files must be dropped entirely.
	if !strings.Contains(out, "[truncated]") {
		t.Errorf("expected at least one truncated file, got:\n%s", out)
	}
	if !strings.Contains(out, "a/one.go") {
		t.Errorf("expected the first file to be included, got:\n%s", out)
	}
}

func TestBuildStructureListsFiles(t *testing.T) {
	root := t.TempDir()
	writeContextTestFile(t, root, "main.go", "package main")
	writeContextTestFile(t, root, "lib/util.go", "package lib")

	e := NewContextEngine(root)
	out, err := e.BuildStructure()
	if err != nil {
		t.Fatalf("BuildStructure: %v", err)
	}
	if !strings.Contains(out, "main.go") || !strings.Contains(out, "lib/util.go") {
		t.Errorf("expected both files in the structure map, got:\n%s", out)
	}
}

func TestRelevantKeywordsDropsStopWords(t *testing.T) {
	kws := relevantKeywords("create a new file for the project and the system")
	for _, stop := range []string{"create", "file", "project", "system", "the", "and"} {
		for _, kw := range kws {
			if kw == stop {
				t.Errorf("expected %q to be dropped as a stop word", stop)
			}
		}
	}
}

func TestIsViewableFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"index.ts", true},
		{"app.jsx", true},
		{"data.json", true},
		{"image.png", false},
		{"lib.dll", false},
		{"archive.zip", false},
		{"db.sqlite", false},
	}
	for _, tc := range cases {
		if got := IsViewableFile(tc.path); got != tc.want {
			t.Errorf("IsViewableFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestBuildStructureIdentifiesTheStack covers a real failure: asked for a
// Duolingo clone inside a Flutter project, the planner proposed React Native.
// The file names were sent, but nothing said which framework they belonged to, so
// the stack was still a guess.
func TestBuildStructureIdentifiesTheStack(t *testing.T) {
	root := t.TempDir()

	// A Flutter project: the same shape of files a React Native project has,
	// plus the marker that actually identifies it.
	writeContextTestFile(t, root, "pubspec.yaml", "name: first_app\ndependencies:\n  flutter:\n    sdk: flutter\n")
	writeContextTestFile(t, root, "lib/main.dart", "void main() {}")
	writeContextTestFile(t, root, "lib/models/lesson.dart", "class Lesson {}")
	writeContextTestFile(t, root, "test/widget_test.dart", "void main() {}")

	engine := NewContextEngine(root)
	structure, err := engine.BuildStructure()
	if err != nil {
		t.Fatalf("BuildStructure: %v", err)
	}

	// pubspec.yaml must be named as the marker that identifies the stack.
	if !strings.Contains(structure, "pubspec.yaml") {
		t.Errorf("structure does not mention the Flutter marker:\n%s", structure)
	}
	if !strings.Contains(structure, "Flutter") {
		t.Errorf("structure does not identify the project as Flutter:\n%s", structure)
	}
	// The dominant language must be stated, so a planner cannot infer Dart from
	// a file called main.dart.
	if !strings.Contains(structure, ".dart") {
		t.Errorf("structure does not report the dominant language:\n%s", structure)
	}
}

// TestBuildStructureSaysWhenItCannotTell guards the honest case: an empty
// project must not be described as though it had a known stack.
func TestBuildStructureSaysWhenItCannotTell(t *testing.T) {
	root := t.TempDir()
	engine := NewContextEngine(root)

	structure, err := engine.BuildStructure()
	if err != nil {
		t.Fatalf("BuildStructure: %v", err)
	}
	if !strings.Contains(structure, "none found") {
		t.Errorf("an empty project should say it has no markers:\n%s", structure)
	}
}

// TestBuildStructureBoundsLongLists keeps a large repository from crowding out
// the markers, which are the part that matters.
func TestBuildStructureBoundsLongLists(t *testing.T) {
	root := t.TempDir()
	writeContextTestFile(t, root, "go.mod", "module example.com/app\n")
	for i := 0; i < 200; i++ {
		writeContextTestFile(t, root, fmt.Sprintf("pkg/file%03d.go", i), "package pkg\n")
	}

	structure, err := NewContextEngine(root).BuildStructure()
	if err != nil {
		t.Fatalf("BuildStructure: %v", err)
	}
	if !strings.Contains(structure, "and 80 more") {
		t.Errorf("long file list should be truncated and counted:\n%s", structure[:min(len(structure), 400)])
	}
	// The marker must survive the truncation, since it is the whole point.
	if !strings.Contains(structure, "Go project") {
		t.Errorf("the stack marker was lost to truncation:\n%s", structure[:min(len(structure), 400)])
	}
}
