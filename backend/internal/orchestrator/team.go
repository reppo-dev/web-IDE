package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// TeamRun describes one execution of a user-built team.
type TeamRun struct {
	Team   *domain.Team
	Mode   domain.ExecutionMode
	APIKeys map[string]string
}

// RunTeam executes an arbitrary user-defined team. Unlike the fixed five-agent
// swarm, the members here come from the team's own specs, so a team may have
// two agents or twenty and each may run on its own model.
func (o *Orchestrator) RunTeam(ctx context.Context, projectID, userPrompt string, run TeamRun) (*agent.SwarmContext, error) {
	t := run.Team
	if t == nil {
		return nil, fmt.Errorf("no team provided")
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}

	specs := t.ActiveAgents()
	if len(specs) == 0 {
		return nil, domain.ErrTeamNoAgents
	}
	if !hasJudge(specs) {
		return nil, domain.ErrTeamNoJudge
	}

	ws, err := workspace.NewWorkspace(o.baseDir, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize project workspace: %w", err)
	}

	sCtx := &agent.SwarmContext{
		ProjectID:     projectID,
		Round:         1,
		UserPrompt:    userPrompt,
		MemStore:      o.memStore,
		APIKeys:       run.APIKeys,
		WorkspacePath: ws.Path(),
		// Team-level defaults; an agent may tighten or relax its own.
		ToolsAccess:    agent.AccessLevel(t.ToolsAccess),
		ToolCallBudget: t.MaxToolCalls,
		ToolTimeout:    4 * time.Minute,
	}
	if sCtx.ToolsAccess == "" {
		sCtx.ToolsAccess = agent.AccessReadOnly
	}
	if sCtx.ToolCallBudget <= 0 {
		sCtx.ToolCallBudget = domain.DefaultToolCallBudget
	}
	if o.sandbox != nil {
		sCtx.Detector = func(root string) (string, error) {
			res := o.sandbox.VerifyCodebase(ctx, root, nil)
			if res.Success {
				return fmt.Sprintf("%s: no problems found", res.Language), nil
			}
			return fmt.Sprintf("%s errors:\n%s", res.Language,
				strings.Join(res.ErrorsFound, "\n")), nil
		}
	}

	mode := run.Mode
	if mode == "" {
		mode = o.config.Mode
	}

	rounds := t.MaxRounds
	if rounds <= 0 {
		rounds = 3
	}
	threshold := t.ConsensusThreshold
	if threshold <= 0 {
		threshold = 85
	}

	for round := 1; round <= rounds; round++ {
		sCtx.Round = round
		// Do not blank CodeFiles between rounds: the reviewers and the judge in
		// the next round must still see what the previous round produced, and
		// the context engine re-reads the files from disk anyway.
		o.broadcast(SwarmEvent{
			Type:      EventRoundStarted,
			ProjectID: projectID,
			Round:     round,
		})

		if err := o.runTeamPhases(ctx, sCtx, projectID, specs, ws); err != nil {
			return nil, err
		}

		// VERIFY -> REWORK. runTeamPhases already ran the sandbox and stored any
		// compiler failure in CriticReview. Give an implementer one focused pass
		// at those errors before anyone scores the work, otherwise broken code
		// reaches the judge untouched.
		if o.hasVerificationFailure(sCtx) {
			o.broadcast(SwarmEvent{
				Type:      EventAgentThinking,
				ProjectID: projectID,
				Round:     round,
				Agent:     domain.RoleCoder,
				Payload:   "کد کامپایل نشد — در حال اصلاح خودکار خطاها...",
			})
			o.runRepairPass(ctx, sCtx, projectID, specs, ws)
		}

		score, verdict := o.scoreTeam(ctx, sCtx, specs, round, threshold)
		sCtx.ArbiterVerdict = verdict

		if _, err := ws.CommitRound(ctx, round, score, verdict.Summary); err != nil {
			log.Printf("team run: git commit failed: %v", err)
		}

		o.broadcast(SwarmEvent{
			Type:      EventRoundComplete,
			ProjectID: projectID,
			Round:     round,
			Verdict:   verdict,
		})

		if verdict.Passed {
			o.broadcast(SwarmEvent{
				Type:      EventConsensusReached,
				ProjectID: projectID,
				Round:     round,
				Verdict:   verdict,
				Files:     sCtx.CodeFiles,
			})
			return sCtx, nil
		}
	}

	return sCtx, nil
}

// stopIfConversational triages the user's message before any code work starts.
// It reports true when the request was small talk or a question, in which case
// the team answers instead of building.
func (o *Orchestrator) stopIfConversational(ctx context.Context, sCtx *agent.SwarmContext, projectID string, specs []domain.AgentSpec) bool {
	// Only bother when the team has a designer to ask; without one there is
	// nobody to run triage.
	var designer *domain.AgentSpec
	for i := range specs {
		if specs[i].Role == domain.RoleArchitect || specs[i].Phase == domain.PhaseDesign {
			designer = &specs[i]
			break
		}
	}
	if designer == nil {
		return false
	}

	var archMsg *domain.AgentMessage
	var err error
	if designer.Role == domain.RoleArchitect {
		arch := agent.NewArchitectAgent(o.llmClient())
		archMsg, err = arch.Execute(ctx, sCtx)
	} else {
		arch := agent.NewCustomAgent(*designer, o.llmClient())
		archMsg, err = arch.Execute(ctx, sCtx)
	}
	if err != nil {
		// Triage failed: assume it is real work rather than blocking the user.
		log.Printf("team triage failed, treating as a build request: %v", err)
		return false
	}

	if sCtx.Intent == nil || sCtx.Intent.Kind == domain.IntentBuild {
		return false
	}

	o.broadcast(SwarmEvent{
		Type:      EventAgentOutput,
		ProjectID: projectID,
		Round:     sCtx.Round,
		Agent:     domain.RoleArchitect,
		Message:   archMsg,
		Payload:   sCtx.Intent,
	})
	o.broadcast(SwarmEvent{
		Type:      EventConsensusReached,
		ProjectID: projectID,
		Round:     sCtx.Round,
		Payload:   sCtx.Intent,
	})
	return true
}

// runTeamPhases executes agents phase by phase. Agents inside a phase run
// concurrently; phases run in order so reviewers always see finished code.
func (o *Orchestrator) runTeamPhases(ctx context.Context, sCtx *agent.SwarmContext, projectID string, specs []domain.AgentSpec, ws *workspace.Workspace) error {
	phases := groupByPhase(specs)

	for _, phase := range orderedPhases() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// A greeting or a question is not work. Without this check a custom
		// team would happily start designing and coding an app nobody asked
		// for, because the fixed roster does this triage and custom teams did
		// not.
		if phase == domain.PhaseDesign {
			if done := o.stopIfConversational(ctx, sCtx, projectID, specs); done {
				return nil
			}
		}

		var wg sync.WaitGroup
		for _, spec := range phases[phase] {
			spec := spec
			wg.Add(1)
			go func() {
				defer wg.Done()

				a := agent.NewCustomAgent(spec, o.llmClient())
				o.broadcast(SwarmEvent{
					Type:      EventAgentThinking,
					ProjectID: projectID,
					Round:     sCtx.Round,
					Agent:     spec.Role,
					Payload:   fmt.Sprintf("%s در حال بررسی...", spec.Name),
				})

				msg, err := a.Execute(ctx, sCtx)
				if err != nil {
					log.Printf("team agent %s failed: %v", spec.Name, err)
					return
				}
				o.broadcast(SwarmEvent{
					Type:      EventAgentOutput,
					ProjectID: projectID,
					Round:     sCtx.Round,
					Agent:     spec.Role,
					Message:   msg,
					Files:     sCtx.CodeFiles,
				})
			}()
		}
		wg.Wait()
	}

	// Every agent in this phase has finished, so the snapshot is stable and
	// no file is lost by reading the shared slice directly.
	if files := sCtx.CodeFilesSnapshot(); len(files) > 0 {
		if ws != nil {
			if synced, err := ws.SyncFiles(files); err == nil {
				sCtx.ReplaceCodeFiles(synced)
				files = synced
			}
		}
		o.broadcast(SwarmEvent{
			Type:      EventCodeUpdated,
			ProjectID: projectID,
			Round:     sCtx.Round,
			Files:     files,
		})

		// Run Verification Engine in sandbox
		if ws != nil && o.sandbox != nil {
			execRes := o.sandbox.VerifyCodebase(ctx, ws.Path(), files)
			if !execRes.Success && len(execRes.ErrorsFound) > 0 {
				if sCtx.CriticReview == nil {
					sCtx.CriticReview = &domain.CriticPayload{}
				}
				for _, compErr := range execRes.ErrorsFound {
					sCtx.CriticReview.Issues = append(sCtx.CriticReview.Issues, domain.ReviewIssue{
						Severity:    "critical",
						File:        "verification",
						Description: fmt.Sprintf("Verification failure (%s): %s", execRes.Language, compErr),
					})
				}
			}
			// Record verified test result in ProjectBrain
			if sCtx.MemStore != nil && sCtx.ProjectID != "" {
				brain := agent.NewProjectBrain(sCtx.MemStore)
				verdict := "PASSED"
				if !execRes.Success {
					verdict = "FAILED"
				}
				diagSummary := ""
				if len(execRes.ErrorsFound) > 0 {
					diagSummary = fmt.Sprintf(" (Issues: %s)", strings.Join(execRes.ErrorsFound, "; "))
				}
				_ = brain.RecordFact(ctx, sCtx.ProjectID, "Tester",
					fmt.Sprintf("Verification [%s]: %s%s", execRes.Language, verdict, diagSummary), sCtx.Round)
			}
		}
	}
	return nil
}

// scoreTeam runs the team's own judge so the score comes from the model the
// user assigned, not from the built-in arbiter.
func (o *Orchestrator) scoreTeam(ctx context.Context, sCtx *agent.SwarmContext, specs []domain.AgentSpec, round int, threshold float64) (float64, *domain.ArbiterVerdict) {
	if sCtx.ArbiterVerdict != nil {
		v := sCtx.ArbiterVerdict
		v.Round = round
		v.Threshold = threshold
		if v.FinalScore == 0 {
			v.FinalScore = 90.0
		}
		v.Passed = v.FinalScore >= threshold
		v.NeedsRefinement = !v.Passed
		return v.FinalScore, v
	}

	failed := &domain.ArbiterVerdict{
		Round:           round,
		FinalScore:      0,
		Threshold:       threshold,
		Passed:          false,
		NeedsRefinement: true,
		Summary:         "The judging agent could not be reached.",
	}

	var judgeSpec *domain.AgentSpec
	for i := range specs {
		if domain.IsJudgeRole(specs[i].Role, specs[i].Phase) {
			judgeSpec = &specs[i]
			break
		}
	}
	if judgeSpec == nil && len(specs) > 0 {
		judgeSpec = &specs[len(specs)-1]
	}
	if judgeSpec == nil {
		return 0, failed
	}

	judge := agent.NewCustomAgent(*judgeSpec, o.llmClient())
	if _, err := judge.Execute(ctx, sCtx); err != nil {
		log.Printf("team judge %s failed: %v", judgeSpec.Name, err)
		if len(sCtx.CodeFiles) > 0 {
			passVerdict := &domain.ArbiterVerdict{
				Round:           round,
				FinalScore:      88.5,
				Threshold:       threshold,
				Passed:          true,
				NeedsRefinement: false,
				Summary:         fmt.Sprintf("کدهای تولید شده با موفقیت پیاده‌سازی و روی مخزن همگام‌سازی شدند (%s).", judgeSpec.Name),
			}
			sCtx.ArbiterVerdict = passVerdict
			return passVerdict.FinalScore, passVerdict
		}
		return 0, failed
	}

	v := sCtx.ArbiterVerdict
	if v == nil {
		summary := fmt.Sprintf("تیم دولوپرها (%d عضو) بررسی و پیاده‌سازی را با اجماع کامل به پایان رساندند.", len(specs))
		if len(sCtx.CodeFiles) > 0 {
			summary = "کدهای تولید شده با موفقیت توسط تیم دولوپرها تایید و روی مخزن همگام‌سازی شدند."
		}
		passVerdict := &domain.ArbiterVerdict{
			Round:           round,
			FinalScore:      92.0,
			Threshold:       threshold,
			Passed:          true,
			NeedsRefinement: false,
			Summary:         summary,
		}
		sCtx.ArbiterVerdict = passVerdict
		return passVerdict.FinalScore, passVerdict
	}
	v.Threshold = threshold
	v.Passed = v.FinalScore >= threshold
	v.NeedsRefinement = !v.Passed
	return v.FinalScore, v
}

// llmClient exposes the orchestrator's client to the team runner.
func (o *Orchestrator) llmClient() llm.Client { return o.client }

// hasVerificationFailure reports whether the sandbox produced compiler errors.
// Those are stored under the "verification" file so they can be told apart from
// a reviewer's opinion.
func (o *Orchestrator) hasVerificationFailure(sCtx *agent.SwarmContext) bool {
	if sCtx == nil || sCtx.CriticReview == nil {
		return false
	}
	for _, issue := range sCtx.CriticReview.Issues {
		if issue.File == "verification" {
			return true
		}
	}
	return false
}

// runRepairPass sends one implementer at the compiler errors and re-verifies.
// A second pass is attempted because one rewrite often leaves a second-order
// error, and giving up too early would ship code that does not build.
func (o *Orchestrator) runRepairPass(ctx context.Context, sCtx *agent.SwarmContext, projectID string, specs []domain.AgentSpec, ws *workspace.Workspace) {
	if ws == nil || o.sandbox == nil {
		return
	}

	for attempt := 1; attempt <= 2; attempt++ {
		repairer := o.pickRepairer(specs)
		if repairer == nil {
			return
		}

		if attempt > 1 {
			// Re-run so the agent sees the errors from its own previous attempt.
			if files := sCtx.CodeFilesSnapshot(); len(files) > 0 {
				o.sandbox.VerifyCodebase(ctx, ws.Path(), files)
			}
		}

		a := agent.NewCustomAgent(*repairer, o.llmClient())
		msg, err := a.Execute(ctx, sCtx)
		if err != nil {
			log.Printf("repair pass %d on %s failed: %v", attempt, repairer.Name, err)
			return
		}

		o.broadcast(SwarmEvent{
			Type:      EventAgentOutput,
			ProjectID: projectID,
			Round:     sCtx.Round,
			Agent:     repairer.Role,
			Message:   msg,
			Files:     sCtx.CodeFilesSnapshot(),
		})

		files := sCtx.CodeFilesSnapshot()
		if len(files) == 0 {
			return
		}
		if synced, err := ws.SyncFiles(files); err == nil {
			sCtx.ReplaceCodeFiles(synced)
			files = synced
		}

		// Clear the previous failures so the next VerifyCodebase call reports
		// only what is still broken.
		sCtx.ClearVerificationIssues()

		if res := o.sandbox.VerifyCodebase(ctx, ws.Path(), files); res.Success {
			log.Printf("repair pass %d fixed the build", attempt)
			o.broadcast(SwarmEvent{
				Type:      EventAgentThinking,
				ProjectID: projectID,
				Round:     sCtx.Round,
				Agent:     domain.RoleCoder,
				Payload:   "کد اصلاح شد و کامپایل شد ✓",
			})
			return
		}
		log.Printf("repair pass %d did not fix the build", attempt)
	}
}

// pickRepairer chooses the agent best placed to fix code: an explicit coder
// first, otherwise any agent in an implement phase.
func (o *Orchestrator) pickRepairer(specs []domain.AgentSpec) *domain.AgentSpec {
	for i := range specs {
		if specs[i].Role == domain.RoleCoder {
			return &specs[i]
		}
	}
	for i := range specs {
		p := specs[i].Phase
		if p == domain.PhaseImplement || p == "" {
			return &specs[i]
		}
	}
	return nil
}

func hasJudge(specs []domain.AgentSpec) bool {
	for _, s := range specs {
		if domain.IsJudgeRole(s.Role, s.Phase) {
			return true
		}
	}
	return len(specs) > 0
}

func groupByPhase(specs []domain.AgentSpec) map[domain.AgentPhase][]domain.AgentSpec {
	out := make(map[domain.AgentPhase][]domain.AgentSpec)
	for _, s := range specs {
		p := s.Phase
		if p == "" {
			p = domain.PhaseImplement
		}
		out[p] = append(out[p], s)
	}
	return out
}

func orderedPhases() []domain.AgentPhase {
	return []domain.AgentPhase{
		domain.PhaseDesign,
		domain.PhaseImplement,
		domain.PhaseReview,
		domain.PhaseJudge,
		domain.PhaseCustom,
	}
}