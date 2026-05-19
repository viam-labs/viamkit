package viz

import (
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"

	commonpb "go.viam.com/api/common/v1"
)

// AttachToGripper builds a Box transform representing a held box
// attached to a gripper frame — for the 3D scene viewer's
// visualization of "the box is in the gripper during transit."
//
// The transform's ObserverFrame is the gripper's name, and the Pose
// is the box CENTER in gripper-local frame: (0, 0, +H/2). The
// gripper's frame +Z points away from the wrist (toward the box
// when arm is wrist-down), so +H/2 along that axis lands the box
// hanging below the wrist when the gripper points down — exactly
// what the operator sees.
//
// Pair with worldstate.GripperHeldBox for the planner's
// collision-body equivalent. Same convention, different wire
// format.
//
// Parameters:
//
//   - uuid: WorldStateStore identifier. Caller decides the rotation
//     strategy: a stable UUID like "held-box" works for "one box in
//     gripper at a time" patterns; a per-cycle UUID like
//     "held-box-N" works when callers want to publish via
//     ADDED/REMOVED (e.g. to dodge the renderer's metadata-update
//     drop on color changes — see the renderer-update-path-matcher
//     memory note).
//   - gripperName: the gripper frame the box is parented to.
//   - boxDimsMM: the box's width / length / height in mm.
//   - color: optional rendering color. Zero Color = renderer default.
func AttachToGripper(
	uuid string,
	gripperName string,
	boxDimsMM r3.Vector,
	color Color,
) *commonpb.Transform {
	return Box{
		UUID:          uuid,
		ObserverFrame: gripperName,
		Pose: spatialmath.NewPoseFromPoint(
			r3.Vector{X: 0, Y: 0, Z: boxDimsMM.Z / 2},
		),
		DimsMM: boxDimsMM,
		Color:  color,
		Label:  "held-box",
	}.ToTransform()
}
