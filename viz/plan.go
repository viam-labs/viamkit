package viz

import (
	"fmt"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"

	commonpb "go.viam.com/api/common/v1"
)

// Default rendering values for trajectory waypoints. 6 mm spheres are
// big enough to see in the 3D scene without overpowering the rest of
// it; teal (~#33b1c4) is high-contrast against the typical floor/wall
// palette and reads as "preview" rather than "real geometry."
var (
	DefaultTrajectoryColor    = Color{R: 51, G: 177, B: 196, Opacity: 0.9}
	DefaultTrajectoryRadiusMM = 6.0
)

// TrajectoryOptions controls how TrajectoryTransforms renders the
// preview chain.
type TrajectoryOptions struct {
	// Color overrides the default teal. Zero Color = default.
	Color Color

	// RadiusMM overrides the default per-waypoint sphere radius.
	// Zero = default.
	RadiusMM float64

	// ObserverFrame sets the parent frame for all emitted transforms.
	// Defaults to "world".
	ObserverFrame string

	// LabelPrefix is the geometry-label prefix; the per-waypoint label
	// becomes "<LabelPrefix> N". Defaults to "trajectory".
	LabelPrefix string
}

// TrajectoryTransforms turns a list of end-effector poses into a chain
// of small Sphere Transforms suitable for previewing a motion plan in
// the Viam 3D scene. UUIDs are stable strings of the form
// "<prefix>-traj-<index>" so the caller can ADD them, then REMOVE the
// same UUIDs (or use viz.Removal) when the preview should clear.
//
// `prefix` should be unique per preview surface (e.g. one prefix per
// arm or per plan slot) so concurrent previews don't collide.
//
// Typical pairing inside a WorldStateStore producer:
//
//	poses, _ := verify.TrajectoryToEEPoses(kin, trajectory, gripperOffset)
//	for _, tr := range viz.TrajectoryTransforms("arm0", poses) {
//	    store.Set(tr)
//	}
//	// later, when the preview should clear:
//	for _, tr := range viz.TrajectoryUUIDs("arm0", len(poses)) {
//	    store.Remove(tr)
//	}
//
// Returns one transform per pose. An empty `poses` slice returns an
// empty result, not nil.
func TrajectoryTransforms(prefix string, poses []spatialmath.Pose, opts ...TrajectoryOptions) []*commonpb.Transform {
	o := mergeTrajectoryOptions(opts...)
	color := o.Color
	if !color.IsSet() {
		color = DefaultTrajectoryColor
	}
	radius := o.RadiusMM
	if radius <= 0 {
		radius = DefaultTrajectoryRadiusMM
	}
	observer := o.ObserverFrame
	if observer == "" {
		observer = DefaultObserverFrame
	}
	labelPrefix := o.LabelPrefix
	if labelPrefix == "" {
		labelPrefix = "trajectory"
	}

	out := make([]*commonpb.Transform, 0, len(poses))
	for i, pose := range poses {
		uuid := trajectoryUUID(prefix, i)
		if pose == nil {
			pose = spatialmath.NewZeroPose()
		}
		out = append(out, Sphere{
			UUID:          uuid,
			ObserverFrame: observer,
			Pose:          pose,
			RadiusMM:      radius,
			Label:         fmt.Sprintf("%s %d", labelPrefix, i),
			Color:         color,
		}.ToTransform())
	}
	return out
}

// TrajectoryUUIDs returns the UUIDs TrajectoryTransforms would mint
// for a preview of length `count`. Useful for cleanup — pass them to
// a WorldStateStore Remove method (or wrap in viz.Removal) when the
// preview should disappear.
func TrajectoryUUIDs(prefix string, count int) []string {
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, trajectoryUUID(prefix, i))
	}
	return out
}

// PoseFromVec builds a position-only spatialmath.Pose. Convenience
// for trajectory callers that have raw position lists.
func PoseFromVec(p r3.Vector) spatialmath.Pose {
	return spatialmath.NewPoseFromPoint(p)
}

func trajectoryUUID(prefix string, i int) string {
	return fmt.Sprintf("%s-traj-%d", prefix, i)
}

func mergeTrajectoryOptions(opts ...TrajectoryOptions) TrajectoryOptions {
	var out TrajectoryOptions
	for _, o := range opts {
		if o.Color.IsSet() {
			out.Color = o.Color
		}
		if o.RadiusMM != 0 {
			out.RadiusMM = o.RadiusMM
		}
		if o.ObserverFrame != "" {
			out.ObserverFrame = o.ObserverFrame
		}
		if o.LabelPrefix != "" {
			out.LabelPrefix = o.LabelPrefix
		}
	}
	return out
}
