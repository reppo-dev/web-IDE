package orchestrator

import (
	"context"
	"fmt"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// runSequential runs the same plan-driven pipeline as role-based mode.
//
// The two modes differed only in scheduling: role-based ran the critic and the
// security auditor concurrently. Keeping a separate bounded loop here meant the
// round ceiling and the snapshot-based verdict survived in sequential mode, so
// the same project would finish in one mode and stall in the other. Sequential
// now shares the loop and differs only in that its review agents run in turn.
func (o *Orchestrator) runSequential(ctx context.Context, projectID string, userPrompt string, agentModels map[domain.AgentRole]string, apiKeys map[string]string) (*agent.SwarmContext, error) {
	sCtx := &agent.SwarmContext{
		ProjectID:   projectID,
		Round:       1,
		UserPrompt:  userPrompt,
		MemStore:    o.memStore,
		AgentModels: agentModels,
		APIKeys:     apiKeys,
		// Sequential mode reviews in turn rather than side by side.
		ReviewSequentially: true,
	}

	sCtx.WorkspacePath = workspace.ProjectPath(o.baseDir, projectID)

	// Read-only mode still needs a Workspace handle to sync and commit files, so
	// it is created without touching the filesystem beyond what the path needs.
	ws, err := workspace.NewWorkspace(o.baseDir, projectID)
	if err != nil {
		return nil, fmt.Errorf("sequential: failed to open project workspace: %w", err)
	}
	sCtx.WorkspacePath = ws.Path()

	if len(sCtx.CodeFilesSnapshot()) == 0 {
		if existing, err := ws.LoadFiles(); err == nil && len(existing) > 0 {
			sCtx.SetCodeFiles(existing)
		}
	}

	return o.runPlanDrivenLoop(ctx, sCtx, projectID, ws)
}