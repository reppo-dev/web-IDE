package agent

import (
	"context"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

type mockLLMClient struct {
	response string
}

func (m *mockLLMClient) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	return &llm.Response{
		Content:      m.response,
		PromptTokens: 10,
		OutputTokens: 20,
	}, nil
}

func TestAgentSwarmPersonas(t *testing.T) {
	ctx := context.Background()
	sCtx := &SwarmContext{
		ProjectID:  "proj-test-1",
		Round:      1,
		UserPrompt: "Build a microservice in Go",
	}

	// 1. Architect Agent
	archClient := &mockLLMClient{
		response: `{"project_overview":"Microservice in Go","file_tree":["main.go"],"key_components":["API Server"],"api_contracts":["GET /health"]}`,
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
