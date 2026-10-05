package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

func (o *Orchestrator) runConcurrent(ctx context.Context, projectID string, userPrompt string, agentModels map[domain.AgentRole]string, apiKeys map[string]string) (*agent.SwarmContext, error) {
	sCtx := &agent.SwarmContext{
		ProjectID:   projectID,
		Round:       1,
		UserPrompt:  userPrompt,
		MemStore:    o.memStore,
		AgentModels: agentModels,
		APIKeys:     apiKeys,
	}

	ws, err := workspace.NewWorkspace(o.baseDir, projectID)
	if err != nil {
		return nil, fmt.Errorf("concurrent: failed to open project workspace: %w", err)
	}
	sCtx.WorkspacePath = ws.Path()

	if len(sCtx.CodeFilesSnapshot()) == 0 {
		if existing, err := ws.LoadFiles(); err == nil && len(existing) > 0 {
			sCtx.SetCodeFiles(existing)
		}
	}

	// Concurrent mode shares the plan-driven loop instead of keeping its own
	// bounded one. That loop used to stop at MaxRounds in every mode, so the
	// same project would stall here while finishing elsewhere. Reviewing the
	// critic and the security auditor together is the only thing that
	// distinguishes this mode now.
	return o.runPlanDrivenLoop(ctx, sCtx, projectID, ws)
}

// runConcurrentLegacy is the previous bounded concurrent loop, kept out of the
// dispatch path until the plan-driven loop has proven stable in this mode too.
func (o *Orchestrator) runConcurrentLegacy(ctx context.Context, projectID string, userPrompt string, agentModels map[domain.AgentRole]string, apiKeys map[string]string) (*agent.SwarmContext, error) {
	sCtx := &agent.SwarmContext{
		ProjectID:   projectID,
		Round:       1,
		UserPrompt:  userPrompt,
		MemStore:    o.memStore,
		AgentModels: agentModels,
		APIKeys:     apiKeys,
	}

	// Let agents read the real project before they write anything. This uses the
	// read-only path resolver so concurrent mode does not create directories or
	// run git init the way rolebased mode does.
	sCtx.WorkspacePath = workspace.ProjectPath(o.baseDir, projectID)

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

			if sCtx.Intent != nil && sCtx.Intent.Kind != domain.IntentBuild {
				return o.runRelayDiscussion(ctx, sCtx, projectID, archMsg)
			}
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
			Files:     sCtx.CodeFilesSnapshot(),
		})
		o.broadcast(SwarmEvent{
			Type:      EventCodeUpdated,
			ProjectID: projectID,
			Round:     round,
			Files:     sCtx.CodeFilesSnapshot(),
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
				Files:     sCtx.CodeFilesSnapshot(),
			})
			return sCtx, nil
		}
	}

	return sCtx, nil
}

