package kinematics

import (
	"math"
	"strings"
	"testing"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const eps = 1e-6

func TestYawFromOrientation(t *testing.T) {
	cases := []struct {
		name string
		oz   float64
		th   float64
		want float64
	}{
		{"down-zero", -1, 0, 0},
		{"down-90", -1, 90, -math.Pi / 2},
		{"down-neg90", -1, -90, math.Pi / 2},
		{"down-180", -1, 180, -math.Pi},
		{"up-90", 1, 90, math.Pi / 2},
		{"up-zero", 1, 0, 0},
	}
	for _, tc := range cases {
		o := &spatialmath.OrientationVectorDegrees{OZ: tc.oz, Theta: tc.th}
		got := YawFromOrientation(o)
		if math.Abs(got-tc.want) > eps {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestLastTrajectoryJoints(t *testing.T) {
	const armName = "arm-simulated"

	// Typed Go path.
	typed := motionplan.Trajectory{
		referenceframe.FrameSystemInputs{armName: {0, 0, 0, 0, 0, 0}},
		referenceframe.FrameSystemInputs{armName: {1, 2, 3, 4, 5, 6}},
	}
	got := LastTrajectoryJoints(typed, armName)
	if len(got) != 6 || math.Abs(float64(got[5])-6) > eps {
		t.Errorf("typed: got %v, want last=[1..6]", got)
	}

	// gRPC path.
	grpc := []interface{}{
		map[string]interface{}{armName: []interface{}{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}},
		map[string]interface{}{armName: []interface{}{0.5, 1.0, 1.5, 2.0, 2.5, 3.0}},
	}
	got = LastTrajectoryJoints(grpc, armName)
	if len(got) != 6 || math.Abs(float64(got[0])-0.5) > eps || math.Abs(float64(got[5])-3.0) > eps {
		t.Errorf("grpc: got %v, want last=[0.5..3.0]", got)
	}

	// Nil / empty / wrong shape.
	if got := LastTrajectoryJoints(nil, armName); got != nil {
		t.Errorf("nil: got %v, want nil", got)
	}
	if got := LastTrajectoryJoints(motionplan.Trajectory{}, armName); got != nil {
		t.Errorf("empty typed: got %v, want nil", got)
	}
	if got := LastTrajectoryJoints([]interface{}{}, armName); got != nil {
		t.Errorf("empty grpc: got %v, want nil", got)
	}
	// Wrong arm in typed → nil/empty
	if got := LastTrajectoryJoints(typed, "other-arm"); len(got) != 0 {
		t.Errorf("typed wrong arm: got %v, want nil/empty", got)
	}
	// gRPC arm name missing
	noArm := []interface{}{
		map[string]interface{}{"some-other-arm": []interface{}{0.0}},
	}
	if got := LastTrajectoryJoints(noArm, armName); got != nil {
		t.Errorf("grpc wrong arm: got %v, want nil", got)
	}
}

func TestTrajectoryToJointPath(t *testing.T) {
	const armName = "arm-simulated"

	typed := motionplan.Trajectory{
		referenceframe.FrameSystemInputs{armName: {0, 0, 0, 0, 0, 0}},
		referenceframe.FrameSystemInputs{armName: {1, 2, 3, 4, 5, 6}},
	}
	got := TrajectoryToJointPath(typed, armName)
	if len(got) != 2 {
		t.Fatalf("typed: got %d steps, want 2", len(got))
	}
	if math.Abs(got[1][3]-4) > eps {
		t.Errorf("typed step1 joint3: got %v, want 4", got[1][3])
	}
	if got := TrajectoryToJointPath(typed, "other-arm"); got != nil {
		t.Errorf("typed wrong arm: got %v, want nil", got)
	}

	grpc := []interface{}{
		map[string]interface{}{armName: []interface{}{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}},
		map[string]interface{}{armName: []interface{}{0.5, 1.0, 1.5, 2.0, 2.5, 3.0}},
	}
	got = TrajectoryToJointPath(grpc, armName)
	if len(got) != 2 {
		t.Fatalf("grpc: got %d steps, want 2", len(got))
	}
	if math.Abs(got[1][5]-3.0) > eps {
		t.Errorf("grpc step1 joint5: got %v, want 3.0", got[1][5])
	}

	if got := TrajectoryToJointPath(motionplan.Trajectory{}, armName); got != nil {
		t.Errorf("empty typed: got %v, want nil", got)
	}
	if got := TrajectoryToJointPath([]interface{}{}, armName); got != nil {
		t.Errorf("empty grpc: got %v, want nil", got)
	}
	if got := TrajectoryToJointPath("not-a-traj", armName); got != nil {
		t.Errorf("garbage: got %v, want nil", got)
	}
}

func TestInterpolateJointPath(t *testing.T) {
	start := []referenceframe.Input{0, 0, 0, 0, 0, 0}
	end := []referenceframe.Input{1, 2, 3, 4, 5, 6}

	// 0 samples → default 32, so 33 points.
	got := InterpolateJointPath(start, end, 0)
	if len(got) != DefaultInterpolateSamples+1 {
		t.Errorf("0 samples (default): got %d, want %d", len(got), DefaultInterpolateSamples+1)
	}

	// 4 samples → 5 points (inclusive endpoints).
	got = InterpolateJointPath(start, end, 4)
	if len(got) != 5 {
		t.Fatalf("4 samples: got %d, want 5", len(got))
	}
	// First = start.
	for j, v := range got[0] {
		if math.Abs(v-float64(start[j])) > eps {
			t.Errorf("first joint %d: got %v, want %v", j, v, start[j])
		}
	}
	// Last = end.
	for j, v := range got[4] {
		if math.Abs(v-float64(end[j])) > eps {
			t.Errorf("last joint %d: got %v, want %v", j, v, end[j])
		}
	}
	// Midpoint = halfway.
	for j, v := range got[2] {
		want := 0.5 * float64(end[j])
		if math.Abs(v-want) > eps {
			t.Errorf("midpoint joint %d: got %v, want %v", j, v, want)
		}
	}

	// Mismatched length → nil.
	if got := InterpolateJointPath(start, []referenceframe.Input{1, 2}, 4); got != nil {
		t.Errorf("mismatched: got %v, want nil", got)
	}
	// Empty → nil.
	if got := InterpolateJointPath(nil, nil, 4); got != nil {
		t.Errorf("empty: got %v, want nil", got)
	}
}

func TestFriendlyPlannerError(t *testing.T) {
	cases := []struct {
		raw, label, wantPart string
	}{
		{"", "place_start", "unknown"},
		{"cbirrt timeout after 15s", "place_end", "timed out"},
		{"context deadline exceeded", "place_start", "timed out"},
		{"no paths found", "place_end", "No collision-free"},
		{"no valid path", "place_end", "No collision-free"},
		{"goal is unreachable", "place_start", "outside joint limits"},
		{"IK solve failed", "place_start", "outside joint limits"},
		{"collision detected", "place_end", "collide"},
		{"partial plan returned", "place_end", "partial path"},
		{"skipped due to upstream failure", "place_end", "Skipped"},
		{"could not extract trajectory", "place_end", "Internal error"},
	}
	for _, tc := range cases {
		got := FriendlyPlannerError(tc.raw, tc.label)
		if !strings.Contains(got, tc.wantPart) {
			t.Errorf("FriendlyPlannerError(%q, %q) = %q, want substring %q",
				tc.raw, tc.label, got, tc.wantPart)
		}
	}
}

func TestFriendlyPlannerError_unknownPatternFallsThrough(t *testing.T) {
	got := FriendlyPlannerError("rpc error: code = Unknown desc = something weird happened", "x")
	if !strings.Contains(got, "something weird happened") {
		t.Errorf("expected trimmed message; got %q", got)
	}
	if strings.Contains(got, "rpc error:") {
		t.Errorf("expected rpc prefix trimmed; got %q", got)
	}
}
