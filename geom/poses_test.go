package geom

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"
)

const eps = 1e-6

func TestPose6DToPose(t *testing.T) {
	p := Pose6D{X: 1, Y: 2, Z: 3, OX: 0, OY: 0, OZ: -1, Theta: 45}
	pose := p.ToPose()
	pt := pose.Point()
	if math.Abs(pt.X-1) > eps || math.Abs(pt.Y-2) > eps || math.Abs(pt.Z-3) > eps {
		t.Fatalf("translation: got %v, want (1,2,3)", pt)
	}
	ov := pose.Orientation().OrientationVectorDegrees()
	if math.Abs(ov.OZ-(-1)) > eps || math.Abs(ov.Theta-45) > eps {
		t.Fatalf("orientation: got OZ=%v Theta=%v, want OZ=-1 Theta=45", ov.OZ, ov.Theta)
	}
}

func TestPoseFromRoundTrip(t *testing.T) {
	original := spatialmath.NewPose(
		r3.Vector{X: 100, Y: 200, Z: 300},
		&spatialmath.OrientationVectorDegrees{OX: 0, OY: 0, OZ: -1, Theta: 45},
	)
	got := PoseFrom(original)
	rebuilt := got.ToPose()
	pt := rebuilt.Point()
	if math.Abs(pt.X-100) > eps || math.Abs(pt.Y-200) > eps || math.Abs(pt.Z-300) > eps {
		t.Errorf("translation round-trip: got %v, want (100,200,300)", pt)
	}
	ov := rebuilt.Orientation().OrientationVectorDegrees()
	if math.Abs(ov.OZ-(-1)) > eps || math.Abs(ov.Theta-45) > eps {
		t.Errorf("orientation round-trip: OZ=%v Theta=%v, want OZ=-1 Theta=45", ov.OZ, ov.Theta)
	}
}

func TestPose6DToMap(t *testing.T) {
	p := Pose6D{X: 1, Y: 2, Z: 3, OX: 0, OY: 0, OZ: -1, Theta: 45}
	m := p.ToMap()
	for k, want := range map[string]float64{
		"x": 1, "y": 2, "z": 3, "o_x": 0, "o_y": 0, "o_z": -1, "theta": 45,
	} {
		got, ok := m[k].(float64)
		if !ok {
			t.Errorf("key %q: missing or wrong type", k)
			continue
		}
		if math.Abs(got-want) > eps {
			t.Errorf("key %q: got %v, want %v", k, got, want)
		}
	}
}

func TestVec3DNormalized(t *testing.T) {
	cases := []struct {
		name    string
		v       Vec3D
		wantX   float64
		wantY   float64
		wantZ   float64
	}{
		{"unit X", Vec3D{X: 1}, 1, 0, 0},
		{"scale-3 X", Vec3D{X: 3}, 1, 0, 0},
		{"zero fallback", Vec3D{}, 0, 0, 1},
		{"diagonal", Vec3D{X: 1, Y: 1, Z: 1}, 1.0 / math.Sqrt(3), 1.0 / math.Sqrt(3), 1.0 / math.Sqrt(3)},
	}
	for _, tc := range cases {
		got := tc.v.Normalized()
		if math.Abs(got.X-tc.wantX) > eps || math.Abs(got.Y-tc.wantY) > eps || math.Abs(got.Z-tc.wantZ) > eps {
			t.Errorf("%s: got %v, want (%v,%v,%v)", tc.name, got, tc.wantX, tc.wantY, tc.wantZ)
		}
	}
}
