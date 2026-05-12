// Package cycle tracks per-cycle durations and rolling statistics for
// modules that do repeated units of work — picking and placing a box,
// running an inspection pass, executing one motion segment. It owns
// no state machine integration and works equally well standalone:
//
//	t := cycle.New(cycle.WithWindow(50))
//	for {
//	    t.Start()
//	    doWork()
//	    t.End()
//	    if s := t.Stats(); s.Count%10 == 0 {
//	        log.Infow("cycle stats", "last", s.Last, "p95", s.P95)
//	    }
//	}
//
// Or wire to viamkit/statemachine's OnEntry/OnExit hooks so cycle
// boundaries match a designated state.
//
// Only one cycle can be in flight at a time. Start while a cycle is
// already running silently discards the prior one (treated as
// cancelled). Use one Tracker per concurrent cycle if you need
// parallelism.
package cycle

import (
	"sort"
	"sync"
	"time"
)

// DefaultWindow is the number of recent cycles retained for rolling
// stats when no WithWindow option is passed.
const DefaultWindow = 100

// Tracker records cycle durations and computes rolling statistics
// over the most recent N completed cycles. Safe for concurrent use.
// Construct with New; the zero value is not usable.
type Tracker struct {
	mu     sync.Mutex
	window int

	// inFlightStartedAt is zero when no cycle is running.
	inFlightStartedAt time.Time

	// durations is the rolling window of completed cycles, oldest
	// first. Length grows up to `window`, then evicts oldest on
	// each new End().
	durations []time.Duration

	// totalCount counts every cycle that successfully called End,
	// across the whole lifetime — not just those still in the window.
	totalCount int
}

// New constructs a Tracker.
func New(opts ...Option) *Tracker {
	t := &Tracker{window: DefaultWindow}
	for _, o := range opts {
		o(t)
	}
	return t
}

// Option configures a Tracker.
type Option func(*Tracker)

// WithWindow sets the rolling-window size for stats (default 100).
// Passing a non-positive value is ignored.
func WithWindow(n int) Option {
	return func(t *Tracker) {
		if n > 0 {
			t.window = n
		}
	}
}

// Start marks the beginning of a new cycle. If a cycle is already in
// flight, it's silently discarded (treated as cancelled) — callers
// who care about that case should call Cancel or End explicitly first.
func (t *Tracker) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inFlightStartedAt = time.Now()
}

// End marks the end of the in-flight cycle and records its duration.
// No-op when no cycle is in flight.
func (t *Tracker) End() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.inFlightStartedAt.IsZero() {
		return
	}
	d := time.Since(t.inFlightStartedAt)
	t.inFlightStartedAt = time.Time{}
	t.totalCount++
	if len(t.durations) >= t.window {
		copy(t.durations, t.durations[1:])
		t.durations = t.durations[:len(t.durations)-1]
	}
	t.durations = append(t.durations, d)
}

// Cancel discards the in-flight cycle without recording. No-op when
// no cycle is in flight. Use when a cycle aborts (e.g. operator
// stop, error mid-cycle) and the partial duration shouldn't pollute
// the rolling stats.
func (t *Tracker) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inFlightStartedAt = time.Time{}
}

// Elapsed returns the duration of the in-flight cycle, or 0 if none
// is running.
func (t *Tracker) Elapsed() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.inFlightStartedAt.IsZero() {
		return 0
	}
	return time.Since(t.inFlightStartedAt)
}

// Running reports whether a cycle is currently in flight.
func (t *Tracker) Running() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.inFlightStartedAt.IsZero()
}

// Count returns the total number of cycles completed since this
// Tracker was constructed (independent of the rolling-window size).
func (t *Tracker) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.totalCount
}

// Reset clears the window and counters. Does not affect an in-flight
// cycle (use Cancel for that).
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.durations = t.durations[:0]
	t.totalCount = 0
}

// Stats returns rolling statistics over the retained window. A zero
// Stats is returned if no cycles have completed yet.
func (t *Tracker) Stats() Stats {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.durations) == 0 {
		return Stats{}
	}

	n := len(t.durations)
	sorted := make([]time.Duration, n)
	copy(sorted, t.durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var sum time.Duration
	for _, d := range t.durations {
		sum += d
	}

	return Stats{
		Count:      t.totalCount,
		WindowSize: n,
		Last:       t.durations[n-1],
		Min:        sorted[0],
		Max:        sorted[n-1],
		Mean:       sum / time.Duration(n),
		P50:        percentile(sorted, 0.50),
		P95:        percentile(sorted, 0.95),
	}
}

// Stats is a snapshot of the rolling statistics. Durations are
// always in nanoseconds (use the time.Duration arithmetic operators
// or `.Seconds()`, `.Milliseconds()` etc. for display).
type Stats struct {
	// Count is the total number of cycles completed since the
	// Tracker was constructed — not the rolling-window size.
	Count int
	// WindowSize is the number of cycles in the rolling window
	// right now (≤ the configured window).
	WindowSize int
	// Last is the most recent cycle's duration.
	Last time.Duration
	Min  time.Duration
	Max  time.Duration
	Mean time.Duration
	// P50, P95 are computed with linear interpolation between
	// adjacent samples; meaningful for windows of ≥ ~10 cycles.
	P50 time.Duration
	P95 time.Duration
}

// percentile returns the duration at the p-th percentile of the
// sorted slice using linear interpolation between adjacent samples.
// p in [0, 1].
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := p * float64(len(sorted)-1)
	lo := int(idx)
	if lo >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	frac := idx - float64(lo)
	// Add 0.5 to round to nearest nanosecond instead of truncating — a
	// frac like 0.8 against a 60ms span can produce 47.999999... ms
	// otherwise, off-by-one from the integer-arithmetic answer.
	return sorted[lo] + time.Duration(frac*float64(sorted[lo+1]-sorted[lo])+0.5)
}
