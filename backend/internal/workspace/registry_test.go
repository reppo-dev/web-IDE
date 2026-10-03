package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryRegisterAndLookup(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "my-project")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}

	reg, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	ref, err := reg.Register(proj)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := reg.Lookup(ref.ID)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got.Root != ref.Root {
		t.Errorf("expected root %q, got %q", ref.Root, got.Root)
	}
	if got.Name != "my-project" {
		t.Errorf("expected name my-project, got %q", got.Name)
	}
}

func TestRegistryPersistsAcrossRestart(t *testing.T) {
	dataDir := t.TempDir()
	proj := filepath.Join(t.TempDir(), "persisted")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}

	first, err := NewRegistry(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := first.Register(proj)
	if err != nil {
		t.Fatal(err)
	}

	// A fresh registry must reload the mapping, otherwise the project would
	// lose its memory on every restart.
	second, err := NewRegistry(dataDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, err := second.Lookup(ref.ID)
	if err != nil {
		t.Fatalf("Lookup after restart: %v", err)
	}
	if got.Root != ref.Root {
		t.Errorf("expected %q after restart, got %q", ref.Root, got.Root)
	}
}

func TestRegisterSameFolderTwiceIsIdempotent(t *testing.T) {
	reg, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	proj := t.TempDir()

	a, err := reg.Register(proj)
	if err != nil {
		t.Fatal(err)
	}
	b, err := reg.Register(proj)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID {
		t.Errorf("the same folder must map to one id: %q vs %q", a.ID, b.ID)
	}
	if len(reg.List()) != 1 {
		t.Errorf("expected 1 project, got %d", len(reg.List()))
	}
}

func TestRegistryListSkipsDeletedFolders(t *testing.T) {
	reg, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	keep := t.TempDir()
	remove := t.TempDir()
	if _, err := reg.Register(keep); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Register(remove); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(remove); err != nil {
		t.Fatal(err)
	}

	list := reg.List()
	if len(list) != 1 {
		t.Fatalf("expected only the existing folder, got %d", len(list))
	}
	if list[0].Root != keep {
		t.Errorf("expected the surviving project, got %q", list[0].Root)
	}
}

func TestRegistryRejectsMissingFolder(t *testing.T) {
	reg, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Register(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected an error for a folder that does not exist")
	}
}

func TestRegistrySurvivesCorruptFile(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "projects.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := NewRegistry(dataDir)
	if err != nil {
		t.Fatalf("a corrupt registry must not stop startup: %v", err)
	}
	if len(reg.List()) != 0 {
		t.Error("expected an empty registry after corruption")
	}

	// And it must still be usable afterwards.
	if _, err := reg.Register(t.TempDir()); err != nil {
		t.Errorf("registry should still accept projects: %v", err)
	}
}

func TestRegistryForget(t *testing.T) {
	dataDir := t.TempDir()
	reg, err := NewRegistry(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := reg.Register(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Forget(ref.ID); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if _, err := reg.Lookup(ref.ID); err == nil {
		t.Error("expected the project to be forgotten")
	}
}

// A registered project must win over the sandbox path, otherwise the swarm
// would edit files inside the data folder instead of the user's project.
func TestProjectPathUsesRegistry(t *testing.T) {
	proj := t.TempDir()
	dataDir := t.TempDir()

	reg, err := NewRegistry(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := reg.Register(proj)
	if err != nil {
		t.Fatal(err)
	}
	SetRegistry(reg)
	defer SetRegistry(nil)

	got := ProjectPath(dataDir, ref.ID)
	if got != proj {
		t.Errorf("expected the real project folder %q, got %q", proj, got)
	}
}

func TestProjectPathFallsBackForUnknownID(t *testing.T) {
	dataDir := t.TempDir()
	SetRegistry(nil)

	got := ProjectPath(dataDir, "unknown-project")
	if filepath.Dir(got) != filepath.Join(dataDir, "workspaces") {
		t.Errorf("unknown ids must stay in the sandbox, got %q", got)
	}
}

func TestSanitizeIDCannotEscape(t *testing.T) {
	dangerous := []string{
		`..\..\Windows`,
		`../../etc`,
		`/absolute/path`,
		`C:\Windows\System32`,
	}
	for _, in := range dangerous {
		got := SanitizeID(in)
		if got == "" {
			t.Errorf("SanitizeID(%q) returned empty", in)
			continue
		}
		if filepath.IsAbs(got) {
			t.Errorf("SanitizeID(%q) = %q is absolute", in, got)
		}
		for _, r := range got {
			if r == '/' || r == '\\' || r == ':' {
				t.Errorf("SanitizeID(%q) = %q still contains %q", in, got, r)
				break
			}
		}
	}
}