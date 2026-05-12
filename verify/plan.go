package verify

import (
	"context"
	"fmt"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"google.golang.org/protobuf/encoding/protojson"
)

// PlanCommand is the DoCommand key the builtin motion service
// dispatches on for plan-only requests. Sending "DoPlan" instead
// silently hits a no-op branch and returns an empty response — must
// be "plan", lowercase.
const PlanCommand = "plan"

// PlanResult is what ParsePlanResponse / Plan return. Trajectory's
// concrete type varies: motionplan.Trajectory when DoPlan runs
// in-process, []interface{} of map[string]interface{} when the
// response went through gRPC. viamkit/kinematics has helpers
// (LastTrajectoryJoints, TrajectoryToJointPath) that handle both
// shapes uniformly.
type PlanResult struct {
	// Feasible is true when the planner returned a complete
	// trajectory to the goal. False for plan errors, partial
	// plans, and missing-trajectory responses.
	Feasible bool

	// Message is a human-readable status. Empty on clean success;
	// describes the failure mode for non-feasible results
	// (partial plan, missing trajectory, planner error string).
	Message string

	// Trajectory is the planner's joint-space trajectory, or nil
	// when the response didn't include one.
	Trajectory interface{}
}

// MarshalPlanRequest serializes a motion.MoveReq plus optional
// planner start state into the JSON shape the builtin motion
// service's "plan" DoCommand expects.
//
// Parameters:
//
//   - req: the motion request (ComponentName, Destination,
//     WorldState, Constraints, etc.). Don't pre-populate req.Extra
//     with "start_state" or "timeout" — this function manages those.
//
//   - serviceName: the motion service's resource name (typically
//     "builtin").
//
//   - armName: the resource name of the arm whose joints startJoints
//     describes. Used to key the start_state map. Pass "" (and nil
//     startJoints) to omit start_state and let the planner use the
//     arm's current joints.
//
//   - startJoints: pre-supplied arm configuration. Lets you chain
//     plans (use one plan's final joints as the next plan's start),
//     or verify reachability "from pallet_home" without first moving
//     the arm there. Pass nil to use current joints.
//
//   - timeoutSecs: planner timeout per call. The SDK default is
//     300s, which makes a wedged plan grind for 5 minutes; passing
//     a smaller value (e.g. 15) fails fast on infeasible plans.
//     Pass 0 to keep the SDK default.
func MarshalPlanRequest(
	req motion.MoveReq,
	serviceName string,
	armName string,
	startJoints []referenceframe.Input,
	timeoutSecs float64,
) (string, error) {
	extra := req.Extra
	if extra == nil {
		extra = map[string]interface{}{}
	}
	if timeoutSecs > 0 {
		extra["timeout"] = timeoutSecs
	}
	if armName != "" && len(startJoints) > 0 {
		// Hand-rolled armplanning.PlanState.Serialize() shape, so the
		// motion service deserializes it back via DeserializePlanState
		// without us pulling the whole armplanning package (and its
		// native nlopt dependency) into consumer modules.
		floats := make([]float64, len(startJoints))
		for i, j := range startJoints {
			floats[i] = float64(j)
		}
		extra["start_state"] = map[string]interface{}{
			"configuration": map[string]interface{}{
				armName: floats,
			},
		}
	}
	req.Extra = extra

	pbReq, err := req.ToProto(serviceName)
	if err != nil {
		return "", fmt.Errorf("build MoveRequest: %w", err)
	}
	jsonBytes, err := protojson.Marshal(pbReq)
	if err != nil {
		return "", fmt.Errorf("marshal MoveRequest: %w", err)
	}
	return string(jsonBytes), nil
}

// ParsePlanResponse extracts the result of a "plan" DoCommand
// response. Handles three cases:
//
//   - Clean success: the response carries a trajectory under one of
//     the known keys.
//   - Partial plan: the response has "plan_partialwp" naming the
//     waypoint where the planner stopped. Feasible is false; the
//     trajectory (up to the partial point) is still included.
//   - Success-without-trajectory: rare, can happen on degenerate
//     plans. Feasible is true; Trajectory is nil; Message
//     explains.
func ParsePlanResponse(resp map[string]interface{}) PlanResult {
	if wp, ok := resp["plan_partialwp"]; ok {
		return PlanResult{
			Feasible:   false,
			Message:    fmt.Sprintf("partial plan, stopped at waypoint %v", wp),
			Trajectory: extractTrajectory(resp),
		}
	}
	traj := extractTrajectory(resp)
	if traj == nil {
		return PlanResult{
			Feasible: true,
			Message:  "plan succeeded but no trajectory in response",
		}
	}
	return PlanResult{Feasible: true, Trajectory: traj}
}

// Plan is a convenience wrapper that marshals a request, sends it
// via DoCommand to the motion service, and parses the response.
// Returns (PlanResult, err) where err is non-nil only for
// transport-level failures (the DoCommand itself errored).
// Planner-level failures (no path, unreachable, etc.) come back as
// PlanResult.Feasible == false with the message in PlanResult.Message.
func Plan(
	ctx context.Context,
	svc motion.Service,
	req motion.MoveReq,
	serviceName string,
	armName string,
	startJoints []referenceframe.Input,
	timeoutSecs float64,
) (PlanResult, error) {
	payload, err := MarshalPlanRequest(req, serviceName, armName, startJoints, timeoutSecs)
	if err != nil {
		return PlanResult{}, err
	}
	resp, err := svc.DoCommand(ctx, map[string]interface{}{PlanCommand: payload})
	if err != nil {
		return PlanResult{Feasible: false, Message: err.Error()}, err
	}
	return ParsePlanResponse(resp), nil
}

// extractTrajectory probes a DoCommand("plan", ...) response for
// the trajectory payload. Key varies by motion-service wire path:
// in-process returns the trajectory under "plan" (motionplan.
// Trajectory); gRPC-serialized returns it under "Plan" or
// "trajectory" / "Trajectory" as a list of maps. Falls back to the
// first list-of-maps value if no canonical key matches.
func extractTrajectory(resp map[string]interface{}) interface{} {
	for _, k := range []string{"plan", "DoPlan", "trajectory", "Trajectory"} {
		if v, ok := resp[k]; ok {
			return v
		}
	}
	// Fallback: first list-of-maps value (handles "Plan" capital P
	// and other variations).
	for _, v := range resp {
		if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
			if _, ok := arr[0].(map[string]interface{}); ok {
				return v
			}
		}
	}
	return nil
}
