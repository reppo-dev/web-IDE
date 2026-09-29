package orchestrator

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

type mockLLM struct{}

func (m *mockLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	// Returns valid JSON based on prompt
	if req.SystemPrompt != "" {
		if req.SystemPrompt[:10] == "You are th" {
			// Architect
			return &llm.Response{
				Content: `{"project_overview":"Go HTTP API","file_tree":["main.go"],"key_components":["Router"],"api_contracts":["GET /ping"]}`,
			}, nil
		}
	}
	return &llm.Response{
		Content: `{"files":[{"path":"main.go","language":"go","content":"package main\n\nfunc main() {}"}]}`,
	}, nil
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
			sCtx, err := orch.Run(ctx, "test-proj", "Create a Go Ping API")
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
