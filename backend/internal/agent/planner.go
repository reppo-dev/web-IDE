package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

// BuildPlan asks the architect for the project's full work list.
//
// This runs once per project, before the first round, because the round loop
// needs a backlog to advance: without it the swarm re-does whatever it just
// finished and stops as soon as one round looks clean.
//
// The prompt is deliberately explicit about the failure this replaces. Asking a
// model for "a plan" gets back the file list for the feature in front of it.
// Asking for "every piece of work that must exist before a user could call the
// project finished, including tests, wiring, and docs" gets a plan that can
// actually end.
func (a *ArchitectAgent) BuildPlan(ctx context.Context, sCtx *SwarmContext) (*domain.ProjectPlan, error) {
	model := a.getModelForRole(sCtx)
	apiKey, baseURL, err := a.getAPIKey(model, sCtx)
	if err != nil {
		return nil, err
	}
	cleanModel := a.cleanModelName(model)

	systemPrompt := `You are the Project Planner in NexusAI, an autonomous engineering team.

Your job is to produce the COMPLETE WORK LIST for a project request, not a summary of it.

Rules:
1. List every piece of work required before a developer could call this project finished.
2. Each task must be independently implementable and verifiable by one coding round.
3. Include ALL of these kinds of work, not only the happy path:
   - core features and their error/edge-case handling
   - wiring: config, registration, routes, dependency injection, imports
   - tests for each significant unit
   - data models and migrations when the project stores anything
   - documentation or a README when the project needs one to be usable
   - any setup or integration step needed before the project can run
4. Order the tasks so each one depends only on tasks listed before it.
5. Never write a task like "finish the project" or "implement the rest". Every task
   must name concrete, verifiable output.
6. "acceptance" must be a checkable statement ("the HTTP server answers /health with 200").
7. Do not include tasks for anything the user did not ask for. No extra frameworks,
   no deployment pipelines, no CI, unless the request implies them.
8. If the request is small, return few tasks. Do not invent work to fill a quota.

Return ONLY valid JSON:
{
  "goal": "One sentence: what must be true for this project to be done",
  "tasks": [
    {
      "id": "T-001",
      "title": "Short imperative title",
      "acceptance": "A concrete, checkable condition for completion"
    }
  ]
}`

	userPrompt := fmt.Sprintf("Project request: %s", sCtx.UserPrompt)
	if sCtx.Blueprint != nil {
		if bp, err := json.Marshal(sCtx.Blueprint); err == nil {
			userPrompt += fmt.Sprintf("\n\nArchitect blueprint for context:\n%s", string(bp))
		}
	}

	resp, err := a.llmClient.Generate(ctx, llm.Request{
		Model:        cleanModel,
		APIKey:       apiKey,
		BaseURL:      baseURL,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
	})
	if err != nil {
		return nil, fmt.Errorf("project planning failed: %w", err)
	}

	var plan domain.ProjectPlan
	if err := json.Unmarshal([]byte(CleanJSON(resp.Content)), &plan); err != nil {
		return nil, fmt.Errorf("planner returned an invalid work list: %w (content: %s)", err, resp.Content)
	}

	plan = normalizePlan(plan)
	if err := validatePlan(plan); err != nil {
		return nil, err
	}

	if sCtx.MemStore != nil {
		_ = sCtx.MemStore.WriteProject(ctx, domain.MemoryEntry{
			ProjectID:  sCtx.ProjectID,
			Agent:      domain.RoleArchitect,
			Type:       domain.MemoryDecision,
			Content:    fmt.Sprintf("Project plan created: %d tasks for goal %q", len(plan.Tasks), plan.Goal),
			Tags:       []string{"plan", "decomposition"},
			Round:      sCtx.Round,
			Confidence: 0.9,
		})
	}
	return &plan, nil
}

// normalizePlan repairs the plan the model returned so the loop can rely on its
// invariants. Models routinely hand back an empty title, a duplicated id, or a
// placeholder task; all three would otherwise break task selection.
func normalizePlan(plan domain.ProjectPlan) domain.ProjectPlan {
	plan.Goal = strings.TrimSpace(plan.Goal)

	out := make([]domain.ProjectTask, 0, len(plan.Tasks))
	seenID := make(map[string]bool, len(plan.Tasks))
	for i, t := range plan.Tasks {
		t.Title = strings.TrimSpace(t.Title)
		if t.Title == "" {
			continue
		}

		t.ID = strings.TrimSpace(t.ID)
		if t.ID == "" || seenID[t.ID] {
			t.ID = fmt.Sprintf("T-%03d", i+1)
		}
		// Two tasks can still collide after the fallback above, so re-check and
		// keep advancing until the id is unique.
		for seenID[t.ID] {
			t.ID += "b"
		}
		seenID[t.ID] = true

		t.Acceptance = strings.TrimSpace(t.Acceptance)
		t.State = domain.TaskTodo
		t.Files = nil
		t.Round = 0
		out = append(out, t)
	}
	plan.Tasks = out
	return plan
}

// validatePlan rejects a work list the loop cannot act on.
//
// An empty plan is the important case: it would make "no open tasks" true, and
// the arbiter would read that as a finished project on the very first round.
func validatePlan(plan domain.ProjectPlan) error {
	if len(plan.Tasks) == 0 {
		return fmt.Errorf("planner returned no tasks; the project cannot be tracked without a work list")
	}
	if plan.Goal == "" {
		return fmt.Errorf("planner returned tasks without a project goal")
	}
	return nil
}

// DerivePlanFromFiles builds a work list from a blueprint's file tree.
//
// It is the fallback when the planning call fails: a task per planned file is
// weaker than a real work list (it never asks for tests or docs), but it still
// gives the round loop something to advance against instead of scoring a single
// snapshot and stopping. Only files that do not exist yet count as work.
func DerivePlanFromFiles(goal string, fileTree []string, existing []domain.CodeFile, round int) *domain.ProjectPlan {
	have := make(map[string]bool, len(existing))
	for _, f := range existing {
		have[f.Path] = true
	}

	plan := &domain.ProjectPlan{Goal: goal}
	used := make(map[string]bool)
	for _, path := range fileTree {
		path = strings.TrimSpace(path)
		if path == "" || have[path] || used[path] {
			continue
		}
		used[path] = true
		plan.Tasks = append(plan.Tasks, domain.ProjectTask{
			ID:    fmt.Sprintf("T-%03d", len(plan.Tasks)+1),
			Title: fmt.Sprintf("Create %s", path),
			Acceptance: fmt.Sprintf("%s exists, compiles, and contains a working "+
				"implementation rather than a placeholder", path),
			State: domain.TaskTodo,
			Round: round,
		})
	}

	if len(plan.Tasks) == 0 {
		return nil
	}
	return plan
}

// LogPlanOutcome records which path the plan came from, so a fallback that
// silently produced a file-per-task list is visible in the logs instead of
// looking like a full decomposition.
//
// It is exported so the orchestrator can report the same thing when it builds a
// plan itself.
func LogPlanOutcome(source string, plan *domain.ProjectPlan, err error) {
	switch {
	case err != nil:
		log.Printf("project planning via %s failed: %v", source, err)
	case plan != nil:
		log.Printf("project plan via %s: %d tasks, goal %q", source, len(plan.Tasks), plan.Goal)
	}
}