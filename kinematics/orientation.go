package kinematics

import (
	"math"

	"go.viam.com/rdk/spatialmath"
)

// YawFromOrientation returns the gripper's world-frame yaw (radians)
// for an orientation expressed as an OrientationVectorDegrees.
//
// Uses Theta from the OrientationVectorDegrees representation, which
// is the wrist rotation around the gripper's Z axis. For a
// straight-down gripper (OZ ≈ -1) that's equivalent — modulo sign —
// to a rotation around world -Z, so we flip the sign when OZ < 0.
//
// Assumes a straight-down or straight-up gripper convention. Off-axis
// orientations get the same algebraic result but the geometric
// interpretation (which axis is "yaw") is no longer meaningful.
func YawFromOrientation(o spatialmath.Orientation) float64 {
	ov := o.OrientationVectorDegrees()
	sign := 1.0
	if ov.OZ < 0 {
		sign = -1.0
	}
	return sign * ov.Theta * math.Pi / 180
}
