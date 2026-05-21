package contracts

import "github.com/viam-labs/viamkit/geom"

// Wire-format types for the viam:workcell-components:pallet generic
// component. The pallet's DoCommand surface is intentionally narrow
// — it's a pose-and-dims source, not a coordinator. Consumers that
// need pallet-relative computations (place_home, top-face center,
// corner poses, etc.) compute them locally from get_pose +
// get_dimensions.

// Pallet DoCommand verb names. Use as map keys when constructing a
// request: `{contracts.VerbPalletGetPose: true}`.
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
// pick-station. Aliased to the shared Color type so producers and
// consumers can't drift apart on field names.
type GetColorResponse = Color

// SetColorRequest is the body for the `set_color` verb on both
// pallet and pick-station. Aliased to Color for the same reason.
type SetColorRequest = Color

// GetPalletAttributesResponse is the response shape of
// `get_attributes` on the pallet. Pack-sequencer reads this once at
// construction and per-pack-order-call (no caching) to track live
// dim updates. As of workcell-components 0.4.0 the response also
// carries the forward-looking visual options (show_axes / visible /
// opacity) and a one-line human summary.
type GetPalletAttributesResponse struct {
	Label       string      `json:"label,omitempty"`
	WidthMM     float64     `json:"width_mm"`
	LengthMM    float64     `json:"length_mm"`
	ThicknessMM float64     `json:"thickness_mm"`
	Color       Color       `json:"color"`
	Pose        geom.Pose6D `json:"pose"`
	Summary     string      `json:"summary,omitempty"`

	// Forward-looking attrs declared on workcell-components 0.4.0.
	// The future viz layer reads them; today they're informational.
	ShowAxes bool    `json:"show_axes,omitempty"`
	Visible  bool    `json:"visible,omitempty"`
	Opacity  float64 `json:"opacity,omitempty"`
}

// New verbs added in workcell-components 0.4.0 — see the module's
// pallet.go doc for full semantics.

// GetPalletHomePoseRequest carries the optional safety height in mm
// for the `get_pallet_home_pose` verb. Defaults to 200 when omitted.
// The verb also accepts a bare `true` (use default) or a flat number
// (interpret as the height directly); the typed struct produces the
// nested-args form.
type GetPalletHomePoseRequest struct {
	SafetyHeightMM float64 `json:"safety_height_mm,omitempty"`
}

// GetCornerPosesResponse is the response shape of `get_corner_poses`
// — four world-frame top-face corner poses, CCW from bottom-left.
type GetCornerPosesResponse struct {
	Corners []geom.Pose6D `json:"corners"`
}

// GetSummaryResponse wraps the one-line human description string.
// Shared between pallet and pick-station (`get_summary` on either).
type GetSummaryResponse struct {
	Summary string `json:"summary"`
}

// GetStatusResponse is the runtime health snapshot. Shared between
// pallet and pick-station.
type GetStatusResponse struct {
	OK         bool   `json:"ok"`
	Model      string `json:"model"`
	Name       string `json:"name"`
	DimsValid  bool   `json:"dims_valid"`
	ColorValid bool   `json:"color_valid"`
	Visible    bool   `json:"visible"`
	ShowAxes   bool   `json:"show_axes"`
}

// SetPalletAttributesRequest is the body of `set_attributes` on the
// pallet. Every field is optional — partial updates are supported,
// omitted fields are no-ops. The response carries a persistence
// hint (see SetPersistResponse).
type SetPalletAttributesRequest struct {
	Label       string   `json:"label,omitempty"`
	WidthMM     float64  `json:"width_mm,omitempty"`
	LengthMM    float64  `json:"length_mm,omitempty"`
	ThicknessMM float64  `json:"thickness_mm,omitempty"`
	Color       *Color   `json:"color,omitempty"`
	ShowAxes    *bool    `json:"show_axes,omitempty"`
	Visible     *bool    `json:"visible,omitempty"`
	Opacity     *float64 `json:"opacity,omitempty"`
}

// SetPersistResponse is the boilerplate annotation embedded in every
// set_* response on workcell-components 0.4.0+. It signals that the
// edit is live-until-reconfigure — the cell config wins on next
// reload. Consumers wanting durable edits must push through
// app.AppClient.UpdateRobotPart.
type SetPersistResponse struct {
	Persisted bool   `json:"persisted"`
	Hint      string `json:"hint"`
}
