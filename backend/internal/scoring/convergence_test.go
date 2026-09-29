package scoring

import (
	"testing"
)

func TestConvergenceDetector_StalemateDetection(t *testing.T) {
	detector := NewConvergenceDetector(3)

	// Round 1: 79.0%
	detector.RecordRound(1, 79.0)
	if detector.IsStalemate() {
		t.Errorf("expected no stalemate on round 1")
	}

	// Round 2: 79.5%
	detector.RecordRound(2, 79.5)
	if detector.IsStalemate() {
		t.Errorf("expected no stalemate on round 2")
	}

	// Round 3: 79.8% (Stagnant delta < 1.5% across 3 rounds)
	detector.RecordRound(3, 79.8)
	if !detector.IsStalemate() {
		t.Errorf("expected stalemate to be detected across 3 stagnant rounds")
	}

	// Resolve stalemate
	resolvedScore, note := detector.ResolveStalemate(79.8)
	if resolvedScore < 85.0 {
		t.Errorf("expected stalemate resolution to cross consensus threshold: %.2f", resolvedScore)
	}
	if note == "" {
		t.Errorf("expected explanation note from tie breaker")
	}
}
