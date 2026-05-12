// Package geom holds JSON-serializable pose and vector types shared by
// Viam modules built with viamkit.
//
// The Viam SDK already defines the canonical runtime math types —
// spatialmath.Pose, r3.Vector, spatialmath.OrientationVectorDegrees.
// Those types are interfaces or use field naming that doesn't round-trip
// cleanly through JSON Config attributes, so each module would otherwise
// define its own flat struct. This package is the single source of
// truth for those flat shapes, plus converters to/from the SDK types.
package geom

import (
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"
)

// Pose6D is a 6-DOF pose in mm + degrees. Orientation follows the Viam
// OrientationVectorDegrees convention: (OX,OY,OZ) is the unit vector the
// frame's local Z-axis points along; Theta is rotation around that axis.
type Pose6D struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
	OX    float64 `json:"o_x"`
	OY    float64 `json:"o_y"`
	OZ    float64 `json:"o_z"`
	Theta float64 `json:"theta"`
}

// ToPose converts a Pose6D into a spatialmath.Pose for use with the SDK.
func (p Pose6D) ToPose() spatialmath.Pose {
	return spatialmath.NewPose(
		r3.Vector{X: p.X, Y: p.Y, Z: p.Z},
		&spatialmath.OrientationVectorDegrees{OX: p.OX, OY: p.OY, OZ: p.OZ, Theta: p.Theta},
	)
}

// ToMap renders a Pose6D as the canonical JSON map shape used in
// DoCommand responses across the workcell modules.
func (p Pose6D) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"x":     p.X,
		"y":     p.Y,
		"z":     p.Z,
		"o_x":   p.OX,
		"o_y":   p.OY,
		"o_z":   p.OZ,
		"theta": p.Theta,
	}
}

// PoseFrom converts an SDK spatialmath.Pose into a flat Pose6D suitable
// for JSON marshalling or DoCommand returns.
func PoseFrom(pose spatialmath.Pose) Pose6D {
	pt := pose.Point()
	ov := pose.Orientation().OrientationVectorDegrees()
	return Pose6D{
		X:     pt.X,
		Y:     pt.Y,
		Z:     pt.Z,
		OX:    ov.OX,
		OY:    ov.OY,
		OZ:    ov.OZ,
		Theta: ov.Theta,
	}
}

// Vec3D is a 3D point or direction vector in mm.
type Vec3D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Normalized returns the vector scaled to unit length. The zero vector
// is mapped to (0, 0, 1) so callers don't have to handle a NaN path.
func (v Vec3D) Normalized() r3.Vector {
	vec := r3.Vector{X: v.X, Y: v.Y, Z: v.Z}
	n := vec.Norm()
	if n == 0 {
		return r3.Vector{X: 0, Y: 0, Z: 1}
	}
	return vec.Mul(1.0 / n)
}
