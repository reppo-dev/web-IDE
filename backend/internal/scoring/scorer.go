package scoring

import (
	"github.com/nexusai/nexusai/backend/internal/domain"
)

const ConsensusThreshold = 85.0

// Scorer calculates the round score across 5 weighted dimensions.
type Scorer struct {
	Threshold float64
}

// NewScorer creates a new Scorer with the default 85% consensus threshold.
func NewScorer() *Scorer {
	return &Scorer{
		Threshold: ConsensusThreshold,
	}
}

// EvaluationInput holds input artifacts from all agents in a round.
type EvaluationInput struct {
	Round              int
	CriticIssues       []domain.ReviewIssue
	SecurityVulns      []domain.ReviewIssue
	CodeFilesCount     int
	UnresolvedBugs     int
	FixedBugsCount     int
	AdheresToBlueprint bool
	CustomQualityScore float64 // Optional direct rating 0-100
}

// Evaluate produces the weighted ArbiterVerdict.
func (s *Scorer) Evaluate(input EvaluationInput) domain.ArbiterVerdict {
	// 1. Correctness (Weight: 30%)
	// Penalty for critical issues
	critPenalty := 0.0
	for _, issue := range input.CriticIssues {
		switch issue.Severity {
		case "critical":
			critPenalty += 25.0
		case "major":
			critPenalty += 10.0
		default:
			critPenalty += 3.0
		}
	}
	correctnessScore := 100.0 - critPenalty
	if correctnessScore < 0 {
		correctnessScore = 0
	}

	// 2. Bug Resolution (Weight: 25%)
	bugResolutionScore := 100.0
	totalBugs := input.UnresolvedBugs + input.FixedBugsCount
	if totalBugs > 0 {
		bugResolutionScore = (float64(input.FixedBugsCount) / float64(totalBugs)) * 100.0
	} else if len(input.CriticIssues) > 0 {
		bugResolutionScore = 70.0 // new round with initial bugs
	}

	// 3. Security Compliance (Weight: 20%)
	secPenalty := 0.0
	for _, v := range input.SecurityVulns {
		switch v.Severity {
		case "critical":
			secPenalty += 35.0
		case "major":
			secPenalty += 15.0
		default:
			secPenalty += 5.0
		}
	}
	securityScore := 100.0 - secPenalty
	if securityScore < 0 {
		securityScore = 0
	}

	// 4. Code Quality (Weight: 15%)
	qualityScore := 85.0
	if input.CustomQualityScore > 0 {
		qualityScore = input.CustomQualityScore
	}
	if input.CodeFilesCount == 0 {
		qualityScore = 40.0
	}

	// 5. Spec Adherence (Weight: 10%)
	specScore := 95.0
	if !input.AdheresToBlueprint {
		specScore = 50.0
	}

	dimensions := []domain.ScoringDimension{
		{Name: "Code Correctness", Weight: 0.30, Score: correctnessScore, Explanation: "Checks for logic bugs and edge case handling"},
		{Name: "Bug Resolution", Weight: 0.25, Score: bugResolutionScore, Explanation: "Tracks critic-demanded refactors fixed"},
		{Name: "Security Compliance", Weight: 0.20, Score: securityScore, Explanation: "Evaluates vulnerability footprint"},
		{Name: "Code Quality", Weight: 0.15, Score: qualityScore, Explanation: "Clean design and maintainability"},
		{Name: "Spec Adherence", Weight: 0.10, Score: specScore, Explanation: "Matches Architect specification"},
	}

	finalScore := 0.0
	for _, d := range dimensions {
		finalScore += d.Score * d.Weight
	}

	passed := finalScore >= s.Threshold
	needsRefinement := !passed

	summary := "Code approved by Arbiter. Consensus reached."
	feedback := "Proceeding to merge code into production workspace."
	if needsRefinement {
		summary = "Consensus not reached. Refinement required."
		feedback = "Please address the critical issues flagged by the Critic and Security Auditor."
	}

	return domain.ArbiterVerdict{
		Round:           input.Round,
		FinalScore:      finalScore,
		Threshold:       s.Threshold,
		Passed:          passed,
		NeedsRefinement: needsRefinement,
		Dimensions:      dimensions,
		FeedbackToCoder: feedback,
		Summary:         summary,
	}
}
