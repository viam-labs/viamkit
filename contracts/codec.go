// Package contracts holds codec helpers for Viam DoCommand wire
// format. DoCommand at the protocol level is just
// `map[string]interface{}` over gRPC; defining typed Go structs that
// describe the wire shape lets producers and consumers refer to the
// contract by name instead of probing map keys, and lets the compiler
// catch typos before they reach a running module.
//
// This package is intentionally generic — it has no module-specific
// types or verb constants. Each consuming module defines its own
// request / response structs and dispatches them through ToMap /
// FromMap[T].
//
// Typical use:
//
//	// Producer side: build a typed response, marshal to wire shape.
//	resp, err := contracts.ToMap(MyResponse{X: 1, Y: 2})
//	return resp, err
//
//	// Consumer side: receive a wire map, parse to typed struct.
//	respMap, err := svc.DoCommand(ctx, map[string]any{"my_verb": true})
//	if err != nil { return err }
//	got, err := contracts.FromMap[MyResponse](respMap)
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
