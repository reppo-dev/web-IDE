package scoring

import (
	"math"
)

// RoundTrend tracks score evolution across iterations.
type RoundTrend struct {
	Round int     `json:"round"`
	Score float64 `json:"score"`
}

// ConvergenceDetector identifies oscillation or deadlock between Coder and Critic.
type ConvergenceDetector struct {
	history         []RoundTrend
	stalemateLimit  int
	minDeltaPercent float64
}

// NewConvergenceDetector creates a new convergence analyzer.
func NewConvergenceDetector(stalemateLimit int) *ConvergenceDetector {
	if stalemateLimit <= 0 {
		stalemateLimit = 3
	}
	return &ConvergenceDetector{
		history:         make([]RoundTrend, 0),
		stalemateLimit:  stalemateLimit,
		minDeltaPercent: 1.5,
	}
}

// RecordRound adds a score point to the progression ledger.
func (c *ConvergenceDetector) RecordRound(round int, score float64) {
	c.history = append(c.history, RoundTrend{Round: round, Score: score})
}

// IsStalemate checks if the swarm is oscillating or stagnant without meaningful progress.
func (c *ConvergenceDetector) IsStalemate() bool {
	n := len(c.history)
	if n < c.stalemateLimit {
		return false
	}

	// Compare last N rounds
	lastScore := c.history[n-1].Score
	allStagnant := true

	for i := n - c.stalemateLimit; i < n-1; i++ {
		delta := math.Abs(c.history[i].Score - lastScore)
		if delta > c.minDeltaPercent {
			allStagnant = false
			break
		}
	}

	return allStagnant
}

// ResolveStalemate provides an adjusted verdict breaking the tie in favor of functional code.
func (c *ConvergenceDetector) ResolveStalemate(baseScore float64) (float64, string) {
	// If code works and is stalled by minor nitpicks, boost to threshold with advisory
	if baseScore >= 78.0 {
		return 86.0, "Arbiter invoked Stalemate-Breaker rule: Functional code prioritized over minor aesthetic critiques."
	}
	return baseScore, "Stalemate unresolved: Core functional issues remain."
}
