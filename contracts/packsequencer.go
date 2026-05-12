package contracts

import "github.com/viam-labs/viamkit/geom"

// Verb constants for pack-sequencer DoCommand dispatch. Both
// pack-sequencer (when dispatching incoming commands) and palletizer
// (when building outbound commands) import these so the wire keys can't
// drift.
const (
	VerbNextBox           = "next_box"
	VerbReportPlacement   = "report_placement"
	VerbGetBoxDims        = "get_box_dims"
	VerbGetPalletHome     = "get_pallet_home"
	VerbGetPackOrder      = "get_pack_order"
	VerbGetProgress       = "get_progress"
	VerbSetBoxTransform   = "set_box_transform"
	VerbClearBoxTransform = "clear_box_transform"
	VerbSkipBox           = "skip_box"
	VerbResetCursor       = "reset_cursor"
)

// BoxDimensions captures the width/length/height field shape used in
// next_box's `box_dimensions_mm` field. Note: the keys here are
// `width` / `length` / `height` (no _mm suffix) — that's the
// historical wire shape; rationalizing it would be a wire-format
// migration.
type BoxDimensions struct {
	Width  float64 `json:"width"`
	Length float64 `json:"length"`
	Height float64 `json:"height"`
}

// NextBoxResponse is the pack-sequencer's reply to `{"next_box": true}`.
// When IsComplete is true, all fields except the counters are zero.
type NextBoxResponse struct {
	Seq                    int           `json:"seq,omitempty"`
	Col                    int           `json:"col,omitempty"`
	Row                    int           `json:"row,omitempty"`
	Layer                  int           `json:"layer,omitempty"`
	PoseInPallet           geom.Pose6D   `json:"pose_in_pallet,omitempty"`
	ApproachOffsetInPallet geom.Vec3D    `json:"approach_offset_in_pallet,omitempty"`
	PlaceEndInWorld        geom.Pose6D   `json:"place_end_in_world,omitempty"`
	PlaceStartInWorld      geom.Pose6D   `json:"place_start_in_world,omitempty"`
	BoxDimensionsMM        BoxDimensions `json:"box_dimensions_mm,omitempty"`
	IsComplete             bool          `json:"is_complete"`
	Total                  int           `json:"total"`
	Placed                 int           `json:"placed"`
	Failed                 int           `json:"failed"`
	Skipped                int           `json:"skipped"`
	Remaining              int           `json:"remaining"`
}

// ReportPlacementArgs are the fields of the inner object passed under
// the `report_placement` key. The outer wire form is
// `{"report_placement": {"seq": N, "success": true, "error": "..."}}`.
type ReportPlacementArgs struct {
	Seq     int    `json:"seq"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ReportPlacementResponse is what the pack-sequencer returns after
// recording a cycle outcome.
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

// GetBoxDimsResponse is the reply to `{"get_box_dims": true}`. Note
// these use the `_mm` suffix; this is the canonical place to read box
// dimensions from (the palletizer pulls them at construction).
type GetBoxDimsResponse struct {
	BoxLengthMM float64 `json:"box_length_mm"`
	BoxWidthMM  float64 `json:"box_width_mm"`
	BoxHeightMM float64 `json:"box_height_mm"`
}
