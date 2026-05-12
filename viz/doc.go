// Package viz holds helpers for building Viam WorldStateStore
// Transform messages — the protobuf objects that populate the live 3D
// scene viewer.
//
// A WorldStateStore producer (typically a `rdk:service:world_state_store`
// service) maintains a set of Transform objects identified by UUID,
// each describing a geometric primitive in a parent frame. The viewer
// subscribes to changes via StreamTransformChanges and renders them.
//
// Each primitive type in this package — Box, Sphere, Capsule, Point —
// is a plain struct with the fields you'd want to set, and a
// ToTransform() method that produces the *commonpb.Transform. Common
// defaults (observer frame = "world", label = UUID) fill in via zero
// values so the call site reads as just the relevant data:
//
//	tf := viz.Box{
//	    UUID:   "placed-box-3",
//	    Pose:   spatialmath.NewPose(r3.Vector{X: 100, Y: 200, Z: 0}, nil),
//	    DimsMM: r3.Vector{X: 100, Y: 200, Z: 80},
//	}.ToTransform()
//
// Use with go.viam.com/rdk/services/worldstatestore's TransformChange
// to emit additions / updates / removals over the change stream.
//
// This package doesn't depend on viamkit/geom or any other workcell
// types — anyone producing WorldStateStore data can use it.
package viz
