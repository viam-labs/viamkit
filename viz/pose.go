package viz

import (
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/spatialmath"
)

// PoseToProto converts a spatialmath.Pose to the *commonpb.Pose wire
// shape used by WorldStateStore Transforms.
func PoseToProto(p spatialmath.Pose) *commonpb.Pose {
	pt := p.Point()
	ov := p.Orientation().OrientationVectorDegrees()
	return &commonpb.Pose{
		X:     pt.X,
		Y:     pt.Y,
		Z:     pt.Z,
		OX:    ov.OX,
		OY:    ov.OY,
		OZ:    ov.OZ,
		Theta: ov.Theta,
	}
}

// PoseFromProto is the inverse of PoseToProto.
func PoseFromProto(p *commonpb.Pose) spatialmath.Pose {
	if p == nil {
		return spatialmath.NewZeroPose()
	}
	return spatialmath.NewPose(
		r3.Vector{X: p.X, Y: p.Y, Z: p.Z},
		&spatialmath.OrientationVectorDegrees{OX: p.OX, OY: p.OY, OZ: p.OZ, Theta: p.Theta},
	)
}

// vecToProto converts an r3.Vector (in mm) to *commonpb.Vector3.
func vecToProto(v r3.Vector) *commonpb.Vector3 {
	return &commonpb.Vector3{X: v.X, Y: v.Y, Z: v.Z}
}
