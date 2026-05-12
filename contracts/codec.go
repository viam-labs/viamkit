// Package contracts holds typed Go structs that describe the
// DoCommand wire format between workcell modules — palletizer ↔
// pack-sequencer ↔ pick-station.
//
// DoCommand at the protocol layer is just `map[string]interface{}` over
// gRPC, so the structs are a compile-time aid: producers `ToMap(struct)`
// to build a return value, consumers `FromMap[T](respMap)` to parse one.
// JSON struct tags drive the wire field names.
//
// Each verb has a string constant for the dispatch key, a Request type
// (omit for verbs that take no args beyond the selector), and a Response
// type. Adding a new verb means: add the constant, add the structs,
// import them on both sides. The compiler then catches every typo.
package contracts

import "encoding/json"

// ToMap converts a struct to the map[string]interface{} shape Viam
// DoCommand carries on the wire. Returns an error only on JSON marshal
// failure (e.g. unsupported field types).
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

// MustToMap is ToMap that panics on error. Useful for cases where the
// input is a well-known struct and a marshal failure is a programmer bug.
func MustToMap(v any) map[string]interface{} {
	m, err := ToMap(v)
	if err != nil {
		panic("contracts.MustToMap: " + err.Error())
	}
	return m
}

// FromMap parses a DoCommand wire map into a typed struct. The generic
// type parameter selects which struct shape to decode into.
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
