package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

// CustomAgent runs any role the user defines. It is what makes the team size
// unbounded: a new persona is a prompt, not a new Go type.
type CustomAgent struct {
	*BaseAgent
	spec domain.AgentSpec
	// lastToolTrace records what the agent did with tools, for the UI.
	lastToolTrace string
}

// NewCustomAgent builds an agent from a user-supplied spec.
func NewCustomAgent(spec domain.AgentSpec, client llm.Client) *CustomAgent {
	role := spec.Role
	if role == "" {
		role = domain.AgentRole(spec.ID)
	}
	return &CustomAgent{
		BaseAgent: NewBaseAgent(role, client),
		spec:      spec,
	}
}

// Spec exposes the definition this agent was built from.
func (c *CustomAgent) Spec() domain.AgentSpec { return c.spec }

// getModelForRole prefers the model pinned on the agent's own spec. The team
// builder sets it there, so a swarm of ten custom agents does not need ten
// entries in a shared map keyed by role.
func (c *CustomAgent) getModelForRole(sCtx *SwarmContext) string {
	if c.spec.Model != "" {
		return c.spec.Model
	}
	return c.BaseAgent.getModelForRole(sCtx)
}

// builtInPrompt returns the shipped persona prompt for a role, or an empty
// string for roles NexusAI does not define.
func builtInPrompt(role domain.AgentRole) string {
	switch role {
	case domain.RoleArchitect:
		return `You are the System Architect in NexusAI.
Your role:
- Analyze user requests (whether in Persian or English).
- Decompose requirements into structured file trees, architecture designs, and API contracts.
- Select the cleanest architectural patterns (Clean Architecture, DDD, Repository Pattern).
- If the user prompt is in Persian, reason and describe components in fluent Persian, but keep file paths and API contracts in standard English.
- Return ONLY valid JSON conforming to the following structure:
{
  "project_overview": "Summary of the system architecture",
  "file_tree": ["path/to/file1.go", "path/to/file2.go"],
  "key_components": ["Component A description", "Component B description"],
  "api_contracts": ["Endpoint/Interface specs"]
}`
	case domain.RoleCoder:
		return `You are the Lead Engineer in NexusAI.
Your role:
- Write complete, robust, production-ready code with no placeholders or TODOs.
- Include proper error handling, logging, concurrency safety, and comments.
- The EXISTING PROJECT CODE is provided below. Treat it as ground truth: match its
  package names, import paths, type names, and error handling style.
- Do NOT invent a different structure, and do NOT rewrite files you were not asked to change.
- Return ONLY valid JSON conforming to:
{
  "files": [
    {"path": "path/to/file.go", "language": "go", "content": "package main\\n..."}
  ],
  "explanation": "Summary of changes and rationale"
}`
	case domain.RoleCritic:
		return `You are the Adversarial Critic (The Red Team) in NexusAI.
Your role:
- Relentlessly hunt logic errors, race conditions, deadlocks, unhandled exceptions, and performance traps.
- Question every assumption the implementation made.
- Demand concrete corrections with specific file and line references.
- Do NOT be agreeable. Your value is in finding what was missed.
- Return ONLY valid JSON conforming to:
{
  "general_review": "Overall assessment",
  "issues": [{"severity": "critical|major|minor", "file": "path", "line": 0, "description": "...", "suggested_fix": "..."}],
  "refactor_plan": "Ordered steps to fix"
}`
	case domain.RoleSecurity:
		return `You are the Security Auditor (The Shield) in NexusAI.
Your role:
- Audit against OWASP Top 10 and common CWE weaknesses.
- Hunt for injection, broken authentication, secret leakage, and missing sanitisation.
- Treat any user input reaching a query, shell, or template as suspect.
- Return ONLY valid JSON conforming to:
{
  "attack_surface": "Description of exposed interfaces",
  "vulnerabilities": [{"severity": "critical|major|minor", "file": "path", "line": 0, "description": "...", "suggested_fix": "..."}],
  "is_clean": false
}`
	case domain.RoleArbiter:
		return `You are the Arbiter (Consensus Judge) in NexusAI.
Your role:
- Weigh every agent's findings and score the result across five dimensions:
  Correctness (30%), Bug Resolution (25%), Security (20%), Quality (15%), Spec Adherence (10%).
- Be strict. An unverified claim is not a passing claim.
- If the work is not yet acceptable, say exactly what must change.
- Return ONLY valid JSON conforming to:
{
  "round": 1,
  "final_score": 0.0,
  "threshold": 85.0,
  "passed": false,
  "needs_refinement": true,
  "dimensions": [{"name": "Correctness", "weight": 0.30, "score": 0.0, "explanation": "..."}],
  "feedback_to_coder": "Specific, actionable next steps",
  "summary": "One sentence verdict"
}`
	}
	return ""
}

// jsonContractFor gives a custom agent the same structured-output contract as
// the built-in role it most resembles, so responses stay parseable.
func jsonContractFor(role domain.AgentRole) string {
	switch role {
	case domain.RoleArchitect:
		return `Return ONLY valid JSON:
{"project_overview":"...","file_tree":["..."],"key_components":["..."],"api_contracts":["..."]}`
	case domain.RoleCoder:
		return `Return ONLY valid JSON:
{"files":[{"path":"...","language":"go","content":"..."}],"explanation":"..."}`
	case domain.RoleCritic:
		return `Return ONLY valid JSON:
{"general_review":"...","issues":[{"severity":"critical|major|minor","file":"...","line":0,"description":"...","suggested_fix":"..."}],"refactor_plan":"..."}`
	case domain.RoleSecurity:
		return `Return ONLY valid JSON:
{"attack_surface":"...","vulnerabilities":[{"severity":"...","file":"...","line":0,"description":"...","suggested_fix":"..."}],"is_clean":false}`
	case domain.RoleArbiter:
		return `Return ONLY valid JSON:
{"round":1,"final_score":0.0,"threshold":85.0,"passed":false,"needs_refinement":true,"dimensions":[{"name":"...","weight":0.3,"score":0.0,"explanation":"..."}],"feedback_to_coder":"...","summary":"..."}`
	}
	// A custom agent in an implement phase is expected to write code. Telling
	// it to return only a summary would mean no file is ever produced, so the
	// default contract asks for files.
	return `Return ONLY valid JSON:
{"files":[{"path":"relative/path.ext","language":"go","content":"the complete file contents"}],"explanation":"..."}`
}

// buildPrompt combines the built-in persona with whatever the user added. A
// user prompt always wins on conflict, so a custom agent can fully override
// the persona.
func (c *CustomAgent) buildPrompt() string {
	parts := make([]string, 0, 3)
	if base := builtInPrompt(c.spec.Role); base != "" {
		parts = append(parts, base)
	}
	if custom := strings.TrimSpace(c.spec.SystemPrompt); custom != "" {
		parts = append(parts, custom)
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf(
			"You are %q, a specialist on a software engineering team. Contribute only from this role's area of expertise.",
			c.spec.Name))
	}
	parts = append(parts, c.jsonContract())
	return strings.Join(parts, "\n\n")
}

// jsonContract picks the response shape from the agent's phase, not just its
// role. A team made only of custom agents still has to produce code, so any
// agent in an implement phase is asked for files rather than a summary.
func (c *CustomAgent) jsonContract() string {
	if c.spec.Role == domain.RoleCustom {
		switch c.spec.Phase {
		case domain.PhaseDesign:
			return `Return ONLY valid JSON:
{"project_overview":"...","file_tree":["path/a.go"],"key_components":["..."],"api_contracts":["..."]}`

		case domain.PhaseReview:
			return `Return ONLY valid JSON:
{"general_review":"...","issues":[{"severity":"critical|major|minor","file":"path","line":0,"description":"...","suggested_fix":"..."}],"refactor_plan":"..."}`

		case domain.PhaseJudge:
			return `Return ONLY valid JSON:
{"round":1,"final_score":0.0,"threshold":85.0,"passed":false,"needs_refinement":true,"dimensions":[{"name":"...","weight":0.3,"score":0.0,"explanation":"..."}],"feedback_to_coder":"...","summary":"..."}`

		default:
			// Implement and unset phases must write real files.
			return `Return ONLY valid JSON:
{"files":[{"path":"relative/path.ext","language":"go","content":"the complete file contents"}],"explanation":"..."}`
		}
	}
	return jsonContractFor(c.spec.Role)
}

// toolLoopFor builds a tool loop when this agent is allowed to use tools and a
// project is open. It returns nil otherwise, so a run without a project keeps
// working exactly as before.
func (c *CustomAgent) toolLoopFor(sCtx *SwarmContext) *ToolLoop {
	if sCtx == nil || sCtx.WorkspacePath == "" {
		return nil
	}
	access := sCtx.ToolsAccess
	if access == "" {
		// A team that did not ask for tools stays read-only rather than gaining
		// write access by default.
		access = AccessReadOnly
	}
	if sCtx.ToolsDisabled {
		return nil
	}

	budget := NewToolBudget(sCtx.ToolCallBudget, sCtx.ToolTimeout)
	executor := NewToolExecutor(sCtx.WorkspacePath, access, nil, budget)
	if sCtx.Detector != nil {
		executor.SetDetector(sCtx.Detector)
	}
	loop := NewToolLoop(executor, c.llmClient)
	loop.agentName = c.spec.Name
	return loop
}

// executeWithTools runs the tool loop and turns the final answer into a normal
// agent message, so the rest of the swarm does not need to know tools existed.
func (c *CustomAgent) executeWithTools(ctx context.Context, sCtx *SwarmContext, loop *ToolLoop, base llm.Request) (*domain.AgentMessage, error) {
	base.SystemPrompt = base.SystemPrompt + "\n\n" + ToolPrompt(sCtx.ToolsAccess)

	res := loop.Run(ctx, base)
	if res.Err != nil && !res.TimedOut {
		return nil, fmt.Errorf("agent %q failed: %w", c.spec.Name, res.Err)
	}

	if res.TimedOut {
		// A stalled provider must not look like a crash. Report what this agent
		// managed to do and let the rest of the team finish the round.
		if len(res.ToolCalls) > 0 {
			c.lastToolTrace = ToolResultsToJSON(res.ToolCalls)
			c.commitResult(ctx, sCtx, res.Content)
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"status":   "timed_out",
			"turns":    res.Turns,
			"done":     len(res.ToolCalls),
			"message":  res.Content,
			"remedy":  "This model did not answer in time. The other agents continued; retry this one or pick a faster model.",
		})
		return c.createMessageWithReasoning(sCtx, domain.MsgAgentReport, payload, res.Content, res.Reasoning)
	}

	if len(res.ToolCalls) > 0 {
		c.commitResult(ctx, sCtx, res.Content)
		c.lastToolTrace = ToolResultsToJSON(res.ToolCalls)
	} else {
		c.commitResult(ctx, sCtx, res.Content)
	}

	cleaned := CleanJSON(res.Content)
	var contentRaw json.RawMessage
	switch {
	case cleaned != "" && json.Valid([]byte(cleaned)):
		contentRaw = json.RawMessage(cleaned)
	case res.Content != "" && json.Valid([]byte(res.Content)):
		contentRaw = json.RawMessage(res.Content)
	case res.Content != "":
		wrapped, _ := json.Marshal(map[string]string{"output": res.Content})
		contentRaw = wrapped
	default:
		contentRaw = json.RawMessage(`{}`)
	}

	summary := fmt.Sprintf("%s completed its pass (%d tool call(s), %d turn(s)).",
		c.spec.Name, len(res.ToolCalls), res.Turns)

	return c.createMessageWithReasoning(sCtx, domain.MsgAgentReport, contentRaw, summary, res.Reasoning)
}

// Execute runs the agent and returns a message the swarm can route.
func (c *CustomAgent) Execute(ctx context.Context, sCtx *SwarmContext) (*domain.AgentMessage, error) {
	work := c.buildWork(ctx, sCtx)

	model := c.getModelForRole(sCtx)
	apiKey, baseURL, err := c.getAPIKey(model, sCtx)
	if err != nil {
		return nil, err
	}

	req := llm.Request{
		Model:        c.cleanModelName(model),
		APIKey:       apiKey,
		BaseURL:      baseURL,
		SystemPrompt: c.buildPrompt(),
		UserPrompt:   work,
		JSONSchema:   map[string]interface{}{"type": "object"},
	}
	if c.spec.Temperature > 0 {
		req.Temperature = c.spec.Temperature
	}

	// With tools available, the agent works through request/response turns so it
	// can read the real project and change it. Without them it falls back to a
	// single call, which is the old behaviour.
	if loop := c.toolLoopFor(sCtx); loop != nil {
		return c.executeWithTools(ctx, sCtx, loop, req)
	}

	resp, err := c.llmClient.Generate(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("agent %q failed: %w", c.spec.Name, err)
	}

	// Persist the structured result so later agents in the team can build on
	// it. Without this every agent would be blind to what came before.
	c.commitResult(ctx, sCtx, resp.Content)

	cleaned := CleanJSON(resp.Content)
	var contentRaw json.RawMessage
	if cleaned != "" && json.Valid([]byte(cleaned)) {
		contentRaw = json.RawMessage(cleaned)
	} else if resp.Content != "" && json.Valid([]byte(resp.Content)) {
		contentRaw = json.RawMessage(resp.Content)
	} else if resp.Content != "" {
		wrapped, _ := json.Marshal(map[string]string{"output": resp.Content})
		contentRaw = wrapped
	} else if resp.Reasoning != "" {
		wrapped, _ := json.Marshal(map[string]string{"summary": resp.Reasoning})
		contentRaw = wrapped
	} else {
		contentRaw = json.RawMessage(`{}`)
	}

	summary := fmt.Sprintf("%s completed its pass.", c.spec.Name)
	if resp.Reasoning != "" && c.spec.Role == domain.RoleArbiter {
		summary = resp.Reasoning
	}

	return c.createMessageWithReasoning(sCtx, domain.MsgAgentReport, contentRaw,
		summary, resp.Reasoning)
}

// commitResult stores the agent's structured output on the shared context,
// keyed by the role that produced it. A malformed payload is ignored rather
// than fatal: the agent still reported, it just did not add anything usable.
func (c *CustomAgent) commitResult(ctx context.Context, sCtx *SwarmContext, content string) {
	if sCtx == nil || content == "" {
		return
	}
	cleaned := CleanJSON(content)

	switch c.spec.Role {
	case domain.RoleArchitect:
		var bp domain.BlueprintPayload
		if json.Unmarshal([]byte(cleaned), &bp) == nil && bp.ProjectOverview != "" {
			sCtx.Blueprint = &bp
		}

	case domain.RoleCoder:
		var cr struct {
			Files []domain.CodeFile `json:"files"`
		}
		if json.Unmarshal([]byte(cleaned), &cr) == nil && len(cr.Files) > 0 {
			sCtx.MergeCodeFiles(cr.Files)
		}

	case domain.RoleCritic:
		var review domain.CriticPayload
		if json.Unmarshal([]byte(cleaned), &review) == nil && review.GeneralReview != "" {
			sCtx.CriticReview = &review
		}

	case domain.RoleSecurity:
		var report domain.SecurityPayload
		if json.Unmarshal([]byte(cleaned), &report) == nil && report.AttackSurface != "" {
			sCtx.SecurityReport = &report
		}

	case domain.RoleArbiter:
		var verdict domain.ArbiterVerdict
		if json.Unmarshal([]byte(cleaned), &verdict) == nil {
			verdict.Round = sCtx.Round
			sCtx.ArbiterVerdict = &verdict
		}

	default:
		// Custom agents run concurrently inside a phase, so appending to the
		// shared slice without a lock would race. Each agent's own files are
		// merged under the context's mutex, and the newest writer wins per
		// path so a later agent can revise an earlier one.
		var cr struct {
			Files []domain.CodeFile `json:"files"`
		}
		if json.Unmarshal([]byte(cleaned), &cr) == nil && len(cr.Files) > 0 {
			sCtx.MergeCodeFiles(cr.Files)
		}
	}

	// Every agent contributes to the shared project brain, including custom
	// ones. Without this a user-built team would learn nothing between runs:
	// only the five built-in personas wrote memories.
	c.recordInBrain(ctx, sCtx)
}

// recordInBrain distils an agent's output into durable project memory. What is
// worth keeping differs by role: a critic's findings are mistakes to avoid
// repeating, an architect's outline is a decision, a coder's rationale is a
// fact about how the code works.
func (c *CustomAgent) recordInBrain(ctx context.Context, sCtx *SwarmContext) {
	if sCtx.MemStore == nil || sCtx.ProjectID == "" || c.spec.Role == domain.RoleCustom {
		// Custom roles have no predictable payload shape, so there is nothing
		// reliable to extract without guessing.
		return
	}

	brain := NewProjectBrain(sCtx.MemStore)
	role := c.spec.Role

	switch role {
	case domain.RoleArchitect:
		if sCtx.Blueprint != nil && sCtx.Blueprint.ProjectOverview != "" {
			_ = brain.RecordDecision(ctx, sCtx.ProjectID, role,
				"Architecture: "+truncate(sCtx.Blueprint.ProjectOverview, 400), sCtx.Round)
		}

	case domain.RoleCoder:
		if files := sCtx.CodeFilesSnapshot(); len(files) > 0 {
			var paths []string
			for _, f := range files {
				paths = append(paths, f.Path)
			}
			_ = brain.RecordFact(ctx, sCtx.ProjectID, role,
				fmt.Sprintf("Round %d changed: %s", sCtx.Round, strings.Join(paths, ", ")), sCtx.Round)
		}

	case domain.RoleCritic:
		if sCtx.CriticReview != nil {
			for _, issue := range sCtx.CriticReview.Issues {
				if issue.Severity != "critical" {
					continue
				}
				_ = brain.RecordInvariant(ctx, sCtx.ProjectID, role,
					"Previously found "+issue.Severity+" issue in "+
						issue.File+": "+issue.Description, sCtx.Round)
			}
		}

	case domain.RoleSecurity:
		if sCtx.SecurityReport != nil {
			for _, v := range sCtx.SecurityReport.Vulnerabilities {
				_ = brain.RecordWarning(ctx, sCtx.ProjectID, role,
					v.File+": "+v.Description, sCtx.Round)
			}
		}

	case domain.RoleArbiter:
		if sCtx.ArbiterVerdict != nil && sCtx.ArbiterVerdict.Summary != "" {
			_ = brain.RecordFact(ctx, sCtx.ProjectID, role,
				fmt.Sprintf("Round %d scored %.1f%% against a %.1f%% threshold. %s",
					sCtx.Round, sCtx.ArbiterVerdict.FinalScore,
					sCtx.ArbiterVerdict.Threshold, sCtx.ArbiterVerdict.Summary), sCtx.Round)
		}
	}
}

// truncate shortens a memory entry so one verbose agent cannot crowd the
// other agents out of the shared context.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// BuildPlan asks a team's designer for the project's complete work list.
//
// It reuses the built-in planner prompt rather than the agent's own, because the
// failure this replaces was not about wording: a team was asked to do a project
// and nobody ever asked it what the project required. The designer's own system
// prompt describes how it reviews or designs; the decomposition rules are what
// the round loop needs.
//
// A team's designer is a CustomAgent, so without this the team path had no way
// to produce a plan at all and silently ran without one.
func (c *CustomAgent) BuildPlan(ctx context.Context, sCtx *SwarmContext) (*domain.ProjectPlan, error) {
	model := c.getModelForRole(sCtx)
	if model == "" {
		return nil, fmt.Errorf("designer %q has no model assigned, so no work list can be built", c.spec.Name)
	}
	apiKey, baseURL, err := c.getAPIKey(model, sCtx)
	if err != nil {
		return nil, err
	}

	designerBrief := ""
	if c.spec.SystemPrompt != "" {
		designerBrief = "\n\nThe designer's own brief, which the work list must respect:\n" + c.spec.SystemPrompt
	}

	userPrompt := fmt.Sprintf("Project request: %s%s", sCtx.UserPrompt, designerBrief)
	if sCtx.Blueprint != nil && len(sCtx.Blueprint.FileTree) > 0 {
		userPrompt += "\n\nFiles already planned:\n" + strings.Join(sCtx.Blueprint.FileTree, "\n")
	}

	resp, err := c.llmClient.Generate(ctx, llm.Request{
		Model:        c.cleanModelName(model),
		APIKey:       apiKey,
		BaseURL:      baseURL,
		SystemPrompt: planSystemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
	})
	if err != nil {
		return nil, fmt.Errorf("team planning failed: %w", err)
	}

	var plan domain.ProjectPlan
	if err := json.Unmarshal([]byte(CleanJSON(resp.Content)), &plan); err != nil {
		return nil, fmt.Errorf("designer returned an invalid work list: %w (content: %s)", err, resp.Content)
	}

	plan = NormalizePlan(plan)
	if err := ValidatePlan(plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

// buildWork assembles the context an agent sees: the shared project brain, the
// real code on disk, and whatever the earlier agents produced this round.
func (c *CustomAgent) buildWork(ctx context.Context, sCtx *SwarmContext) string {
	var b strings.Builder

	if sCtx.UserPrompt != "" {
		b.WriteString(fmt.Sprintf("User Request: %s\n", sCtx.UserPrompt))
	}

	if c.spec.Phase == domain.PhaseImplement || c.spec.Role == domain.RoleCoder {
		if task := sCtx.ActiveTaskSnapshot(); task != nil {
			b.WriteString(fmt.Sprintf("\nCURRENT TASK: %s (%s)", task.ID, task.Title))
			if task.Acceptance != "" {
				b.WriteString(fmt.Sprintf("\nDONE WHEN: %s", task.Acceptance))
			}
			if len(task.Files) > 0 {
				b.WriteString(fmt.Sprintf("\nEXPECTED FILES: %s", strings.Join(task.Files, ", ")))
			}
			if done := completedTitles(sCtx); len(done) > 0 {
				b.WriteString(fmt.Sprintf("\nALREADY COMPLETED (do not redo): %s", strings.Join(done, "; ")))
			}
			b.WriteString("\nWork ONLY on this task. Do not start work that belongs to another task.\n")
		}
	} else {
		if brief := activeTaskBrief(sCtx); brief != "" {
			b.WriteString(brief)
			b.WriteString("\n")
		}
	}

	if brain := c.fetchMemories(ctx, sCtx); brain != "" {
		b.WriteString(brain)
	}

	if sCtx.WorkspacePath != "" && c.spec.Role != domain.RoleArbiter {
		engine := NewContextEngine(sCtx.WorkspacePath)
		if code, err := engine.BuildCodeContext(sCtx.UserPrompt, 8); err == nil && code != "" {
			b.WriteString("\n--- EXISTING PROJECT CODE ---\n")
			b.WriteString(code)
		}

		// Make the boundary explicit. Without this a model may try to read or
		// write outside the project, and the change would be silently dropped
		// when the workspace refuses it.
		b.WriteString(fmt.Sprintf(
			"\n--- SCOPE ---\nYou work only inside this project folder: %s\n"+
				"Never use absolute paths, parent-directory references (..), or paths outside it.\n"+
				"Every \"path\" you emit must be relative to that folder.\n",
			sCtx.WorkspacePath))
	}

	if sCtx.Blueprint != nil {
		if data, err := json.Marshal(sCtx.Blueprint); err == nil {
			b.WriteString("\n--- ARCHITECT BLUEPRINT ---\n")
			b.Write(data)
		}
	}
	if files := sCtx.CodeFilesSnapshot(); len(files) > 0 {
		if data, err := json.Marshal(files); err == nil {
			b.WriteString("\n--- CURRENT CODE IN WORKSPACE ---\n")
			b.Write(data)
		}
	}
	if sCtx.CriticReview != nil {
		if data, err := json.Marshal(sCtx.CriticReview); err == nil {
			b.WriteString("\n--- CRITIC FINDINGS ---\n")
			b.Write(data)
		}
	}
	if sCtx.SecurityReport != nil {
		if data, err := json.Marshal(sCtx.SecurityReport); err == nil {
			b.WriteString("\n--- SECURITY FINDINGS ---\n")
			b.Write(data)
		}
	}

	if b.Len() == 0 {
		return "No context available. Report that you cannot proceed rather than inventing requirements."
	}
	return b.String()
}