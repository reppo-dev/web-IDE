package scoring

import (
	"fmt"
	"strings"

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

	// PlanPercent is how much of the project plan is finished, 0-100. The
	// arbiter must not declare the project done while open work remains,
	// however clean the current code happens to be.
	PlanPercent float64
	// OpenTasks is the number of tasks still standing. Zero with a non-empty
	// plan means the project is finished.
	OpenTasks int
	// HasPlan distinguishes "no plan was made" from "the plan is complete".
	// Without a plan the arbiter can only judge this round, never the project.
	HasPlan bool
	// TaskPassed reports whether the task this round worked on survived
	// verification against the real code. A round can be clean code and still
	// have failed its task, so this is scored separately from code quality.
	TaskPassed bool
	// TaskGaps are the reasons the active task is not finished. Any gap means
	// the task goes back to rework.
	TaskGaps []string
	// TotalTasks is the size of the plan, so progress can be reported as
	// "3 of 11" rather than as a percentage the reader cannot place.
	TotalTasks int
}

// IsProjectComplete reports whether the evidence says the whole project is
// finished.
//
// This is deliberately not just "score >= threshold". The failure this replaces
// was a run that scored 87% on a clean single file and stopped, with almost none
// of the project built. Code quality is necessary but never sufficient: work is
// only complete when the plan says so, and the plan can only say so when a
// verification gate closed each task against the real code.
func (i EvaluationInput) IsProjectComplete() bool {
	if !i.HasPlan {
		return true
	}
	return i.OpenTasks == 0 && i.TaskPassed
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

	// The round is "good" on its own terms when the code is clean, and that is a
	// real signal worth keeping: it is what drives the run forward. It is not,
	// however, a claim that the project is finished.
	roundClean := finalScore >= s.Threshold

	// Completion is decided by the plan, never by the score. This is the fix for
	// the original bug: a high score on an incomplete project used to end the
	// run, and a low score on a complete project used to keep it going forever.
	projectComplete := input.IsProjectComplete()
	passed := roundClean && projectComplete
	needsRefinement := !roundClean

	summary := "Code approved by Arbiter. Consensus reached."
	feedback := "Proceeding to merge code into production workspace."

	switch {
	case !roundClean:
		summary = "Consensus not reached. Refinement required."
		feedback = "Please address the critical issues flagged by the Critic and Security Auditor."
	case !input.HasPlan:
		// A clean round with no plan proves nothing about the project, so the
		// run continues: without a work list there is no way to know what
		// remains, and stopping here is exactly the bug this gate exists for.
		summary = "Code is clean, but no project plan exists, so completion cannot be proven."
		feedback = "Build a task plan first. A clean round alone never proves the project is finished."
	case !input.TaskPassed:
		// Checked before the open-task count so a failed task is reported as
		// rework on the task it failed, rather than being hidden behind a
		// message about tasks that are merely still to do.
		summary = fmt.Sprintf("Task failed verification in round %d. Returning it to rework.", input.Round)
		feedback = "The task did not meet its acceptance condition: " +
			strings.Join(input.TaskGaps, "; ")
	case input.OpenTasks > 0:
		summary = fmt.Sprintf(
			"Round %d is clean (%.1f%%), but %d of %d tasks remain open (%.0f%% complete). Continuing with the next task.",
			input.Round, finalScore, input.OpenTasks, input.TotalTasks, input.PlanPercent)
		feedback = "Move to the next open task. The project is not finished yet."
	default:
		summary = fmt.Sprintf("Project complete: every task verified against the real code (%.0f%% complete, score %.1f%%).",
			input.PlanPercent, finalScore)
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
