package skills

import (
	"os"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestSkillsManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nexus-skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewManager(tempDir)
	if err := mgr.LoadAll(); err != nil {
		t.Fatalf("failed to load skills: %v", err)
	}

	skills := mgr.ListSkills()
	if len(skills) < 4 {
		t.Errorf("expected at least 4 built-in skills, got %d", len(skills))
	}

	// Match skills
	matched := mgr.MatchSkills("Please add docker deployment for this API", domain.RoleCoder)
	if len(matched) == 0 {
		t.Errorf("expected docker skill to match query")
	}

	foundDocker := false
	for _, m := range matched {
		if m.Name == "docker-deploy" {
			foundDocker = true
			break
		}
	}
	if !foundDocker {
		t.Errorf("expected docker-deploy in matched skills")
	}
}
