package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDomainSerialization(t *testing.T) {
	verdict := ArbiterVerdict{
		Round:           1,
		FinalScore:      88.5,
		Threshold:       85.0,
		Passed:          true,
		NeedsRefinement: false,
		Dimensions: []ScoringDimension{
			{Name: "Correctness", Weight: 0.30, Score: 90.0, Explanation: "Clean logic"},
			{Name: "Bug Resolution", Weight: 0.25, Score: 85.0, Explanation: "All fixed"},
			{Name: "Security Compliance", Weight: 0.20, Score: 95.0, Explanation: "No leaks"},
			{Name: "Code Quality", Weight: 0.15, Score: 80.0, Explanation: "Good structure"},
			{Name: "Spec Adherence", Weight: 0.10, Score: 90.0, Explanation: "Followed blueprint"},
		},
		FeedbackToCoder: "Approved for merge.",
		Summary:         "Passed quality consensus.",
	}

	bytes, err := json.Marshal(verdict)
	if err != nil {
		t.Fatalf("failed to marshal ArbiterVerdict: %v", err)
	}

	var parsed ArbiterVerdict
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal ArbiterVerdict: %v", err)
	}

	if parsed.FinalScore != 88.5 || !parsed.Passed {
		t.Errorf("unexpected parsed verdict: %+v", parsed)
	}
}

func TestMemoryEntryJSON(t *testing.T) {
	mem := MemoryEntry{
		ID:         "mem-123",
		ProjectID:  "proj-abc",
		Agent:      RoleArchitect,
		Type:       MemoryDecision,
		Content:    "Use gRPC for fast internal messaging",
		Tags:       []string{"grpc", "networking"},
		Round:      1,
		Confidence: 0.95,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	bytes, err := json.Marshal(mem)
	if err != nil {
		t.Fatalf("failed to marshal MemoryEntry: %v", err)
	}

	var unmarshaled MemoryEntry
	if err := json.Unmarshal(bytes, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal MemoryEntry: %v", err)
	}

	if unmarshaled.ID != "mem-123" || unmarshaled.Agent != RoleArchitect {
		t.Errorf("mismatched memory entry: %+v", unmarshaled)
	}
}
