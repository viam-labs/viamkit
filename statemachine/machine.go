// Package statemachine is a small, generic finite state machine for
// driving Viam modules. It captures the same idiom every cycle-driven
// module reinvents:
//
//   - a typed set of states
//   - a handler per state that does the work and returns the next state
//   - a loop that runs handlers in sequence, bailing on context
//     cancellation, halting on terminal states, and routing handler
//     errors to a designated error state (or surfacing them if none
//     is configured)
//
// What you write becomes the *content* — what each state does. What
// the package gives you is the *control flow* — when to run, when to
// stop, when to error, and the read-only state-inspection accessors
// every operator UI eventually needs (Current, Running, LastError).
//
// Minimal usage:
//
//	type S string
//	const (
//	    SIdle S = "IDLE"
//	    SRun  S = "RUN"
//	    SDone S = "DONE"
//	    SErr  S = "ERROR"
//	)
//
//	m := statemachine.New(SIdle,
//	    statemachine.WithHandler(SIdle, func(ctx context.Context) (S, error) {
//	        return SRun, nil
//	    }),
//	    statemachine.WithHandler(SRun, func(ctx context.Context) (S, error) {
//	        // do work...
//	        return SDone, nil
//	    }),
//	    statemachine.WithTerminal(SDone, SErr),
//	    statemachine.WithErrorState(SErr),
//	)
//
//	go m.Run(ctx) // blocks until terminal, error, or ctx cancellation
//
// Concurrency: Run, Step, Goto, Reset, and the read accessors are all
// safe to call from multiple goroutines. Run and Step are mutually
// exclusive — Step returns an error if Run is in flight.
package statemachine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Handler runs the work for a single state and returns the state to
// transition to. Receive ctx for cancellation; an error halts the loop
// (or routes to the configured error state if one is set).
type Handler[S comparable] func(ctx context.Context) (S, error)

// EntryHook runs when the machine enters a state, before the state's
// handler runs. Use for per-state setup. An error from the hook is
// routed the same as a handler error: into the configured error state,
// or surfaced from Run/Step if none is set.
type EntryHook func(ctx context.Context) error

// ExitHook runs when the machine leaves a state, after the handler
// returned a different state and before the transition is finalised.
// timeInState is the elapsed time since this state was entered. Use
// for per-state teardown. Hook errors route the same as handler errors.
type ExitHook func(ctx context.Context, timeInState time.Duration) error

// Machine is a generic finite state machine. Construct with New;
// configure via Option values passed to New.
type Machine[S comparable] struct {
	initial      S
	handlers     map[S]Handler[S]
	entryHooks   map[S]EntryHook
	exitHooks    map[S]ExitHook
	terminals    map[S]struct{}
	errState     S
	hasErrState  bool
	onTransition func(from, to S)

	mu       sync.Mutex
	current  S
	running  bool
	lastErr  error
	// runDone is signalled (closed) when an in-flight Run exits, so
	// Stop callers can wait for the loop to settle if they want.
	runDone chan struct{}

	// Time tracking. All reads/writes under mu.
	// cycleStartedAt is set on the first Run after construction or
	// after Reset; zero before any cycle has started. Survives Stop
	// (so a Pause→Resume measures the same cycle).
	cycleStartedAt time.Time
	// stateEnteredAt is reset on every real transition (where the
	// next state differs from the current state). Self-loop returns
	// do not reset it — matches OnTransition's "only fires on actual
	// change" semantics.
	stateEnteredAt time.Time
	// lastSeen records the most recent time each state was entered.
	// Used by TimeSinceState.
	lastSeen map[S]time.Time

	// needsOnEntry signals that the OnEntry hook for the current
	// state should fire on the next Run/Step tick. Set by the
	// constructor (initial state needs entry), by transitionLocked
	// after every real transition, and by Goto/Reset.
	needsOnEntry bool

	// exitRequest holds a pending RequestExit. hasExitRequest gates
	// the value so the zero S value isn't ambiguous (a caller might
	// legitimately request the zero state). exitReason is reported
	// on LastError() when the request is honored — typically a
	// watchdog supplies "seal lost mid-transit" or similar so the
	// operator UI can show *why* the loop was redirected.
	exitRequest    S
	exitReason     error
	hasExitRequest bool
}

// New constructs a Machine starting in the given initial state.
// Options are applied in order; later options can override earlier
// ones for the same state.
func New[S comparable](initial S, opts ...Option[S]) *Machine[S] {
	now := time.Now()
	m := &Machine[S]{
		initial:        initial,
		current:        initial,
		handlers:       map[S]Handler[S]{},
		entryHooks:     map[S]EntryHook{},
		exitHooks:      map[S]ExitHook{},
		terminals:      map[S]struct{}{},
		stateEnteredAt: now,
		lastSeen:       map[S]time.Time{initial: now},
		needsOnEntry:   true,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Current returns the machine's current state.
func (m *Machine[S]) Current() S {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

// Running reports whether Run is currently driving the loop.
func (m *Machine[S]) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// LastError returns the error that ended the most recent Run, or nil
// if Run exited cleanly (terminal state reached or context cancelled).
// Cleared by Reset.
func (m *Machine[S]) LastError() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastErr
}

// Run drives the state machine until one of:
//   - a terminal state is entered (returns nil)
//   - ctx is cancelled (returns nil; cancellation is not an error)
//   - a handler returns an error and no error state is configured
//     (returns the error)
//
// If an error state is configured, handler errors route the machine
// there and Run continues — the error state's handler (or its
// terminality) decides what happens next. The error is recorded on
// LastError().
//
// Run blocks. Spawn it with `go m.Run(ctx)` if you want concurrency.
// Calling Run while it's already running returns ErrAlreadyRunning.
func (m *Machine[S]) Run(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}
	m.running = true
	m.lastErr = nil
	m.runDone = make(chan struct{})
	if m.cycleStartedAt.IsZero() {
		m.cycleStartedAt = time.Now()
	}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.running = false
		close(m.runDone)
		m.mu.Unlock()
	}()

	for {
		if err := ctx.Err(); err != nil {
			return nil // cancellation isn't a state-machine error
		}

		// Fire OnEntry for the current state if it hasn't fired
		// since we entered. Hook errors route the same as handler
		// errors (to errState if configured, else surfaced).
		if err := m.fireEntryIfNeeded(ctx); err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}
			if m.recordErrorAndRoute(err) {
				continue
			}
			return err
		}

		m.mu.Lock()
		current := m.current
		m.mu.Unlock()

		if _, terminal := m.terminals[current]; terminal {
			return nil
		}

		handler, ok := m.handlers[current]
		if !ok {
			err := fmt.Errorf("statemachine: no handler for state %v", current)
			m.mu.Lock()
			m.lastErr = err
			m.mu.Unlock()
			return err
		}

		next, err := handler(ctx)

		// RequestExit overrides whatever the handler decided. The
		// canonical use case: a background watchdog detects a
		// runtime fault, cancels the loop's context (handler returns
		// promptly with ctx.Canceled), and wants the machine to
		// land in StateError instead of unwinding to "no state
		// change." Honoring the request here means the machine ends
		// in the requested state before ctx.Err() exits the loop.
		if exit, reason, requested := m.takeExitRequest(); requested {
			if reason != nil {
				m.mu.Lock()
				m.lastErr = reason
				m.mu.Unlock()
			} else if err != nil {
				m.mu.Lock()
				m.lastErr = err
				m.mu.Unlock()
			}
			next = exit
			err = nil // routed to the requested state instead
		} else if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}
			if m.recordErrorAndRoute(err) {
				continue
			}
			return err
		}

		// Real transition? Fire OnExit on the state we're leaving,
		// then transition. Self-loops don't fire either hook.
		if next != current {
			if err := m.fireExit(ctx, current); err != nil {
				if errors.Is(err, context.Canceled) || ctx.Err() != nil {
					return nil
				}
				if m.recordErrorAndRoute(err) {
					continue
				}
				return err
			}
		}

		m.mu.Lock()
		fire := m.transitionLocked(next)
		m.mu.Unlock()
		fire()
	}
}

// transitionLocked records a transition into `next` and returns a
// closure that fires OnTransition (if configured and the state really
// changed). Caller must hold m.mu while calling transitionLocked, then
// release the lock before invoking the returned closure — that way
// the callback can do anything without risk of deadlocking on m.mu.
func (m *Machine[S]) transitionLocked(next S) func() {
	prev := m.current
	m.current = next
	if prev == next {
		return func() {}
	}
	now := time.Now()
	m.stateEnteredAt = now
	m.lastSeen[next] = now
	m.needsOnEntry = true
	cb := m.onTransition
	if cb == nil {
		return func() {}
	}
	return func() { cb(prev, next) }
}

// recordErrorAndRoute records err on LastError and, if WithErrorState
// was configured, transitions to that state. Returns true if a
// transition was made (caller can continue a loop), false if not
// (caller should surface the error). Caller must NOT hold m.mu.
func (m *Machine[S]) recordErrorAndRoute(err error) bool {
	m.mu.Lock()
	m.lastErr = err
	if !m.hasErrState {
		m.mu.Unlock()
		return false
	}
	fire := m.transitionLocked(m.errState)
	m.mu.Unlock()
	fire()
	return true
}

// fireEntryIfNeeded consults needsOnEntry and runs the OnEntry hook
// for the current state if it hasn't fired since last entry. Returns
// (hookErr, currentAfterRoute):
//   - hookErr is nil if the hook didn't run or ran successfully
//   - if the hook errored and routed to errState, current advances
//
// Caller must NOT hold m.mu when calling.
func (m *Machine[S]) fireEntryIfNeeded(ctx context.Context) error {
	m.mu.Lock()
	if !m.needsOnEntry {
		m.mu.Unlock()
		return nil
	}
	m.needsOnEntry = false
	hook := m.entryHooks[m.current]
	m.mu.Unlock()
	if hook == nil {
		return nil
	}
	return hook(ctx)
}

// fireExit runs the OnExit hook for `prev` (the state we're leaving),
// with the elapsed time-in-state. Caller must NOT hold m.mu.
func (m *Machine[S]) fireExit(ctx context.Context, prev S) error {
	m.mu.Lock()
	hook := m.exitHooks[prev]
	timeInState := time.Since(m.stateEnteredAt)
	m.mu.Unlock()
	if hook == nil {
		return nil
	}
	return hook(ctx, timeInState)
}

// Step runs the current state's handler exactly once and transitions
// to the returned state. Returns ErrAlreadyRunning if Run is in
// flight — stop the run first. Useful for testing and for operator
// step-through interfaces.
//
// Step does NOT advance through a terminal state; if the current
// state is terminal, Step is a no-op and returns nil.
func (m *Machine[S]) Step(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}
	m.mu.Unlock()

	if err := m.fireEntryIfNeeded(ctx); err != nil {
		m.recordErrorAndRoute(err)
		return err
	}

	m.mu.Lock()
	current := m.current
	if _, terminal := m.terminals[current]; terminal {
		m.mu.Unlock()
		return nil
	}
	handler, ok := m.handlers[current]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("statemachine: no handler for state %v", current)
	}

	next, err := handler(ctx)

	// RequestExit semantics in Step mirror Run: a pending request
	// overrides the handler's decision and lands the machine at the
	// requested state. Reason (if any) goes on LastError.
	if exit, reason, requested := m.takeExitRequest(); requested {
		if reason != nil {
			m.mu.Lock()
			m.lastErr = reason
			m.mu.Unlock()
		} else if err != nil {
			m.mu.Lock()
			m.lastErr = err
			m.mu.Unlock()
		}
		next = exit
		err = nil
	} else if err != nil {
		m.recordErrorAndRoute(err)
		return err
	}

	if next != current {
		if err := m.fireExit(ctx, current); err != nil {
			m.recordErrorAndRoute(err)
			return err
		}
	}

	m.mu.Lock()
	fire := m.transitionLocked(next)
	m.mu.Unlock()
	fire()
	return nil
}

// RequestExit asks the machine to transition to `state` at its next
// checkpoint — after the currently-running handler returns. The
// returned state from that handler is discarded and replaced with
// `state`. Useful when a background goroutine (watchdog, health check,
// operator panic-stop) needs to redirect the loop without violating
// the "no Goto during Run" invariant.
//
// `reason` is an optional error that gets recorded on LastError so an
// operator UI can show *why* the redirect happened — typical use:
// a watchdog passes `errors.New("seal lost mid-transit")`. If `reason`
// is nil and the handler returned a non-nil error, that handler error
// is recorded instead; otherwise LastError is left as-is.
//
// Pair RequestExit with cancelling the handler's context (e.g. via
// lifecycle.Stop) — RequestExit alone only stages the transition; it
// doesn't interrupt an in-flight handler. The combined idiom in a
// watchdog handler is:
//
//	m.RequestExit(StateError, errors.New("seal lost"))
//	lifecycle.Stop()
//
// If called multiple times before being honored, the most recent call
// wins. Cleared automatically once the redirect is applied.
func (m *Machine[S]) RequestExit(state S, reason error) {
	m.mu.Lock()
	m.exitRequest = state
	m.exitReason = reason
	m.hasExitRequest = true
	m.mu.Unlock()
}

// takeExitRequest atomically retrieves and clears any pending
// RequestExit. The boolean is true if one was pending.
func (m *Machine[S]) takeExitRequest() (S, error, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.hasExitRequest {
		var zero S
		return zero, nil, false
	}
	state := m.exitRequest
	reason := m.exitReason
	m.hasExitRequest = false
	m.exitReason = nil
	var zero S
	m.exitRequest = zero
	return state, reason, true
}

// Goto forces the current state without running the destination's
// handler. Returns ErrAlreadyRunning if Run is in flight.
//
// Useful for testing isolated states (pair with Step) and for
// recovery — e.g. jumping back to a safe state after an error.
func (m *Machine[S]) Goto(state S) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}
	// A state without a handler is only valid if it's terminal —
	// otherwise the next Run or Step will immediately error.
	if _, hasHandler := m.handlers[state]; !hasHandler {
		if _, terminal := m.terminals[state]; !terminal {
			m.mu.Unlock()
			return fmt.Errorf("statemachine: cannot goto unknown state %v (no handler, not terminal)", state)
		}
	}
	fire := m.transitionLocked(state)
	m.mu.Unlock()
	fire()
	return nil
}

// Reset returns the machine to its initial state and clears LastError.
// Returns ErrAlreadyRunning if Run is in flight.
func (m *Machine[S]) Reset() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}
	m.lastErr = nil
	m.cycleStartedAt = time.Time{} // next Run starts a fresh cycle
	// A pending RequestExit from a prior cycle is stale by definition
	// — drop it so the new cycle starts clean.
	m.hasExitRequest = false
	m.exitReason = nil
	var zero S
	m.exitRequest = zero
	fire := m.transitionLocked(m.initial)
	// transitionLocked sets needsOnEntry only when the state actually
	// changes. Reset semantics ("treat as a fresh start") imply we
	// want OnEntry to re-fire even if we were already in the initial
	// state, so set it explicitly.
	m.needsOnEntry = true
	m.mu.Unlock()
	fire()
	return nil
}

// States returns the registered state set: every state that has a
// handler plus every state declared terminal. Order is not guaranteed.
// Useful for operator UIs that present a goto dropdown.
func (m *Machine[S]) States() []S {
	seen := map[S]struct{}{}
	for s := range m.handlers {
		seen[s] = struct{}{}
	}
	for s := range m.terminals {
		seen[s] = struct{}{}
	}
	out := make([]S, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	return out
}

// IsTerminal reports whether the given state is registered as terminal.
func (m *Machine[S]) IsTerminal(state S) bool {
	_, ok := m.terminals[state]
	return ok
}

// IsDone reports whether the machine has reached a terminal state.
// Equivalent to IsTerminal(Current()).
func (m *Machine[S]) IsDone() bool {
	return m.IsTerminal(m.Current())
}

// TimeInState returns the elapsed time since the machine most recently
// transitioned into its current state. Self-loop returns (handler
// returning the same state) do not reset the timer.
//
// On a freshly constructed Machine, returns the time since construction.
// After Reset, returns the time since Reset.
func (m *Machine[S]) TimeInState() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return time.Since(m.stateEnteredAt)
}

// TimeInCycle returns the elapsed time since the current cycle began
// — defined as the most recent Run start that wasn't preceded by a
// Stop+Resume. Pause→Resume is treated as one cycle (the timer
// survives Stop); only Reset begins a fresh cycle.
//
// Returns 0 if Run has never been called.
func (m *Machine[S]) TimeInCycle() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cycleStartedAt.IsZero() {
		return 0
	}
	return time.Since(m.cycleStartedAt)
}

// TimeSinceState returns the elapsed time since the machine was most
// recently in `state`. The boolean is false if `state` has never been
// entered (so callers can distinguish "right now" from "never").
//
// If `state` is the current state, the duration equals TimeInState.
func (m *Machine[S]) TimeSinceState(state S) (time.Duration, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.lastSeen[state]
	if !ok {
		return 0, false
	}
	return time.Since(t), true
}

// ErrAlreadyRunning is returned by Step, Goto, and Reset when a Run
// is in flight, and by Run when a second concurrent Run is attempted.
var ErrAlreadyRunning = errors.New("statemachine: already running")
