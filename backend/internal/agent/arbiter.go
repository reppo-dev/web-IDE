package agent

import (
	"context"
	"fmt"
	"strings"

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

	// The plan is what proves the project is finished, so the arbiter reads it
	// directly instead of inferring completion from how clean the code looks.
	planPercent, openTasks, hasPlan := sCtx.PlanProgress()
	plan := sCtx.PlanSnapshot()
	totalTasks := 0
	if plan != nil {
		totalTasks = len(plan.Tasks)
	}

	taskPassed := false
	var taskGaps []string
	if v := sCtx.LastTaskVerdict; v != nil {
		taskPassed = v.Passed
		taskGaps = v.Gaps
	}

	evalInput := scoring.EvaluationInput{
		Round:              sCtx.Round,
		CriticIssues:       criticIssues,
		SecurityVulns:      secVulns,
		CodeFilesCount:     len(sCtx.CodeFilesSnapshot()),
		UnresolvedBugs:     len(criticIssues),
		FixedBugsCount:     fixedBugs,
		AdheresToBlueprint: sCtx.Blueprint != nil,
		PlanPercent:        planPercent,
		OpenTasks:          openTasks,
		HasPlan:            hasPlan,
		TaskPassed:         taskPassed,
		TaskGaps:           taskGaps,
		TotalTasks:         totalTasks,
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

	summary := strings.TrimSpace(verdict.Summary)
	if summary == "" {
		status := "تایید شد"
		if !verdict.Passed {
			status = "نیاز به بازنگری"
		}
		summary = fmt.Sprintf("رأی نهایی داور: امتیاز کیفیت %.1f%% (%s)", verdict.FinalScore, status)
	}
	reason := verdict.FeedbackToCoder
	if reason == "" {
		reason = "ارزیابی تجمیعی کارایی، امنیت و سازگاری کدها برای صدور رأی نهایی."
	}
	return arb.createMessageWithReasoning(sCtx, domain.MsgArbiterVerdict, verdict, summary, reason)
}

