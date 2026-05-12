package fakes

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
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
