package llm

import (
	"errors"
	"fmt"
	"sync"
)

// ErrBudgetExhausted is returned by guarded clients once the session token
// budget is fully spent (PROJECT_VISION §26: stop at 100%).
var ErrBudgetExhausted = errors.New("llm token budget exhausted")

// warnThreshold is the fraction of the budget at which a one-time warning
// fires (PROJECT_VISION §26: warn at 80%).
const warnThreshold = 0.8

// Budget tracks cumulative LLM token usage for the whole daemon session.
// It is safe for concurrent use. A maxTokens value of 0 means unlimited.
type Budget struct {
	mu        sync.Mutex
	maxTokens int
	used      int
	warned    bool
}

// NewBudget creates a budget with the given total-token cap.
// maxTokens <= 0 disables the limit (tracking only).
func NewBudget(maxTokens int) *Budget {
	return &Budget{maxTokens: maxTokens}
}

// Exhausted reports whether the budget is fully spent.
func (b *Budget) Exhausted() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxTokens > 0 && b.used >= b.maxTokens
}

// Record adds token usage from a completed call. It reports true the first
// time cumulative usage crosses the 80% warning threshold.
func (b *Budget) Record(promptTokens, outputTokens int) (warned bool) {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if promptTokens < 0 {
		promptTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	b.used += promptTokens + outputTokens

	if b.maxTokens > 0 && !b.warned && b.used >= int(float64(b.maxTokens)*warnThreshold) {
		b.warned = true
		warned = true
	}
	return warned
}

// BudgetSnapshot is a point-in-time view of budget state for logging and UI.
type BudgetSnapshot struct {
	MaxTokens  int  `json:"max_tokens"`
	UsedTokens int  `json:"used_tokens"`
	Remaining  int  `json:"remaining_tokens"` // -1 when unlimited
	Warned     bool `json:"warned"`
	Exhausted  bool `json:"exhausted"`
}

// Snapshot returns the current budget state.
func (b *Budget) Snapshot() BudgetSnapshot {
	if b == nil {
		return BudgetSnapshot{Remaining: -1}
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	snap := BudgetSnapshot{
		MaxTokens:  b.maxTokens,
		UsedTokens: b.used,
		Warned:     b.warned,
		Exhausted:  b.maxTokens > 0 && b.used >= b.maxTokens,
	}
	if b.maxTokens <= 0 {
		snap.Remaining = -1
	} else if rest := b.maxTokens - b.used; rest > 0 {
		snap.Remaining = rest
	}
	return snap
}

// Summary renders a short human-readable status line for logs.
func (b *Budget) Summary() string {
	snap := b.Snapshot()
	if snap.MaxTokens <= 0 {
		return fmt.Sprintf("tokens used: %d (no budget cap)", snap.UsedTokens)
	}
	return fmt.Sprintf("tokens used: %d/%d (warn=%v, exhausted=%v)",
		snap.UsedTokens, snap.MaxTokens, snap.Warned, snap.Exhausted)
}
