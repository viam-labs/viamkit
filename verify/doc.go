// Package verify wraps the Viam motion service's "plan but don't
// execute" path: send a planning request via DoCommand, parse the
// response, get back (feasible bool, friendly message, trajectory).
//
// Why this isn't just motion.Service.Move:
//
// The builtin motion service exposes a DoCommand("plan", ...) verb
// that runs the planner without committing the arm to motion. The
// SDK doesn't surface this as a typed method, so callers hand-roll a
// motion.MoveReq, marshal it via protojson, send through DoCommand,
// and probe the response for the trajectory. This package
// encapsulates the protocol so a module just calls Plan(ctx, svc,
// req, ...) and gets a PlanResult.
//
// SDK quirks this package handles:
//
//   - The DoCommand key is "plan" (lowercase). Sending "DoPlan" hits
//     a no-op branch and returns an empty response.
//   - The response key carrying the trajectory varies: "plan" for
//     in-process motion service, "Plan" / "trajectory" / "Trajectory"
//     when the response went through gRPC.
//   - Partial plans are flagged via a "plan_partialwp" key naming
//     the waypoint where the planner stopped.
//   - The start_state JSON shape is the hand-rolled equivalent of
//     armplanning.PlanState.Serialize(), so consumers don't need to
//     import armplanning (which pulls in native nlopt).
//
// Typical usage. The four trailing arguments are easy to mix up, so
// they read most clearly one per line:
//
//	result, err := verify.Plan(ctx, motionSvc,
//	    motion.MoveReq{
//	        ComponentName: gripperName,
//	        Destination:   referenceframe.NewPoseInFrame("world", goalPose),
//	        WorldState:    ws,
//	        Constraints:   &motionplan.Constraints{ /* ... */ },
//	    },
//	    "builtin",   // name of the motion service resource
//	    armName,     // arm being planned for; startJoints describes its joints
//	    startJoints, // joint configuration the plan starts from
//	    15.0,        // planner timeout, in seconds
//	)
//	if err != nil {
//	    return err
//	}
//	if !result.Feasible {
//	    log.Warnw("can't reach goal", "reason", result.Message)
//	}
//	// result.Trajectory is motionplan.Trajectory or []interface{} —
//	// viamkit/kinematics has LastTrajectoryJoints / TrajectoryToJointPath
//	// helpers that handle both shapes.
package verify
