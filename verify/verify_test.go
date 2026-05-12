package verify

import (
	"strings"
	"testing"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
)

func TestMarshalPlanRequest(t *testing.T) {
	req := motion.MoveReq{ComponentName: "gripper"}
	payload, err := MarshalPlanRequest(req, "builtin", "arm-1",
		[]referenceframe.Input{0, 1, 2, 3, 4, 5}, 15.0)
	if err != nil {
		t.Fatalf("MarshalPlanRequest: %v", err)
	}
	if payload == "" {
		t.Fatal("payload should not be empty")
	}
	// The payload is protojson — sanity-check that the configurable
	// pieces appear in some form.
	if !strings.Contains(payload, "arm-1") {
		t.Errorf("payload should reference armName 'arm-1'; got %s", payload)
	}
}

func TestMarshalPlanRequest_omitsStartStateWhenNilJoints(t *testing.T) {
	req := motion.MoveReq{ComponentName: "gripper"}
	payload, err := MarshalPlanRequest(req, "builtin", "arm-1", nil, 15.0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, "start_state") {
		t.Errorf("nil startJoints should omit start_state from extra; got %s", payload)
	}
}

func TestParsePlanResponse_success(t *testing.T) {
	// Simulate the in-process motion service shape: motionplan.Trajectory
	// under the "plan" key.
	traj := motionplan.Trajectory{
		referenceframe.FrameSystemInputs{"arm": {0, 0, 0, 0, 0, 0}},
		referenceframe.FrameSystemInputs{"arm": {1, 2, 3, 4, 5, 6}},
	}
	resp := map[string]interface{}{"plan": traj}
	result := ParsePlanResponse(resp)
	if !result.Feasible {
		t.Errorf("expected feasible, got %+v", result)
	}
	if result.Trajectory == nil {
		t.Error("expected non-nil trajectory")
	}
}

func TestParsePlanResponse_partial(t *testing.T) {
	resp := map[string]interface{}{
		"plan_partialwp": 3.0,
		"plan":           []interface{}{map[string]interface{}{"arm": []interface{}{0.0}}},
	}
	result := ParsePlanResponse(resp)
	if result.Feasible {
		t.Errorf("partial plan should be infeasible; got %+v", result)
	}
	if !strings.Contains(result.Message, "partial plan") {
		t.Errorf("partial plan message: got %q", result.Message)
	}
	if result.Trajectory == nil {
		t.Error("partial plan should still return the partial trajectory")
	}
}

func TestParsePlanResponse_noTrajectory(t *testing.T) {
	resp := map[string]interface{}{"unrelated": "value"}
	result := ParsePlanResponse(resp)
	if !result.Feasible {
		t.Errorf("missing trajectory should still be feasible (plan succeeded); got %+v", result)
	}
	if result.Trajectory != nil {
		t.Error("expected nil trajectory")
	}
	if result.Message == "" {
		t.Error("message should explain the missing trajectory")
	}
}

func TestExtractTrajectory_keyVariations(t *testing.T) {
	for _, k := range []string{"plan", "DoPlan", "trajectory", "Trajectory"} {
		resp := map[string]interface{}{k: "sentinel"}
		if got := extractTrajectory(resp); got != "sentinel" {
			t.Errorf("key %q: got %v, want sentinel", k, got)
		}
	}
}

func TestExtractTrajectory_fallback(t *testing.T) {
	// Unknown key, but value is a list of maps — fallback hits it.
	fallback := []interface{}{map[string]interface{}{"a": 1.0}}
	resp := map[string]interface{}{"random_key": fallback}
	if got := extractTrajectory(resp); got == nil {
		t.Error("fallback should pick up list-of-maps under any key")
	}
}

func TestExtractTrajectory_canonicalOrder(t *testing.T) {
	// When multiple known keys are present, "plan" wins (in-process shape).
	resp := map[string]interface{}{"DoPlan": "second", "plan": "first"}
	if got := extractTrajectory(resp); got != "first" {
		t.Errorf("canonical order: got %v, want 'first'", got)
	}
}

func TestExtractTrajectory_empty(t *testing.T) {
	if got := extractTrajectory(map[string]interface{}{}); got != nil {
		t.Errorf("empty map: got %v, want nil", got)
	}
}

func TestJointStepsFromTrajectory_typedShape(t *testing.T) {
	traj := motionplan.Trajectory{
		referenceframe.FrameSystemInputs{"arm": {0, 0, 0}},
		referenceframe.FrameSystemInputs{"arm": {1, 2, 3}},
	}
	steps := jointStepsFromTrajectory(traj, "arm")
	if len(steps) != 2 {
		t.Errorf("steps: got %d, want 2", len(steps))
	}
}

func TestJointStepsFromTrajectory_grpcShape(t *testing.T) {
	traj := []interface{}{
		map[string]interface{}{"arm": []interface{}{0.0, 0.0}},
		map[string]interface{}{"arm": []interface{}{1.0, 2.0}},
	}
	steps := jointStepsFromTrajectory(traj, "arm")
	if len(steps) != 2 {
		t.Errorf("steps: got %d, want 2", len(steps))
	}
}

func TestJointStepsFromTrajectory_wrongArm(t *testing.T) {
	traj := motionplan.Trajectory{
		referenceframe.FrameSystemInputs{"arm": {0, 0, 0}},
	}
	if steps := jointStepsFromTrajectory(traj, "other-arm"); len(steps) != 0 {
		t.Errorf("wrong arm name: got %d steps, want 0", len(steps))
	}
}
