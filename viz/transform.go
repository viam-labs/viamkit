package viz

import (
	commonpb "go.viam.com/api/common/v1"
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"
)

// DefaultObserverFrame is used when a primitive doesn't set its own
// ObserverFrame. Matches the WorldStateStore convention.
const DefaultObserverFrame = "world"

// Box is a rectangular-prism Transform. Width × length × height
// dimensions are in millimeters; Pose places the box center in the
// ObserverFrame.
type Box struct {
	// UUID is the WorldStateStore identifier. Required.
	UUID string

	// ReferenceFrame is the Transform's own frame name. Defaults to
	// UUID — i.e. each Transform names a frame matching its UUID.
	ReferenceFrame string

	// ObserverFrame is the parent frame the Pose is expressed in.
	// Defaults to "world".
	ObserverFrame string

	// Pose is the box center in ObserverFrame.
	Pose spatialmath.Pose

	// DimsMM is width / length / height in millimeters.
	DimsMM r3.Vector

	// Label is the geometry's display label. Defaults to UUID.
	Label string
}

// ToTransform produces the *commonpb.Transform.
func (b Box) ToTransform() *commonpb.Transform {
	return &commonpb.Transform{
		Uuid:           []byte(b.UUID),
		ReferenceFrame: refOr(b.ReferenceFrame, b.UUID),
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: refOr(b.ObserverFrame, DefaultObserverFrame),
			Pose:           PoseToProto(poseOrZero(b.Pose)),
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{DimsMm: vecToProto(b.DimsMM)},
			},
			Label: labelOr(b.Label, b.UUID),
		},
	}
}

// Sphere is a sphere Transform. RadiusMM is the sphere radius in
// millimeters; Pose places the sphere center in ObserverFrame.
type Sphere struct {
	UUID           string
	ReferenceFrame string
	ObserverFrame  string
	Pose           spatialmath.Pose
	RadiusMM       float64
	Label          string
}

func (s Sphere) ToTransform() *commonpb.Transform {
	return &commonpb.Transform{
		Uuid:           []byte(s.UUID),
		ReferenceFrame: refOr(s.ReferenceFrame, s.UUID),
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: refOr(s.ObserverFrame, DefaultObserverFrame),
			Pose:           PoseToProto(poseOrZero(s.Pose)),
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Sphere{
				Sphere: &commonpb.Sphere{RadiusMm: s.RadiusMM},
			},
			Label: labelOr(s.Label, s.UUID),
		},
	}
}

// Capsule is a capsule (a cylinder with hemispherical caps) Transform.
// RadiusMM is the cylinder radius; LengthMM is the cylinder length
// (NOT including the caps).
type Capsule struct {
	UUID           string
	ReferenceFrame string
	ObserverFrame  string
	Pose           spatialmath.Pose
	RadiusMM       float64
	LengthMM       float64
	Label          string
}

func (c Capsule) ToTransform() *commonpb.Transform {
	return &commonpb.Transform{
		Uuid:           []byte(c.UUID),
		ReferenceFrame: refOr(c.ReferenceFrame, c.UUID),
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: refOr(c.ObserverFrame, DefaultObserverFrame),
			Pose:           PoseToProto(poseOrZero(c.Pose)),
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Capsule{
				Capsule: &commonpb.Capsule{RadiusMm: c.RadiusMM, LengthMm: c.LengthMM},
			},
			Label: labelOr(c.Label, c.UUID),
		},
	}
}

// Point is a degenerate-geometry Transform — a single point in space,
// rendered as a small marker. Useful for waypoints, target poses, etc.
type Point struct {
	UUID           string
	ReferenceFrame string
	ObserverFrame  string
	Pose           spatialmath.Pose
	Label          string
}

// ToTransform produces a Transform with a zero-radius sphere as the
// physical object — the renderer interprets it as a point.
func (p Point) ToTransform() *commonpb.Transform {
	return &commonpb.Transform{
		Uuid:           []byte(p.UUID),
		ReferenceFrame: refOr(p.ReferenceFrame, p.UUID),
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: refOr(p.ObserverFrame, DefaultObserverFrame),
			Pose:           PoseToProto(poseOrZero(p.Pose)),
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Sphere{
				Sphere: &commonpb.Sphere{RadiusMm: 0},
			},
			Label: labelOr(p.Label, p.UUID),
		},
	}
}

// Removal returns a "removal" Transform message — UUID only, no
// geometry — for use in WorldStateStore TransformChange streams when
// signalling that an object is gone.
func Removal(uuid string) *commonpb.Transform {
	return &commonpb.Transform{Uuid: []byte(uuid)}
}

// ---- helpers ----

func refOr(set, fallback string) string {
	if set == "" {
		return fallback
	}
	return set
}

func labelOr(set, fallback string) string {
	if set == "" {
		return fallback
	}
	return set
}

func poseOrZero(p spatialmath.Pose) spatialmath.Pose {
	if p == nil {
		return spatialmath.NewZeroPose()
	}
	return p
}
