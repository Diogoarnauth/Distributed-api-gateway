package retry

import (
	"math"
	"sync"
)

// Budget limits retries to a fraction of the traffic, so retries help with
// isolated failures but switch themselves off when everything is failing.
//
// Without it, an overloaded upstream gets even more load: 100 requests/s
// that all fail become 200 requests/s with one retry each, which keeps it
// down. With a 10% budget it gets at most ~110.
//
// It works like a piggy bank of tokens, updated on the spot (no timers, no
// time windows):
//
//	every request    deposits `ratio` tokens (0.1 = 10%)
//	every retry      needs 1 whole token; no token -> no retry
//	tokens are capped at `max`, so hours of calm can't pile up into a
//	huge burst of retries later
//
// The bank starts full, so the first failures after startup can be retried.
// Safe for concurrent use: every request runs in its own goroutine.
//
// Tokens are counted in thousandths with integers, not as float64: 0.1 has
// no exact binary representation, so adding 0.1 ten times gives
// 0.9999999999999999, which is less than one whole token.
type Budget struct {
	mu     sync.Mutex
	tokens int64 // in thousandths of a token
	ratio  int64
	max    int64
}

const milli = 1000 // thousandths per token

func NewBudget(ratio, max float64) *Budget {
	maxMilli := int64(math.Round(max * milli))

	return &Budget{
		tokens: maxMilli,
		ratio:  int64(math.Round(ratio * milli)),
		max:    maxMilli,
	}
}

// OnRequest records one incoming request (call once per request, not per
// attempt).
func (b *Budget) OnRequest() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.tokens = min(b.tokens+b.ratio, b.max)
}

// TryRetry spends one token if available and reports whether the retry may
// happen.
func (b *Budget) TryRetry() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.tokens < milli {
		return false
	}

	b.tokens -= milli
	return true
}
