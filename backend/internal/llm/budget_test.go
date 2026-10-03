package llm

import (
	"sync"
	"testing"
)

func TestBudgetWarnAtEightyPercent(t *testing.T) {
	b := NewBudget(100)

	if warned := b.Record(79, 0); warned {
		t.Error("79% must not trigger the 80% warning")
	}
	if warned := b.Record(1, 0); !warned {
		t.Error("crossing 80% (80/100) must trigger the warning once")
	}
	if warned := b.Record(10, 0); warned {
		t.Error("warning must fire only once")
	}

	snap := b.Snapshot()
	if snap.UsedTokens != 90 {
		t.Errorf("expected 90 used tokens, got %d", snap.UsedTokens)
	}
	if snap.Remaining != 10 {
		t.Errorf("expected 10 remaining, got %d", snap.Remaining)
	}
	if snap.Exhausted {
		t.Error("90/100 is not exhausted yet")
	}
}

func TestBudgetExhaustion(t *testing.T) {
	b := NewBudget(50)

	if b.Exhausted() {
		t.Error("fresh budget must not be exhausted")
	}
	if warned := b.Record(50, 0); !warned {
		t.Error("50/50 crosses the 80% threshold, warning expected")
	}
	if !b.Exhausted() {
		t.Error("budget should be exhausted at 50/50")
	}

	snap := b.Snapshot()
	if snap.Exhausted {
		if snap.Remaining != 0 {
			t.Errorf("expected 0 remaining, got %d", snap.Remaining)
		}
	} else {
		t.Error("snapshot must report exhausted state")
	}
}

func TestBudgetUnlimited(t *testing.T) {
	b := NewBudget(0)

	if b.Exhausted() {
		t.Error("maxTokens=0 means unlimited, never exhausted")
	}
	if warned := b.Record(1_000_000, 2_000_000); warned {
		t.Error("no warning without a cap")
	}
	if snap := b.Snapshot(); snap.Remaining != -1 {
		t.Errorf("unlimited budget must report Remaining=-1, got %d", snap.Remaining)
	}
}

func TestNilBudgetIsSafe(t *testing.T) {
	var b *Budget
	if b.Exhausted() {
		t.Error("nil budget must never be exhausted")
	}
	if warned := b.Record(5, 5); warned {
		t.Error("nil budget must never warn")
	}
	if snap := b.Snapshot(); snap.Remaining != -1 {
		t.Errorf("nil budget snapshot should be unlimited, got %+v", snap)
	}
}

func TestBudgetConcurrentRecord(t *testing.T) {
	b := NewBudget(1000)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Record(10, 0)
		}()
	}
	wg.Wait()

	if got := b.Snapshot().UsedTokens; got != 1000 {
		t.Errorf("expected exactly 1000 tokens from 100 concurrent adds, got %d", got)
	}
}

func TestBudgetNegativeTokensClamped(t *testing.T) {
	b := NewBudget(100)
	b.Record(-50, -30)
	if got := b.Snapshot().UsedTokens; got != 0 {
		t.Errorf("negative usage must clamp to 0, got %d", got)
	}
}
