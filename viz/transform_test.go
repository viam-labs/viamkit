package viz

import (
	"bytes"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"
)

const eps = 1e-6

func TestBoxToTransformDefaults(t *testing.T) {
	tf := Box{
		UUID:   "box-1",
		Pose:   spatialmath.NewPoseFromPoint(r3.Vector{X: 10, Y: 20, Z: 30}),
		DimsMM: r3.Vector{X: 100, Y: 200, Z: 80},
	}.ToTransform()

	if !bytes.Equal(tf.Uuid, []byte("box-1")) {
		t.Errorf("Uuid: got %q, want %q", tf.Uuid, "box-1")
	}
	if tf.ReferenceFrame != "box-1" {
		t.Errorf("ReferenceFrame default: got %q, want %q", tf.ReferenceFrame, "box-1")
	}
	if tf.PoseInObserverFrame.ReferenceFrame != "world" {
		t.Errorf("ObserverFrame default: got %q, want %q", tf.PoseInObserverFrame.ReferenceFrame, "world")
	}
	if got := tf.PhysicalObject.Label; got != "box-1" {
		t.Errorf("Label default: got %q, want %q", got, "box-1")
	}
	box := tf.PhysicalObject.GetBox()
	if box == nil {
		t.Fatal("PhysicalObject.GetBox() is nil")
	}
	if math.Abs(box.DimsMm.X-100) > eps {
		t.Errorf("DimsMm.X: got %v, want 100", box.DimsMm.X)
	}
}

func TestBoxToTransformOverrides(t *testing.T) {
	tf := Box{
		UUID:           "box-2",
		ReferenceFrame: "custom-frame",
		ObserverFrame:  "pallet",
		Pose:           spatialmath.NewPoseFromPoint(r3.Vector{}),
		DimsMM:         r3.Vector{X: 50, Y: 60, Z: 70},
		Label:          "my-label",
	}.ToTransform()

	if tf.ReferenceFrame != "custom-frame" {
		t.Errorf("ReferenceFrame override: got %q", tf.ReferenceFrame)
	}
	if tf.PoseInObserverFrame.ReferenceFrame != "pallet" {
		t.Errorf("ObserverFrame override: got %q", tf.PoseInObserverFrame.ReferenceFrame)
	}
	if tf.PhysicalObject.Label != "my-label" {
		t.Errorf("Label override: got %q", tf.PhysicalObject.Label)
	}
}

func TestSphereToTransform(t *testing.T) {
	tf := Sphere{UUID: "s", RadiusMM: 25}.ToTransform()
	s := tf.PhysicalObject.GetSphere()
	if s == nil {
		t.Fatal("expected Sphere geometry")
	}
	if math.Abs(s.RadiusMm-25) > eps {
		t.Errorf("RadiusMm: got %v, want 25", s.RadiusMm)
	}
}

func TestCapsuleToTransform(t *testing.T) {
	tf := Capsule{UUID: "c", RadiusMM: 10, LengthMM: 100}.ToTransform()
	c := tf.PhysicalObject.GetCapsule()
	if c == nil {
		t.Fatal("expected Capsule geometry")
	}
	if math.Abs(c.RadiusMm-10) > eps || math.Abs(c.LengthMm-100) > eps {
		t.Errorf("Capsule dims: got radius=%v length=%v, want 10 / 100", c.RadiusMm, c.LengthMm)
	}
}

func TestPointToTransform(t *testing.T) {
	tf := Point{UUID: "p"}.ToTransform()
	s := tf.PhysicalObject.GetSphere()
	if s == nil {
		t.Fatal("expected zero-radius Sphere geometry for Point")
	}
	if math.Abs(s.RadiusMm-0) > eps {
		t.Errorf("Point RadiusMm: got %v, want 0", s.RadiusMm)
	}
}

func TestPoseToProtoRoundTrip(t *testing.T) {
	orig := spatialmath.NewPose(
		r3.Vector{X: 100, Y: -50, Z: 200},
		&spatialmath.OrientationVectorDegrees{OX: 0, OY: 0, OZ: -1, Theta: 45},
	)
	proto := PoseToProto(orig)
	rebuilt := PoseFromProto(proto)
	if math.Abs(rebuilt.Point().X-100) > eps || math.Abs(rebuilt.Point().Y+50) > eps {
		t.Errorf("translation round-trip: %v", rebuilt.Point())
	}
	ov := rebuilt.Orientation().OrientationVectorDegrees()
	if math.Abs(ov.OZ-(-1)) > eps || math.Abs(ov.Theta-45) > eps {
		t.Errorf("orientation round-trip: OZ=%v Theta=%v", ov.OZ, ov.Theta)
	}
}

func TestRemoval(t *testing.T) {
	r := Removal("removed-box")
	if !bytes.Equal(r.Uuid, []byte("removed-box")) {
		t.Errorf("Removal UUID: got %q, want %q", r.Uuid, "removed-box")
	}
	if r.PhysicalObject != nil {
		t.Error("Removal should have no PhysicalObject")
	}
}

func TestBoxNilPoseDefaultsToZero(t *testing.T) {
	tf := Box{UUID: "b", DimsMM: r3.Vector{X: 10, Y: 10, Z: 10}}.ToTransform()
	if tf.PoseInObserverFrame.Pose.X != 0 {
		t.Errorf("nil Pose should default to zero, got %+v", tf.PoseInObserverFrame.Pose)
	}
}
