package kinematics

import (
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

// LastTrajectoryJoints returns the final joint configuration of the
// named arm from a planning trajectory. The trajectory's wire shape
// varies depending on whether the motion service's DoPlan was called
// in-process (yields motionplan.Trajectory) or over gRPC (yields a
// structpb-serialized []interface{} of map[string]interface{}).
//
// Returns nil if traj is nil, empty, the wrong type, or doesn't
// contain the named arm.
//
// Typical usage: chain successive plans by feeding the previous
// plan's final joints as the next plan's start state, so the planner
// sees a consistent kinematic trajectory rather than two disjoint
// plans from the same starting configuration.
func LastTrajectoryJoints(traj interface{}, armName string) []referenceframe.Input {
	if traj == nil {
		return nil
	}
	// Typed Go path (in-process motion service).
	if t, ok := traj.(motionplan.Trajectory); ok {
		if len(t) == 0 {
			return nil
		}
		return t[len(t)-1][armName]
	}
	// structpb-serialized path (gRPC / proto roundtrip).
	arr, ok := traj.([]interface{})
	if !ok || len(arr) == 0 {
		return nil
	}
	lastStep, ok := arr[len(arr)-1].(map[string]interface{})
	if !ok {
		return nil
	}
	rawJoints, ok := lastStep[armName]
	if !ok {
		return nil
	}
	values, ok := rawJoints.([]interface{})
	if !ok {
		return nil
	}
	out := make([]referenceframe.Input, len(values))
	for i, v := range values {
		if f, ok := v.(float64); ok {
			out[i] = referenceframe.Input(f)
		}
	}
	return out
}

// TrajectoryToJointPath flattens a planning trajectory into a slice
// of joint vectors, one per planning step. Same wire-shape handling
// as LastTrajectoryJoints — accepts either motionplan.Trajectory or
// the gRPC-serialized []interface{} form. Returns nil if the
// trajectory is empty or doesn't include the named arm.
//
// Useful for animating a trajectory on the client side via forward
// kinematics, or for sampling joint configurations along the path
// for downstream collision checks.
func TrajectoryToJointPath(traj interface{}, armName string) [][]float64 {
	var steps [][]referenceframe.Input
	if t, ok := traj.(motionplan.Trajectory); ok {
		for _, fsi := range t {
			if j, ok := fsi[armName]; ok {
				steps = append(steps, j)
			}
		}
	} else if arr, ok := traj.([]interface{}); ok {
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
	}
	if len(steps) == 0 {
		return nil
	}
	out := make([][]float64, len(steps))
	for i, s := range steps {
		fs := make([]float64, len(s))
		for j, v := range s {
			fs[j] = float64(v)
		}
		out[i] = fs
	}
	return out
}

// DefaultInterpolateSamples is the sample count used by
// InterpolateJointPath when the caller passes nSamples <= 0. Chosen
// to give a visibly smooth animation in operator UIs without being
// expensive to FK-evaluate.
const DefaultInterpolateSamples = 32

// InterpolateJointPath linearly interpolates from start to end in
// nSamples+1 inclusive points. Returns nil if the two joint vectors
// don't have the same non-zero length.
//
// nSamples <= 0 falls back to DefaultInterpolateSamples. The
// returned path always includes both endpoints, so a request for
// nSamples=4 yields 5 points (start, three intermediate, end).
//
// Useful for animating a joint-space transit on the client side, or
// for sampling intermediate configurations to collision-check a
// joint-space move that the planner didn't return a trajectory for.
func InterpolateJointPath(start, end []referenceframe.Input, nSamples int) [][]float64 {
	if len(start) != len(end) || len(start) == 0 {
		return nil
	}
	if nSamples <= 0 {
		nSamples = DefaultInterpolateSamples
	}
	out := make([][]float64, nSamples+1)
	for i := 0; i <= nSamples; i++ {
		t := float64(i) / float64(nSamples)
		step := make([]float64, len(start))
		for j := range start {
			step[j] = float64(start[j]) + t*(float64(end[j])-float64(start[j]))
		}
		out[i] = step
	}
	return out
}
