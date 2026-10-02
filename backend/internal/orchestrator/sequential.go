package orchestrator

import (
	"context"
	"fmt"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

func (o *Orchestrator) runSequential(ctx context.Context, projectID string, userPrompt string, agentModels map[domain.AgentRole]string, apiKeys map[string]string) (*agent.SwarmContext, error) {
	sCtx := &agent.SwarmContext{
		ProjectID:   projectID,
		Round:       1,
		UserPrompt:  userPrompt,
		MemStore:    o.memStore,
		AgentModels: agentModels,
		APIKeys: apiKeys,
	}

	// Let agents read the real project before they write anything. This uses the
// read-only path resolver so sequential mode does not create directories or
// run git init the way rolebased mode does.
	sCtx.WorkspacePath = workspace.ProjectPath(o.baseDir, projectID)

	for round := 1; round <= o.config.MaxRounds; round++ {
		sCtx.Round = round
		o.broadcast(SwarmEvent{
			Type:      EventRoundStarted,
			ProjectID: projectID,
			Round:     round,
		})

		// 1. Architect (only on round 1 or if major restructure is requested)
		if round == 1 {
			o.broadcast(SwarmEvent{
				Type:      EventAgentThinking,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleArchitect,
			})
			archMsg, err := o.architect.Execute(ctx, sCtx)
			if err != nil {
				return nil, fmt.Errorf("sequential: architect failed: %w", err)
			}
			o.broadcast(SwarmEvent{
				Type:      EventAgentOutput,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleArchitect,
				Message:   archMsg,
			})

			if sCtx.Intent != nil && sCtx.Intent.Kind != domain.IntentBuild {
				return o.runRelayDiscussion(ctx, sCtx, projectID, archMsg)
			}
		}

		// 2. Lead Coder
		o.broadcast(SwarmEvent{
			Type:      EventAgentThinking,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleCoder,
		})
		coderMsg, err := o.coder.Execute(ctx, sCtx)
		if err != nil {
			return nil, fmt.Errorf("sequential: coder failed: %w", err)
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

		// 3. Critic (Red Team)
		o.broadcast(SwarmEvent{
			Type:      EventAgentThinking,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleCritic,
		})
		criticMsg, err := o.critic.Execute(ctx, sCtx)
		if err != nil {
			return nil, fmt.Errorf("sequential: critic failed: %w", err)
		}
		o.broadcast(SwarmEvent{
			Type:      EventAgentOutput,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleCritic,
			Message:   criticMsg,
		})

		// 4. Security Auditor
		o.broadcast(SwarmEvent{
			Type:      EventAgentThinking,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleSecurity,
		})
		secMsg, err := o.security.Execute(ctx, sCtx)
		if err != nil {
			return nil, fmt.Errorf("sequential: security auditor failed: %w", err)
		}
		o.broadcast(SwarmEvent{
			Type:      EventAgentOutput,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleSecurity,
			Message:   secMsg,
		})

		// 5. Arbiter (Judgment)
		o.broadcast(SwarmEvent{
			Type:      EventAgentThinking,
			ProjectID: projectID,
			Round:     round,
			Agent:     domain.RoleArbiter,
		})
		arbMsg, err := o.arbiter.Execute(ctx, sCtx)
		if err != nil {
			return nil, fmt.Errorf("sequential: arbiter failed: %w", err)
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

