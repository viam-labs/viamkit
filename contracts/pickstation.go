package contracts

import "github.com/viam-labs/viamkit/geom"

// Wire-format types for the viam:workcell-components:pick-station
// generic component. Pose responses are shaped as flat Pose6D
// (x, y, z, o_x, o_y, o_z, theta).
//
// The arg-bearing verbs accept the nested calling convention as of
// workcell-components 0.3.0 — earlier versions only honored args
// at the top of the cmd map, which silently zeroed box_height_mm
// when the consumer used the more idiomatic nested form.

const (
	VerbPickStationGetPose         = "get_pose"
	VerbPickStationGetDimensions   = "get_dimensions"
	VerbPickStationGetColor        = "get_color"
	VerbPickStationGetPickupPose   = "get_pickup_pose"
	VerbPickStationGetPickHomePose = "get_pick_home_pose"
	VerbPickStationGetVacuumPose   = "get_vacuum_pose"
	VerbPickStationGetAttributes   = "get_attributes"
	VerbPickStationSetDimensions   = "set_dimensions"
	VerbPickStationSetColor        = "set_color"
	VerbPickStationSetAttributes   = "set_attributes"
)

// GetVacuumPoseRequest is the body for the `get_vacuum_pose` verb.
// Pack as `{"get_vacuum_pose": {"box_height_mm": h}}`.
type GetVacuumPoseRequest struct {
	BoxHeightMM float64 `json:"box_height_mm"`
}

// GetPickHomePoseRequest is the body for the `get_pick_home_pose`
// verb. Pack as `{"get_pick_home_pose": {"box_height_mm": h}}`.
type GetPickHomePoseRequest struct {
	BoxHeightMM float64 `json:"box_height_mm"`
}

// GetDimensionsResponse is the shape of `get_dimensions` /
// `set_dimensions` responses on both pallet and pick-station.
type GetDimensionsResponse struct {
	WidthMM     float64 `json:"width_mm"`
	LengthMM    float64 `json:"length_mm"`
	ThicknessMM float64 `json:"thickness_mm"`
}

// SetDimensionsRequest is the body for the `set_dimensions` verb on
// both pallet and pick-station. Any zero-valued field is left
// unchanged.
type SetDimensionsRequest struct {
	WidthMM     float64 `json:"width_mm,omitempty"`
	LengthMM    float64 `json:"length_mm,omitempty"`
	ThicknessMM float64 `json:"thickness_mm,omitempty"`
}

// PoseResponse is the canonical pose shape returned by every pose-
// returning verb on pallet + pick-station (`get_pose`,
// `get_pickup_pose`, `get_pick_home_pose`, `get_vacuum_pose`).
type PoseResponse = geom.Pose6D
