package axes

import (
	"bytes"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"

	"github.com/viam-labs/viamkit/viz"
)

const eps = 1e-6

func TestAxesReturnsThreeTransforms(t *testing.T) {
	trs := Axes("arm-base", spatialmath.NewZeroPose(), 100)
	if len(trs) != 3 {
		t.Fatalf("Axes returned %d transforms, want 3", len(trs))
	}
}

func TestAxesUUIDs(t *testing.T) {
	trs := Axes("arm-base", spatialmath.NewZeroPose(), 100)
	want := []string{"arm-base-x", "arm-base-y", "arm-base-z"}
	for i, tr := range trs {
		if !bytes.Equal(tr.Uuid, []byte(want[i])) {
			t.Errorf("UUID[%d]: got %q, want %q", i, tr.Uuid, want[i])
		}
	}
}

func TestAxesColors(t *testing.T) {
	trs := Axes("a", spatialmath.NewZeroPose(), 100)
	// X axis = red
	cv := trs[0].Metadata.Fields["color"].GetStructValue()
	if cv.Fields["r"].GetNumberValue() <= cv.Fields["g"].GetNumberValue() ||
		cv.Fields["r"].GetNumberValue() <= cv.Fields["b"].GetNumberValue() {
		t.Errorf("X axis should be red-dominant, got r=%v g=%v b=%v",
			cv.Fields["r"].GetNumberValue(), cv.Fields["g"].GetNumberValue(), cv.Fields["b"].GetNumberValue())
	}
	// Y axis = green
	cv = trs[1].Metadata.Fields["color"].GetStructValue()
	if cv.Fields["g"].GetNumberValue() <= cv.Fields["r"].GetNumberValue() ||
		cv.Fields["g"].GetNumberValue() <= cv.Fields["b"].GetNumberValue() {
		t.Errorf("Y axis should be green-dominant, got r=%v g=%v b=%v",
			cv.Fields["r"].GetNumberValue(), cv.Fields["g"].GetNumberValue(), cv.Fields["b"].GetNumberValue())
	}
	// Z axis = blue
	cv = trs[2].Metadata.Fields["color"].GetStructValue()
	if cv.Fields["b"].GetNumberValue() <= cv.Fields["r"].GetNumberValue() ||
		cv.Fields["b"].GetNumberValue() <= cv.Fields["g"].GetNumberValue() {
		t.Errorf("Z axis should be blue-dominant, got r=%v g=%v b=%v",
			cv.Fields["r"].GetNumberValue(), cv.Fields["g"].GetNumberValue(), cv.Fields["b"].GetNumberValue())
	}
}

func TestAxesCapsuleLengthAndRadius(t *testing.T) {
	trs := Axes("a", spatialmath.NewZeroPose(), 100)
	for i, tr := range trs {
		capsule := tr.PhysicalObject.GetCapsule()
		if capsule == nil {
			t.Fatalf("axis %d: not a capsule", i)
		}
		if math.Abs(capsule.LengthMm-100) > eps {
			t.Errorf("axis %d length: got %v, want 100", i, capsule.LengthMm)
		}
		// Default radius is 3% of length = 3 mm.
		if math.Abs(capsule.RadiusMm-3) > eps {
			t.Errorf("axis %d radius default: got %v, want ~3", i, capsule.RadiusMm)
		}
	}
}

func TestAxesOriginTranslated(t *testing.T) {
	// Anchor at (10, 20, 30); X-axis capsule center should be at
	// (10 + 50, 20, 30) — half the length along +X.
	origin := spatialmath.NewPoseFromPoint(r3.Vector{X: 10, Y: 20, Z: 30})
	trs := Axes("a", origin, 100)
	xPose := trs[0].PoseInObserverFrame.Pose
	if math.Abs(xPose.X-60) > eps {
		t.Errorf("X axis center X: got %v, want 60", xPose.X)
	}
	if math.Abs(xPose.Y-20) > eps {
		t.Errorf("X axis center Y: got %v, want 20", xPose.Y)
	}
}

func TestAxesOptionsOverride(t *testing.T) {
	custom := viz.Color{R: 250, G: 250, B: 0, Opacity: 0.5}
	trs := Axes("a", spatialmath.NewZeroPose(), 100, Options{
		ObserverFrame: "arm-base",
		RadiusMM:      10,
		ColorX:        custom,
	})
	if trs[0].PoseInObserverFrame.ReferenceFrame != "arm-base" {
		t.Errorf("ObserverFrame: got %q, want %q", trs[0].PoseInObserverFrame.ReferenceFrame, "arm-base")
	}
	capsule := trs[0].PhysicalObject.GetCapsule()
	if math.Abs(capsule.RadiusMm-10) > eps {
		t.Errorf("RadiusMM override: got %v, want 10", capsule.RadiusMm)
	}
	// X color should be the override (yellow), not the default red.
	cv := trs[0].Metadata.Fields["color"].GetStructValue()
	if cv.Fields["g"].GetNumberValue() <= 100 {
		t.Errorf("ColorX override didn't take effect: g=%v (expected high green for yellow)",
			cv.Fields["g"].GetNumberValue())
	}
}

func TestAxesNilOrigin(t *testing.T) {
	trs := Axes("a", nil, 100)
	if len(trs) != 3 {
		t.Fatalf("nil origin should not crash; got %d transforms", len(trs))
	}
}
