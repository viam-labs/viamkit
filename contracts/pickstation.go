package contracts

// Verb constants for pick-station DoCommand dispatch.
const (
	VerbGetPickHomePose = "get_pick_home_pose"
	VerbGetVacuumPose   = "get_vacuum_pose"
	VerbGetPickupPose   = "get_pickup_pose"
)

// PoseRequest is the shape of pick-station verbs that take a box
// height. The verb selector lives alongside `box_height_mm`; build it
// with the appropriate Verb* constant as the JSON key set to true.
//
// Wire form:
//
//	{"get_vacuum_pose": true, "box_height_mm": 80}
//
// Because the selector key is verb-specific, callers typically build
// the request map directly rather than via this struct — but the field
// list is documented here for reference.
type PoseRequest struct {
	BoxHeightMM float64 `json:"box_height_mm"`
}

// Pose responses on pick-station are returned in the Pose6D wire shape
// (`{x, y, z, o_x, o_y, o_z, theta}`) — use wcsh.Pose6D directly to
// parse them. No dedicated response struct is needed because the entire
// response *is* a Pose6D map.
