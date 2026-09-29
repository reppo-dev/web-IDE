package config

import (
	"os"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

// AppConfig represents global configuration for the NexusAI daemon.
type AppConfig struct {
	Port      string               `json:"port"`
	DataDir   string               `json:"data_dir"`
	SkillsDir string               `json:"skills_dir"`
	Mode      domain.ExecutionMode `json:"mode"`
	MaxRounds int                  `json:"max_rounds"`
	LLM       llm.Config           `json:"llm"`
}

// LoadConfig loads configuration with sensible defaults and environment variable overrides.
func LoadConfig() AppConfig {
	port := os.Getenv("NEXUS_PORT")
	if port == "" {
		port = "8080"
	}

	dataDir := os.Getenv("NEXUS_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	skillsDir := os.Getenv("NEXUS_SKILLS_DIR")
	if skillsDir == "" {
		skillsDir = "./skills"
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}

	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	model := os.Getenv("NEXUS_MODEL")
	if model == "" {
		model = "gpt-4o"
	}

	modeStr := os.Getenv("NEXUS_MODE")
	mode := domain.ModeRoleBased
	if modeStr != "" {
		mode = domain.ExecutionMode(modeStr)
	}

	return AppConfig{
		Port:      port,
		DataDir:   dataDir,
		SkillsDir: skillsDir,
		Mode:      mode,
		MaxRounds: 5,
		LLM: llm.Config{
			Name:        "default",
			BaseURL:     baseURL,
			APIKey:      apiKey,
			Model:       model,
			Temperature: 0.2,
		},
	}
}
