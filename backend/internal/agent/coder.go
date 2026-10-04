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
		// State the boundary so the model does not waste a turn proposing a
		// path the workspace would reject anyway.
		projectCode += fmt.Sprintf(
			"\n--- SCOPE ---\nYou may only touch files inside %s\n"+
				"Every \"path\" must be relative to that folder. No absolute paths, no .. references.\n",
			sCtx.WorkspacePath)
	}

	userPrompt := fmt.Sprintf("Goal: %s\n%s", sCtx.UserPrompt, memories)

	// The active task is what this round is for. Without it the coder only ever
	// sees the original goal, so every round produces the same slice of work and
	// a multi-task project never advances past its first task.
	if task := sCtx.ActiveTaskSnapshot(); task != nil {
		userPrompt += fmt.Sprintf("\n\nTASK: %s\nTASK TITLE: %s", task.ID, task.Title)
		if task.Acceptance != "" {
			userPrompt += fmt.Sprintf("\nDONE WHEN: %s", task.Acceptance)
		}
		if len(task.Files) > 0 {
			userPrompt += fmt.Sprintf("\nEXPECTED FILES: %s", strings.Join(task.Files, ", "))
		}
		// Naming what is already finished keeps the coder from redoing it.
		if done := completedTitles(sCtx); len(done) > 0 {
			userPrompt += fmt.Sprintf("\nALREADY COMPLETED (do not redo): %s", strings.Join(done, "; "))
		}
		userPrompt += "\nWork ONLY on this task. Do not start work that belongs to another task."
	}

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
	actualModel := cleanModel
	if err != nil {
		if cleanModel != "flash_5" {
			log.Printf("Coder model %s failed or timed out (%v); falling back to flash_5", cleanModel, err)
			actualModel = "flash_5"
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

	var usage *domain.AgentUsage
	if resp != nil {
		usage = sCtx.RecordAgentUsage(string(domain.RoleCoder), actualModel, resp.PromptTokens, resp.OutputTokens)
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

	sCtx.MergeCodeFiles(parsed.Files)

	summary := strings.TrimSpace(parsed.Explanation)
	if summary == "" {
		summary = fmt.Sprintf("کدنویسی فایل‌های پروژه انجام شد (%d فایل).", len(parsed.Files))
	}
	reason := resp.Reasoning
	if reason == "" {
		reason = "پیاده‌سازی کدهای تمیز و بدون باگ مطابق نیازهای پروژه."
	}
	msg, err := c.createMessageWithReasoning(sCtx, domain.MsgCodeDraft, parsed, summary, reason)
	if err == nil && msg != nil && usage != nil {
		msg.Tokens = usage
	}
	return msg, err
}


// completedTitles lists the tasks already verified done.
//
// Showing them to the coder is what stops a later round from re-implementing
// work that was accepted earlier: the model cannot see the plan's state on its
// own, only what the prompt tells it.
func completedTitles(sCtx *SwarmContext) []string {
	plan := sCtx.PlanSnapshot()
	if plan == nil {
		return nil
	}
	var out []string
	for _, t := range plan.Tasks {
		if t.State == domain.TaskDone {
			out = append(out, t.ID+" "+t.Title)
		}
	}
	return out
}

// activeTaskBrief describes the round's task and the project's remaining work.
//
// Review agents previously saw only the original goal, so they judged the whole
// project on every round and reported problems belonging to tasks that had not
// started yet. Telling them the active task and what is already finished keeps
// the review about the work this round actually did.
func activeTaskBrief(sCtx *SwarmContext) string {
	task := sCtx.ActiveTaskSnapshot()
	if task == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n\nCURRENT TASK: %s (%s)", task.ID, task.Title)
	if task.Acceptance != "" {
		fmt.Fprintf(&b, "\nIts condition for being done: %s", task.Acceptance)
	}
	if done := completedTitles(sCtx); len(done) > 0 {
		fmt.Fprintf(&b, "\nAlready completed, do not report as missing: %s", strings.Join(done, "; "))
	}
	if plan := sCtx.PlanSnapshot(); plan != nil {
		if open := plan.OpenTasks(); len(open) > 0 {
			var titles []string
			for _, t := range open {
				if t.ID != task.ID {
					titles = append(titles, t.Title)
				}
			}
			if len(titles) > 0 {
				fmt.Fprintf(&b, "\nStill not started, do not report as missing: %s", strings.Join(titles, "; "))
			}
		}
	}
	b.WriteString("\nReview only what belongs to the current task.")
	return b.String()
}
