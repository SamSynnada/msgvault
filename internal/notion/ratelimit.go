package notion

import (
	"context"
	"sync"
	"time"
)

// Operation represents a Notion API operation type.
type Operation string

const (
	OpSearch       Operation = "search"
	OpGetPage      Operation = "get_page"
	OpGetBlocks    Operation = "get_blocks"
	OpQueryDatabase Operation = "query_db"
	OpUpdatePage   Operation = "update_page"
)

// Clock abstracts time operations for testability.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// realClock implements Clock using the standard time package.
type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// RateLimiter implements a token bucket rate limiter for Notion API calls.
// It is safe for concurrent use.
type RateLimiter struct {
	mu             sync.Mutex
	clock          Clock
	tokens         float64
	lastRefill     time.Time
	tokensPerSec   float64
	maxBurst       float64
	acquiredCount  map[Operation]int64
}

const (
	// DefaultTokensPerSec is the Notion API rate limit (3 requests per second).
	DefaultTokensPerSec = 3.0

	// DefaultMaxBurst is the maximum burst capacity.
	DefaultMaxBurst = 10.0
)

// NewRateLimiter creates a rate limiter with the default Notion API rate limits.
func NewRateLimiter(tokensPerSec float64) *RateLimiter {
	return newRateLimiter(realClock{}, tokensPerSec)
}

// newRateLimiter creates a rate limiter with the given clock and token rate.
// Panics if clk is nil.
func newRateLimiter(clk Clock, tokensPerSec float64) *RateLimiter {
	if clk == nil {
		panic("notion: RateLimiter requires a non-nil Clock")
	}

	// Clamp tokensPerSec to reasonable bounds
	if tokensPerSec <= 0 {
		tokensPerSec = DefaultTokensPerSec
	}

	maxBurst := tokensPerSec * DefaultMaxBurst / DefaultTokensPerSec
	if maxBurst < 1 {
		maxBurst = 1
	}

	return &RateLimiter{
		clock:        clk,
		tokens:       maxBurst, // Start with full burst capacity
		lastRefill:   clk.Now(),
		tokensPerSec: tokensPerSec,
		maxBurst:     maxBurst,
		acquiredCount: make(map[Operation]int64),
	}
}

// refill adds tokens based on elapsed time. Must be called with lock held.
func (rl *RateLimiter) refill() {
	now := rl.clock.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.lastRefill = now

	rl.tokens += elapsed * rl.tokensPerSec
	if rl.tokens > rl.maxBurst {
		rl.tokens = rl.maxBurst
	}
}

// reserve attempts to acquire one token. Returns 0 if successful, or the
// duration to wait before retrying.
func (rl *RateLimiter) reserve(op Operation) time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= 1 {
		rl.tokens--
		rl.acquiredCount[op]++
		return 0
	}

	// Calculate wait time to accumulate 1 token
	waitTime := time.Duration((1 - rl.tokens) / rl.tokensPerSec * float64(time.Second))
	if waitTime < time.Millisecond {
		waitTime = time.Millisecond
	}
	return waitTime
}

// Acquire blocks until a token is available and acquires it.
// Returns an error if the context is cancelled while waiting.
func (rl *RateLimiter) Acquire(ctx context.Context, op Operation) error {
	for {
		waitTime := rl.reserve(op)
		if waitTime == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-rl.clock.After(waitTime):
			continue
		}
	}
}

// Stats returns a copy of the operation counters.
func (rl *RateLimiter) Stats() map[Operation]int64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	stats := make(map[Operation]int64, len(rl.acquiredCount))
	for op, count := range rl.acquiredCount {
		stats[op] = count
	}
	return stats
}

// Available returns the current number of available tokens.
func (rl *RateLimiter) Available() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	return rl.tokens
}
