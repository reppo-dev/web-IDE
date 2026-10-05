package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"

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
	// MaxParallelTasks is how many independent tasks the swarm may advance at
	// once. Zero or one means strictly serial.
	//
	// It is wired through from here on purpose. The orchestrator reads this field
	// from its own config and nothing else populates it, which is precisely how
	// the parallel path came to exist in the code and then never actually run:
	// the field stayed zero, so the feature was silently off at runtime while
	// every test that exercised it directly passed.
	MaxParallelTasks int        `json:"max_parallel_tasks"`
	LLM              llm.Config `json:"llm"`

	// Resilience knobs for the shared LLM Guard (llm.Guard):
	// LLMMaxConcurrent caps in-flight requests toward the provider;
	// LLMMaxAttempts is the total tries per request on transient errors;
	// MaxTotalTokens is the session token budget (0 = unlimited).
	LLMMaxConcurrent int `json:"llm_max_concurrent"`
	LLMMaxAttempts   int `json:"llm_max_attempts"`
	MaxTotalTokens   int `json:"max_total_tokens"`
}

func loadDotEnv() {
	paths := []string{".env", "../.env", "../../.env"}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				if os.Getenv(key) == "" {
					os.Setenv(key, val)
				}
			}
		}
		break
	}
}

// envInt reads an integer environment variable with a default fallback.
func envInt(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return def
	}
	return v
}

// LoadConfig loads configuration with sensible defaults and environment variable overrides.
func LoadConfig() AppConfig {
	loadDotEnv()

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
		// Three independent tasks at a time. Higher mostly adds cost: the agents
		// compete for the same provider limit, so past a small number the wall
		// time stops improving while spend keeps rising.
		MaxParallelTasks: 3,
		LLM: llm.Config{
			Name:        "default",
			BaseURL:     baseURL,
			APIKey:      apiKey,
			Model:       model,
			Temperature: 0.2,
		},

		// LLM resilience guard (PROJECT_VISION §13 fallback, §26 limits).
		LLMMaxConcurrent: maxInt(1, envInt("NEXUS_LLM_MAX_CONCURRENT", 2)),
		LLMMaxAttempts:   maxInt(1, envInt("NEXUS_LLM_MAX_ATTEMPTS", 3)),
		MaxTotalTokens:   envInt("NEXUS_MAX_TOTAL_TOKENS", 0),
	}
}

// maxInt returns the larger of two ints.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
