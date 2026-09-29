package scoring

import (
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestScorer_Evaluate(t *testing.T) {
	scorer := NewScorer()

	// Case 1: High quality round - should reach consensus (>= 85%)
	cleanInput := EvaluationInput{
		Round:              1,
		CriticIssues:       nil,
		SecurityVulns:      nil,
		CodeFilesCount:     4,
		UnresolvedBugs:     0,
		FixedBugsCount:     2,
		AdheresToBlueprint: true,
		CustomQualityScore: 92.0,
	}

	verdict := scorer.Evaluate(cleanInput)
	if !verdict.Passed || verdict.NeedsRefinement {
		t.Errorf("expected clean input to pass consensus: score=%.2f", verdict.FinalScore)
	}
	if verdict.FinalScore < 85.0 {
		t.Errorf("expected score >= 85.0, got %.2f", verdict.FinalScore)
	}

	// Case 2: Buggy input with critical vulnerability - should fail (< 85%)
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
	}

	failVerdict := scorer.Evaluate(vulnerableInput)
	if failVerdict.Passed || !failVerdict.NeedsRefinement {
		t.Errorf("expected vulnerable input to require refinement: score=%.2f", failVerdict.FinalScore)
	}
	if failVerdict.FinalScore >= 85.0 {
		t.Errorf("expected score < 85.0, got %.2f", failVerdict.FinalScore)
	}
}
