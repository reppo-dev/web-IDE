package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

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
- Focus on the 1-3 essential files for the requested feature to maintain rapid generation latency.
- If in Round 2 or higher, directly address and fix all bugs reported by the Critic and Security Auditor.
- The EXISTING PROJECT CODE is provided below. Treat it as ground truth: match its
  package names, import paths, type names, and error handling style.
  Do NOT invent a different structure, and do NOT rewrite files you were not asked to change.
- If the request contradicts the existing code, follow the request but say so in "explanation".
- Return ONLY valid JSON conforming to:
{
  "files": [
    {"path": "path/to/file.go", "language": "go", "content": "package main\\n..."}
  ],
  "explanation": "Summary of changes and rationale"
}`

	// Read the real project before writing anything. Generating from the prompt
	// alone is what produced code that ignored the actual codebase.
	var projectCode string
	if sCtx.WorkspacePath != "" {
		engine := NewContextEngine(sCtx.WorkspacePath)
		if code, err := engine.BuildCodeContext(sCtx.UserPrompt, 8); err == nil && code != "" {
			projectCode = code
		}
		if structure, err := engine.BuildStructure(); err == nil && structure != "" {
			projectCode += "\n--- PROJECT STRUCTURE ---\n" + structure
		}
	}

	userPrompt := fmt.Sprintf("Goal: %s\n%s", sCtx.UserPrompt, memories)
	if projectCode != "" {
		userPrompt += "\n\nEXISTING PROJECT CODE (ground truth):\n" + projectCode
	}
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

	model := c.getModelForRole(sCtx)
	apiKey, baseURL, err := c.getAPIKey(model, sCtx)
	if err != nil {
		return nil, err
	}
	cleanModel := c.cleanModelName(model)

	callCtx := ctx
	var cancel context.CancelFunc
	if cleanModel != "flash_5" {
		callCtx, cancel = context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
	}

	resp, err := c.llmClient.Generate(callCtx, llm.Request{
		Model:        cleanModel,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
		APIKey:       apiKey,
		BaseURL:      baseURL,
	})
	if err != nil {
		if cleanModel != "flash_5" {
			log.Printf("Coder model %s failed or timed out (%v); falling back to flash_5", cleanModel, err)
			resp, err = c.llmClient.Generate(ctx, llm.Request{
				Model:        "flash_5",
				SystemPrompt: systemPrompt,
				UserPrompt:   userPrompt,
				JSONSchema:   map[string]interface{}{"type": "object"},
				APIKey:       apiKey,
				BaseURL:      baseURL,
			})
		}
		if err != nil {
			return nil, fmt.Errorf("coder generation failed: %w", err)
		}
	}

	var parsed coderResponse
	cleaned := CleanJSON(resp.Content)
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		// The model did not return the agreed JSON contract. Writing its raw
		// prose into main.go would corrupt the project, so fail loudly and let
		// the next round retry instead.
		return nil, fmt.Errorf("coder returned invalid JSON: %w (content: %s)", err, resp.Content)
	}
	if len(parsed.Files) == 0 {
		return nil, fmt.Errorf("coder returned no files")
	}

	sCtx.CodeFiles = parsed.Files

	summary := strings.TrimSpace(parsed.Explanation)
	if summary == "" {
		summary = fmt.Sprintf("کدنویسی فایل‌های پروژه انجام شد (%d فایل).", len(parsed.Files))
	}
	reason := resp.Reasoning
	if reason == "" {
		reason = "پیاده‌سازی کدهای تمیز و بدون باگ مطابق نیازهای پروژه."
	}
	return c.createMessageWithReasoning(sCtx, domain.MsgCodeDraft, parsed, summary, reason)
}

