package orchestrator

import (
	"context"
	"fmt"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// domain and workspace are both used below: domain for the agent role and
// intent constants, workspace for the on-disk project.

// runRoleBased runs the default plan-driven pipeline.
//
// The bounded loop that used to live here stopped at MaxRounds, which meant a
// project with more tasks than rounds could never finish: the team did one
// slice of work, scored it, and declared victory. It now delegates to
// runPlanDrivenLoop, which works through the task list and stops only on real
// completion.
func (o *Orchestrator) runRoleBased(ctx context.Context, projectID string, userPrompt string, agentModels map[domain.AgentRole]string, apiKeys map[string]string) (*agent.SwarmContext, error) {
	sCtx := &agent.SwarmContext{
		ProjectID:   projectID,
		Round:       1,
		UserPrompt:  userPrompt,
		MemStore:    o.memStore,
		AgentModels: agentModels,
		APIKeys:     apiKeys,
	}
	sCtx.TokenStreamer = func(role domain.AgentRole, as string, taskID string, chunk string) {
		o.streamToken(projectID, sCtx.Round, taskID, role, as, chunk)
	}

	// 1. Initialize Disk Workspace with Git tracking
	ws, err := workspace.NewWorkspace(o.baseDir, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize project workspace: %w", err)
	}

	// Let agents read the real project before they write anything.
	sCtx.WorkspacePath = ws.Path()

	// 1B. Load existing files from workspace if present on disk
	if len(sCtx.CodeFilesSnapshot()) == 0 {
		existingFiles, _ := ws.LoadFiles()
		if len(existingFiles) > 0 {
			sCtx.SetCodeFiles(existingFiles)
			o.broadcast(SwarmEvent{
				Type:      EventCodeUpdated,
				ProjectID: projectID,
				Round:     1,
				Files:     existingFiles,
			})
		}
	}

	return o.runPlanDrivenLoop(ctx, sCtx, projectID, ws)
}