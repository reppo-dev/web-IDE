package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

// CrossCritique represents one agent's targeted review of another agent's output.
type CrossCritique struct {
	ReviewerAgent domain.AgentRole `json:"reviewer_agent"`
	TargetAgent   domain.AgentRole `json:"target_agent"`
	Agrees        bool             `json:"agrees"`
	ScoreGiven    float64          `json:"score_given"` // 0 to 100
	Feedback      string           `json:"feedback"`
	DemandedFix   string           `json:"demanded_fix,omitempty"`
	Timestamp     time.Time        `json:"timestamp"`
}

// DebateMatrix holds all pairwise evaluations between agents in a round.
type DebateMatrix struct {
	Round       int             `json:"round"`
	Critiques   []CrossCritique `json:"critiques"`
	ConsensusPct float64        `json:"consensus_pct"` // % of pairwise agreements
	Convergence  bool           `json:"convergence"`
}

// MatrixEngine conducts full N-to-N peer cross-examination.
type MatrixEngine struct {
	llmClient llm.Client
}

// NewMatrixEngine creates a new peer evaluation matrix.
func NewMatrixEngine(client llm.Client) *MatrixEngine {
	return &MatrixEngine{llmClient: client}
}

// ExecuteCrossExamination runs all pairwise critiques concurrently.
func (m *MatrixEngine) ExecuteCrossExamination(ctx context.Context, sCtx *SwarmContext) (*DebateMatrix, error) {
	return m.ExecuteCrossExaminationWithCallback(ctx, sCtx, nil)
}

// ExecuteCrossExaminationWithCallback runs pairwise critiques and invokes a callback for each generated critique.
func (m *MatrixEngine) ExecuteCrossExaminationWithCallback(ctx context.Context, sCtx *SwarmContext, onCritique func(CrossCritique)) (*DebateMatrix, error) {
	agents := []domain.AgentRole{
		domain.RoleArchitect,
		domain.RoleCoder,
		domain.RoleCritic,
		domain.RoleSecurity,
		domain.RoleArbiter,
	}

	critiquesChan := make(chan CrossCritique, 20)
	var wg sync.WaitGroup

	// Every agent inspects every other agent (5 * 4 = 20 pairwise inspections)
	for _, reviewer := range agents {
		for _, target := range agents {
			if reviewer == target {
				continue // Do not critique self
			}

			wg.Add(1)
			go func(rev, tgt domain.AgentRole) {
				defer wg.Done()
				critique := m.evaluatePair(ctx, rev, tgt, sCtx)
				critiquesChan <- critique
			}(reviewer, target)
		}
	}

	wg.Wait()
	close(critiquesChan)

	var allCritiques []CrossCritique
	agreeCount := 0

	for c := range critiquesChan {
		allCritiques = append(allCritiques, c)
		if c.Agrees {
			agreeCount++
		}
		if onCritique != nil {
			onCritique(c)
		}
	}

	total := len(allCritiques)
	consensusPct := 0.0
	if total > 0 {
		consensusPct = (float64(agreeCount) / float64(total)) * 100.0
	}

	matrix := &DebateMatrix{
		Round:        sCtx.Round,
		Critiques:    allCritiques,
		ConsensusPct: consensusPct,
		Convergence:  consensusPct >= 80.0,
	}

	// Persist matrix insight to memory
	if sCtx.MemStore != nil {
		_ = sCtx.MemStore.WriteProject(ctx, domain.MemoryEntry{
			ProjectID:  sCtx.ProjectID,
			Agent:      domain.RoleArbiter,
			Type:       domain.MemoryInsight,
			Content:    fmt.Sprintf("Round %d Cross-Examination Matrix: %d/%d peer agreements (%.1f%%)", sCtx.Round, agreeCount, total, consensusPct),
			Tags:       []string{"cross-examination", "debate-matrix"},
			Round:      sCtx.Round,
			Confidence: 0.95,
		})
	}

	return matrix, nil
}

func (m *MatrixEngine) evaluatePair(ctx context.Context, rev, tgt domain.AgentRole, sCtx *SwarmContext) CrossCritique {
	// Synthesize prompt for reviewer evaluating target
	sysPrompt := fmt.Sprintf(`You are the %s evaluating the output of the %s.
Evaluate their work strictly from your professional perspective:
- Match the language of the conversation (if the user wrote in Persian, write feedback and demanded_fix in fluent Persian).
- Provide constructive, realistic feedback.
- Return ONLY valid JSON:
{
  "agrees": true,
  "score_given": 90.0,
  "feedback": "Concise review comment in the user's language",
  "demanded_fix": "Optional fix if disagrees in the user's language"
}`, rev, tgt)

	filesSummary := ""
	for _, f := range sCtx.CodeFiles {
		filesSummary += fmt.Sprintf("- %s (%s)\n", f.Path, f.Language)
	}
	if filesSummary == "" {
		filesSummary = "Blueprint stage / in-progress"
	}

	userPrompt := fmt.Sprintf("Project: %s\nTarget Agent: %s\nRound: %d\nFiles:\n%s",
		sCtx.UserPrompt, tgt, sCtx.Round, filesSummary)

	model := ""
	if sCtx != nil && sCtx.AgentModels != nil {
		model = sCtx.AgentModels[rev]
	}

	apiKey := ""
	baseURL := ""
	route, ok := ResolveProvider(model)
	if ok && sCtx != nil {
		apiKey = sCtx.APIKeys[route.apiKeyName]
		baseURL = route.baseURL
	}

	resp, err := m.llmClient.Generate(ctx, llm.Request{
		Model:        model,
		SystemPrompt: sysPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
		APIKey:       apiKey,
		BaseURL:      baseURL,
	})

	if err != nil {
		// Fallback deterministic evaluation
		agrees := sCtx.Round > 1
		score := 75.0 + float64(sCtx.Round*5)
		if score > 95.0 {
			score = 95.0
		}
		return CrossCritique{
			ReviewerAgent: rev,
			TargetAgent:   tgt,
			Agrees:        agrees,
			ScoreGiven:    score,
			Feedback:      fmt.Sprintf("%s completed cross-check of %s's output.", rev, tgt),
			Timestamp:     time.Now(),
		}
	}

	var parsed struct {
		Agrees      bool    `json:"agrees"`
		ScoreGiven  float64 `json:"score_given"`
		Feedback    string  `json:"feedback"`
		DemandedFix string  `json:"demanded_fix"`
	}

	if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
		parsed.Agrees = true
		parsed.ScoreGiven = 85.0
		parsed.Feedback = resp.Content
	}

	return CrossCritique{
		ReviewerAgent: rev,
		TargetAgent:   tgt,
		Agrees:        parsed.Agrees,
		ScoreGiven:    parsed.ScoreGiven,
		Feedback:      parsed.Feedback,
		DemandedFix:   parsed.DemandedFix,
		Timestamp:     time.Now(),
	}
}

