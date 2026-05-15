package viz

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"
)

func TestTrajectoryTransformsBasics(t *testing.T) {
	poses := []spatialmath.Pose{
		spatialmath.NewPoseFromPoint(r3.Vector{X: 0}),
		spatialmath.NewPoseFromPoint(r3.Vector{X: 100}),
		spatialmath.NewPoseFromPoint(r3.Vector{X: 200}),
	}
	trs := TrajectoryTransforms("plan1", poses)
	if len(trs) != 3 {
		t.Fatalf("len: got %d, want 3", len(trs))
	}
	for i, tr := range trs {
		want := trajectoryUUID("plan1", i)
		if string(tr.Uuid) != want {
			t.Errorf("UUID[%d]: got %q, want %q", i, tr.Uuid, want)
		}
		if tr.PhysicalObject.GetSphere() == nil {
			t.Errorf("transform %d: expected Sphere geometry", i)
		}
		if tr.Metadata == nil {
			t.Errorf("transform %d: expected color metadata", i)
		}
	}
}

func TestTrajectoryTransformsEmpty(t *testing.T) {
	trs := TrajectoryTransforms("p", nil)
	if trs == nil {
		t.Fatal("expected empty (not nil) slice for empty input")
	}
	if len(trs) != 0 {
		t.Errorf("len: got %d, want 0", len(trs))
	}
}

func TestTrajectoryUUIDsRoundtrip(t *testing.T) {
	poses := []spatialmath.Pose{
		spatialmath.NewPoseFromPoint(r3.Vector{}),
		spatialmath.NewPoseFromPoint(r3.Vector{X: 10}),
	}
	trs := TrajectoryTransforms("foo", poses)
	uuids := TrajectoryUUIDs("foo", len(poses))
	if len(uuids) != len(trs) {
		t.Fatalf("UUID count mismatch: %d vs %d", len(uuids), len(trs))
	}
	for i := range trs {
		if string(trs[i].Uuid) != uuids[i] {
			t.Errorf("UUID[%d] mismatch: %q vs %q", i, trs[i].Uuid, uuids[i])
		}
	}
}

func TestTrajectoryOptionsOverride(t *testing.T) {
	poses := []spatialmath.Pose{spatialmath.NewPoseFromPoint(r3.Vector{})}
	custom := Color{R: 255, G: 0, B: 255, Opacity: 0.5}
	trs := TrajectoryTransforms("p", poses, TrajectoryOptions{
		Color:         custom,
		RadiusMM:      12,
		ObserverFrame: "arm-base",
		LabelPrefix:   "preview",
	})
	if trs[0].PoseInObserverFrame.ReferenceFrame != "arm-base" {
		t.Errorf("ObserverFrame: got %q, want arm-base", trs[0].PoseInObserverFrame.ReferenceFrame)
	}
	if trs[0].PhysicalObject.GetSphere().RadiusMm != 12 {
		t.Errorf("RadiusMM: got %v, want 12", trs[0].PhysicalObject.GetSphere().RadiusMm)
	}
	if trs[0].PhysicalObject.Label != "preview 0" {
		t.Errorf("Label: got %q, want %q", trs[0].PhysicalObject.Label, "preview 0")
	}
}

func TestTrajectoryNilPoseDefaultsToZero(t *testing.T) {
	trs := TrajectoryTransforms("p", []spatialmath.Pose{nil})
	if trs[0].PoseInObserverFrame.Pose.X != 0 {
		t.Errorf("nil pose should default to zero, got %+v", trs[0].PoseInObserverFrame.Pose)
	}
}
