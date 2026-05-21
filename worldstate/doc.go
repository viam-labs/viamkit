// Package worldstate holds helpers for composing
// `referenceframe.WorldState` values — the structure the Viam motion
// service consumes when planning collision-free trajectories.
//
// Two distinct cases come up often in motion-heavy modules:
//
//   - Static obstacles in the world frame. Examples: placed objects,
//     fixed fixtures, walls. Represented as GeometriesInFrame
//     attached to "world".
//
//   - Dynamic obstacles attached to a moving frame. Example: a box
//     being held by a gripper. Represented as a LinkInFrame whose
//     parent is the gripper, so as the arm moves, the planner knows
//     the held object moves with it.
//
// This package gives both shapes simple constructors plus a Combined
// helper that merges them into a single WorldState. The held-object
// pattern in particular is one of the easier things to get wrong —
// the geometry's offset is relative to the gripper frame's origin
// (the vacuum tip, the finger center, etc.), not the world.
//
// Minimal usage:
//
//	heldGeom, _ := spatialmath.NewBox(spatialmath.NewZeroPose(),
//	    r3.Vector{X: 100, Y: 200, Z: 80}, "held-box")
//	heldLink := worldstate.HeldObject(
//	    "gripper",        // frame the object is attached to
//	    spatialmath.NewPoseFromPoint(r3.Vector{Z: 40}), // offset from that frame's origin
//	    "held-box-frame", // name for the attached link
//	    heldGeom,         // the geometry being carried
//	)
//
//	placedBox, _ := worldstate.NewBoxObstacle(
//	    placedPose, r3.Vector{X: 100, Y: 200, Z: 80}, "placed-box-1",
//	)
//
//	ws, err := worldstate.Combined(
//	    []*referenceframe.GeometriesInFrame{
//	        referenceframe.NewGeometriesInFrame("world", []spatialmath.Geometry{placedBox}),
//	    },
//	    []*referenceframe.LinkInFrame{heldLink},
//	)
package worldstate
