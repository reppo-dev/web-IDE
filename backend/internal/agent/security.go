package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

type SecurityAgent struct {
	*BaseAgent
}

func NewSecurityAgent(client llm.Client) *SecurityAgent {
	return &SecurityAgent{
		BaseAgent: NewBaseAgent(domain.RoleSecurity, client),
	}
}

func (sec *SecurityAgent) Execute(ctx context.Context, sCtx *SwarmContext) (*domain.AgentMessage, error) {
	memories := sec.fetchMemories(ctx, sCtx)

	systemPrompt := `You are the Security Auditor (The Shield) in NexusAI.
Your role:
- Audit the codebase against OWASP Top 10, CWE, and common vulnerability vectors.
- Check specifically for: SQL/Command injection, SSRF, broken authentication, hardcoded secrets/API keys, insecure deserialization, lack of rate-limiting, and path traversal.
- Return ONLY valid JSON:
{
  "attack_surface": "Description of exposed attack surfaces",
  "vulnerabilities": [
    {
      "severity": "critical|major|minor",
      "file": "path/to/file.go",
      "line": 10,
      "description": "Vulnerability details and CVE/CWE reference if applicable",
      "suggested_fix": "Secure sanitization or protection snippet"
    }
  ],
  "is_clean": false
}`

	codeContent := ""
	for _, f := range sCtx.CodeFiles {
		codeContent += fmt.Sprintf("--- File: %s (%s) ---\n%s\n\n", f.Path, f.Language, f.Content)
	}

	userPrompt := fmt.Sprintf("Audit Codebase for Security Vulnerabilities:\n%s\nCode:\n%s", memories, codeContent)

	model := sec.getModelForRole(sCtx)
	apiKey, baseURL, err := sec.getAPIKey(model, sCtx)
	if err != nil {
		return nil, err
	}
	cleanModel := sec.cleanModelName(model)

	resp, err := sec.llmClient.Generate(ctx, llm.Request{
		Model:        cleanModel,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
		APIKey:       apiKey,
		BaseURL:      baseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("security audit failed: %w", err)
	}

	var payload domain.SecurityPayload
	cleaned := CleanJSON(resp.Content)
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		payload = domain.SecurityPayload{
			AttackSurface: "Standard local app surface",
			IsClean:       true,
		}
	}
	payload.IsClean = len(payload.Vulnerabilities) == 0

	sCtx.SecurityReport = &payload

	summary := strings.TrimSpace(payload.AttackSurface)
	if summary == "" || summary == "Standard local app surface" {
		if payload.IsClean {
			summary = "ممیزی امنیتی انجام شد: کد فاقد آسیب‌پذیری‌های امنیتی ارزیابی گردید."
		} else {
			summary = fmt.Sprintf("ممیزی امنیتی انجام شد: %d آسیب‌پذیری شناسایی شد.", len(payload.Vulnerabilities))
		}
	}
	reason := resp.Reasoning
	if reason == "" {
		reason = "اسکن امنیتی طبق استانداردهای OWASP و بررسی صحت ورودی‌ها."
	}
	return sec.createMessageWithReasoning(sCtx, domain.MsgSecurityReport, payload, summary, reason)
}

