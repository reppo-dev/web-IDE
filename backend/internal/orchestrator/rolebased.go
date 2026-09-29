package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
)

func (o *Orchestrator) runRoleBased(ctx context.Context, projectID string, userPrompt string) (*agent.SwarmContext, error) {
	sCtx := &agent.SwarmContext{
		ProjectID:  projectID,
		Round:      1,
		UserPrompt: userPrompt,
		MemStore:   o.memStore,
	}

	for round := 1; round <= o.config.MaxRounds; round++ {
		sCtx.Round = round
		o.broadcast(SwarmEvent{
			Type:      EventRoundStarted,
			ProjectID: projectID,
			Round:     round,
		})

		// PHASE 1: Architect (runs on round 1)
		if round == 1 {
			o.broadcast(SwarmEvent{
				Type:      EventAgentThinking,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleArchitect,
			})
			archMsg, err := o.architect.Execute(ctx, sCtx)
			if err != nil {
				return nil, fmt.Errorf("role-based [phase 1]: architect failed: %w", err)
			}
			o.broadcast(SwarmEvent{
				Type:      EventAgentOutput,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleArchitect,
				Message:   archMsg,
			})
		}

		// PHASE 2A: Lead Coder generates or refactors code
		o.broadcast(SwarmEvent{
			Type:      EventAgentThinking,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleCoder,
		})
		coderMsg, err := o.coder.Execute(ctx, sCtx)
		if err != nil {
			return nil, fmt.Errorf("role-based [phase 2a]: coder failed: %w", err)
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

		// PHASE 2B: Critic AND Security Auditor run CONCURRENTLY on the generated code
		var wg sync.WaitGroup
		var criticErr, secErr error
		var criticMsg, secMsg *domain.AgentMessage

		wg.Add(2)

		// Goroutine 1: Critic
		go func() {
			defer wg.Done()
			o.broadcast(SwarmEvent{
				Type:      EventAgentThinking,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleCritic,
			})
			criticMsg, criticErr = o.critic.Execute(ctx, sCtx)
			if criticErr == nil {
				o.broadcast(SwarmEvent{
					Type:      EventAgentOutput,
					ProjectID: projectID,
					Round:     round,
					Agent:     domain.RoleCritic,
					Message:   criticMsg,
				})
			}
		}()

		// Goroutine 2: Security Auditor
		go func() {
			defer wg.Done()
			o.broadcast(SwarmEvent{
				Type:      EventAgentThinking,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleSecurity,
			})
			secMsg, secErr = o.security.Execute(ctx, sCtx)
			if secErr == nil {
				o.broadcast(SwarmEvent{
					Type:      EventAgentOutput,
					ProjectID: projectID,
					Round:     round,
					Agent:     domain.RoleSecurity,
					Message:   secMsg,
				})
			}
		}()

		wg.Wait()

		if criticErr != nil {
			return nil, fmt.Errorf("role-based [phase 2b]: critic review failed: %w", criticErr)
		}
		if secErr != nil {
			return nil, fmt.Errorf("role-based [phase 2b]: security audit failed: %w", secErr)
		}

		// PHASE 3: Arbiter Evaluates Consensus
		o.broadcast(SwarmEvent{
			Type:      EventAgentThinking,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleArbiter,
		})
		arbMsg, err := o.arbiter.Execute(ctx, sCtx)
		if err != nil {
			return nil, fmt.Errorf("role-based [phase 3]: arbiter judgment failed: %w", err)
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

		// Check Consensus
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
