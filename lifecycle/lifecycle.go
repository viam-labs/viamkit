// Package lifecycle manages the two contexts a Viam module typically
// needs to juggle: a cancellable "loop" context tied to whatever
// in-flight work is running (state machine, scan loop, polling
// goroutine), and short-lived "cleanup" contexts that must execute
// even after the loop has been cancelled — to release a gripper, park
// an arm, tell a peer module the cycle ended, etc.
//
// The pattern is small but mistakes are subtle. If your cleanup code
// uses the same context as the loop, a Stop() that's the whole REASON
// for cleanup will make every cleanup RPC fail with context.Canceled
// before it runs. If your cleanup code uses context.Background() with
// no timeout, a wedged peer module will hang the cleanup forever and
// you'll be stuck in a half-stopped state. This package gives you the
// right shape: loop context is cancellable on demand and refreshes
// idempotently; cleanup contexts are short, independent, and always
// bounded.
//
// How it maps onto a Viam module:
//
// A module owns one Lifecycle for the life of its process. Three of
// its methods touch it — construction, Reconfigure, and Close — and
// each one wants a different context. The sketch below drives a
// viamkit/statemachine loop, but the shape is the same for any
// long-running goroutine (a scan loop, a poller).
//
//	type cellModule struct {
//	    resource.Named
//	    life *lifecycle.Lifecycle
//	    sm   *statemachine.Machine[state]
//	    peer resource.Resource // a sibling module reached via DoCommand
//	    arm  arm.Arm
//	}
//
//	// Construction: create the Lifecycle, start the loop on its ctx.
//	func newCellModule(deps resource.Dependencies) (*cellModule, error) {
//	    m := &cellModule{life: lifecycle.New()}
//	    m.sm = statemachine.New(stateIdle /*, WithHandlers(...), ... */)
//	    go m.sm.Run(m.life.Ctx()) // loop is cancellable via m.life.Stop()
//	    return m, nil
//	}
//
//	// Reconfigure: stop the running loop, tell a peer the old cycle
//	// ended, then start a fresh loop.
//	func (m *cellModule) Reconfigure(ctx context.Context, _ resource.Dependencies, _ resource.Config) error {
//	    m.life.Stop() // cancels m.life.Ctx(); the loop goroutine unwinds
//
//	    // CleanupCtx is independent of the (now-cancelled) loop ctx, so
//	    // this DoCommand still lands — and it's 5s-bounded, so a wedged
//	    // peer can't hang Reconfigure.
//	    cctx, cancel := m.life.CleanupCtx()
//	    defer cancel()
//	    m.peer.DoCommand(cctx, map[string]any{"reset_cursor": true})
//
//	    go m.sm.Run(m.life.EnsureLive()) // EnsureLive mints a fresh ctx
//	    return nil
//	}
//
//	// Close: terminate permanently, then do last-gasp teardown.
//	func (m *cellModule) Close(ctx context.Context) error {
//	    m.life.Close() // loop ctx cancelled for good; EnsureLive won't revive it
//
//	    // CleanupCtx keeps working after Close, so the arm still parks.
//	    cctx, cancel := m.life.CleanupCtx()
//	    defer cancel()
//	    return m.arm.MoveToJointPositions(cctx, parkPositions, nil)
//	}
//
// Why each context choice matters:
//
//   - The loop reads Ctx() so a Stop cancels every in-flight RPC under
//     it. Hand the loop a fresh context instead and Stop becomes a
//     no-op.
//   - Teardown work (the peer DoCommand, the arm park) uses CleanupCtx,
//     never Ctx() — Ctx() is cancelled by the very Stop/Close that
//     triggered the teardown, so a cleanup RPC on it would fail
//     instantly with context.Canceled.
//   - The next run uses EnsureLive(), not Ctx(): after a Stop the loop
//     ctx is cancelled, and EnsureLive mints a fresh one (and is a
//     no-op when the ctx is already live).
//
// For an operation that should be cancellable while the loop runs but
// must still complete during teardown — e.g. an operator-triggered
// DoCommand handler — CtxOrCleanup() folds that choice into one call.
package lifecycle

import (
	"context"
	"sync"
	"time"
)

// DefaultCleanupTimeout bounds CleanupCtx-derived contexts unless
// overridden via WithCleanupTimeout. Sized so a wedged peer fails fast
// enough to keep an operator's stop/reset feeling responsive, while
// still giving healthy peers room to finish.
const DefaultCleanupTimeout = 5 * time.Second

// Lifecycle owns a cancellable loop context plus a cleanup-context
// factory. Concurrent use is safe — every public method takes the
// internal mutex briefly.
//
// The zero value is NOT usable; construct with New.
type Lifecycle struct {
	cleanupTimeout time.Duration

	mu         sync.Mutex
	loopCtx    context.Context
	loopCancel context.CancelFunc
	closed     bool
}

// Option configures a Lifecycle at construction.
type Option func(*Lifecycle)

// WithCleanupTimeout sets the per-call timeout used by CleanupCtx and
// CtxOrCleanup. Defaults to DefaultCleanupTimeout (5s).
func WithCleanupTimeout(d time.Duration) Option {
	return func(l *Lifecycle) {
		if d > 0 {
			l.cleanupTimeout = d
		}
	}
}

// New constructs a Lifecycle with a fresh, non-cancelled loop context
// ready to use.
func New(opts ...Option) *Lifecycle {
	l := &Lifecycle{cleanupTimeout: DefaultCleanupTimeout}
	for _, o := range opts {
		o(l)
	}
	l.loopCtx, l.loopCancel = context.WithCancel(context.Background())
	return l
}

// Ctx returns the current loop context. It may already be cancelled if
// Stop or Close were called — by design. Callers driving cancellable
// operations want to *see* that cancellation propagate; auto-refreshing
// here would defeat Stop. Use EnsureLive when you explicitly want a
// fresh context (e.g. at the start of a new run).
func (l *Lifecycle) Ctx() context.Context {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loopCtx
}

// EnsureLive returns a non-cancelled loop context. If the current one
// has been cancelled (by Stop, or by an upstream context), a fresh one
// is minted; otherwise the existing one is returned unchanged.
//
// Idempotent on a live context — safe to call repeatedly. After Close,
// EnsureLive returns the already-cancelled context (no resurrection).
//
// Typical callers: the start path on a state machine; one-shot
// DoCommand handlers that need a live context whether or not a prior
// run was stopped.
func (l *Lifecycle) EnsureLive() context.Context {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return l.loopCtx
	}
	if l.loopCtx == nil || l.loopCtx.Err() != nil {
		l.loopCtx, l.loopCancel = context.WithCancel(context.Background())
	}
	return l.loopCtx
}

// Stop cancels the current loop context. Idempotent and non-terminal:
// a subsequent EnsureLive can mint a fresh loop context for the next
// run. Use Close to terminate permanently.
func (l *Lifecycle) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.loopCancel != nil {
		l.loopCancel()
	}
}

// CleanupCtx returns a fresh, timeout-bounded context independent of
// the loop context. Use for operations that must run during teardown
// or after Stop — releasing a gripper, parking an arm, notifying a
// peer module that the cycle ended. Always pair with the returned
// cancel (defer cancel()) to release timer resources.
//
// CleanupCtx keeps working after Close — last-gasp teardown still
// needs a context.
func (l *Lifecycle) CleanupCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), l.cleanupTimeout)
}

// CtxOrCleanup returns the loop context if it's still live, otherwise
// a fresh cleanup context. Use for operations that should be
// cancellable during normal running but must still execute during
// teardown. Always pair with the returned cancel; for the live-loop
// case it's a no-op, so `defer cancel()` is safe and correct.
func (l *Lifecycle) CtxOrCleanup() (context.Context, context.CancelFunc) {
	l.mu.Lock()
	ctx := l.loopCtx
	l.mu.Unlock()
	if ctx != nil && ctx.Err() == nil {
		return ctx, func() {}
	}
	return l.CleanupCtx()
}

// Close terminates the lifecycle. The loop context is cancelled and
// EnsureLive becomes a no-op (returns the cancelled context). Idempotent.
// CleanupCtx continues to function so Close-driven teardown can still
// run bounded RPCs.
func (l *Lifecycle) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	if l.loopCancel != nil {
		l.loopCancel()
	}
}

// Closed reports whether Close has been called.
func (l *Lifecycle) Closed() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closed
}

// CleanupCtx is the package-level convenience for code that doesn't
// want to construct a Lifecycle just to get a timeout-bounded context
// derived from Background. Equivalent to:
//
//	context.WithTimeout(context.Background(), timeout)
//
// but lives here so the intent ("this is cleanup, not loop work") is
// readable at the call site.
func CleanupCtx(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
