package statemachine

// Option configures a Machine at construction. Options are applied in
// order; later options can override earlier ones for the same state.
type Option[S comparable] func(*Machine[S])

// WithHandler registers the handler for one state.
func WithHandler[S comparable](state S, h Handler[S]) Option[S] {
	return func(m *Machine[S]) {
		m.handlers[state] = h
	}
}

// WithHandlers registers handlers for many states at once. Useful when
// you want to define your state→handler table as a literal map.
func WithHandlers[S comparable](table map[S]Handler[S]) Option[S] {
	return func(m *Machine[S]) {
		for s, h := range table {
			m.handlers[s] = h
		}
	}
}

// WithTerminal marks one or more states as terminal. Entering a
// terminal state cleanly exits Run with nil. Terminal states need no
// handler.
func WithTerminal[S comparable](states ...S) Option[S] {
	return func(m *Machine[S]) {
		for _, s := range states {
			m.terminals[s] = struct{}{}
		}
	}
}

// WithErrorState routes handler errors into the named state instead
// of returning them from Run. The error is recorded on LastError().
// If the error state itself is terminal, Run exits after the
// transition; otherwise its handler runs next.
//
// Use when you want a recoverable error state (e.g. "ERROR" that an
// operator can clear with Reset and Resume). Omit when handler errors
// should bubble all the way up.
func WithErrorState[S comparable](state S) Option[S] {
	return func(m *Machine[S]) {
		m.errState = state
		m.hasErrState = true
	}
}

// WithOnEntry registers a hook that fires when the machine enters the
// given state, before the state's handler runs. Use for per-state
// setup that should run every time the state is entered fresh.
//
// Fires:
//   - on the first Run/Step tick after construction (for the initial state)
//   - on every transition into the state via a handler return
//   - on the first Run/Step tick after Goto or Reset to the state
//
// Does NOT fire on Pause→Resume (state was never left). Does NOT fire
// on self-loops (handler returned the same state).
//
// Hook errors are treated like handler errors: routed to WithErrorState
// if configured, otherwise surfaced from Run/Step.
func WithOnEntry[S comparable](state S, hook EntryHook) Option[S] {
	return func(m *Machine[S]) {
		m.entryHooks[state] = hook
	}
}

// WithOnExit registers a hook that fires when the machine leaves the
// given state — after the handler returned a different state but
// before the transition is finalised. The hook receives the elapsed
// time in the state, useful for logging cycle durations or for
// time-bounded teardown.
//
// Does NOT fire on self-loops, on Goto away from the state, or on
// Reset. Goto/Reset are "low-level" state changes that skip the
// lifecycle hooks — by design, since they typically mean "we're
// abandoning whatever was happening and starting fresh."
//
// Hook errors are routed the same as handler errors.
func WithOnExit[S comparable](state S, hook ExitHook) Option[S] {
	return func(m *Machine[S]) {
		m.exitHooks[state] = hook
	}
}

// OnTransition installs a callback that fires after every state change
// (whether from a successful handler, an error route, or Step/Goto).
// The callback receives the (from, to) pair. Use for logging, metrics,
// or a "current state" stream to operator UIs.
//
// The callback is invoked from the goroutine that performed the
// transition. Don't block in it — for slow work, push onto a channel
// the callback reads from.
func OnTransition[S comparable](fn func(from, to S)) Option[S] {
	return func(m *Machine[S]) {
		m.onTransition = fn
	}
}
