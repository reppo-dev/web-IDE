package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

type ArchitectAgent struct {
	*BaseAgent
}

func NewArchitectAgent(client llm.Client) *ArchitectAgent {
	return &ArchitectAgent{
		BaseAgent: NewBaseAgent(domain.RoleArchitect, client),
	}
}

// classifyIntent triages the user message before any code is generated. It
// costs one cheap model call and prevents the swarm from inventing a project
// out of a greeting or a question.
func (a *ArchitectAgent) classifyIntent(ctx context.Context, sCtx *SwarmContext, model, apiKey, baseURL string) (*domain.IntentRoute, error) {
	systemPrompt := `You are the request triage layer of an autonomous coding team.
Analyze the user's message and respond naturally.

Respond with ONLY this JSON:
{
  "kind": "conversation" | "question" | "build",
  "reply": "Your natural, direct, friendly, and complete response to the user. Greet back warmly if they greet (e.g. 'سلام' -> 'سلام! در خدمتم...'). If they ask a question, answer it clearly. ALWAYS WRITE IN THE EXACT SAME LANGUAGE AS THE USER.",
  "reason": "Your internal thoughts and reasoning in the SAME LANGUAGE as the user.",
  "reworded": "For kind=build only: the specific coding task restated clearly. Otherwise empty."
}

Guidance:
- Greetings, small talk, pleasantries (e.g. 'سلام', 'درود', 'hi', 'hello', 'خوبی') -> 'conversation'.
- General questions, explanations, asking about concepts -> 'question'.
- Direct commands to write code, create files, implement features, or build (e.g. 'برو کار کن', 'کد بنویس', 'پروژه بساز', 'یک فاکتوریل بنویس', 'build', 'create') -> 'build'.
- CRITICAL: Never force English when the user speaks Persian. The AI must speak and think naturally in the user's language.`

	userPrompt := fmt.Sprintf("User message: %s", sCtx.UserPrompt)

	resp, err := a.llmClient.Generate(ctx, llm.Request{
		Model:        model,
		APIKey:       apiKey,
		BaseURL:      baseURL,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
	})
	if resp != nil {
		sCtx.RecordAgentUsage(string(domain.RoleArchitect), model, resp.PromptTokens, resp.OutputTokens)
	}
	if err != nil {
		return nil, fmt.Errorf("intent classification failed: %w", err)
	}

	var route domain.IntentRoute
	cleaned := CleanJSON(resp.Content)
	if err := json.Unmarshal([]byte(cleaned), &route); err != nil {
		// If triage itself is unusable, fall back to a helpful default response
		// rather than echoing the user's prompt as the AI's reply.
		fallbackReply := "درود! پیام شما دریافت شد. در حال بررسی درخواست و آماده‌سازی سیستم برای پاسخگویی به شما هستم."
		if strings.Contains(sCtx.UserPrompt, "سلام") || strings.Contains(sCtx.UserPrompt, "درود") {
			fallbackReply = "سلام! در خدمتم. چه کاری یا پروژه‌ای مد نظرتان است تا با تیم شروع کنیم؟"
		}
		return &domain.IntentRoute{
			Kind:   domain.IntentQuestion,
			Reply:  fallbackReply,
			Reason: "تحلیل پیام کاربر و هدایت هوشمند گفتگو.",
		}, nil
	}

	// Never allow the AI's reply to echo the user prompt or be empty
	if strings.TrimSpace(route.Reply) == "" || strings.TrimSpace(route.Reply) == strings.TrimSpace(sCtx.UserPrompt) {
		if strings.Contains(sCtx.UserPrompt, "سلام") || strings.Contains(sCtx.UserPrompt, "درود") {
			route.Reply = "سلام! در خدمتم. چه کاری یا پروژه‌ای مد نظرتان است تا با تیم شروع کنیم؟"
		} else {
			route.Reply = "درود! در خدمتم، آماده‌ام تا به عنوان معمار سیستم به پیشبرد اهداف شما کمک کنم."
		}
	}
	if strings.TrimSpace(route.Reason) == "" || strings.TrimSpace(route.Reason) == strings.TrimSpace(sCtx.UserPrompt) {
		route.Reason = "بررسی نیت کاربر و ارائه پاسخ متناسب در نقش معمار سیستم."
	}

	switch route.Kind {
	case domain.IntentBuild:
		if route.Reworded != "" {
			sCtx.UserPrompt = route.Reworded
		}
	default:
		// Any other intent is treated as question/conversation.
	}
	return &route, nil
}

func (a *ArchitectAgent) Execute(ctx context.Context, sCtx *SwarmContext) (*domain.AgentMessage, error) {
	memories := a.fetchMemories(ctx, sCtx)

	systemPrompt := `You are the System Architect in NexusAI.
Your role:
- Analyze user requests (whether in Persian or English).
- Decompose requirements into structured file trees, architecture designs, and API contracts.
- For simple requests or single functions, plan 1-2 concise files (e.g. main.go or main_test.go) for rapid execution.
- If the user prompt is in Persian, reason and describe components in fluent Persian, but keep file paths and API contracts in standard English.
- Return ONLY valid JSON conforming to the following structure:
{
  "project_overview": "Summary of the system architecture in the user's language",
  "file_tree": ["path/to/file1.go"],
  "key_components": ["Component description in the user's language"],
  "api_contracts": ["Endpoint/Interface specs"]
}`

	model := a.getModelForRole(sCtx)
	apiKey, baseURL, err := a.getAPIKey(model, sCtx)
	if err != nil {
		return nil, err
	}
	cleanModel := a.cleanModelName(model)

	// Triage first: greetings and questions must not trigger a build. This must
	// run before the blueprint prompt is composed, because triage may restate
	// the request as a clearer engineering task.
	intent, err := a.classifyIntent(ctx, sCtx, cleanModel, apiKey, baseURL)
	if err != nil {
		return nil, err
	}
	if intent.Kind != domain.IntentBuild {
		sCtx.Intent = intent
		reply := strings.TrimSpace(intent.Reply)
		if reply == "" {
			if strings.Contains(sCtx.UserPrompt, "سلام") || strings.Contains(sCtx.UserPrompt, "درود") {
				reply = "سلام! در خدمتم. چه کاری یا پروژه‌ای مد نظرتونه؟"
			} else {
				reply = "درود! آماده به کارم، بفرمایید چه کمکی می‌تونم انجام بدم؟"
			}
		}
		reason := strings.TrimSpace(intent.Reason)
		if reason == "" {
			reason = "تحلیل درخواست کاربر و پاسخ متناسب به زبان پیام."
		}
		return a.createMessageWithReasoning(sCtx, domain.MsgBlueprint, intent, reply, reason)
	}

	userPrompt := fmt.Sprintf("User Request: %s\n%s", sCtx.UserPrompt, memories)

	resp, err := a.llmClient.Generate(ctx, llm.Request{
		Model:        cleanModel,
		APIKey:       apiKey,
		BaseURL:      baseURL,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
	})
	var usage *domain.AgentUsage
	if resp != nil {
		usage = sCtx.RecordAgentUsage(string(domain.RoleArchitect), cleanModel, resp.PromptTokens, resp.OutputTokens)
	}
	if err != nil {
		return nil, fmt.Errorf("architect generation failed: %w", err)
	}

	var blueprint domain.BlueprintPayload
	cleaned := CleanJSON(resp.Content)
	if err := json.Unmarshal([]byte(cleaned), &blueprint); err != nil {
		// Writing the model's prose into a fake file tree would fabricate a
		// project the user never asked for, so surface the problem instead.
		return nil, fmt.Errorf("architect returned an invalid blueprint: %w (content: %s)", err, resp.Content)
	}

	sCtx.Blueprint = &blueprint

	// Write decision to memory
	if sCtx.MemStore != nil {
		_ = sCtx.MemStore.WriteProject(ctx, domain.MemoryEntry{
			ProjectID:  sCtx.ProjectID,
			Agent:      domain.RoleArchitect,
			Type:       domain.MemoryDecision,
			Content:    fmt.Sprintf("Architectural Blueprint designed: %s", blueprint.ProjectOverview),
			Tags:       []string{"blueprint", "architecture"},
			Round:      sCtx.Round,
			Confidence: 0.95,
		})
	}

	summary := strings.TrimSpace(blueprint.ProjectOverview)
	if summary == "" {
		summary = fmt.Sprintf("معماری و ساختار پروژه با %d فایل طراحی شد.", len(blueprint.FileTree))
	}
	reason := resp.Reasoning
	if reason == "" {
		reason = "تحلیل نیازمندی‌ها، لایه‌بندی معماری و ساختار فایل‌های پروژه."
	}
	msg, err := a.createMessageWithReasoning(sCtx, domain.MsgBlueprint, blueprint, summary, reason)
	if err == nil && msg != nil && usage != nil {
		msg.Tokens = usage
	}
	return msg, err
}
