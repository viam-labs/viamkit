package kinematics

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
)

const jointsEps = 1e-9

func newInputs(vs ...float64) []referenceframe.Input {
	out := make([]referenceframe.Input, len(vs))
	for i, v := range vs {
		out[i] = referenceframe.Input(v)
	}
	return out
}

func TestPreRotatedJoints_OnlyJ0AndJ5Move(t *testing.T) {
	curr := newInputs(0.1, 0.2, 0.3, 0.4, 0.5, 0.6)
	got := PreRotatedJoints(
		curr,
		r3.Vector{X: 500, Y: 0}, // current EE at +X
		0,                       // current yaw 0
		r3.Vector{X: 0, Y: 500}, // target EE at +Y → 90° world rotation
		math.Pi/2,               // target yaw +90°
		SignConventionURStyle,
	)
	if len(got) != 6 {
		t.Fatalf("len: got %d, want 6", len(got))
	}
	// Joints 1..4 must be unchanged.
	for i := 1; i <= 4; i++ {
		if float64(got[i]) != float64(curr[i]) {
			t.Errorf("joint %d: got %v, want %v (unchanged)", i, got[i], curr[i])
		}
	}
}

func TestPreRotatedJoints_URConventionJ0Sign(t *testing.T) {
	// EE at (+X, 0), target at (0, +Y) → world delta = +90°.
	// UR convention: deltaJ0 = -world_delta = -90° = -π/2.
	curr := newInputs(0, 0, 0, 0, 0, 0)
	got := PreRotatedJoints(
		curr,
		r3.Vector{X: 500, Y: 0},
		0,
		r3.Vector{X: 0, Y: 500},
		0, // yaw stays at 0
		SignConventionURStyle,
	)
	wantJ0 := -math.Pi / 2
	if math.Abs(float64(got[0])-wantJ0) > jointsEps {
		t.Errorf("J0: got %v, want %v (UR convention is inverted)", got[0], wantJ0)
	}
}

func TestPreRotatedJoints_FlippedConventionJ0Sign(t *testing.T) {
	curr := newInputs(0, 0, 0, 0, 0, 0)
	got := PreRotatedJoints(
		curr,
		r3.Vector{X: 500, Y: 0},
		0,
		r3.Vector{X: 0, Y: 500},
		0,
		SignConventionFlipped,
	)
	wantJ0 := math.Pi / 2
	if math.Abs(float64(got[0])-wantJ0) > jointsEps {
		t.Errorf("J0 flipped: got %v, want %v", got[0], wantJ0)
	}
}

func TestPreRotatedJoints_YawConsistencyRule(t *testing.T) {
	// world_yaw = sign·(J0 + J5).  UR sign = -1.
	// Starting yaw 0, target yaw +π/4.
	// Pure yaw change with same XY (no J0 motion → deltaJ0 = 0).
	curr := newInputs(0, 0, 0, 0, 0, 0)
	got := PreRotatedJoints(
		curr,
		r3.Vector{X: 500, Y: 0},
		0,
		r3.Vector{X: 500, Y: 0}, // same XY → no J0 swing
		math.Pi/4,
		SignConventionURStyle,
	)
	// deltaJ0 = 0; deltaJ5 = sign·deltaYaw = -π/4.
	wantJ5 := -math.Pi / 4
	if math.Abs(float64(got[5])-wantJ5) > jointsEps {
		t.Errorf("J5 (yaw only): got %v, want %v", got[5], wantJ5)
	}
	if math.Abs(float64(got[0])) > jointsEps {
		t.Errorf("J0 should be unchanged when XY doesn't move: got %v", got[0])
	}
}

func TestPreRotatedJoints_DoesNotMutateInput(t *testing.T) {
	curr := newInputs(0.1, 0.2, 0.3, 0.4, 0.5, 0.6)
	orig := make([]referenceframe.Input, len(curr))
	copy(orig, curr)
	_ = PreRotatedJoints(curr, r3.Vector{X: 1}, 0, r3.Vector{X: 0, Y: 1}, math.Pi/2, SignConventionURStyle)
	for i := range orig {
		if curr[i] != orig[i] {
			t.Errorf("input mutated at %d: %v vs %v", i, curr[i], orig[i])
		}
	}
}

func TestPreRotatedJoints_ShortestPath(t *testing.T) {
	// Target world angle of 350° from current 10° should rotate
	// -20° (the short way), not +340° (the long way).
	curr := newInputs(0, 0, 0, 0, 0, 0)
	got := PreRotatedJoints(
		curr,
		// 10° in world = (cos10°, sin10°)
		r3.Vector{X: math.Cos(10 * math.Pi / 180), Y: math.Sin(10 * math.Pi / 180)},
		0,
		// 350° = -10° → (cos(-10), sin(-10))
		r3.Vector{X: math.Cos(-10 * math.Pi / 180), Y: math.Sin(-10 * math.Pi / 180)},
		0,
		SignConventionURStyle,
	)
	// World delta = -20° (short way). UR sign → deltaJ0 = +20° = +π/9.
	wantJ0 := 20.0 * math.Pi / 180
	if math.Abs(float64(got[0])-wantJ0) > 1e-6 {
		t.Errorf("shortest path: got J0=%v, want %v (+20°, not +340°)", got[0], wantJ0)
	}
}

func TestPreRotatedJoints_ShortInputUnchanged(t *testing.T) {
	// If fewer than 6 joints, return a copy with no mutations.
	curr := newInputs(0.1, 0.2, 0.3)
	got := PreRotatedJoints(curr, r3.Vector{X: 1}, 0, r3.Vector{X: 0, Y: 1}, math.Pi/2, SignConventionURStyle)
	if len(got) != 3 {
		t.Fatalf("len: got %d, want 3", len(got))
	}
	for i := range curr {
		if float64(got[i]) != float64(curr[i]) {
			t.Errorf("short input shouldn't mutate; index %d got %v want %v", i, got[i], curr[i])
		}
	}
}

func TestAlignStartJointsToPlaceYaw_OnlyJ5Moves(t *testing.T) {
	saved := newInputs(0.1, 0.2, 0.3, 0.4, 0.5, 0.6)
	got := AlignStartJointsToPlaceYaw(saved, 0, math.Pi/3, SignConventionURStyle)
	for i := 0; i <= 4; i++ {
		if float64(got[i]) != float64(saved[i]) {
			t.Errorf("joint %d: got %v, want %v (only J5 should move)", i, got[i], saved[i])
		}
	}
}

func TestAlignStartJointsToPlaceYaw_J5DeltaSign(t *testing.T) {
	saved := newInputs(0, 0, 0, 0, 0, 0)
	// saved yaw 0, target yaw +π/2. UR convention: deltaJ5 = sign·delta = -π/2.
	got := AlignStartJointsToPlaceYaw(saved, 0, math.Pi/2, SignConventionURStyle)
	want := -math.Pi / 2
	if math.Abs(float64(got[5])-want) > jointsEps {
		t.Errorf("UR J5 delta: got %v, want %v", got[5], want)
	}
}

func TestAlignStartJointsToPlaceYaw_PreservesSavedJ5Offset(t *testing.T) {
	// saved J5 already at +0.3; target yaw +π/4 from saved yaw of 0.
	// UR: deltaJ5 = -π/4; new J5 = 0.3 - π/4.
	saved := newInputs(0, 0, 0, 0, 0, 0.3)
	got := AlignStartJointsToPlaceYaw(saved, 0, math.Pi/4, SignConventionURStyle)
	want := 0.3 - math.Pi/4
	if math.Abs(float64(got[5])-want) > jointsEps {
		t.Errorf("J5 preserves saved offset: got %v, want %v", got[5], want)
	}
}

func TestWrapToShortestPath(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{0, 0},
		{math.Pi / 2, math.Pi / 2},
		{math.Pi + 0.1, math.Pi + 0.1 - 2*math.Pi},   // wraps to negative
		{-math.Pi - 0.1, -math.Pi - 0.1 + 2*math.Pi}, // wraps to positive
		{3 * math.Pi, math.Pi},                       // 3π → π
	}
	for _, c := range cases {
		got := wrapToShortestPath(c.in)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("wrapToShortestPath(%v): got %v, want %v", c.in, got, c.want)
		}
	}
}
