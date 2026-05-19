package contracts

import "github.com/viam-labs/viamkit/geom"

// Wire-format types for the viam:pack-sequencer service. Producers
// (the pack-sequencer module) and consumers (a palletizer module)
// import these so the JSON tags stay locked — a typo'd field on
// either end becomes a compile error rather than a silent zero.
//
// Versioning: any change here is a wire-format break for either
// producer or consumer. Bump viamkit's tag and require both modules
// to update in lock-step. Adding new fields with `,omitempty` is
// safe; renaming or removing existing fields isn't.

// Pack-sequencer DoCommand verb names. Use as map keys when
// constructing a request: `{contracts.VerbNextBox: true, ...}`.
const (
	VerbNextBox            = "next_box"
	VerbReportPlacement    = "report_placement"
	VerbGetBoxDims         = "get_box_dims"
	VerbGetPalletHome      = "get_pallet_home"
	VerbGetPackOrder       = "get_pack_order"
	VerbGetProgress        = "get_progress"
	VerbResetCursor        = "reset_cursor"
	VerbSkipBox            = "skip_box"
	VerbSetBoxTransform    = "set_box_transform"
	VerbClearBoxTransform  = "clear_box_transform"
	VerbGetAttributes      = "get_attributes"
	VerbSetAttributes      = "set_attributes"
)

// NextBoxResponse is the response shape of `next_box`. Note the
// `_in_world` suffix on the pose fields — pre-composed world-frame
// poses so the consumer doesn't need its own pallet_origin compose
// step. The dryrun's silent-zero failure mode came from a
// hand-written struct using `place_start` / `place_end` (without
// `_in_world`); this struct's tags match the wire and prevent that
// regression.
type NextBoxResponse struct {
	Seq                    int             `json:"seq"`
	Col                    int             `json:"col"`
	Row                    int             `json:"row"`
	Layer                  int             `json:"layer"`
	PoseInPallet           geom.Pose6D     `json:"pose_in_pallet"`
	ApproachOffsetInPallet ApproachOffset  `json:"approach_offset_in_pallet"`
	PlaceEndInWorld        geom.Pose6D     `json:"place_end_in_world"`
	PlaceStartInWorld      geom.Pose6D     `json:"place_start_in_world"`
	BoxDimensionsMM        BoxDimensions   `json:"box_dimensions_mm"`
	IsComplete             bool            `json:"is_complete"`
	Total                  int             `json:"total"`
	Placed                 int             `json:"placed"`
	Failed                 int             `json:"failed"`
	Skipped                int             `json:"skipped"`
	Remaining              int             `json:"remaining"`
}

// ApproachOffset is the per-box angled-approach delta in pallet-local
// frame. Applied to the place_end slot to produce place_start.
type ApproachOffset struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// BoxDimensions is the per-placement footprint as `next_box` returns
// it. Note `length` here means the box's pack-order Y dimension
// (which may be the box's "long" or "short" side depending on
// rotation in the pack), not the box's physical length.
type BoxDimensions struct {
	Width  float64 `json:"width"`
	Length float64 `json:"length"`
	Height float64 `json:"height"`
}

// ReportPlacementRequest is the body of a `report_placement` call.
// Success advances the cursor; failure leaves it for retry.
type ReportPlacementRequest struct {
	Seq     int    `json:"seq"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ReportPlacementResponse is the cursor + counter snapshot returned
// after a placement report.
type ReportPlacementResponse struct {
	Acknowledged bool   `json:"acknowledged"`
	NextSeq      int    `json:"next_seq"`
	Placed       int    `json:"placed"`
	Failed       int    `json:"failed"`
	Skipped      int    `json:"skipped"`
	Remaining    int    `json:"remaining"`
	Complete     bool   `json:"complete"`
	LastError    string `json:"last_error,omitempty"`
}

// GetBoxDimsResponse is the response shape of `get_box_dims`. Note
// the `box_*` prefix — the dryrun's silent-zero came from tags using
// bare `width_mm` / `length_mm` / `height_mm`, which silently
// unmarshalled to all zeros.
type GetBoxDimsResponse struct {
	BoxLengthMM float64 `json:"box_length_mm"`
	BoxWidthMM  float64 `json:"box_width_mm"`
	BoxHeightMM float64 `json:"box_height_mm"`
}

// GetPalletHomeResponse is the response shape of `get_pallet_home`.
// Local is in pallet-local frame; world is the same composed with
// the pallet's world pose.
type GetPalletHomeResponse struct {
	PalletHomeLocal geom.Pose6D `json:"pallet_home_local"`
	PalletHomeWorld geom.Pose6D `json:"pallet_home_world"`
}

// SetBoxTransformRequest is the body of `set_box_transform`. As of
// pack-sequencer 0.3.0, Pose is honored (under either the nested
// `pose:` key or flat at the args level — this struct produces the
// nested form, which is the recommended shape). Color is an optional
// per-call override that beats the service's `box_color` Config attr
// for this one transform; a zero-value Color means "use the service
// default."
type SetBoxTransformRequest struct {
	Seq    int         `json:"seq"`
	Parent string      `json:"parent,omitempty"`
	Pose   geom.Pose6D `json:"pose,omitempty"`
	Color  *Color      `json:"color,omitempty"`
}

// SetBoxTransformResponse is the ack returned after set_box_transform.
type SetBoxTransformResponse struct {
	Acknowledged bool   `json:"acknowledged"`
	Seq          int    `json:"seq"`
	UUID         string `json:"uuid"`
	Parent       string `json:"parent,omitempty"`
}

// ClearBoxTransformRequest is the body of `clear_box_transform`.
type ClearBoxTransformRequest struct {
	Seq int `json:"seq"`
}

// SkipBoxRequest is the body of `skip_box`. Reason is recorded on
// the cursor's err map for later inspection.
type SkipBoxRequest struct {
	Seq    int    `json:"seq"`
	Reason string `json:"reason,omitempty"`
}

// GetProgressResponse is the cursor's full state, suitable for an
// operator UI's status panel.
type GetProgressResponse struct {
	NextSeq      int   `json:"next_seq"`
	DoneSeqs     []int `json:"done_seqs"`
	SkippedSeqs  []int `json:"skipped_seqs"`
	FailedSeqs   []int `json:"failed_seqs"`
	PlacedCount  int   `json:"placed_count"`
	FailedCount  int   `json:"failed_count"`
	SkippedCount int   `json:"skipped_count"`
	Total        int   `json:"total"`
	Complete     bool  `json:"complete"`
}

// PackOrderPlacement is one entry in the GetPackOrderResponse's
// `placements` list — a single per-box pose + dims + metadata. Note
// the field-name pattern: dimensions use BARE `width_mm` /
// `length_mm` / `height_mm` (NOT the `box_*_mm` prefix that
// get_box_dims uses); poses appear in both pallet-local and
// world frames. Dryrun-4 caught a hand-written struct using the
// wrong field-name set and silent-zeroing every field as a result.
type PackOrderPlacement struct {
	Seq                    int            `json:"seq"`
	Col                    int            `json:"col"`
	Row                    int            `json:"row"`
	Layer                  int            `json:"layer"`
	XMM                    float64        `json:"x_mm"`
	YMM                    float64        `json:"y_mm"`
	ZMM                    float64        `json:"z_mm"`
	Label                  string         `json:"label,omitempty"`
	LengthMM               float64        `json:"length_mm"`
	WidthMM                float64        `json:"width_mm"`
	HeightMM               float64        `json:"height_mm"`
	PoseInPallet           geom.Pose6D    `json:"pose_in_pallet"`
	PoseInWorld            geom.Pose6D    `json:"pose_in_world"`
	ApproachOffsetInPallet ApproachOffset `json:"approach_offset_in_pallet"`
}

// GetPackOrderResponse is the full pack-sequencer pack-order dump.
// Used by `verify_pallet` to iterate every placement without
// stepping the cursor, by webapp 3D-preview to render the planned
// pallet, and by any consumer that needs the geometry of every
// placement up-front.
//
// The dryrun-4 silent-zero risk this struct prevents: a hand-rolled
// `type packOrderResponse struct { PackOrder [...] }` reads zero
// placements because the actual wire key is `placements`, and per-
// placement field-name drift between this and `get_box_dims` is
// easy to miss without the typed struct.
type GetPackOrderResponse struct {
	Placements        []PackOrderPlacement `json:"placements"`
	Cols              int                  `json:"cols"`
	Rows              int                  `json:"rows"`
	Layers            int                  `json:"layers"`
	Capacity          int                  `json:"capacity"`
	Quantity          int                  `json:"quantity"`
	Overflow          int                  `json:"overflow"`
	Mode              string               `json:"mode"`
	Warnings          []string             `json:"warnings,omitempty"`
	PalletThicknessMM float64              `json:"pallet_thickness_mm"`
	PalletWidthMM     float64              `json:"pallet_width_mm"`
	PalletLengthMM    float64              `json:"pallet_length_mm"`
	PalletPose        geom.Pose6D          `json:"pallet_pose"`
}
