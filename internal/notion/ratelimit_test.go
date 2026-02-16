package notion

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockClock provides deterministic time control for tests.
type mockClock struct {
	mu          sync.Mutex
	current     time.Time
	timers      []mockTimer
	timerNotify chan struct{}
	notifyOnce  sync.Once
}

type mockTimer struct {
	deadline time.Time
	ch       chan time.Time
}

func newMockClock() *mockClock {
	return &mockClock{
		current:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		timerNotify: make(chan struct{}, 1),
	}
}

// ensureNotifyChannel lazily initializes timerNotify to prevent blocking on a
// nil channel if mockClock{} is instantiated directly without newMockClock().
func (c *mockClock) ensureNotifyChannel() {
	c.notifyOnce.Do(func() {
		if c.timerNotify == nil {
			c.timerNotify = make(chan struct{}, 1)
		}
	})
}

func (c *mockClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

func (c *mockClock) After(d time.Duration) <-chan time.Time {
	c.ensureNotifyChannel()
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	deadline := c.current.Add(d)
	if !c.current.Before(deadline) {
		ch <- c.current
		return ch
	}
	c.timers = append(c.timers, mockTimer{deadline: deadline, ch: ch})
	// Notify waiters that a new timer was registered.
	select {
	case c.timerNotify <- struct{}{}:
	default:
	}
	return ch
}

// TimerCount returns the number of pending timers.
func (c *mockClock) TimerCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

// waitForTimers blocks until the mock clock has at least n pending timers.
func waitForTimers(t *testing.T, clk *mockClock, n int) {
	t.Helper()
	clk.ensureNotifyChannel()
	timeout := time.After(2 * time.Second)
	for clk.TimerCount() < n {
		select {
		case <-clk.timerNotify:
		case <-timeout:
			t.Fatalf("timed out waiting for %d timer(s); have %d", n, clk.TimerCount())
		}
	}
}

// Advance moves the clock forward and fires any pending timers.
func (c *mockClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.current = c.current.Add(d)
	now := c.current
	var remaining []mockTimer
	for _, t := range c.timers {
		if !now.Before(t.deadline) {
			t.ch <- now
		} else {
			remaining = append(remaining, t)
		}
	}
	c.timers = remaining
	c.mu.Unlock()
}

// rlFixture encapsulates the mock clock and rate limiter for test setup.
type rlFixture struct {
	clk *mockClock
	rl  *RateLimiter
}

func newRLFixture() *rlFixture {
	clk := newMockClock()
	return &rlFixture{
		clk: clk,
		rl:  newRateLimiter(clk, DefaultTokensPerSec),
	}
}

// drain sets tokens to zero.
func (f *rlFixture) drain() {
	f.rl.mu.Lock()
	defer f.rl.mu.Unlock()
	f.rl.tokens = 0
}

// state returns a snapshot of the limiter's internal fields under the mutex.
func (f *rlFixture) state() (tokens float64, tokensPerSec float64, maxBurst float64) {
	f.rl.mu.Lock()
	defer f.rl.mu.Unlock()
	return f.rl.tokens, f.rl.tokensPerSec, f.rl.maxBurst
}

// assertAvailable checks the current available tokens.
func (f *rlFixture) assertAvailable(t *testing.T, expected float64) {
	t.Helper()
	if got := f.rl.Available(); got != expected {
		t.Errorf("Available() = %v, want %v", got, expected)
	}
}

// acquireAsync runs Acquire in a background goroutine and returns a channel
// that receives the result. It waits for the goroutine to either register a
// timer on the mock clock or complete immediately.
func (f *rlFixture) acquireAsync(t *testing.T, ctx context.Context, op Operation) <-chan error {
	t.Helper()
	f.clk.ensureNotifyChannel()
	timersBefore := f.clk.TimerCount()
	ch := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		ch <- f.rl.Acquire(ctx, op)
		close(done)
	}()
	// Wait until either a new timer appears or the goroutine completes.
	timeout := time.After(2 * time.Second)
	for {
		select {
		case <-f.clk.timerNotify:
			if f.clk.TimerCount() > timersBefore {
				return ch
			}
		case <-done:
			return ch
		case <-timeout:
			t.Fatal("acquireAsync: timed out waiting for timer or completion")
			return ch
		}
	}
}

// Test Cases

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(DefaultTokensPerSec)

	if rl.maxBurst != DefaultMaxBurst {
		t.Errorf("maxBurst = %v, want %v", rl.maxBurst, DefaultMaxBurst)
	}

	if rl.tokens != DefaultMaxBurst {
		t.Errorf("initial tokens = %v, want %v", rl.tokens, DefaultMaxBurst)
	}

	if rl.tokensPerSec != DefaultTokensPerSec {
		t.Errorf("tokensPerSec = %v, want %v", rl.tokensPerSec, DefaultTokensPerSec)
	}
}

func TestNewRateLimiter_WithCustomRate(t *testing.T) {
	rl := NewRateLimiter(6.0) // 6 tokens per second

	expectedMaxBurst := 6.0 * DefaultMaxBurst / DefaultTokensPerSec // 20
	if rl.maxBurst != expectedMaxBurst {
		t.Errorf("maxBurst = %v, want %v", rl.maxBurst, expectedMaxBurst)
	}

	if rl.tokensPerSec != 6.0 {
		t.Errorf("tokensPerSec = %v, want %v", rl.tokensPerSec, 6.0)
	}
}

func TestNewRateLimiter_InvalidRate(t *testing.T) {
	rl := NewRateLimiter(0)
	if rl.tokensPerSec != DefaultTokensPerSec {
		t.Errorf("tokensPerSec = %v, want %v (clamped to default)", rl.tokensPerSec, DefaultTokensPerSec)
	}

	rl = NewRateLimiter(-5.0)
	if rl.tokensPerSec != DefaultTokensPerSec {
		t.Errorf("tokensPerSec = %v, want %v (clamped to default)", rl.tokensPerSec, DefaultTokensPerSec)
	}
}

func TestNewRateLimiter_NilClockPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("newRateLimiter(nil, ...) should panic")
		}
	}()
	newRateLimiter(nil, DefaultTokensPerSec)
}

func TestRateLimiter_InitialBurstCapacity(t *testing.T) {
	f := newRLFixture()

	// Should be able to acquire DefaultMaxBurst tokens without blocking
	for i := 0; i < int(DefaultMaxBurst); i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		if err := f.rl.Acquire(ctx, OpSearch); err != nil {
			t.Errorf("Acquire(%d) failed: %v", i, err)
		}
	}

	// Next acquire should require waiting
	f.clk.ensureNotifyChannel()
	done := f.acquireAsync(t, context.Background(), OpSearch)

	// Should have a timer registered (blocked waiting for tokens)
	if f.clk.TimerCount() < 1 {
		t.Fatal("expected acquire to block and register a timer")
	}

	// Advance clock to let more tokens accumulate
	f.clk.Advance(500 * time.Millisecond)

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Acquire() after delay failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Acquire() did not complete after token refill")
	}
}

func TestRateLimiter_ExcessRequestsBlock(t *testing.T) {
	f := newRLFixture()
	f.drain()

	// Try to acquire when no tokens are available
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := f.acquireAsync(t, ctx, OpGetPage)

	// Should register a timer
	if f.clk.TimerCount() < 1 {
		t.Fatal("expected acquire to block and register a timer")
	}

	// Advance slightly (but not enough for full token refill)
	f.clk.Advance(200 * time.Millisecond) // Should refill 0.6 tokens at 3/sec

	// Should still be waiting or succeed with refilled tokens
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Acquire() failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Acquire() did not complete")
	}
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	f := newRLFixture()
	f.drain()

	ctx, cancel := context.WithCancel(context.Background())

	done := f.acquireAsync(t, ctx, OpGetBlocks)

	// Wait for the acquire to register a timer
	waitForTimers(t, f.clk, 1)

	// Cancel the context
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Acquire() error = %v, want context.Canceled", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Acquire() did not return after context cancellation")
	}
}

func TestRateLimiter_ContextTimeout(t *testing.T) {
	f := newRLFixture()
	f.drain()
	// Set a slow refill so tokens won't accumulate quickly
	f.rl.mu.Lock()
	f.rl.tokensPerSec = 0.01 // Very slow
	f.rl.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Schedule cancellation when the mock clock advances
	go func() {
		<-f.clk.After(50 * time.Millisecond)
		cancel()
	}()
	waitForTimers(t, f.clk, 1)

	done := f.acquireAsync(t, ctx, OpQueryDatabase)

	// Advance mock clock past the cancel point
	f.clk.Advance(100 * time.Millisecond)

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Acquire() = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Acquire() did not return after context cancelled")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	f := newRLFixture()
	f.drain()

	f.assertAvailable(t, 0)

	// Advance clock by 1 second: should refill exactly DefaultTokensPerSec tokens
	f.clk.Advance(1 * time.Second)

	f.assertAvailable(t, DefaultTokensPerSec)
}

func TestRateLimiter_RefillWithoutExceedingCapacity(t *testing.T) {
	f := newRLFixture()

	// Advance significantly — tokens should not exceed maxBurst
	f.clk.Advance(10 * time.Second)

	avail := f.rl.Available()
	if avail > DefaultMaxBurst {
		t.Errorf("Available() = %v, should not exceed maxBurst %v", avail, DefaultMaxBurst)
	}
	if avail != DefaultMaxBurst {
		t.Errorf("Available() = %v, want %v (capped at maxBurst)", avail, DefaultMaxBurst)
	}
}

func TestRateLimiter_Available(t *testing.T) {
	f := newRLFixture()

	f.assertAvailable(t, DefaultMaxBurst)

	// Acquire one token
	f.rl.mu.Lock()
	f.rl.tokens--
	f.rl.mu.Unlock()

	f.assertAvailable(t, DefaultMaxBurst-1)
}

func TestRateLimiter_Stats(t *testing.T) {
	f := newRLFixture()

	ctx := context.Background()

	// Acquire some tokens with different operations
	f.rl.Acquire(ctx, OpSearch)
	f.rl.Acquire(ctx, OpSearch)
	f.rl.Acquire(ctx, OpGetPage)

	stats := f.rl.Stats()

	if stats[OpSearch] != 2 {
		t.Errorf("OpSearch count = %d, want 2", stats[OpSearch])
	}
	if stats[OpGetPage] != 1 {
		t.Errorf("OpGetPage count = %d, want 1", stats[OpGetPage])
	}
	if stats[OpGetBlocks] != 0 {
		t.Errorf("OpGetBlocks count = %d, want 0", stats[OpGetBlocks])
	}
}

func TestRateLimiter_StatsIsACopy(t *testing.T) {
	f := newRLFixture()

	ctx := context.Background()
	f.rl.Acquire(ctx, OpSearch)

	stats := f.rl.Stats()
	stats[OpSearch] = 999 // Modify the returned map

	// Original stats should not be affected
	stats2 := f.rl.Stats()
	if stats2[OpSearch] != 1 {
		t.Errorf("original stats were modified; OpSearch count = %d, want 1", stats2[OpSearch])
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	// Use real clock for concurrency test since goroutine scheduling is inherent
	rl := NewRateLimiter(DefaultTokensPerSec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errors := make(chan error, 50)
	successCount := atomic.Int32{}

	// Launch 50 concurrent requests
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rl.Acquire(ctx, OpSearch); err != nil {
				errors <- err
			} else {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent Acquire() error = %v", err)
	}

	if successCount.Load() != 50 {
		t.Errorf("only %d requests succeeded (expected 50)", successCount.Load())
	}
}

func TestRateLimiter_ThroughputApproximately3PerSecond(t *testing.T) {
	// Use real time to test actual throughput
	rl := NewRateLimiter(DefaultTokensPerSec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var startTime time.Time
	var endTime time.Time
	requestCount := 0

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			mu.Lock()
			if startTime.IsZero() {
				startTime = time.Now()
			}
			mu.Unlock()

			if err := rl.Acquire(ctx, OpSearch); err != nil {
				return
			}

			mu.Lock()
			requestCount++
			endTime = time.Now()
			mu.Unlock()
		}()

		// Small delay between starting goroutines to avoid race conditions on time tracking
		time.Sleep(10 * time.Millisecond)
	}

	wg.Wait()

	if requestCount < 20 {
		t.Fatalf("expected all 20 requests to succeed; got %d", requestCount)
	}

	duration := endTime.Sub(startTime).Seconds()
	actualThroughput := float64(requestCount) / duration

	// Allow ±50% tolerance (2.25 to 4.5 req/sec, nominal 3)
	minExpected := 1.5
	maxExpected := 4.5

	if actualThroughput < minExpected || actualThroughput > maxExpected {
		t.Logf("throughput = %.2f req/sec (duration = %.2f sec, requests = %d)",
			actualThroughput, duration, requestCount)
		t.Logf("nominal throughput is ~%.1f req/sec", DefaultTokensPerSec)
		if actualThroughput < minExpected {
			t.Errorf("throughput too low: %.2f < %.2f req/sec", actualThroughput, minExpected)
		}
		if actualThroughput > maxExpected {
			t.Errorf("throughput too high: %.2f > %.2f req/sec", actualThroughput, maxExpected)
		}
	}
}

func TestRateLimiter_OperationTypes(t *testing.T) {
	f := newRLFixture()
	ctx := context.Background()

	ops := []Operation{OpSearch, OpGetPage, OpGetBlocks, OpQueryDatabase, OpUpdatePage}

	for _, op := range ops {
		f.rl.Acquire(ctx, op)
	}

	stats := f.rl.Stats()

	for _, op := range ops {
		if stats[op] != 1 {
			t.Errorf("operation %s count = %d, want 1", op, stats[op])
		}
	}
}

func TestMockClock_ZeroValueSafe(t *testing.T) {
	// Verify that a zero-value mockClock{} won't block forever due to nil channel.
	clk := &mockClock{}

	// After should work without hanging
	ch := clk.After(10 * time.Millisecond)
	if ch == nil {
		t.Fatal("After() returned nil channel")
	}

	// timerNotify should be lazily initialized
	if clk.timerNotify == nil {
		t.Fatal("timerNotify should be initialized after After() call")
	}
}

func TestRateLimiter_RaceConditions(t *testing.T) {
	rl := NewRateLimiter(DefaultTokensPerSec)
	ctx := context.Background()

	var wg sync.WaitGroup

	// Multiple goroutines acquiring tokens
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			op := Operation("op_" + string(rune(id)))
			_ = rl.Acquire(ctx, op)
		}(i)
	}

	// Multiple goroutines checking available tokens
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = rl.Available()
		}()
	}

	// Multiple goroutines checking stats
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = rl.Stats()
		}()
	}

	wg.Wait()
	// Test passes if no race conditions detected by -race flag
}

func TestRateLimiter_TokenPrecision(t *testing.T) {
	f := newRLFixture()
	f.drain()

	// After 0.5 seconds, should have 1.5 tokens available (3 tokens/sec * 0.5)
	f.clk.Advance(500 * time.Millisecond)

	avail := f.rl.Available()
	expected := 1.5

	if avail != expected {
		t.Errorf("Available() after 500ms = %v, want %v", avail, expected)
	}
}

func TestRateLimiter_SequentialAcquisitions(t *testing.T) {
	f := newRLFixture()

	ctx := context.Background()

	// Acquire several tokens sequentially
	for i := 0; i < 10; i++ {
		done := f.acquireAsync(t, ctx, OpSearch)

		// Each should complete within reasonable time
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Acquire(%d) failed: %v", i, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("Acquire(%d) timed out", i)
		}

		// Advance clock a bit to refill tokens
		f.clk.Advance(time.Second / 3) // ~1 token per 1/3 second
	}
}
