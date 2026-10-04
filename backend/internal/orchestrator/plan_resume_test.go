package orchestrator

import (
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

// A stored plan used to be resumed unconditionally. That made a second, different
// request in the same project silently continue the first one's work: the user
// asked for a Duolingo clone and the swarm kept building whatever was there
// before. Nothing errored; the output merely answered the wrong question.

// TestPlanMatchesSameRequest proves a re-run of the same request resumes.
func TestPlanMatchesSameRequest(t *testing.T) {
	plan := &domain.ProjectPlan{
		Goal: "Build a Duolingo clone for English learners",
		Tasks: []domain.ProjectTask{
			{Title: "Create the lesson data model"},
			{Title: "Build the exercise screen"},
		},
	}

	if !planMatchesRequest(plan, "Build a Duolingo clone for English learners") {
		t.Error("the same request should match its own plan")
	}
}

// TestPlanRejectsDifferentRequest is the regression this file exists for. The
// prompt deliberately shares only filler words with the plan.
func TestPlanRejectsDifferentRequest(t *testing.T) {
	plan := &domain.ProjectPlan{
		Goal: "Build a REST API for managing invoices",
		Tasks: []domain.ProjectTask{
			{Title: "Create the invoice database model"},
			{Title: "Build the invoice PDF exporter"},
		},
	}

	unrelated := []string{
		"یک اپلیکیشن کپی دوالینگو بسازید برای یادگیری زبان انگلیسی برای فارسی زبانان",
		"make a todo list app",
		"add dark mode to the settings page",
	}
	for _, prompt := range unrelated {
		if planMatchesRequest(plan, prompt) {
			t.Errorf("an unrelated request (%q) must not resume this plan", prompt)
		}
	}
}

// TestPlanRejectsSupersededPlan keeps an archived plan from being picked back up.
func TestPlanRejectsSupersededPlan(t *testing.T) {
	plan := &domain.ProjectPlan{
		Goal:  "(superseded) Build a Duolingo clone",
		Tasks: []domain.ProjectTask{{Title: "Create the lesson model"}},
	}
	if planMatchesRequest(plan, "Build a Duolingo clone") {
		t.Error("a superseded plan must never be resumed")
	}
}

// TestPlanMatchingNeedsRealContent guards against a plan or prompt made only of
// stop words being treated as a match on empty evidence.
func TestPlanMatchingNeedsRealContent(t *testing.T) {
	empty := &domain.ProjectPlan{Goal: "", Tasks: []domain.ProjectTask{{Title: "x"}}}
	if planMatchesRequest(empty, "build something") {
		t.Error("a plan with no goal cannot be matched")
	}

	stopWordsOnly := &domain.ProjectPlan{
		Goal:  "the and for with",
		Tasks: []domain.ProjectTask{{Title: "the and for"}},
	}
	if planMatchesRequest(stopWordsOnly, "the and for") {
		t.Error("matching on stop words alone is not evidence of the same request")
	}
	if planMatchesRequest(stopWordsOnly, "") {
		t.Error("an empty prompt must never match")
	}
}

// TestSignificantWordsKeepsContentWords pins the tokenisation the matcher relies
// on, including Persian text.
func TestSignificantWordsKeepsContentWords(t *testing.T) {
	words := significantWords("Build a Duolingo clone برای فارسی زبانان")
	if len(words) < 3 {
		t.Fatalf("expected content words to survive, got %v", words)
	}
	for _, w := range words {
		if w == "build" || w == "برای" {
			t.Errorf("filler word %q should have been dropped", w)
		}
	}
}
