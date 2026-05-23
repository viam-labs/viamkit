package fakes

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/vision/classification"
)

func TestGripperDefaults(t *testing.T) {
	g := NewGripper("test")
	ctx := context.Background()

	if ok, err := g.Grab(ctx, nil); err != nil || !ok {
		t.Errorf("default Grab: got (%v, %v), want (true, nil)", ok, err)
	}
	if err := g.Open(ctx, nil); err != nil {
		t.Errorf("default Open: %v", err)
	}
	if hs, err := g.IsHoldingSomething(ctx, nil); err != nil || !hs.IsHoldingSomething {
		t.Errorf("default IsHoldingSomething: got (%+v, %v), want IsHoldingSomething=true", hs, err)
	}

	if g.GrabCalls() != 1 || g.OpenCalls() != 1 || g.IsHoldingSomethingCalls() != 1 {
		t.Errorf("call counts wrong: grab=%d open=%d ish=%d", g.GrabCalls(), g.OpenCalls(), g.IsHoldingSomethingCalls())
	}
}

func TestGripperOverrideAndReset(t *testing.T) {
	g := NewGripper("test")
	// Script a sequence: first two IsHoldingSomething calls return false,
	// then true. Simulates "seal takes a moment to form."
	seq := []bool{false, false, true}
	idx := 0
	g.IsHoldingSomethingFn = func(_ context.Context, _ map[string]interface{}) (gripper.HoldingStatus, error) {
		v := seq[idx]
		idx++
		return gripper.HoldingStatus{IsHoldingSomething: v}, nil
	}

	for i, want := range seq {
		hs, _ := g.IsHoldingSomething(context.Background(), nil)
		if hs.IsHoldingSomething != want {
			t.Errorf("call %d: got %v, want %v", i, hs.IsHoldingSomething, want)
		}
	}
	if g.IsHoldingSomethingCalls() != 3 {
		t.Errorf("expected 3 calls, got %d", g.IsHoldingSomethingCalls())
	}

	g.Reset()
	if g.IsHoldingSomethingCalls() != 0 {
		t.Errorf("after Reset: got %d, want 0", g.IsHoldingSomethingCalls())
	}
}

func TestResourceSetResponse(t *testing.T) {
	r := NewResource(generic.API, "pack-sequencer")
	r.SetResponse("get_box_dims", map[string]interface{}{
		"box_length_mm": 200.0,
		"box_width_mm":  100.0,
		"box_height_mm": 80.0,
	})

	resp, err := r.DoCommand(context.Background(), map[string]interface{}{"get_box_dims": true})
	if err != nil {
		t.Fatalf("DoCommand: %v", err)
	}
	if got, _ := resp["box_length_mm"].(float64); got != 200 {
		t.Errorf("box_length_mm: got %v, want 200", got)
	}

	calls := r.CallsFor("get_box_dims")
	if len(calls) != 1 {
		t.Errorf("expected 1 recorded call, got %d", len(calls))
	}
}

func TestResourceSetError(t *testing.T) {
	r := NewResource(generic.API, "pack-sequencer")
	want := errors.New("transient")
	r.SetError("next_box", want)

	_, err := r.DoCommand(context.Background(), map[string]interface{}{"next_box": true})
	if !errors.Is(err, want) {
		t.Errorf("DoCommand: got %v, want %v", err, want)
	}
}

func TestArmDefaults(t *testing.T) {
	a := NewArm("test")
	ctx := context.Background()

	joints, err := a.JointPositions(ctx, nil)
	if err != nil || len(joints) != 6 {
		t.Errorf("default JointPositions: got %v (err=%v), want 6 zeros", joints, err)
	}
	pose, err := a.EndPosition(ctx, nil)
	if err != nil || pose == nil {
		t.Errorf("default EndPosition: got %v (err=%v)", pose, err)
	}
	if a.EndPositionCalls() != 1 || a.JointPositionsCalls() != 1 {
		t.Errorf("call counts: end=%d joints=%d", a.EndPositionCalls(), a.JointPositionsCalls())
	}
}

func TestArmMoveToJointPositionsUpdatesState(t *testing.T) {
	a := NewArm("test")
	target := []referenceframe.Input{1, 2, 3, 4, 5, 6}
	if err := a.MoveToJointPositions(context.Background(), target, nil); err != nil {
		t.Fatal(err)
	}
	got := a.LastMoveToJointPositions()
	if len(got) != 6 || got[3] != 4 {
		t.Errorf("LastMoveToJointPositions: got %v, want %v", got, target)
	}
	// Subsequent JointPositions reflects the move (mirrors real arm behavior).
	now, _ := a.JointPositions(context.Background(), nil)
	if len(now) != 6 || now[3] != 4 {
		t.Errorf("JointPositions after move: got %v, want %v", now, target)
	}
}

func TestSwitchSetPositionCaptured(t *testing.T) {
	s := NewSwitch("home-switch")
	if err := s.SetPosition(context.Background(), 2, nil); err != nil {
		t.Fatal(err)
	}
	if s.LastSetPosition() != 2 {
		t.Errorf("LastSetPosition: got %d, want 2", s.LastSetPosition())
	}
	pos, _ := s.GetPosition(context.Background(), nil)
	if pos != 2 {
		t.Errorf("GetPosition after Set: got %d, want 2", pos)
	}
}

func TestSwitchDoCommandForCfg(t *testing.T) {
	// Simulate the arm-position-saver's saved-joints retrieval.
	s := NewSwitch("home-switch")
	wantJoints := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6}
	s.DoCommandFn = func(_ context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		if _, ok := cmd["cfg"]; !ok {
			return nil, nil
		}
		out := make([]interface{}, len(wantJoints))
		for i, v := range wantJoints {
			out[i] = v
		}
		return map[string]interface{}{"joints": out}, nil
	}
	resp, err := s.DoCommand(context.Background(), map[string]interface{}{"cfg": true})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := resp["joints"].([]interface{})
	if len(got) != 6 {
		t.Errorf("joints len: got %d, want 6", len(got))
	}
}

func TestVisionClassificationsFromCameraIsScriptable(t *testing.T) {
	v := NewVision("classifier")
	if cls, _ := v.ClassificationsFromCamera(context.Background(), "camera", 1, nil); cls != nil {
		t.Errorf("default ClassificationsFromCamera: got %v, want nil", cls)
	}
	called := false
	v.ClassificationsFromCameraFn = func(_ context.Context, _ string, _ int, _ map[string]interface{}) (classification.Classifications, error) {
		called = true
		return classification.Classifications{
			classification.NewClassification(0.95, "box"),
		}, nil
	}
	cls, err := v.ClassificationsFromCamera(context.Background(), "camera", 1, nil)
	if err != nil || !called || len(cls) != 1 {
		t.Errorf("scripted ClassificationsFromCamera: got cls=%v err=%v called=%v", cls, err, called)
	}
}

func TestResourceCallLog(t *testing.T) {
	r := NewResource(generic.API, "test")
	r.SetResponse("a", map[string]interface{}{"x": 1})
	r.SetResponse("b", map[string]interface{}{"y": 2})

	for _, verb := range []string{"a", "b", "a"} {
		_, _ = r.DoCommand(context.Background(), map[string]interface{}{verb: true})
	}

	if got := r.DoCommandCalls(); got != 3 {
		t.Errorf("total calls: got %d, want 3", got)
	}
	if got := len(r.CallsFor("a")); got != 2 {
		t.Errorf("a calls: got %d, want 2", got)
	}
	if got := len(r.CallsFor("b")); got != 1 {
		t.Errorf("b calls: got %d, want 1", got)
	}
}

// TestStatusFakes asserts every fake's Status returns an empty map + nil
// error, satisfying resource.Resource's Status method.
func TestStatusFakes(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		fn   func() (map[string]interface{}, error)
	}{
		{"Arm", func() (map[string]interface{}, error) { return NewArm("a").Status(ctx) }},
		{"Gripper", func() (map[string]interface{}, error) { return NewGripper("g").Status(ctx) }},
		{"Resource", func() (map[string]interface{}, error) { return NewResource(generic.API, "r").Status(ctx) }},
		{"Switch", func() (map[string]interface{}, error) { return NewSwitch("s").Status(ctx) }},
		{"Vision", func() (map[string]interface{}, error) { return NewVision("v").Status(ctx) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.fn()
			if err != nil {
				t.Errorf("Status err: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("Status map: got %v, want empty", got)
			}
		})
	}
}
