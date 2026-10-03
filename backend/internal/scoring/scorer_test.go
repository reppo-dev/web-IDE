package scoring

import (
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestScorer_Evaluate(t *testing.T) {
	scorer := NewScorer()

	// Case 1: High quality round with completed plan - should reach consensus and complete
	cleanInput := EvaluationInput{
		Round:              1,
		CriticIssues:       nil,
		SecurityVulns:      nil,
		CodeFilesCount:     4,
		UnresolvedBugs:     0,
		FixedBugsCount:     2,
		AdheresToBlueprint: true,
		CustomQualityScore: 92.0,
		HasPlan:            true,
		TaskPassed:         true,
		OpenTasks:          0,
		TotalTasks:         2,
		PlanPercent:        100.0,
	}

	verdict := scorer.Evaluate(cleanInput)
	if !verdict.Passed || verdict.NeedsRefinement {
		t.Errorf("expected clean input with complete plan to pass consensus: score=%.2f", verdict.FinalScore)
	}
	if verdict.FinalScore < 85.0 {
		t.Errorf("expected score >= 85.0, got %.2f", verdict.FinalScore)
	}

	// Case 2: Clean code but open tasks remain - should not mark project complete, but round is clean
	incompleteInput := cleanInput
	incompleteInput.OpenTasks = 1
	incompleteInput.PlanPercent = 50.0

	incompleteVerdict := scorer.Evaluate(incompleteInput)
	if incompleteVerdict.Passed {
		t.Errorf("expected open tasks to prevent project completion")
	}
	if incompleteVerdict.NeedsRefinement {
		t.Errorf("clean round with open tasks should advance without code refinement")
	}

	// Case 3: Clean code with no plan - passes on code quality alone (for plan-less modes)
	noPlanInput := cleanInput
	noPlanInput.HasPlan = false

	noPlanVerdict := scorer.Evaluate(noPlanInput)
	if !noPlanVerdict.Passed || noPlanVerdict.NeedsRefinement {
		t.Errorf("expected plan-less input with clean code to pass consensus: score=%.2f", noPlanVerdict.FinalScore)
	}

	// Case 4: Buggy input with critical vulnerability - should fail (< 85%)
	vulnerableInput := EvaluationInput{
		Round: 1,
		CriticIssues: []domain.ReviewIssue{
			{Severity: "critical", Description: "Unchecked nil pointer in handler"},
			{Severity: "major", Description: "Missing mutex in concurrent cache"},
		},
		SecurityVulns: []domain.ReviewIssue{
			{Severity: "critical", Description: "SQL Injection in query concatenation"},
		},
		CodeFilesCount:     2,
		UnresolvedBugs:     3,
		FixedBugsCount:     0,
		AdheresToBlueprint: true,
		HasPlan:            true,
	}

	failVerdict := scorer.Evaluate(vulnerableInput)
	if failVerdict.Passed || !failVerdict.NeedsRefinement {
		t.Errorf("expected vulnerable input to require refinement: score=%.2f", failVerdict.FinalScore)
	}
	if failVerdict.FinalScore >= 85.0 {
		t.Errorf("expected score < 85.0, got %.2f", failVerdict.FinalScore)
	}
}
