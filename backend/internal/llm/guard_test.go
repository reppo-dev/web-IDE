package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fastGuard builds a Guard with millisecond backoffs so retry tests stay quick.
func fastGuard(inner Client, opts GuardOptions) *Guard {
	opts.BaseBackoff = time.Millisecond
	opts.MaxBackoff = 2 * time.Millisecond
	return NewGuard(inner, opts)
}

func okJSON(content string, prompt, completion int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatCompletionResponse{
			Choices: []chatCompletionChoice{{
				Message: chatMessage{Role: "assistant", Content: content},
			}},
			Usage: chatCompletionUsage{PromptTokens: prompt, CompletionTokens: completion},
		})
	}
}

func TestGuardRetriesTransientErrors(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			http.Error(w, "upstream busy", http.StatusServiceUnavailable)
			return
		}
		okJSON("recovered", 3, 3)(w, r)
	}))
	defer server.Close()

	g := fastGuard(NewOpenAICompatClient(Config{BaseURL: server.URL, Model: "m"}), GuardOptions{MaxAttempts: 3})
	resp, err := g.Generate(context.Background(), Request{UserPrompt: "test"})
	if err != nil {
		t.Fatalf("expected recovery after retries, got error: %v", err)
	}
	if resp.Content != "recovered" {
		t.Errorf("expected 'recovered', got %q", resp.Content)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 attempts (2x503 + success), got %d", got)
	}
}

func TestGuardDoesNotRetryPermanentErrors(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "bad api key", http.StatusUnauthorized)
	}))
	defer server.Close()

	g := fastGuard(NewOpenAICompatClient(Config{BaseURL: server.URL, Model: "m"}), GuardOptions{MaxAttempts: 3})
	_, err := g.Generate(context.Background(), Request{UserPrompt: "test"})
	if err == nil {
		t.Fatal("expected an error for 401")
	}
	if IsRetryable(err) {
		t.Errorf("401 must not be retryable, err=%v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("permanent errors must not be retried, got %d calls", got)
	}
}

func TestGuardGivesUpAfterMaxAttempts(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "overloaded", http.StatusTooManyRequests)
	}))
	defer server.Close()

	g := fastGuard(NewOpenAICompatClient(Config{BaseURL: server.URL, Model: "m"}), GuardOptions{MaxAttempts: 2})
	_, err := g.Generate(context.Background(), Request{UserPrompt: "test"})
	if err == nil {
		t.Fatal("expected error after exhausting attempts")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected exactly MaxAttempts (2) calls, got %d", got)
	}
}

func TestGuardLimitsConcurrentRequests(t *testing.T) {
	var cur, peak int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := atomic.AddInt32(&cur, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if now <= old || atomic.CompareAndSwapInt32(&peak, old, now) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		atomic.AddInt32(&cur, -1)
		okJSON("ok", 1, 1)(w, r)
	}))
	defer server.Close()

	g := fastGuard(NewOpenAICompatClient(Config{BaseURL: server.URL, Model: "m"}), GuardOptions{MaxConcurrent: 2})

	const total = 6
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := g.Generate(context.Background(), Request{UserPrompt: "test"}); err != nil {
				t.Errorf("request failed: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&peak); got > 2 {
		t.Errorf("observed %d concurrent server hits, limit is 2", got)
	} else if got < 2 {
		t.Errorf("limiter over-serialized: peak concurrency %d (want 2)", got)
	}
}

func TestGuardQueueHonorsContext(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		okJSON("ok", 1, 1)(w, r)
	}))
	defer server.Close()
	defer close(release)

	g := fastGuard(NewOpenAICompatClient(Config{BaseURL: server.URL, Model: "m"}), GuardOptions{MaxConcurrent: 1})

	done := make(chan error, 1)
	go func() {
		_, err := g.Generate(context.Background(), Request{UserPrompt: "holder"})
		done <- err
	}()

	time.Sleep(30 * time.Millisecond) // let the first call take the only slot

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := g.Generate(ctx, Request{UserPrompt: "queued"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("queued call should fail when its context expires")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context deadline error, got: %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("queued call waited %s; context should cut the line", elapsed)
	}
}

func TestGuardDoesNotRetryCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(okJSON("ok", 1, 1)))
	defer server.Close()

	g := fastGuard(NewOpenAICompatClient(Config{BaseURL: server.URL, Model: "m"}), GuardOptions{MaxAttempts: 3})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := g.Generate(ctx, Request{UserPrompt: "test"})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestGuardBudgetBlocksNextCallAfterExhaustion(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		okJSON("ok", 60, 40)(w, r)
	}))
	defer server.Close()

	budget := NewBudget(100)
	g := fastGuard(NewOpenAICompatClient(Config{BaseURL: server.URL, Model: "m"}),
		GuardOptions{MaxAttempts: 1, Budget: budget})

	if _, err := g.Generate(context.Background(), Request{UserPrompt: "first"}); err != nil {
		t.Fatalf("first call should pass, got: %v", err)
	}
	if _, err := g.Generate(context.Background(), Request{UserPrompt: "second"}); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("second call should be blocked by budget, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("blocked call must never reach the provider, got %d calls", got)
	}
	if !budget.Exhausted() {
		t.Error("budget should report exhausted after 100/100 tokens")
	}
}

func TestGuardRecordsBudgetAndWarns(t *testing.T) {
	server := httptest.NewServer(okJSON("ok", 10, 5))
	defer server.Close()

	budget := NewBudget(15)
	g := fastGuard(NewOpenAICompatClient(Config{BaseURL: server.URL, Model: "m"}),
		GuardOptions{MaxAttempts: 1, Budget: budget})

	if _, err := g.Generate(context.Background(), Request{UserPrompt: "test"}); err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	snap := budget.Snapshot()
	if snap.UsedTokens != 15 {
		t.Errorf("expected 15 used tokens, got %d", snap.UsedTokens)
	}
	if !snap.Warned {
		t.Error("expected 80% warning at 15/15 tokens")
	}
}

// TestGuardRetriesUpstreamOverloadWithHTTP200 reproduces the live failure seen
// through NineRouter: the upstream is overloaded, but the router answers with
// HTTP 200 and an error body instead of a 5xx status.
func TestGuardRetriesUpstreamOverloadWithHTTP200(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"error":{"message":"Upstream error from Nvidia: Service temporarily overloaded","type":"upstream_error"}}`))
			return
		}
		okJSON("all good", 4, 4)(w, r)
	}))
	defer server.Close()

	g := fastGuard(NewOpenAICompatClient(Config{BaseURL: server.URL, Model: "m"}), GuardOptions{MaxAttempts: 3})
	resp, err := g.Generate(context.Background(), Request{UserPrompt: "سلام"})
	if err != nil {
		t.Fatalf("expected the retry to recover, got: %v", err)
	}
	if resp.Content != "all good" {
		t.Errorf("expected recovered content, got %q", resp.Content)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 attempts (overload then success), got %d", got)
	}
}

func TestIsRetryableClassification(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"upstream overloaded", errors.New("llm returned error: Upstream error from Nvidia: Service temporarily overloaded"), true},
		{"rate limited body", errors.New("llm returned error: Rate limit reached, try again later"), true},
		{"gateway timeout body", errors.New("llm returned error: upstream request timed out"), true},
		{"http 503", errors.New("api error (status 503): service unavailable"), true},
		{"http 429", errors.New("api error (status 429): too many requests"), true},
		{"transport failure", errors.New("api request failed: Post \"http://x\": connection refused"), true},
		{"invalid key wins over busy", errors.New("llm returned error: invalid api key (upstream busy)"), false},
		{"model not found", errors.New("llm returned error: model not found"), false},
		{"context length", errors.New("llm returned error: maximum context length exceeded"), false},
		{"http 401", errors.New("api error (status 401): bad key"), false},
		{"http 400", errors.New("api error (status 400): malformed"), false},
		{"context canceled", fmt.Errorf("wrapped: %w", context.Canceled), false},
		{"budget exhausted", fmt.Errorf("wrapped: %w", ErrBudgetExhausted), false},
		{"plain decode failure", errors.New("failed to decode response: unexpected EOF"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsRetryable(tc.err); got != tc.want {
				t.Errorf("IsRetryable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
