package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

// SkillManifest defines metadata in skill.yaml.
type SkillManifest struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Version     string              `json:"version"`
	Agents      []domain.AgentRole  `json:"agents"`
	Triggers    []string            `json:"triggers"`
	Requires    []string            `json:"requires,omitempty"`
	Prompt      string              `json:"prompt,omitempty"`
	Path        string              `json:"path,omitempty"`
	Enabled     bool                `json:"enabled"`
}

// Manager handles skill loading, trigger matching, and prompt enrichment.
type Manager struct {
	skillsDir string
	skills    map[string]*SkillManifest
	mu        sync.RWMutex
}

// NewManager creates a new Skills Manager.
func NewManager(skillsDir string) *Manager {
	return &Manager{
		skillsDir: skillsDir,
		skills:    make(map[string]*SkillManifest),
	}
}

// LoadAll loads all skills found in the skills directory and creates defaults if empty.
func (m *Manager) LoadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.skillsDir, 0755); err != nil {
		return err
	}

	// Create built-in skills if directory is empty
	m.ensureBuiltins()

	entries, err := os.ReadDir(m.skillsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			skillPath := filepath.Join(m.skillsDir, entry.Name())
			promptFile := filepath.Join(skillPath, "prompt.md")
			promptBytes, _ := os.ReadFile(promptFile)

			triggers := []string{strings.ToLower(entry.Name())}
			parts := strings.Split(entry.Name(), "-")
			for _, p := range parts {
				if len(p) > 2 {
					triggers = append(triggers, strings.ToLower(p))
				}
			}

			manifest := &SkillManifest{
				Name:        entry.Name(),
				Description: fmt.Sprintf("Built-in skill for %s", entry.Name()),
				Version:     "1.0.0",
				Agents:      []domain.AgentRole{domain.RoleCoder, domain.RoleCritic},
				Triggers:    triggers,
				Prompt:      string(promptBytes),
				Path:        skillPath,
				Enabled:     true,
			}
			m.skills[entry.Name()] = manifest
		}
	}

	return nil
}

func (m *Manager) ensureBuiltins() {
	builtins := map[string]string{
		"code-review": "Analyze code thoroughly for edge cases, resource leaks, and readability.",
		"api-designer": "Design RESTful APIs and gRPC contracts following industry best practices.",
		"docker-deploy": "Create robust, multi-stage Dockerfiles and container configurations.",
		"test-generator": "Generate idiomatic unit, integration, and table-driven tests.",
	}

	for name, prompt := range builtins {
		dir := filepath.Join(m.skillsDir, name)
		_ = os.MkdirAll(dir, 0755)
		promptPath := filepath.Join(dir, "prompt.md")
		if _, err := os.Stat(promptPath); os.IsNotExist(err) {
			_ = os.WriteFile(promptPath, []byte(prompt), 0644)
		}
	}
}

// ListSkills returns all registered skills.
func (m *Manager) ListSkills() []*SkillManifest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*SkillManifest, 0, len(m.skills))
	for _, s := range m.skills {
		res = append(res, s)
	}
	return res
}

// MatchSkills returns active skills relevant to a user prompt or role.
func (m *Manager) MatchSkills(userPrompt string, role domain.AgentRole) []*SkillManifest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lowerPrompt := strings.ToLower(userPrompt)
	var matched []*SkillManifest

	for _, s := range m.skills {
		if !s.Enabled {
			continue
		}

		// Check role matching
		roleMatch := len(s.Agents) == 0
		for _, r := range s.Agents {
			if r == role {
				roleMatch = true
				break
			}
		}
		if !roleMatch {
			continue
		}

		// Check trigger keywords
		for _, trig := range s.Triggers {
			if strings.Contains(lowerPrompt, strings.ToLower(trig)) {
				matched = append(matched, s)
				break
			}
		}
	}

	return matched
}
