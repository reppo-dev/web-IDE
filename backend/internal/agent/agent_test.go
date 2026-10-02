package agent

import (
	"context"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

type mockLLMClient struct {
	response string
	// responses, when set, is returned per call in order. The architect now
	// makes a triage call before its blueprint call, so one fixed response is
	// no longer enough for every agent.
	responses []string
	calls     int
}

func (m *mockLLMClient) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	content := m.response
	if len(m.responses) > 0 {
		if m.calls < len(m.responses) {
			content = m.responses[m.calls]
		} else {
			content = m.responses[len(m.responses)-1]
		}
	}
	m.calls++
	return &llm.Response{
		Content:      content,
		PromptTokens: 10,
		OutputTokens: 20,
	}, nil
}

// testContext returns a SwarmContext wired for a working OpenRouter-backed
// agent, since agents now refuse to run without an explicit model and key.
func testContext(prompt string) *SwarmContext {
	models := map[domain.AgentRole]string{}
	for _, r := range []domain.AgentRole{
		domain.RoleArchitect, domain.RoleCoder, domain.RoleCritic,
		domain.RoleSecurity, domain.RoleArbiter,
	} {
		models[r] = "openrouter/free"
	}
	return &SwarmContext{
		ProjectID:   "proj-test-1",
		Round:       1,
		UserPrompt:  prompt,
		AgentModels: models,
		APIKeys:     map[string]string{"openrouter": "test-key"},
	}
}

func TestGetAPIKeyRequiresExplicitModel(t *testing.T) {
	a := NewArchitectAgent(&mockLLMClient{})

	if _, _, err := a.getAPIKey("", nil); err == nil {
		t.Error("expected error when no model is assigned, got nil")
	}

	sCtx := testContext("build")
	if _, _, err := a.getAPIKey("totally-unknown-model", sCtx); err == nil {
		t.Error("expected error for unknown model, got nil")
	}

	sCtx.APIKeys = map[string]string{}
	if _, _, err := a.getAPIKey("openrouter/free", sCtx); err == nil {
		t.Error("expected error for missing key, got nil")
	}

	key, baseURL, err := a.getAPIKey("openrouter/free", testContext("build"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "test-key" {
		t.Errorf("expected test-key, got %q", key)
	}
	if baseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("unexpected baseURL %q", baseURL)
	}
}

func TestResolveProvider(t *testing.T) {
	cases := []struct {
		model   string
		keyName string
	}{
		{"openrouter/free", "openrouter"},
		{"deepseek/deepseek-r1", "openrouter"},
		{"google/gemini-2.5-flash", "openrouter"},
		{"mimo", "ninerouter"},
		{"flash_5", "ninerouter"},
		{"ninerouter/anything", "ninerouter"},
		{"ollama/deepseek-r1:8b", "ollama"},
	}
	for _, tc := range cases {
		route, ok := ResolveProvider(tc.model)
		if !ok {
			t.Errorf("model %q: expected a provider, got none", tc.model)
			continue
		}
		if route.apiKeyName != tc.keyName {
			t.Errorf("model %q: expected key %q, got %q", tc.model, tc.keyName, route.apiKeyName)
		}
	}

	if _, ok := ResolveProvider(""); ok {
		t.Error("empty model must not resolve to a provider")
	}
	if _, ok := ResolveProvider("random-gibberish"); ok {
		t.Error("unknown model must not resolve to a provider")
	}
}

func TestArchitectStopsOnGreeting(t *testing.T) {
	sCtx := testContext("سلام")
	client := &mockLLMClient{
		// First call is triage; a blueprint call would follow only for build.
		responses: []string{`{"kind":"conversation","reply":"سلام! چطور می‌تونم کمک کنم؟","reason":"Greeting"}`},
	}
	arch := NewArchitectAgent(client)

	if _, err := arch.Execute(context.Background(), sCtx); err != nil {
		t.Fatalf("architect failed: %v", err)
	}
	if sCtx.Intent == nil {
		t.Fatal("expected a triage result on the context")
	}
	if sCtx.Intent.Kind != domain.IntentConversation {
		t.Errorf("expected conversation, got %q", sCtx.Intent.Kind)
	}
	if sCtx.Blueprint != nil {
		t.Error("a greeting must not produce a blueprint")
	}
}

func TestAgentSwarmPersonas(t *testing.T) {
	ctx := context.Background()
	sCtx := testContext("Build a microservice in Go")

	// 1. Architect Agent (triage call first, then blueprint)
	archClient := &mockLLMClient{
		responses: []string{
			`{"kind":"build","reply":"","reason":"Explicit build request","reworded":"Build a microservice in Go"}`,
			`{"project_overview":"Microservice in Go","file_tree":["main.go"],"key_components":["API Server"],"api_contracts":["GET /health"]}`,
		},
	}
	arch := NewArchitectAgent(archClient)
	archMsg, err := arch.Execute(ctx, sCtx)
	if err != nil {
		t.Fatalf("architect failed: %v", err)
	}
	if archMsg.From != domain.RoleArchitect || sCtx.Blueprint == nil {
		t.Errorf("architect output invalid: %+v", archMsg)
	}

	// 2. Coder Agent
	coderClient := &mockLLMClient{
		response: `{"files":[{"path":"main.go","language":"go","content":"package main\n\nfunc main() {}"}],"explanation":"Base service"}`,
	}
	coder := NewCoderAgent(coderClient)
	coderMsg, err := coder.Execute(ctx, sCtx)
	if err != nil {
		t.Fatalf("coder failed: %v", err)
	}
	if coderMsg.From != domain.RoleCoder || len(sCtx.CodeFiles) != 1 {
		t.Errorf("coder output invalid: %+v", coderMsg)
	}

	// 3. Critic Agent
	criticClient := &mockLLMClient{
		response: `{"general_review":"Good baseline","issues":[{"severity":"minor","file":"main.go","description":"No graceful shutdown"}],"refactor_plan":"Add signal notify"}`,
	}
	critic := NewCriticAgent(criticClient)
	criticMsg, err := critic.Execute(ctx, sCtx)
	if err != nil {
		t.Fatalf("critic failed: %v", err)
	}
	if criticMsg.From != domain.RoleCritic || sCtx.CriticReview == nil {
		t.Errorf("critic output invalid: %+v", criticMsg)
	}

	// 4. Security Agent
	secClient := &mockLLMClient{
		response: `{"attack_surface":"Local port only","vulnerabilities":[],"is_clean":true}`,
	}
	sec := NewSecurityAgent(secClient)
	secMsg, err := sec.Execute(ctx, sCtx)
	if err != nil {
		t.Fatalf("security failed: %v", err)
	}
	if secMsg.From != domain.RoleSecurity || !sCtx.SecurityReport.IsClean {
		t.Errorf("security output invalid: %+v", secMsg)
	}

	// 5. Arbiter Agent
	arb := NewArbiterAgent(&mockLLMClient{response: ""})
	arbMsg, err := arb.Execute(ctx, sCtx)
	if err != nil {
		t.Fatalf("arbiter failed: %v", err)
	}
	if arbMsg.From != domain.RoleArbiter || sCtx.ArbiterVerdict == nil {
		t.Errorf("arbiter output invalid: %+v", arbMsg)
	}
	if sCtx.ArbiterVerdict.FinalScore <= 0 {
		t.Errorf("expected positive score, got %.2f", sCtx.ArbiterVerdict.FinalScore)
	}
}
