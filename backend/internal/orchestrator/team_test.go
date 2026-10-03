package orchestrator

import (
	"context"
	"fmt"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)


// The sandbox runs after every phase, so compiler errors used to be recorded
// and then judged without anyone ever fixing them. These tests cover the
// VERIFY -> REWORK link that closes that gap.
func TestHasVerificationFailure(t *testing.T) {
	o := &Orchestrator{}

	if o.hasVerificationFailure(&agent.SwarmContext{}) {
		t.Error("no review means no verification failure")
	}

	sCtx := &agent.SwarmContext{CriticReview: &domain.CriticPayload{Issues: []domain.ReviewIssue{
		{Severity: "major", File: "main.go", Description: "style"},
	}}}
	if o.hasVerificationFailure(sCtx) {
		t.Error("a reviewer's opinion must not be read as a compiler failure")
	}

	sCtx.CriticReview.Issues = append(sCtx.CriticReview.Issues, domain.ReviewIssue{
		Severity: "critical", File: "verification", Description: "compile error",
	})
	if !o.hasVerificationFailure(sCtx) {
		t.Error("a compiler failure must trigger the repair pass")
	}
}

func TestClearVerificationIssues(t *testing.T) {
	sCtx := &agent.SwarmContext{CriticReview: &domain.CriticPayload{Issues: []domain.ReviewIssue{
		{Severity: "major", File: "main.go", Description: "style"},
		{Severity: "critical", File: "verification", Description: "compile error"},
	}}}

	sCtx.ClearVerificationIssues()

	if len(sCtx.CriticReview.Issues) != 1 {
		t.Fatalf("expected only the review issue to remain, got %d", len(sCtx.CriticReview.Issues))
	}
	if sCtx.CriticReview.Issues[0].File != "main.go" {
		t.Errorf("the reviewer's own finding was removed: %+v", sCtx.CriticReview.Issues[0])
	}
}

func TestPickRepairer(t *testing.T) {
	o := &Orchestrator{}

	specs := []domain.AgentSpec{
		{ID: "a", Role: domain.RoleCustom, Phase: domain.PhaseDesign},
		{ID: "b", Role: domain.RoleCustom, Phase: domain.PhaseImplement},
		{ID: "c", Role: domain.RoleCustom, Phase: domain.PhaseReview},
	}
	got := o.pickRepairer(specs)
	if got == nil || got.ID != "b" {
		t.Fatalf("expected the implement-phase agent, got %+v", got)
	}

	// An explicit coder always wins, even in another phase.
	specs[2].Role = domain.RoleCoder
	got = o.pickRepairer(specs)
	if got == nil || got.ID != "c" {
		t.Errorf("expected the explicit coder, got %+v", got)
	}

	if o.pickRepairer([]domain.AgentSpec{
		{ID: "x", Role: domain.RoleCustom, Phase: domain.PhaseDesign},
	}) != nil {
		t.Error("a team with no implementer has nobody to repair")
	}
}

// An agent with no phase set defaults to implement, so it must still be chosen.
func TestPickRepairerDefaultsEmptyPhase(t *testing.T) {
	o := &Orchestrator{}
	got := o.pickRepairer([]domain.AgentSpec{{ID: "no-phase", Role: domain.RoleCustom}})
	if got == nil || got.ID != "no-phase" {
		t.Errorf("an agent with no phase should be treated as implement, got %+v", got)
	}
}

// A custom team has no built-in triage, so "سلام" used to start a full build.
func TestStopIfConversational(t *testing.T) {
	o := &Orchestrator{client: &intentLLM{kind: domain.IntentConversation}}

	sCtx := &agent.SwarmContext{
		ProjectID:  "p",
		Round:      1,
		UserPrompt: "سلام",
		APIKeys:    map[string]string{"openrouter": "k"},
		AgentModels: map[domain.AgentRole]string{
			domain.RoleArchitect: "openrouter/free",
		},
	}

	specs := []domain.AgentSpec{
		{ID: "arch", Role: domain.RoleArchitect, Phase: domain.PhaseDesign, Model: "openrouter/free", Enabled: true},
		{ID: "coder", Role: domain.RoleCustom, Phase: domain.PhaseImplement, Model: "openrouter/free", Enabled: true},
	}

	stopped := o.stopIfConversational(context.Background(), sCtx, "p", specs)
	if !stopped {
		t.Error("a greeting must stop the team before any code work")
	}
	if sCtx.Intent == nil || sCtx.Intent.Kind != domain.IntentConversation {
		t.Errorf("expected a conversation verdict, got %+v", sCtx.Intent)
	}
}

// A real build request must pass triage and let the phases run.
func TestStopIfBuildRequestContinues(t *testing.T) {
	o := &Orchestrator{client: &intentLLM{kind: domain.IntentBuild}}

	sCtx := &agent.SwarmContext{
		ProjectID:  "p",
		Round:      1,
		UserPrompt: "بساز یک اپ فلاتر",
		APIKeys:    map[string]string{"openrouter": "k"},
	}

	specs := []domain.AgentSpec{
		{ID: "arch", Role: domain.RoleArchitect, Phase: domain.PhaseDesign, Model: "openrouter/free", Enabled: true},
	}

	if o.stopIfConversational(context.Background(), sCtx, "p", specs) {
		t.Error("a build request must not be stopped by triage")
	}
}

// If triage itself fails the user should not be locked out of their project.
func TestStopIfTriageFailsContinues(t *testing.T) {
	o := &Orchestrator{client: &intentLLM{err: fmt.Errorf("upstream down")}}

	sCtx := &agent.SwarmContext{
		ProjectID:  "p",
		Round:      1,
		UserPrompt: "ساخت اپ",
		APIKeys:    map[string]string{"openrouter": "k"},
	}
	specs := []domain.AgentSpec{
		{ID: "arch", Role: domain.RoleArchitect, Phase: domain.PhaseDesign, Model: "openrouter/free", Enabled: true},
	}

	if o.stopIfConversational(context.Background(), sCtx, "p", specs) {
		t.Error("a triage failure must fall through to the real work")
	}
}

// A team with nobody who can triage should not stall on every request.
func TestStopIfNoDesigner(t *testing.T) {
	o := &Orchestrator{client: &intentLLM{kind: domain.IntentConversation}}

	sCtx := &agent.SwarmContext{ProjectID: "p", APIKeys: map[string]string{"openrouter": "k"}}
	specs := []domain.AgentSpec{
		{ID: "c1", Role: domain.RoleCustom, Phase: domain.PhaseImplement, Model: "openrouter/free", Enabled: true},
	}

	if o.stopIfConversational(context.Background(), sCtx, "p", specs) {
		t.Error("without a designer there is nobody to triage, so work should proceed")
	}
}

// intentLLM answers the triage call and then stops the agent from doing work.
type intentLLM struct {
	kind domain.IntentKind
	err  error
}

func (m *intentLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.Response{Content: `{"kind":"` + string(m.kind) + `","reply":"سلام!","reason":"greeting"}`}, nil
}
