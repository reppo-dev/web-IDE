package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	for _, f := range sCtx.CodeFilesSnapshot() {
		codeContent += fmt.Sprintf("--- File: %s (%s) ---\n%s\n\n", f.Path, f.Language, f.Content)
	}

	userPrompt := fmt.Sprintf("Project Goal: %s\n%s\nCodebase:\n%s", sCtx.UserPrompt, memories, codeContent)
	userPrompt += activeTaskBrief(sCtx)

	model := cr.getModelForRole(sCtx)
	apiKey, baseURL, err := cr.getAPIKey(model, sCtx)
	if err != nil {
		return nil, err
	}
	cleanModel := cr.cleanModelName(model)

	callCtx := ctx
	var cancel context.CancelFunc
	if cleanModel != "flash_5" {
		callCtx, cancel = context.WithTimeout(ctx, 35*time.Second)
		defer cancel()
	}

	resp, err := cr.llmClient.Generate(callCtx, llm.Request{
		Model:        cleanModel,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
		APIKey:       apiKey,
		BaseURL:      baseURL,
	})
	if err != nil {
		if cleanModel != "flash_5" {
			resp, err = cr.llmClient.Generate(ctx, llm.Request{
				Model:        "flash_5",
				SystemPrompt: systemPrompt,
				UserPrompt:   userPrompt,
				JSONSchema:   map[string]interface{}{"type": "object"},
				APIKey:       apiKey,
				BaseURL:      baseURL,
			})
		}
		if err != nil {
			return nil, fmt.Errorf("critic review failed: %w", err)
		}
	}

	var payload domain.CriticPayload
	cleaned := CleanJSON(resp.Content)
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		// Keep the raw review as prose, but flag it so the arbiter does not
		// mistake a non-JSON answer for "no issues found".
		payload = domain.CriticPayload{
			GeneralReview: resp.Content,
			Issues: []domain.ReviewIssue{{
				Severity:    "major",
				File:        "(unparsed)",
				Description: "Critic returned an unparseable review; treat this round as unreviewed.",
			}},
		}
	}

	// The critic's own findings are merged with any issues the pipeline already
	// recorded rather than replacing them. Assignment here used to discard the
	// compiler's errors, which let a project that did not compile be scored on
	// the critic's opinion alone.
	if sCtx.CriticReview == nil {
		sCtx.CriticReview = &domain.CriticPayload{}
	}
	sCtx.CriticReview.GeneralReview = payload.GeneralReview
	sCtx.CriticReview.RefactorPlan = payload.RefactorPlan
	sCtx.CriticReview.Issues = append(sCtx.CriticReview.Issues, payload.Issues...)

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

	summary := strings.TrimSpace(payload.GeneralReview)
	if summary == "" {
		crit := countCritical(payload.Issues)
		if len(payload.Issues) == 0 {
			summary = "بازبینی کد انجام شد: هیچ باگ یا خطای منطقی یافت نشد."
		} else {
			summary = fmt.Sprintf("بازبینی کد انجام شد: %d باگ شناسایی شد (%d بحرانی).", len(payload.Issues), crit)
		}
	}
	reason := resp.Reasoning
	if reason == "" {
		reason = "بررسی کیفیت، ساختار و پایداری کد برای کشف باگ‌ها و مشکلات عملکردی."
	}
	return cr.createMessageWithReasoning(sCtx, domain.MsgCriticReview, payload, summary, reason)
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

