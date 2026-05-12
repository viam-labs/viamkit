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
