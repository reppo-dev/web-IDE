package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
)

func (o *Orchestrator) runConcurrent(ctx context.Context, projectID string, userPrompt string) (*agent.SwarmContext, error) {
	sCtx := &agent.SwarmContext{
		ProjectID:  projectID,
		Round:      1,
		UserPrompt: userPrompt,
		MemStore:   o.memStore,
	}

	// Channel for inter-agent pipeline signals
	msgChan := make(chan *domain.AgentMessage, 20)

	for round := 1; round <= o.config.MaxRounds; round++ {
		sCtx.Round = round
		o.broadcast(SwarmEvent{
			Type:      EventRoundStarted,
			ProjectID: projectID,
			Round:     round,
		})

		// Step 1: Architect (Round 1)
		if round == 1 {
			o.broadcast(SwarmEvent{
				Type:      EventAgentThinking,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleArchitect,
			})
			archMsg, err := o.architect.Execute(ctx, sCtx)
			if err != nil {
				return nil, fmt.Errorf("concurrent: architect failed: %w", err)
			}
			o.broadcast(SwarmEvent{
				Type:      EventAgentOutput,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleArchitect,
				Message:   archMsg,
			})
		}

		// Step 2: Coder
		o.broadcast(SwarmEvent{
			Type:      EventAgentThinking,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleCoder,
		})
		coderMsg, err := o.coder.Execute(ctx, sCtx)
		if err != nil {
			return nil, fmt.Errorf("concurrent: coder failed: %w", err)
		}
		o.broadcast(SwarmEvent{
			Type:      EventAgentOutput,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleCoder,
			Message:   coderMsg,
			Files:     sCtx.CodeFiles,
		})
		o.broadcast(SwarmEvent{
			Type:      EventCodeUpdated,
			ProjectID: projectID,
			Round:     round,
			Files:     sCtx.CodeFiles,
		})

		// Step 3: Fan-out to Critic and Security Auditor concurrently via channels
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			o.broadcast(SwarmEvent{
				Type:      EventAgentThinking,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleCritic,
			})
			cMsg, cErr := o.critic.Execute(ctx, sCtx)
			if cErr == nil {
				msgChan <- cMsg
				o.broadcast(SwarmEvent{
					Type:      EventAgentOutput,
					ProjectID: projectID,
					Round:     round,
					Agent:     domain.RoleCritic,
					Message:   cMsg,
				})
			}
		}()

		go func() {
			defer wg.Done()
			o.broadcast(SwarmEvent{
				Type:      EventAgentThinking,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleSecurity,
			})
			sMsg, sErr := o.security.Execute(ctx, sCtx)
			if sErr == nil {
				msgChan <- sMsg
				o.broadcast(SwarmEvent{
					Type:      EventAgentOutput,
					ProjectID: projectID,
					Round:     round,
					Agent:     domain.RoleSecurity,
					Message:   sMsg,
				})
			}
		}()

		wg.Wait()

		// Drain messages for this round
		received := 0
		for received < 2 {
			select {
			case <-msgChan:
				received++
			default:
				received = 2
			}
		}

		// Step 4: Arbiter evaluates
		o.broadcast(SwarmEvent{
			Type:      EventAgentThinking,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleArbiter,
		})
		arbMsg, err := o.arbiter.Execute(ctx, sCtx)
		if err != nil {
			return nil, fmt.Errorf("concurrent: arbiter failed: %w", err)
		}
		o.broadcast(SwarmEvent{
			Type:      EventAgentOutput,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleArbiter,
			Message:   arbMsg,
			Verdict:   sCtx.ArbiterVerdict,
		})

		o.broadcast(SwarmEvent{
			Type:      EventRoundComplete,
			ProjectID: projectID,
			Round:     round,
			Verdict:   sCtx.ArbiterVerdict,
		})

		if sCtx.ArbiterVerdict != nil && sCtx.ArbiterVerdict.Passed {
			o.broadcast(SwarmEvent{
				Type:      EventConsensusReached,
				ProjectID: projectID,
				Round:     round,
				Verdict:   sCtx.ArbiterVerdict,
				Files:     sCtx.CodeFiles,
			})
			return sCtx, nil
		}
	}

	return sCtx, nil
}
