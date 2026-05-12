package verify

import (
	"context"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// DefaultEEPointDownsample is the per-trajectory cap on FK'd
// end-effector points returned by TrajectoryToEEPoses when its
// maxPoints argument is non-positive. Sized to keep operator-UI
// preview polylines smooth without bloating the response.
const DefaultEEPointDownsample = 60

// TrajectoryToEEPoses runs forward kinematics at each step of a
// planning trajectory and returns the world-frame end-effector
// poses. Use this when a client (operator UI, verify display) needs
// to render the trajectory as a 3D polyline without doing FK itself.
//
// traj accepts both shapes the motion service can return:
// motionplan.Trajectory (in-process) and []interface{} of
// map[string]interface{} (gRPC-serialized). armName is the arm's
// resource name (used to look up the joints in each step).
//
// maxPoints downsamples evenly when the trajectory is longer. The
// last point is always included so the resulting polyline reaches
// the goal. Pass 0 for the package default
// (DefaultEEPointDownsample = 60); pass a negative value to keep
// every step.
//
// Returns nil if traj is empty, the wrong shape, or the arm's
// Kinematics model errors.
func TrajectoryToEEPoses(
	ctx context.Context,
	traj interface{},
	armName string,
	a arm.Arm,
	maxPoints int,
) []spatialmath.Pose {
	steps := jointStepsFromTrajectory(traj, armName)
	if len(steps) == 0 {
		return nil
	}
	model, err := a.Kinematics(ctx)
	if err != nil {
		return nil
	}

	if maxPoints == 0 {
		maxPoints = DefaultEEPointDownsample
	}
	stride := 1
	if maxPoints > 0 && len(steps) > maxPoints {
		stride = (len(steps) + maxPoints - 1) / maxPoints
	}

	out := make([]spatialmath.Pose, 0, maxPoints+1)
	for i := 0; i < len(steps); i += stride {
		pose, err := model.Transform(steps[i])
		if err != nil {
			continue
		}
		out = append(out, pose)
	}
	// Always include the final step so the rendered polyline ends at
	// the goal, even if the stride didn't land there.
	if last := steps[len(steps)-1]; len(last) > 0 {
		if pose, err := model.Transform(last); err == nil {
			out = append(out, pose)
		}
	}
	return out
}

// jointStepsFromTrajectory unwraps the motion service's trajectory
// shape into a [][]referenceframe.Input. Handles both the typed
// motionplan.Trajectory and the gRPC []interface{} form. Returns
// nil if traj is empty or doesn't contain armName.
func jointStepsFromTrajectory(traj interface{}, armName string) [][]referenceframe.Input {
	var steps [][]referenceframe.Input
	if t, ok := traj.(motionplan.Trajectory); ok {
		for _, fsi := range t {
			if j, ok := fsi[armName]; ok {
				steps = append(steps, j)
			}
		}
		return steps
	}
	arr, ok := traj.([]interface{})
	if !ok {
		return nil
	}
	for _, stepIface := range arr {
		step, ok := stepIface.(map[string]interface{})
		if !ok {
			continue
		}
		rawJoints, ok := step[armName]
		if !ok {
			continue
		}
		values, ok := rawJoints.([]interface{})
		if !ok {
			continue
		}
		inputs := make([]referenceframe.Input, len(values))
		for i, v := range values {
			if f, ok := v.(float64); ok {
				inputs[i] = referenceframe.Input(f)
			}
		}
		steps = append(steps, inputs)
	}
	return steps
}
