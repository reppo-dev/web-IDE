package orchestrator

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

type mockLLM struct{}

func (m *mockLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	switch {
	// The architect triages the request before doing any design work.
	case strings.HasPrefix(req.SystemPrompt, "You are the request triage layer"):
		if strings.Contains(req.UserPrompt, "Hello") || strings.Contains(req.UserPrompt, "سلام") {
			return &llm.Response{
				Content: `{"kind":"conversation","reply":"سلام! در خدمتم.","reason":"Greeting"}`,
			}, nil
		}
		return &llm.Response{
			Content: `{"kind":"build","reply":"","reason":"Explicit build request","reworded":"Create a Go Ping API"}`,
		}, nil

	case strings.Contains(req.SystemPrompt, "relay discussion"):
		return &llm.Response{
			Content: `{"thought":"تحلیل دیدگاه و تفکر","reply":"دیدگاه عاملی در گفتگو"}`,
		}, nil

	case strings.HasPrefix(req.SystemPrompt, "You are the System Architect"):
		return &llm.Response{
			Content: `{"project_overview":"Go HTTP API","file_tree":["main.go"],"key_components":["Router"],"api_contracts":["GET /ping"]}`,
		}, nil

	case strings.Contains(req.SystemPrompt, "Project Planner"):
		return &llm.Response{
			Content: `{"goal":"Create a Go Ping API","tasks":[{"id":"T-001","title":"Create main.go","acceptance":"main.go exists and compiles"}]}`,
		}, nil

	case strings.Contains(req.SystemPrompt, "strict verifier"):
		return &llm.Response{
			Content: `{"gaps":[],"summary":"Task T-001 verified against real code."}`,
		}, nil
	}
	return &llm.Response{
		Content: `{"files":[{"path":"main.go","language":"go","content":"package main\n\nfunc main() {}"}]}`,
	}, nil
}

// testModels assigns every agent an explicit model, which agents now require.
func testModels() map[domain.AgentRole]string {
	return map[domain.AgentRole]string{
		domain.RoleArchitect: "openrouter/free",
		domain.RoleCoder:     "openrouter/free",
		domain.RoleCritic:    "openrouter/free",
		domain.RoleSecurity:  "openrouter/free",
		domain.RoleArbiter:   "openrouter/free",
	}
}

func testAPIKeys() map[string]string {
	return map[string]string{"openrouter": "test-key"}
}

func setupTestOrchestrator(mode domain.ExecutionMode) *Orchestrator {
	mockClient := &mockLLM{}
	arch := agent.NewArchitectAgent(mockClient)
	coder := agent.NewCoderAgent(mockClient)
	critic := agent.NewCriticAgent(mockClient)
	sec := agent.NewSecurityAgent(mockClient)
	arb := agent.NewArbiterAgent(mockClient)
	matrix := agent.NewMatrixEngine(mockClient)

	cfg := Config{
		MaxRounds: 3,
		Mode:      mode,
	}
	tempDir, _ := os.MkdirTemp("", "orch-test-*")
	return NewOrchestrator(cfg, arch, coder, critic, sec, arb, nil, matrix, tempDir)
}

func TestOrchestratorStopsOnGreeting(t *testing.T) {
	greetingClient := &greetingLLM{}
	arch := agent.NewArchitectAgent(greetingClient)
	orch := NewOrchestrator(
		Config{MaxRounds: 3, Mode: domain.ModeRoleBased},
		arch,
		agent.NewCoderAgent(greetingClient),
		agent.NewCriticAgent(greetingClient),
		agent.NewSecurityAgent(greetingClient),
		agent.NewArbiterAgent(greetingClient),
		nil,
		agent.NewMatrixEngine(greetingClient),
		t.TempDir(),
	)

	sCtx, err := orch.RunWithOptions(context.Background(), "greet-proj", "سلام",
		domain.ModeRoleBased, testModels(), testAPIKeys())
	if err != nil {
		t.Fatalf("greeting run failed: %v", err)
	}
	if sCtx.Intent == nil || sCtx.Intent.Kind != domain.IntentConversation {
		t.Fatalf("expected a conversation triage, got %+v", sCtx.Intent)
	}
	if sCtx.Blueprint != nil {
		t.Error("a greeting must not produce a blueprint")
	}
	if len(sCtx.CodeFiles) != 0 {
		t.Errorf("a greeting must not produce code, got %d files", len(sCtx.CodeFiles))
	}
	if sCtx.ArbiterVerdict != nil {
		t.Error("a greeting must not be judged by the arbiter")
	}
}

// greetingLLM reports every request as small talk.
type greetingLLM struct{}

func (m *greetingLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	if strings.HasPrefix(req.SystemPrompt, "You are the request triage layer") {
		return &llm.Response{
			Content: `{"kind":"conversation","reply":"سلام! چطور می‌تونم کمک کنم؟","reason":"Greeting"}`,
		}, nil
	}
	return &llm.Response{Content: "{}"}, nil
}

func TestOrchestrator_Modes(t *testing.T) {
	modes := []domain.ExecutionMode{
		domain.ModeSequential,
		domain.ModeRoleBased,
		domain.ModeConcurrent,
	}

	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			orch := setupTestOrchestrator(mode)
			eventCh := orch.Subscribe()
			defer orch.Unsubscribe(eventCh)

			var receivedEvents []SwarmEvent
			doneListening := make(chan struct{})

			go func() {
				defer close(doneListening)
				for {
					select {
					case evt, ok := <-eventCh:
						if !ok {
							return
						}
						receivedEvents = append(receivedEvents, evt)
						if evt.Type == EventConsensusReached {
							return
						}
					case <-time.After(2 * time.Second):
						return
					}
				}
			}()

			ctx := context.Background()
			sCtx, err := orch.RunWithOptions(ctx, "test-proj", "Create a Go Ping API", mode, testModels(), testAPIKeys())
			if err != nil {
				t.Fatalf("mode %s failed: %v", mode, err)
			}

			<-doneListening

			if sCtx == nil || sCtx.ArbiterVerdict == nil {
				t.Fatalf("mode %s produced nil result", mode)
			}

			if !sCtx.ArbiterVerdict.Passed {
				t.Errorf("mode %s expected consensus to pass", mode)
			}

			if len(receivedEvents) == 0 {
				t.Errorf("mode %s expected events on subscription channel", mode)
			}
		})
	}
}

func TestRelayDiscussion_AllAgentsParticipate(t *testing.T) {
	orch := setupTestOrchestrator(domain.ModeRoleBased)
	eventCh := orch.Subscribe()
	defer orch.Unsubscribe(eventCh)

	var receivedAgents []domain.AgentRole
	doneListening := make(chan struct{})

	go func() {
		defer close(doneListening)
		for {
			select {
			case evt, ok := <-eventCh:
				if !ok {
					return
				}
				if evt.Type == EventAgentOutput && evt.Message != nil {
					receivedAgents = append(receivedAgents, evt.Message.From)
				}
				if evt.Type == EventConsensusReached {
					return
				}
			case <-time.After(3 * time.Second):
				return
			}
		}
	}()

	ctx := context.Background()
	_, err := orch.RunWithOptions(ctx, "test-proj", "سلام! چطورید؟", domain.ModeRoleBased, testModels(), testAPIKeys())
	if err != nil {
		t.Fatalf("relay discussion failed: %v", err)
	}

	<-doneListening

	// Verify all 5 agents spoke in sequence: Architect, Coder, Critic, Security, Arbiter
	expected := []domain.AgentRole{
		domain.RoleArchitect,
		domain.RoleCoder,
		domain.RoleCritic,
		domain.RoleSecurity,
		domain.RoleArbiter,
	}

	if len(receivedAgents) != len(expected) {
		t.Fatalf("expected 5 agents to speak, got %d: %v", len(receivedAgents), receivedAgents)
	}

	for i, exp := range expected {
		if receivedAgents[i] != exp {
			t.Errorf("agent at step %d: expected %s, got %s", i, exp, receivedAgents[i])
		}
	}
}

