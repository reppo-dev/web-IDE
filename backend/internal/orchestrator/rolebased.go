package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

func (o *Orchestrator) runRoleBased(ctx context.Context, projectID string, userPrompt string) (*agent.SwarmContext, error) {
	sCtx := &agent.SwarmContext{
		ProjectID:  projectID,
		Round:      1,
		UserPrompt: userPrompt,
		MemStore:   o.memStore,
	}

	// 1. Initialize Disk Workspace with Git tracking
	ws, err := workspace.NewWorkspace(o.baseDir, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize project workspace: %w", err)
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

		// Sync files to physical workspace disk and compute diffs
		if len(sCtx.CodeFiles) > 0 {
			syncedFiles, err := ws.SyncFiles(sCtx.CodeFiles)
			if err == nil {
				sCtx.CodeFiles = syncedFiles
			}
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

		// Run real compilation verification in sandbox
		if len(sCtx.CodeFiles) > 0 && o.sandbox != nil {
			execRes := o.sandbox.VerifyCodebase(ctx, ws.Path(), sCtx.CodeFiles)
			if !execRes.Success && len(execRes.ErrorsFound) > 0 {
				if sCtx.CriticReview == nil {
					sCtx.CriticReview = &domain.CriticPayload{}
				}
				for _, compErr := range execRes.ErrorsFound {
					sCtx.CriticReview.Issues = append(sCtx.CriticReview.Issues, domain.ReviewIssue{
						Severity:    "critical",
						File:        "compiler",
						Description: fmt.Sprintf("Compiler/Syntax failure: %s", compErr),
					})
				}
			}
		}

		// PHASE 2B: Critic AND Security Auditor run CONCURRENTLY
		var wg sync.WaitGroup
		var criticErr, secErr error
		var criticMsg, secMsg *domain.AgentMessage

		wg.Add(2)

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

		// PHASE 2C: Full N-to-N Peer Cross-Examination (Every agent reviews the other 4)
		if o.matrixEngine != nil {
			matrix, err := o.matrixEngine.ExecuteCrossExamination(ctx, sCtx)
			if err == nil {
				o.broadcast(SwarmEvent{
					Type:      EventAgentOutput,
					ProjectID: projectID,
					Round:     round,
					Payload:   matrix,
				})
			}
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

		// Check for oscillation/stalemate
		if o.detector != nil && sCtx.ArbiterVerdict != nil {
			o.detector.RecordRound(round, sCtx.ArbiterVerdict.FinalScore)
			if o.detector.IsStalemate() {
				resolvedScore, note := o.detector.ResolveStalemate(sCtx.ArbiterVerdict.FinalScore)
				sCtx.ArbiterVerdict.FinalScore = resolvedScore
				sCtx.ArbiterVerdict.Passed = resolvedScore >= sCtx.ArbiterVerdict.Threshold
				sCtx.ArbiterVerdict.Summary += " [" + note + "]"
			}
		}

		// Git auto-commit this round
		if sCtx.ArbiterVerdict != nil {
			_, _ = ws.CommitRound(ctx, round, sCtx.ArbiterVerdict.FinalScore, sCtx.ArbiterVerdict.Summary)
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
