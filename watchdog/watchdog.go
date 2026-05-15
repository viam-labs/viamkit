// Package watchdog provides a background-goroutine pattern for
// monitoring a condition at regular intervals and triggering a
// callback when it fails. Typical use cases:
//
//   - A gripper module watching IsHoldingSomething during transit, so
//     a dropped box mid-cycle aborts the motion instead of being
//     discovered only at the destination.
//   - A sensor module watching a temperature / current / pressure
//     reading for an unsafe value during an active operation.
//   - A peer module watching an RPC heartbeat to detect that the
//     other side has gone away.
//
// The pattern is small but easy to get wrong: a poll loop that
// doesn't honour context cancellation leaks goroutines; one that
// treats every RPC blip as a failure trips false alarms; one that
// has no graceful "the watching is no longer needed" exit grows
// orphan watchdogs.
//
// Minimal usage:
//
//	wd := watchdog.New(
//	    watchdog.WithInterval(200 * time.Millisecond),
//	    watchdog.WithCheck(func(ctx context.Context) (watchdog.Result, error) {
//	        status, err := gripper.IsHoldingSomething(ctx, nil)
//	        if err != nil {
//	            return watchdog.Transient, err
//	        }
//	        if !status.IsHoldingSomething {
//	            return watchdog.Lost, errors.New("seal lost")
//	        }
//	        return watchdog.Healthy, nil
//	    }),
//	    watchdog.WithShouldExit(func() bool { return !holding || stopped }),
//	    watchdog.WithOnFail(func(err error) {
//	        // signal abort: cancel lifecycle, set state, ...
//	    }),
//	)
//	wd.Start(parentCtx)
//	defer wd.Stop()
//
// Callback contract:
//   - OnFail fires exactly once per Watchdog run, when Check returns
//     Lost. The watchdog then exits.
//   - OnTransient fires every time Check returns Transient. Polling
//     continues; OnFail does not fire.
//   - Healthy returns invoke neither callback.
//
// Pair with statemachine.Machine.RequestExit if the watchdog needs
// to redirect a running FSM into an error state from its OnFail.
package watchdog

import (
	"context"
	"sync"
	"time"
)

// Result indicates the outcome of one Check tick.
type Result int

const (
	// Healthy: the watched condition is good. Keep polling.
	Healthy Result = iota
	// Lost: a failure was detected. The watchdog calls OnFail with
	// the returned error and exits.
	Lost
	// Transient: a temporary error occurred (e.g. RPC timeout). The
	// watchdog calls OnTransient (if set) and continues polling. Use
	// for cases where one bad poll shouldn't trigger an abort.
	Transient
)

// CheckFn is invoked every Interval to assess the watched condition.
//
// Return values:
//
//   - (Healthy, nil): everything's fine; continue.
//   - (Lost, err): failure detected; OnFail will be called with err,
//     then the watchdog exits.
//   - (Transient, err): transient error; OnTransient (if set) is
//     called with err, then polling continues. Use for retryable
//     blips that shouldn't abort the operation.
//
// The err returned with Healthy is ignored (callers can return nil).
type CheckFn func(ctx context.Context) (Result, error)

// Watchdog runs a background goroutine that polls a condition and
// fires a callback on failure. Construct with New; the zero value is
// not usable.
type Watchdog struct {
	interval    time.Duration
	check       CheckFn
	shouldExit  func() bool
	onFail      func(err error)
	onTransient func(err error)

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
	done    chan struct{}
}

// Option configures a Watchdog at construction.
type Option func(*Watchdog)

// WithInterval sets the poll interval. Must be > 0. Default: 200ms.
func WithInterval(d time.Duration) Option {
	return func(w *Watchdog) {
		if d > 0 {
			w.interval = d
		}
	}
}

// WithCheck sets the per-tick condition check. Required.
func WithCheck(fn CheckFn) Option {
	return func(w *Watchdog) { w.check = fn }
}

// WithShouldExit sets an optional predicate consulted before each
// check tick. If it returns true, the watchdog exits cleanly without
// firing OnFail. Use for "this watcher is no longer needed" cases
// (e.g. the gripper has released the box, the operator paused).
func WithShouldExit(fn func() bool) Option {
	return func(w *Watchdog) { w.shouldExit = fn }
}

// WithOnFail sets the callback fired when Check returns Lost. Called
// exactly once per Watchdog run. Required for the watchdog to do
// anything useful on failure.
func WithOnFail(fn func(err error)) Option {
	return func(w *Watchdog) { w.onFail = fn }
}

// WithOnTransient sets an optional callback fired whenever Check
// returns Transient. Use for logging or metrics; the watchdog
// continues polling either way.
func WithOnTransient(fn func(err error)) Option {
	return func(w *Watchdog) { w.onTransient = fn }
}

// DefaultInterval is the poll interval used when WithInterval isn't
// passed. 5 Hz is fast enough to catch most transit-window failures
// without flooding the watched RPC.
const DefaultInterval = 200 * time.Millisecond

// New constructs a Watchdog.
func New(opts ...Option) *Watchdog {
	w := &Watchdog{interval: DefaultInterval}
	for _, o := range opts {
		o(w)
	}
	return w
}

// Start spawns the watchdog goroutine. If a goroutine is already
// running, it's cancelled and the new one starts fresh. Use Stop to
// terminate.
//
// The parent context bounds the watchdog's lifetime: when parent is
// cancelled, the watchdog exits cleanly without firing OnFail
// (cancellation isn't a failure).
//
// Returns immediately; the goroutine runs in the background. No-op
// if CheckFn was not set via WithCheck.
func (w *Watchdog) Start(parent context.Context) {
	if w.check == nil {
		return
	}

	w.mu.Lock()
	if w.running {
		prevCancel := w.cancel
		prevDone := w.done
		w.mu.Unlock()
		if prevCancel != nil {
			prevCancel()
		}
		if prevDone != nil {
			<-prevDone
		}
		w.mu.Lock()
	}

	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	w.done = make(chan struct{})
	w.running = true
	done := w.done
	w.mu.Unlock()

	go w.run(ctx, done)
}

// Stop cancels the watchdog goroutine and blocks until it exits.
// Idempotent: safe to call when no goroutine is running.
func (w *Watchdog) Stop() {
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// Running reports whether a goroutine is currently active.
func (w *Watchdog) Running() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

func (w *Watchdog) run(ctx context.Context, done chan struct{}) {
	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
		close(done)
	}()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if w.shouldExit != nil && w.shouldExit() {
				return
			}
			result, err := w.check(ctx)
			switch result {
			case Healthy:
				// keep polling
			case Transient:
				if w.onTransient != nil {
					w.onTransient(err)
				}
			case Lost:
				if w.onFail != nil {
					w.onFail(err)
				}
				return
			}
		}
	}
}
