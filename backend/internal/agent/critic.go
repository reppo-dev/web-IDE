package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

type CriticAgent struct {
	*BaseAgent
}

func NewCriticAgent(client llm.Client) *CriticAgent {
	return &CriticAgent{
		BaseAgent: NewBaseAgent(domain.RoleCritic, client),
	}
}

func (cr *CriticAgent) Execute(ctx context.Context, sCtx *SwarmContext) (*domain.AgentMessage, error) {
	memories := cr.fetchMemories(ctx, sCtx)

	systemPrompt := `You are the Adversarial Critic (The Red Team) in NexusAI.
Your role:
- Thoroughly inspect the code for logic flaws, race conditions, edge-case failures, unhandled errors, and performance bottlenecks.
- Be constructively demanding: catch real bugs, do not invent superficial nitpicks.
- Return ONLY valid JSON:
{
  "general_review": "Overall critique of code correctness and logic",
  "issues": [
    {
      "severity": "critical|major|minor",
      "file": "path/to/file.go",
      "line": 42,
      "description": "Explanation of the bug or race condition",
      "suggested_fix": "Exact code correction"
    }
  ],
  "refactor_plan": "High-level guidance for Coder in next round"
}`

	codeContent := ""
	for _, f := range sCtx.CodeFiles {
		codeContent += fmt.Sprintf("--- File: %s (%s) ---\n%s\n\n", f.Path, f.Language, f.Content)
	}

	userPrompt := fmt.Sprintf("Project Goal: %s\n%s\nCodebase:\n%s", sCtx.UserPrompt, memories, codeContent)

	resp, err := cr.llmClient.Generate(ctx, llm.Request{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
	})
	if err != nil {
		return nil, fmt.Errorf("critic review failed: %w", err)
	}

	var payload domain.CriticPayload
	if err := json.Unmarshal([]byte(resp.Content), &payload); err != nil {
		payload = domain.CriticPayload{
			GeneralReview: resp.Content,
			Issues:        nil,
		}
	}

	sCtx.CriticReview = &payload

	// Record any discovered mistakes into project memory
	if sCtx.MemStore != nil && len(payload.Issues) > 0 {
		for _, issue := range payload.Issues {
			if issue.Severity == "critical" {
				_ = sCtx.MemStore.WriteProject(ctx, domain.MemoryEntry{
					ProjectID:  sCtx.ProjectID,
					Agent:      domain.RoleCritic,
					Type:       domain.MemoryMistake,
					Content:    fmt.Sprintf("Critical flaw identified in %s: %s", issue.File, issue.Description),
					Tags:       []string{"bug", issue.File, "critical"},
					Round:      sCtx.Round,
					Confidence: 0.90,
				})
			}
		}
	}

	summary := fmt.Sprintf("Critic flagged %d issues (%d critical).", len(payload.Issues), countCritical(payload.Issues))
	return cr.createMessage(sCtx, domain.MsgCriticReview, payload, summary)
}

func countCritical(issues []domain.ReviewIssue) int {
	c := 0
	for _, i := range issues {
		if i.Severity == "critical" {
			c++
		}
	}
	return c
}
