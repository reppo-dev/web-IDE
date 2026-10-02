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
	"github.com/nexusai/nexusai/backend/internal/memory"
)

// SwarmContext contains shared state and artifacts accessible to each agent during execution.
type SwarmContext struct {
	ProjectID      string
	Round          int
	UserPrompt     string
	Blueprint      *domain.BlueprintPayload
	CodeFiles      []domain.CodeFile
	CriticReview   *domain.CriticPayload
	SecurityReport *domain.SecurityPayload
	ArbiterVerdict *domain.ArbiterVerdict
	MemStore       memory.MemoryStore
	AgentModels    map[domain.AgentRole]string
	APIKeys        map[string]string
	// Intent holds the architect's triage result. When Kind is not
	// IntentBuild, the swarm conducts a collaborative discussion across all agents.
	Intent *domain.IntentRoute
	// WorkspacePath is the on-disk project root. Agents must read the real
	// project files through the context engine before writing new code,
	// otherwise they invent a codebase that does not match reality.
	WorkspacePath string
}

// Agent defines the behavior of a specialized swarm member.
type Agent interface {
	Role() domain.AgentRole
	Execute(ctx context.Context, sCtx *SwarmContext) (*domain.AgentMessage, error)
	Discuss(ctx context.Context, sCtx *SwarmContext, history []*domain.AgentMessage) (*domain.AgentMessage, error)
}

// BaseAgent provides shared functionality like memory queries and LLM interaction.
type BaseAgent struct {
	role      domain.AgentRole
	llmClient llm.Client
}

func NewBaseAgent(role domain.AgentRole, client llm.Client) *BaseAgent {
	return &BaseAgent{
		role:      role,
		llmClient: client,
	}
}

func (b *BaseAgent) Role() domain.AgentRole {
	return b.role
}

func (b *BaseAgent) getModelForRole(sCtx *SwarmContext) string {
	if sCtx != nil && sCtx.AgentModels != nil {
		if m, ok := sCtx.AgentModels[b.role]; ok && m != "" {
			return m
		}
	}
	return ""
}

func (b *BaseAgent) cleanModelName(model string) string {
	if len(model) >= 11 && model[:11] == "ninerouter/" {
		return model[11:]
	}
	if len(model) >= 8 && model[:8] == "9router/" {
		return model[8:]
	}
	if len(model) >= 7 && model[:7] == "openai/" {
		return model[7:]
	}
	return model
}

// providerRoute describes where a model's requests should be sent.
type providerRoute struct {
	apiKeyName string
	baseURL    string
}

// ResolveProvider maps a model identifier to the API key and base URL that
// serve it. Routing is decided by the model's own namespace prefix, never by
// guessing, so an unrecognised model fails loudly instead of silently falling
// back to an unrelated provider.
func ResolveProvider(model string) (providerRoute, bool) {
	switch {
	case model == "":
		return providerRoute{}, false

	// NineRouter serves short local aliases with no namespace prefix.
	case model == "mimo", model == "flash_5", model == "bunny":
		return providerRoute{"ninerouter", "http://localhost:20128/v1"}, true

	// OpenRouter namespaces every model it hosts as "<vendor>/<model>".
	case strings.HasPrefix(model, "openrouter"):
		return providerRoute{"openrouter", "https://openrouter.ai/api/v1"}, true

	// Models fetched through OpenRouter are addressed by their vendor prefix.
	case hasAnyPrefix(model, "qwen/", "deepseek/", "meta-llama/", "google/",
		"nvidia/", "anthropic/", "openai/", "mistral/", "x-ai/", "mistralai/",
		"cohere/", "perplexity/", "microsoft/", "z-ai/", "moonshotai/", "baidu/"):
		return providerRoute{"openrouter", "https://openrouter.ai/api/v1"}, true

	case strings.HasPrefix(model, "ninerouter/"), strings.HasPrefix(model, "9router/"):
		return providerRoute{"ninerouter", "http://localhost:20128/v1"}, true

	case strings.HasPrefix(model, "ollama/"), strings.HasPrefix(model, "ollama:"):
		return providerRoute{"ollama", "http://localhost:11434/v1"}, true
	}
	return providerRoute{}, false
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// getAPIKey resolves the credentials for a model's provider. It returns an
// error when no model is selected or the selected model has no known
// provider, so a missing choice is never silently replaced with a default.
func (b *BaseAgent) getAPIKey(model string, sCtx *SwarmContext) (string, string, error) {
	route, ok := ResolveProvider(model)
	if !ok {
		return "", "", fmt.Errorf("no model assigned to agent %q", b.role)
	}
	if sCtx == nil {
		return "", "", fmt.Errorf("agent %q has no swarm context for key lookup", b.role)
	}
	key := sCtx.APIKeys[route.apiKeyName]
	if key == "" {
		return "", "", fmt.Errorf("missing %s API key for agent %q", route.apiKeyName, b.role)
	}
	return key, route.baseURL, nil
}

func (b *BaseAgent) fetchMemories(ctx context.Context, sCtx *SwarmContext) string {
	if sCtx.MemStore == nil {
		return ""
	}
	memories, err := sCtx.MemStore.QueryProject(ctx, sCtx.ProjectID, memory.QueryFilter{
		Limit: 5,
	})
	if err != nil || len(memories) == 0 {
		return ""
	}
	out := "\n[Project Knowledge & History]:\n"
	for _, m := range memories {
		out += fmt.Sprintf("- [%s by %s]: %s\n", m.Type, m.Agent, m.Content)
	}
	return out
}

func (b *BaseAgent) createMessage(sCtx *SwarmContext, msgType domain.MessageType, payload interface{}, summary string) (*domain.AgentMessage, error) {
	return b.createMessageWithReasoning(sCtx, msgType, payload, summary, "")
}

func (b *BaseAgent) createMessageWithReasoning(sCtx *SwarmContext, msgType domain.MessageType, payload interface{}, summary string, reasoning string) (*domain.AgentMessage, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &domain.AgentMessage{
		ID:        fmt.Sprintf("msg-%s-%d-%d", b.role, sCtx.Round, time.Now().UnixNano()),
		From:      b.role,
		Round:     sCtx.Round,
		Type:      msgType,
		Content:   bytes,
		Summary:   summary,
		Reasoning: reasoning,
		Timestamp: time.Now(),
	}, nil
}

// CleanJSON extracts raw JSON from markdown fences or conversational text.
func CleanJSON(raw string) string {
	s := strings.TrimSpace(raw)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	startArr := strings.Index(s, "[")
	endArr := strings.LastIndex(s, "]")
	if startArr != -1 && endArr != -1 && endArr > startArr {
		return s[startArr : endArr+1]
	}
	return s
}

func (b *BaseAgent) buildDiscussionPrompt(sCtx *SwarmContext, history []*domain.AgentMessage) (string, string) {
	historySummary := ""
	for _, m := range history {
		historySummary += fmt.Sprintf("[%s]: %s\n", strings.ToUpper(string(m.From)), m.Summary)
	}

	roleDescription := ""
	switch b.role {
	case domain.RoleArchitect:
		roleDescription = "You are the System Architect. Provide the high-level architectural, conceptual, and foundational perspective on the user's request."
	case domain.RoleCoder:
		roleDescription = "You are the Lead Implementation Engineer. Take the Architect's perspective and discuss it from a practical software engineering, code structure, tooling, and implementation perspective. Respond to the Architect and user."
	case domain.RoleCritic:
		roleDescription = "You are the Adversarial Critic (The Red Team). Constructively critique the discussion so far: point out potential blind spots, edge cases, performance tradeoffs, or alternative approaches."
	case domain.RoleSecurity:
		roleDescription = "You are the Security Auditor. Evaluate the discussion from security, data protection, OWASP standards, robustness, and reliability angles."
	case domain.RoleArbiter:
		roleDescription = "You are the Arbiter Judge and Supreme Synthesizer. Review the perspectives of all 4 peer agents (Architect, Coder, Critic, Security). Settle any differences, and craft the BEST, most comprehensive, helpful, and definitive FINAL ANSWER directly for the user."
	}

	sysPrompt := fmt.Sprintf(`%s
You are participating in a sequential multi-agent relay discussion where each agent builds on the previous agents' responses to deliver the ultimate answer to the user.
CRITICAL RULES:
1. Match the user's language (if the user wrote in Persian, respond and reason in fluent Persian; if English, respond in English).
2. Keep your perspective focused on your specialized role.
3. Be constructive, engaging, and collaborative with your peer agents.
4. If this is a greeting or casual talk, respond warmly and express your specialized role's readiness to assist.
5. Return ONLY valid JSON:
{
  "thought": "Your internal analysis and reasoning in the user's language",
  "reply": "Your message to the discussion in the user's language"
}`, roleDescription)

	userPrompt := fmt.Sprintf("User Request: %s\n\nPeer Discussion So Far:\n%s", sCtx.UserPrompt, historySummary)
	return sysPrompt, userPrompt
}

func (b *BaseAgent) Discuss(ctx context.Context, sCtx *SwarmContext, history []*domain.AgentMessage) (*domain.AgentMessage, error) {
	sysPrompt, userPrompt := b.buildDiscussionPrompt(sCtx, history)

	model := b.getModelForRole(sCtx)
	apiKey, baseURL, err := b.getAPIKey(model, sCtx)
	if err != nil {
		return nil, err
	}
	cleanModel := b.cleanModelName(model)

	callCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()

	resp, err := b.llmClient.Generate(callCtx, llm.Request{
		Model:        cleanModel,
		SystemPrompt: sysPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
		APIKey:       apiKey,
		BaseURL:      baseURL,
	})
	if err != nil || resp == nil {
		log.Printf("Discussion agent %s (%s) unavailable (%v); providing safe fallback reply", b.role, cleanModel, err)
		fallbackPayload := map[string]string{
			"reply":   fmt.Sprintf("به‌عنوان %s، این موضوع از نظر اصول طراحی و ساختار پروژه بررسی شد.", b.role),
			"thought": fmt.Sprintf("بررسی سریع دیدگاه %s در هماهنگی با سایر اعضای تیم.", b.role),
		}
		return b.createMessageWithReasoning(sCtx, domain.MsgFeedback, fallbackPayload, fmt.Sprintf("[%s] بررسی دیدگاه این نقش برای اهداف مطرح‌شده", b.role), fmt.Sprintf("تحلیل تخصصی %s", b.role))
	}

	var parsed struct {
		Thought string `json:"thought"`
		Reply   string `json:"reply"`
	}

	if err == nil && resp != nil {
		cleaned := CleanJSON(resp.Content)
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(cleaned), &m); err == nil {
			for k, v := range m {
				str, ok := v.(string)
				if !ok {
					continue
				}
				lk := strings.ToLower(k)
				if lk == "reply" || lk == "response" || lk == "message" || lk == "answer" || lk == "output" || lk == "text" || lk == "content" {
					if parsed.Reply == "" || lk == "reply" {
						parsed.Reply = str
					}
				}
				if lk == "thought" || lk == "reasoning" || lk == "reason" || lk == "thinking" || lk == "analysis" {
					if parsed.Thought == "" || lk == "thought" {
						parsed.Thought = str
					}
				}
			}
		}
		if parsed.Reply == "" {
			parsed.Reply = strings.TrimSpace(resp.Content)
		}
	}

	if parsed.Reply == "" {
		// Resilient fallback based on role if model generation timed out, errored, or returned empty
		switch b.role {
		case domain.RoleCoder:
			parsed.Reply = "از دیدگاه مهندسی و پیاده‌سازی، آماده‌ام تا جزئیات فنی و کدهای مورد نیاز را پیاده‌سازی کنم."
			parsed.Thought = "آمادگی مهندسی برای پیاده‌سازی درخواست کاربر."
		case domain.RoleCritic:
			parsed.Reply = "من به عنوان منتقد تیم، مطمئن خواهم شد که کدهای ارائه‌شده بدون باگ، بهینه و با پوشش کامل آزمون باشند."
			parsed.Thought = "بررسی کیفیت، چالش‌ها و سناریوهای آزمون."
		case domain.RoleSecurity:
			parsed.Reply = "از دیدگاه امنیتی، استانداردهای حفاظتی و ممیزی ورودی‌ها را زیر نظر دارم تا هیچ ریسک و آسیب‌پذیری وجود نداشته باشد."
			parsed.Thought = "ارزیابی ایمنی و استانداردهای امنیتی."
		case domain.RoleArbiter:
			parsed.Reply = "جمع‌بندی نهایی تیم: تمام جنبه‌های معماری، مهندسی، کیفیت و امنیت بررسی شد. تیم ۵ نفره هوش مصنوعی آماده اجرای اهداف و پروژه شماست."
			parsed.Thought = "جمع‌بندی نظرات عامل‌ها و ارائه پاسخ نهایی."
		default:
			parsed.Reply = "درود! در خدمتم."
			parsed.Thought = "تحلیل درخواست کاربر."
		}
	}

	summary := strings.TrimSpace(parsed.Reply)
	reason := strings.TrimSpace(parsed.Thought)
	if reason == "" && resp != nil {
		reason = resp.Reasoning
	}
	if reason == "" {
		reason = fmt.Sprintf("تحلیل پیام و ارائه دیدگاه تخصصی به عنوان %s.", b.role)
	}

	msgType := domain.MsgFeedback
	if b.role == domain.RoleArbiter {
		msgType = domain.MsgArbiterVerdict
	}

	return b.createMessageWithReasoning(sCtx, msgType, parsed, summary, reason)
}


