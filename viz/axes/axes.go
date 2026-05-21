// Package axes builds coordinate-axis Transforms for the Viam 3D
// scene viewer. One call drops a colored X/Y/Z triad at a chosen
// origin pose — useful for visualizing arm bases, calibration targets,
// per-component frames, etc. without re-deriving the capsule rotation
// math each time.
//
// Each axis renders as a colored capsule extending from the origin
// along its world-frame direction. Conventions match the standard
// robotics palette: X = red, Y = green, Z = blue.
//
// Typical use inside a WorldStateStore producer:
//
//	// "arm-base": UUID prefix for the three Transforms; 200: axis length in mm.
//	for _, tr := range axes.Axes("arm-base", armBasePose, 200) {
//	    store.Set(tr)
//	}
package axes

import (
	"fmt"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"

	commonpb "go.viam.com/api/common/v1"

	"github.com/viam-labs/viamkit/viz"
)

// Default axis colors. Standard X=red, Y=green, Z=blue convention.
var (
	ColorX = viz.Color{R: 230, G: 60, B: 60, Opacity: 1}
	ColorY = viz.Color{R: 60, G: 200, B: 80, Opacity: 1}
	ColorZ = viz.Color{R: 80, G: 120, B: 240, Opacity: 1}
)

// Default axis radius. ~3% of the length looks like an arrow shaft
// without overwhelming the rest of the scene.
const defaultRadiusFactor = 0.03

// Options control the axes' rendering.
type Options struct {
	// ObserverFrame for all three transforms. Defaults to "world".
	ObserverFrame string

	// RadiusMM overrides the per-axis capsule radius. When zero, uses
	// 3% of LengthMM.
	RadiusMM float64

	// Colors override the default X/Y/Z palette. Zero Color keeps the
	// matching default.
	ColorX, ColorY, ColorZ viz.Color
}

// Axes returns three Transforms (X, Y, Z) rooted at `origin`, each
// extending `lengthMM` along the corresponding world-frame axis.
// UUIDs are derived from `prefix`: "<prefix>-x", "<prefix>-y",
// "<prefix>-z" — use a unique prefix per axes instance so multiple
// triads don't collide in the WorldStateStore.
//
// `origin` is the pose where the axis triad is anchored — for an
// arm-base triad, pass the arm-base pose. The capsules extend OUT
// from the origin along world +X, +Y, +Z; the origin itself sits at
// the back end of each capsule.
//
// `lengthMM` is the visible length of each axis along its direction.
// `opts` allows overriding colors, radius, and observer frame.
func Axes(prefix string, origin spatialmath.Pose, lengthMM float64, opts ...Options) []*commonpb.Transform {
	o := mergeOptions(opts...)
	radius := o.RadiusMM
	if radius <= 0 {
		radius = lengthMM * defaultRadiusFactor
	}
	if origin == nil {
		origin = spatialmath.NewZeroPose()
	}

	return []*commonpb.Transform{
		axisCapsule(prefix, "x", origin, axisDirX, lengthMM, radius, o.ObserverFrame, defaultIfUnset(o.ColorX, ColorX)),
		axisCapsule(prefix, "y", origin, axisDirY, lengthMM, radius, o.ObserverFrame, defaultIfUnset(o.ColorY, ColorY)),
		axisCapsule(prefix, "z", origin, axisDirZ, lengthMM, radius, o.ObserverFrame, defaultIfUnset(o.ColorZ, ColorZ)),
	}
}

// axisDir captures the unit direction and orientation-vector for an
// axis. Capsules are oriented by their OV: the local-Z axis of the
// capsule rotates to point along (OX, OY, OZ).
type axisDir struct {
	dir      r3.Vector
	ovDegree spatialmath.OrientationVectorDegrees
}

var (
	axisDirX = axisDir{
		dir:      r3.Vector{X: 1},
		ovDegree: spatialmath.OrientationVectorDegrees{OX: 1, OY: 0, OZ: 0},
	}
	axisDirY = axisDir{
		dir:      r3.Vector{Y: 1},
		ovDegree: spatialmath.OrientationVectorDegrees{OX: 0, OY: 1, OZ: 0},
	}
	axisDirZ = axisDir{
		dir:      r3.Vector{Z: 1},
		ovDegree: spatialmath.OrientationVectorDegrees{OX: 0, OY: 0, OZ: 1},
	}
)

func axisCapsule(prefix, suffix string, origin spatialmath.Pose, axis axisDir, lengthMM, radiusMM float64, observer string, color viz.Color) *commonpb.Transform {
	// Capsule center sits halfway along the axis so the geometry
	// extends from origin to origin + lengthMM·dir.
	half := lengthMM / 2
	localPose := spatialmath.NewPose(
		r3.Vector{X: axis.dir.X * half, Y: axis.dir.Y * half, Z: axis.dir.Z * half},
		&axis.ovDegree,
	)
	return viz.Capsule{
		UUID:          fmt.Sprintf("%s-%s", prefix, suffix),
		ObserverFrame: observer,
		Pose:          spatialmath.Compose(origin, localPose),
		RadiusMM:      radiusMM,
		LengthMM:      lengthMM,
		Color:         color,
	}.ToTransform()
}

func mergeOptions(opts ...Options) Options {
	var out Options
	for _, o := range opts {
		if o.ObserverFrame != "" {
			out.ObserverFrame = o.ObserverFrame
		}
		if o.RadiusMM != 0 {
			out.RadiusMM = o.RadiusMM
		}
		if o.ColorX.IsSet() {
			out.ColorX = o.ColorX
		}
		if o.ColorY.IsSet() {
			out.ColorY = o.ColorY
		}
		if o.ColorZ.IsSet() {
			out.ColorZ = o.ColorZ
		}
	}
	return out
}

func defaultIfUnset(c, fallback viz.Color) viz.Color {
	if c.IsSet() {
		return c
	}
	return fallback
}
