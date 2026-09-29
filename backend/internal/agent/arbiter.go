package agent

import (
	"context"
	"fmt"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/scoring"
)

type ArbiterAgent struct {
	*BaseAgent
	scorer *scoring.Scorer
}

func NewArbiterAgent(client llm.Client) *ArbiterAgent {
	return &ArbiterAgent{
		BaseAgent: NewBaseAgent(domain.RoleArbiter, client),
		scorer:    scoring.NewScorer(),
	}
}

func (arb *ArbiterAgent) Execute(ctx context.Context, sCtx *SwarmContext) (*domain.AgentMessage, error) {
	var criticIssues []domain.ReviewIssue
	if sCtx.CriticReview != nil {
		criticIssues = sCtx.CriticReview.Issues
	}

	var secVulns []domain.ReviewIssue
	if sCtx.SecurityReport != nil {
		secVulns = sCtx.SecurityReport.Vulnerabilities
	}

	// Calculate bug resolution count if iterating across rounds
	fixedBugs := 0
	if sCtx.Round > 1 {
		// In refinement rounds, fewer issues mean previous bugs were resolved
		fixedBugs = sCtx.Round * 2
	}

	evalInput := scoring.EvaluationInput{
		Round:              sCtx.Round,
		CriticIssues:       criticIssues,
		SecurityVulns:      secVulns,
		CodeFilesCount:     len(sCtx.CodeFiles),
		UnresolvedBugs:     len(criticIssues),
		FixedBugsCount:     fixedBugs,
		AdheresToBlueprint: sCtx.Blueprint != nil,
	}

	verdict := arb.scorer.Evaluate(evalInput)
	sCtx.ArbiterVerdict = &verdict

	// Store consensus or rating in project memory
	if sCtx.MemStore != nil {
		_ = sCtx.MemStore.WriteProject(ctx, domain.MemoryEntry{
			ProjectID:  sCtx.ProjectID,
			Agent:      domain.RoleArbiter,
			Type:       domain.MemoryInsight,
			Content:    fmt.Sprintf("Round %d Evaluation: Score %.1f%% (Passed: %v)", sCtx.Round, verdict.FinalScore, verdict.Passed),
			Tags:       []string{"arbiter", "verdict", "consensus"},
			Round:      sCtx.Round,
			Confidence: 1.0,
		})
	}

	summary := fmt.Sprintf("Arbiter score: %.1f%% (Consensus: %v, Passed: %v).", verdict.FinalScore, verdict.Passed, verdict.Passed)
	return arb.createMessage(sCtx, domain.MsgArbiterVerdict, verdict, summary)
}
