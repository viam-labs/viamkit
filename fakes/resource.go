package fakes

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go.viam.com/rdk/resource"
)

// Resource is a minimal fake of resource.Resource for components and
// services that the system-under-test only interacts with via
// DoCommand — e.g. pack-sequencer and pick-station in the palletizer
// ecosystem. The fake captures each call so tests can assert "the
// palletizer sent report_placement with success=false."
//
// Two ways to script behavior:
//
//   - DoCommandFn: a single function consulted for every call. Useful
//     for simple cases ("always return this").
//   - SetResponse(verb, response): pre-registered responses keyed by
//     the top-level verb in the request map. Useful when you want
//     "calling next_box returns X; calling report_placement returns Y."
//
// SetResponse takes precedence over DoCommandFn for matching verbs.
type Resource struct {
	name resource.Name

	DoCommandFn func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)

	mu        sync.Mutex
	responses map[string]map[string]interface{}
	errors    map[string]error
	calls     []DoCommandCall

	doCommandCalls atomic.Int32
}

// DoCommandCall records one DoCommand invocation.
type DoCommandCall struct {
	Verb string                 // the first top-level key in the request map (best-effort)
	Cmd  map[string]interface{} // full request map
}

// NewResource constructs a Resource fake with the given API + name.
// The api argument is the SDK resource API the system-under-test
// expects — typically generic.API or worldstatestore.API. The fake
// only uses it for the resource.Name; method dispatch is entirely
// DoCommand-based.
func NewResource(api resource.API, name string) *Resource {
	return &Resource{
		name:      resource.NewName(api, name),
		responses: map[string]map[string]interface{}{},
		errors:    map[string]error{},
	}
}

// SetResponse pre-registers a response for a specific verb (the
// top-level key in the request map). The response is returned with a
// nil error. If both SetResponse and DoCommandFn match a verb,
// SetResponse wins.
func (r *Resource) SetResponse(verb string, response map[string]interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responses[verb] = response
	delete(r.errors, verb)
}

// SetError pre-registers an error for a specific verb. The DoCommand
// returns (nil, err).
func (r *Resource) SetError(verb string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errors[verb] = err
	delete(r.responses, verb)
}

// Calls returns a copy of every recorded DoCommand invocation in
// order. Safe to call concurrently with DoCommand.
func (r *Resource) Calls() []DoCommandCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]DoCommandCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// CallsFor returns the recorded calls whose verb matches the given
// string. Useful for "show me every report_placement that fired."
func (r *Resource) CallsFor(verb string) []DoCommandCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []DoCommandCall
	for _, c := range r.calls {
		if c.Verb == verb {
			out = append(out, c)
		}
	}
	return out
}

// DoCommandCalls returns the total invocation count.
func (r *Resource) DoCommandCalls() int { return int(r.doCommandCalls.Load()) }

// Reset clears the call log and counters but preserves
// SetResponse/SetError registrations.
func (r *Resource) Reset() {
	r.mu.Lock()
	r.calls = nil
	r.mu.Unlock()
	r.doCommandCalls.Store(0)
}

// ---- resource.Resource ----

// Name returns the resource name.
func (r *Resource) Name() resource.Name { return r.name }

// Close is a no-op.
func (r *Resource) Close(_ context.Context) error { return nil }

// Reconfigure is a no-op.
func (r *Resource) Reconfigure(_ context.Context, _ resource.Dependencies, _ resource.Config) error {
	return nil
}

// DoCommand records the call, then resolves the response in this
// order: SetResponse / SetError for the matching verb → DoCommandFn
// callback → resource.ErrDoUnimplemented.
//
// The "verb" in a DoCommand request is the top-level map key that
// dispatches the handler. When the request map has multiple keys
// (typical pattern: `{"verb": true, "arg_a": ..., "arg_b": ...}`),
// the fake picks the key whose value matches a registered response
// or error. Falls back to any registered key, then to the first
// key. This is what real DoCommand dispatchers do — they look up
// which key matches a known handler — and it makes the fake
// deterministic regardless of Go's random map iteration order.
func (r *Resource) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.doCommandCalls.Add(1)
	r.mu.Lock()
	verb := r.resolveVerbLocked(cmd)
	r.calls = append(r.calls, DoCommandCall{Verb: verb, Cmd: cmd})
	resp, hasResp := r.responses[verb]
	err, hasErr := r.errors[verb]
	fn := r.DoCommandFn
	r.mu.Unlock()

	if hasErr {
		return nil, err
	}
	if hasResp {
		return resp, nil
	}
	if fn != nil {
		return fn(ctx, cmd)
	}
	return nil, fmt.Errorf("fakes.Resource: no response registered for verb %q", verb)
}

// resolveVerbLocked picks the dispatch key for a request map. Prefers
// a key that has a registered response or error (so multi-key
// requests like `{"verb": true, "args": ...}` dispatch deterministically
// regardless of Go's random map iteration order). Falls back to any
// key if no match.
//
// Caller must hold r.mu.
func (r *Resource) resolveVerbLocked(cmd map[string]interface{}) string {
	for k := range cmd {
		if _, ok := r.responses[k]; ok {
			return k
		}
		if _, ok := r.errors[k]; ok {
			return k
		}
	}
	// No registered match — return any key so the unregistered-verb
	// error path can report a meaningful name.
	for k := range cmd {
		return k
	}
	return ""
}
