package worldstate

import (
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// NewBoxObstacle creates a spatialmath.Box geometry suitable for
// planner-obstacle use. dimsMM is width × length × height in mm; pose
// places the box CENTER (not corner) in whichever frame the geometry
// will be attached to.
//
// The returned geometry has Label = label so collision results name
// it that way.
func NewBoxObstacle(pose spatialmath.Pose, dimsMM r3.Vector, label string) (spatialmath.Geometry, error) {
	return spatialmath.NewBox(pose, dimsMM, label)
}

// NewSphereObstacle creates a spatialmath.Sphere geometry. radiusMM
// is the sphere radius in mm; pose places the center.
func NewSphereObstacle(pose spatialmath.Pose, radiusMM float64, label string) (spatialmath.Geometry, error) {
	return spatialmath.NewSphere(pose, radiusMM, label)
}

// HeldObject constructs a LinkInFrame representing an object
// attached to a moving parent frame — typically a gripper. The
// motion planner uses this to know the object moves with the parent.
//
// Parameters:
//
//   - parentFrame: the frame the object is attached to (the gripper
//     name, the wrist, etc.). The planner composes its kinematics
//     so the object moves correctly.
//   - offset: the geometry's pose relative to the parent frame's
//     origin. For a vacuum-grasped box centered on the gripper tip,
//     this is typically (0, 0, box_height/2).
//   - linkName: a unique name for the link (appears in collision
//     reports). Has no functional meaning beyond identification.
//   - geometry: the object's shape, with its OWN pose set to zero
//     (the LinkInFrame's offset is what positions it).
func HeldObject(
	parentFrame string,
	offset spatialmath.Pose,
	linkName string,
	geometry spatialmath.Geometry,
) *referenceframe.LinkInFrame {
	return referenceframe.NewLinkInFrame(parentFrame, offset, linkName, geometry)
}

// Combined merges a list of static obstacle frames and a list of
// dynamic (attached) links into a single WorldState. Pass nil for
// either to omit that category.
//
// Returns an error if both are empty (an empty WorldState is usually
// a programming bug — the motion service treats it as "no
// constraints" which is rarely what you want).
func Combined(
	obstacles []*referenceframe.GeometriesInFrame,
	links []*referenceframe.LinkInFrame,
) (*referenceframe.WorldState, error) {
	return referenceframe.NewWorldState(obstacles, links)
}

// WorldObstacles is a convenience builder that wraps a list of
// geometries into a "world"-framed GeometriesInFrame and returns a
// slice suitable for Combined's first argument. Use when all your
// static obstacles share the "world" parent frame, which is the
// common case.
func WorldObstacles(geoms ...spatialmath.Geometry) []*referenceframe.GeometriesInFrame {
	if len(geoms) == 0 {
		return nil
	}
	return []*referenceframe.GeometriesInFrame{
		referenceframe.NewGeometriesInFrame(referenceframe.World, geoms),
	}
}

// DefaultHeldBoxZPadMM is the safety pad applied to each end of a
// held box's Z dimension to avoid clipping the gripper geometry at
// grasp and the pallet wood at place_end. The held-box collision
// body shrinks INSIDE the real box's volume by this much on each
// face. 2.5 mm is the value the production palletizer uses.
const DefaultHeldBoxZPadMM = 2.5

// GripperHeldBox returns a *referenceframe.LinkInFrame for a box
// attached to a gripper frame — the held-box collision body the
// motion planner needs while a box is in transit between grasp and
// release. Bakes the offset and orientation so consumers don't
// re-derive it (and re-derive it wrong, which dryruns of the
// palletizer arc have caught repeatedly).
//
// Convention: the gripper's frame +Z points away from the wrist
// (toward the held box when the arm is wrist-down). The box's
// CENTER sits at +H/2 along the gripper's local Z; with a safety
// pad on each Z face, the centroid offset is +H/2 still but the
// Z dimension shrinks by 2*pad. The pad lets the box geometry stay
// disjoint from the gripper's own collision body at grasp and from
// the pallet wood at place_end.
//
// Parameters:
//
//   - gripperName: the gripper component's name (e.g. "gripper"). The
//     LinkInFrame's parent is set to this so the planner moves the
//     box with the gripper.
//   - linkName: a unique name for the link (appears in collision
//     reports). Has no functional meaning beyond identification.
//   - boxDimsMM: the real box's width / length / height in mm.
//
// Use DefaultHeldBoxZPadMM as the pad in most cases. For custom
// pads, call HeldObject + NewBoxObstacle directly.
func GripperHeldBox(
	gripperName string,
	linkName string,
	boxDimsMM r3.Vector,
) (*referenceframe.LinkInFrame, error) {
	pad := DefaultHeldBoxZPadMM
	if boxDimsMM.Z <= 2*pad {
		// Don't shrink below zero — caller's box is shorter than the
		// pad budget. Skip the pad in that case (rare for real
		// palletizing boxes).
		pad = 0
	}
	dims := r3.Vector{X: boxDimsMM.X, Y: boxDimsMM.Y, Z: boxDimsMM.Z - 2*pad}
	geom, err := NewBoxObstacle(spatialmath.NewZeroPose(), dims, linkName)
	if err != nil {
		return nil, err
	}
	offset := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: boxDimsMM.Z / 2})
	return HeldObject(gripperName, offset, linkName, geom), nil
}
