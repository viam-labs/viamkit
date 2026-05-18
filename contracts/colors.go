package contracts

// Color is the shared RGB+opacity color shape used across the
// workcell ecosystem's wire contracts (set_color requests on pallet
// + pick-station, per-call color overrides on pack-sequencer's
// set_box_transform, embedded in attribute-response shapes). RGB is
// 0..255, opacity is 0..1. Opacity == 0 means "unspecified" — the
// consumer's default applies (typically opaque).
//
// Use a pointer when "unset" must be distinguishable from "set to
// (0, 0, 0)" — e.g. on a Request where omitting the field means
// "keep current."
type Color struct {
	R       int     `json:"r"`
	G       int     `json:"g"`
	B       int     `json:"b"`
	Opacity float64 `json:"opacity,omitempty"`
}
