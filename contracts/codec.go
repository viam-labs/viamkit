// Package contracts holds codec helpers for Viam DoCommand wire
// format. DoCommand at the protocol level is just
// `map[string]interface{}` over gRPC; defining typed Go structs that
// describe the wire shape lets producers and consumers refer to the
// contract by name instead of probing map keys, and lets the compiler
// catch typos before they reach a running module.
//
// The package has two layers:
//
//   - The generic codec helpers (ToMap, FromMap[T], MustToMap) in this
//     file. They are module-agnostic — any consumer can define its own
//     request / response structs and dispatch them through these.
//   - Typed wire-format structs and verb constants for the Viam
//     workcell ecosystem (pack-sequencer, pick-station, pallet), in
//     packsequencer.go / pickstation.go / pallet.go / colors.go. These
//     let a producer and its consumers share a single definition, so a
//     renamed JSON field becomes a compile error rather than a silent
//     zero value.
//
// Typical use of the generic helpers:
//
//	// A wire type — its JSON tags become the field names on the wire.
//	type StatusResponse struct {
//	    Phase   string `json:"phase"`
//	    Healthy bool   `json:"healthy"`
//	}
//
//	// Producer side: typed struct → wire map.
//	return contracts.ToMap(StatusResponse{Phase: "running", Healthy: true})
//
//	// Consumer side: wire map → typed struct.
//	respMap, err := svc.DoCommand(ctx, map[string]any{"get_status": true})
//	if err != nil {
//	    return err
//	}
//	resp, err := contracts.FromMap[StatusResponse](respMap)
package contracts

import "encoding/json"

// ToMap converts a struct to the map[string]interface{} shape Viam
// DoCommand carries on the wire. JSON struct tags drive the field
// names. Returns an error only on JSON marshal failure (e.g.
// unsupported field types).
func ToMap(v any) (map[string]interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MustToMap is ToMap that panics on error. Useful when the input is a
// well-known struct and a marshal failure is a programmer bug.
func MustToMap(v any) map[string]interface{} {
	m, err := ToMap(v)
	if err != nil {
		panic("contracts.MustToMap: " + err.Error())
	}
	return m
}

// FromMap parses a DoCommand wire map into a typed struct. The
// generic type parameter selects which struct shape to decode into.
func FromMap[T any](m map[string]interface{}) (T, error) {
	var zero T
	b, err := json.Marshal(m)
	if err != nil {
		return zero, err
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		return zero, err
	}
	return out, nil
}
