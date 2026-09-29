package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

type CoderAgent struct {
	*BaseAgent
}

func NewCoderAgent(client llm.Client) *CoderAgent {
	return &CoderAgent{
		BaseAgent: NewBaseAgent(domain.RoleCoder, client),
	}
}

type coderResponse struct {
	Files       []domain.CodeFile `json:"files"`
	Explanation string            `json:"explanation"`
}

func (c *CoderAgent) Execute(ctx context.Context, sCtx *SwarmContext) (*domain.AgentMessage, error) {
	memories := c.fetchMemories(ctx, sCtx)

	systemPrompt := `You are the Lead Engineer in NexusAI.
Your role:
- Write complete, robust, production-ready code with no placeholders or TODOs.
- Include proper error handling, logging, concurrency safety, and comments.
- If in Round 2 or higher, directly address and fix all bugs reported by the Critic and Security Auditor.
- Return ONLY valid JSON conforming to:
{
  "files": [
    {"path": "path/to/file.go", "language": "go", "content": "package main\\n..."}
  ],
  "explanation": "Summary of changes and rationale"
}`

	userPrompt := fmt.Sprintf("Goal: %s\n%s", sCtx.UserPrompt, memories)
	if sCtx.Blueprint != nil {
		bpBytes, _ := json.Marshal(sCtx.Blueprint)
		userPrompt += fmt.Sprintf("\nArchitect Blueprint: %s\n", string(bpBytes))
	}
	if sCtx.Round > 1 {
		if sCtx.CriticReview != nil {
			crBytes, _ := json.Marshal(sCtx.CriticReview)
			userPrompt += fmt.Sprintf("\nCritic Bugs To Fix: %s\n", string(crBytes))
		}
		if sCtx.SecurityReport != nil {
			secBytes, _ := json.Marshal(sCtx.SecurityReport)
			userPrompt += fmt.Sprintf("\nSecurity Vulnerabilities To Fix: %s\n", string(secBytes))
		}
	}

	resp, err := c.llmClient.Generate(ctx, llm.Request{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
	})
	if err != nil {
		return nil, fmt.Errorf("coder generation failed: %w", err)
	}

	var parsed coderResponse
	if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
		parsed = coderResponse{
			Files: []domain.CodeFile{
				{Path: "main.go", Language: "go", Content: resp.Content},
			},
			Explanation: "Code generated.",
		}
	}

	sCtx.CodeFiles = parsed.Files

	summary := fmt.Sprintf("Lead Coder produced %d files (Round %d).", len(parsed.Files), sCtx.Round)
	return c.createMessage(sCtx, domain.MsgCodeDraft, parsed, summary)
}
