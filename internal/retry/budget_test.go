package retry

import (
	"sync"
	"testing"
)

func TestBudgetStartsFull(t *testing.T) {
	b := NewBudget(0.1, 3)

	for i := 0; i < 3; i++ {
		if !b.TryRetry() {
			t.Fatalf("retry %d should be allowed by the initial tokens", i+1)
		}
	}

	if b.TryRetry() {
		t.Fatal("4th retry should be refused: bank is empty")
	}
}

func TestBudgetRefillsWithTraffic(t *testing.T) {
	b := NewBudget(0.1, 10)

	for b.TryRetry() { // empty the bank
	}

	// 9 requests deposit 0.9 tokens: not enough for a retry yet.
	for i := 0; i < 9; i++ {
		b.OnRequest()
	}
	if b.TryRetry() {
		t.Fatal("0.9 tokens should not allow a retry")
	}

	// The 10th request completes the token.
	b.OnRequest()
	if !b.TryRetry() {
		t.Fatal("1 token should allow a retry")
	}
}

func TestBudgetIsCapped(t *testing.T) {
	b := NewBudget(0.1, 2)

	for i := 0; i < 1000; i++ { // a long calm period
		b.OnRequest()
	}

	allowed := 0
	for b.TryRetry() {
		allowed++
	}

	if allowed != 2 {
		t.Fatalf("expected the cap of 2 retries, got %d", allowed)
	}
}

func TestBudgetLimitsRetriesUnderTotalFailure(t *testing.T) {
	// 1000 requests, all failing, each wanting one retry, from many
	// goroutines at once: retries must stay near 10% (+ the initial 10).
	b := NewBudget(0.1, 10)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		retries int
	)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.OnRequest()
			if b.TryRetry() {
				mu.Lock()
				retries++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if retries > 110 {
		t.Fatalf("expected at most 110 retries (10%% + initial 10), got %d", retries)
	}
}
