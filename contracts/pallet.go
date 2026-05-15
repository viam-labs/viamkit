package contracts

import "github.com/viam-labs/viamkit/geom"

// Wire-format types for the viam:workcell-components:pallet generic
// component. The pallet's DoCommand surface is intentionally narrow
// — it's a pose-and-dims source, not a coordinator. Consumers that
// need pallet-relative computations (place_home, top-face center,
// corner poses, etc.) compute them locally from get_pose +
// get_dimensions.

const (
	VerbPalletGetPose       = "get_pose"
	VerbPalletGetDimensions = "get_dimensions"
	VerbPalletGetColor      = "get_color"
	VerbPalletGetAttributes = "get_attributes"
	VerbPalletSetDimensions = "set_dimensions"
	VerbPalletSetColor      = "set_color"
	VerbPalletSetAttributes = "set_attributes"
)

// GetColorResponse is the shape of `get_color` on both pallet and
// pick-station. Opacity is omitted when unset (renderer default).
type GetColorResponse struct {
	R       int     `json:"r"`
	G       int     `json:"g"`
	B       int     `json:"b"`
	Opacity float64 `json:"opacity,omitempty"`
}

// SetColorRequest is the body for the `set_color` verb on both
// pallet and pick-station.
type SetColorRequest struct {
	R       int     `json:"r"`
	G       int     `json:"g"`
	B       int     `json:"b"`
	Opacity float64 `json:"opacity,omitempty"`
}

// GetPalletAttributesResponse is the response shape of
// `get_attributes` on the pallet. Pack-sequencer reads this once at
// construction and per-pack-order-call (no caching) to track live
// dim updates.
type GetPalletAttributesResponse struct {
	Label       string           `json:"label,omitempty"`
	WidthMM     float64          `json:"width_mm"`
	LengthMM    float64          `json:"length_mm"`
	ThicknessMM float64          `json:"thickness_mm"`
	Color       GetColorResponse `json:"color"`
	Pose        geom.Pose6D      `json:"pose"`
}
