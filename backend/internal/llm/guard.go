package llm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

// GuardOptions configures the resilience middleware around a Client.
type GuardOptions struct {
	// MaxConcurrent caps in-flight requests toward the provider. Extra
	// callers wait in a queue instead of hammering the endpoint (STATUS.md
	// priority 3: prevent 5 heavy parallel requests against local NineRouter).
	// Values < 1 default to 1.
	MaxConcurrent int

	// MaxAttempts is the total number of tries per request (including the
	// first). Values < 1 default to 1 (no retries).
	MaxAttempts int

	// BaseBackoff is the delay before the first retry; it doubles each
	// attempt, capped at MaxBackoff, with full jitter. Defaults: 500ms / 8s.
	BaseBackoff time.Duration
	MaxBackoff  time.Duration

	// Budget, when set, blocks calls once the session token budget is
	// exhausted and records usage of every successful call (PROJECT_VISION §26).
	Budget *Budget
}

// Guard wraps any Client with a concurrency limiter, transient-error retry
// with exponential backoff, and an optional token budget. It implements
// Client, so it is a drop-in replacement: agents keep calling Generate
// unchanged while the guard absorbs 429/5xx/network blips from the provider.
type Guard struct {
	inner Client
	opts  GuardOptions
	sem   chan struct{}
}

// NewGuard wraps inner with the given options, applying defaults for unset
// fields.
func NewGuard(inner Client, opts GuardOptions) *Guard {
	if opts.MaxConcurrent < 1 {
		opts.MaxConcurrent = 1
	}
	if opts.MaxAttempts < 1 {
		opts.MaxAttempts = 1
	}
	if opts.BaseBackoff <= 0 {
		opts.BaseBackoff = 500 * time.Millisecond
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = 8 * time.Second
	}
	return &Guard{
		inner: inner,
		opts:  opts,
		sem:   make(chan struct{}, opts.MaxConcurrent),
	}
}

// Generate runs the guarded call: budget pre-check, queue for a slot, then
// retry transient failures with capped exponential backoff and full jitter.
func (g *Guard) Generate(ctx context.Context, req Request) (*Response, error) {
	if g.opts.Budget != nil && g.opts.Budget.Exhausted() {
		return nil, fmt.Errorf("%w: %s", ErrBudgetExhausted, g.opts.Budget.Summary())
	}

	// Queue for a provider slot, honoring cancellation while waiting.
	select {
	case g.sem <- struct{}{}:
		defer func() { <-g.sem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("llm guard: %w", ctx.Err())
	}

	var lastErr error
	for attempt := 1; attempt <= g.opts.MaxAttempts; attempt++ {
		resp, err := g.inner.Generate(ctx, req)
		if err == nil {
			if g.opts.Budget != nil {
				if warned := g.opts.Budget.Record(resp.PromptTokens, resp.OutputTokens); warned {
					log.Printf("[Guard] token budget at 80%%: %s", g.opts.Budget.Summary())
				}
			}
			return resp, nil
		}
		lastErr = err

		if !IsRetryable(err) || attempt == g.opts.MaxAttempts {
			return nil, err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("llm guard: %w", ctxErr)
		}

		delay := g.backoffFor(attempt)
		log.Printf("[Guard] transient error (attempt %d/%d), retrying in %s: %v",
			attempt, g.opts.MaxAttempts, delay, err)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, fmt.Errorf("llm guard: %w", ctx.Err())
		}
	}
	return nil, lastErr
}

// backoffFor computes the full-jitter exponential delay for an attempt
// (1-based), capped at MaxBackoff.
func (g *Guard) backoffFor(attempt int) time.Duration {
	exp := g.opts.BaseBackoff << (attempt - 1)
	if exp <= 0 || exp > g.opts.MaxBackoff {
		exp = g.opts.MaxBackoff
	}
	// Full jitter: uniform in [exp/2, exp].
	half := exp / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

// Snapshot returns the budget state (or an unlimited snapshot when unset),
// for observability dashboards and logs (PROJECT_VISION §37).
func (g *Guard) Snapshot() BudgetSnapshot {
	return g.opts.Budget.Snapshot()
}

// transientMarkers are phrases providers use inside an error body (frequently
// with HTTP 200, e.g. "Upstream error from Nvidia: Service temporarily
// overloaded") to mean "try again shortly". Local routers such as NineRouter
// surface upstream outages this way, so string matching is the only signal
// available at the Client interface boundary.
var transientMarkers = []string{
	"temporarily", "temporary", "overload", "rate limit", "ratelimit",
	"too many requests", "timeout", "timed out", "try again", "retry later",
	"service unavailable", "upstream error", "bad gateway", "capacity",
	"busy", "throttl", "server error", "internal error",
}

// permanentMarkers are phrases that must never be retried, even when they
// appear alongside a transient-looking word.
var permanentMarkers = []string{
	"invalid api key", "incorrect api key", "unauthorized", "authentication",
	"permission denied", "model not found", "does not exist", "invalid model",
	"context length", "too many tokens", "invalid request", "bad request",
}

// IsRetryable classifies provider errors as transient. It relies on the
// error strings produced by this package ("api error (status N): ...",
// "api request failed: ...") and on the provider's own error text, which are
// the only signals a Client interface error carries. Permanent failures (bad
// request, auth, not found) and caller cancellations are never retried.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, ErrBudgetExhausted) {
		return false
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	// Transport-level failures (connection refused, reset, timeout).
	if strings.Contains(lower, "api request failed:") {
		return true
	}
	// HTTP status failures: retry only the transient codes.
	if idx := strings.Index(lower, "api error (status "); idx >= 0 {
		rest := lower[idx+len("api error (status "):]
		end := strings.IndexAny(rest, ":)")
		if end > 0 {
			if code, convErr := strconv.Atoi(strings.TrimSpace(rest[:end])); convErr == nil {
				return code == 429 || code == 500 || code == 502 || code == 503 || code == 504
			}
		}
	}
	// Provider error bodies, often delivered with HTTP 200.
	for _, marker := range permanentMarkers {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	for _, marker := range transientMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
