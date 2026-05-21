package kinematics

import (
	"math"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
)

// SignConvention captures the relationship between a joint rotation
// and the resulting world-frame rotation for the J0 (base) and J5
// (wrist) joints. Empirically UR-class arms (UR5e, UR7e) have
// `world_yaw ≈ -(J0 + J5) + C`, so a +1 rad world rotation requires
// -1 rad of joint rotation. Other arms may flip the sign.
//
// SignConventionURStyle is the default for UR-class arms; pass
// SignConventionFlipped for arms where +world ≈ +joint.
type SignConvention int

const (
	// SignConventionURStyle is the default for UR-class arms, where
	// world_yaw = -(J0 + J5) + C.
	SignConventionURStyle SignConvention = -1

	// SignConventionFlipped is for arms where the joint-to-world
	// rotation isn't inverted, i.e. world_yaw = +(J0 + J5) + C.
	SignConventionFlipped SignConvention = 1
)

// PreRotatedJoints returns a joint configuration that pre-rotates J0
// (base swing) toward a target world-XY direction and J5 (wrist)
// toward a target world yaw. Joints 1..4 stay at currentJoints —
// only the base and wrist move.
//
// Why: long task-space moves under an orientation constraint
// (e.g. a wide base swing with the gripper locked downward) frequently
// blow the cartesian planner's budget because the planner has to
// find a path that simultaneously satisfies the constraint AND the
// joint kinematics. Pre-rotating J0+J5 in joint space first puts
// the arm in a configuration that's already close to the target;
// the subsequent cartesian plan is then short and feasible.
//
// The function is pure — it computes the joint values without
// invoking the planner or moving the arm. Caller is responsible for
// actually executing the resulting joints (typically via
// arm.MoveToJointPositions).
//
// Parameters:
//
//   - currentJoints: the arm's current joint configuration. Length
//     must be ≥ 6 (we only touch indices 0 and 5).
//   - currentEEXY: the end-effector's current world XY (from
//     arm.EndPosition or motion.GetPose — caller decides the source).
//   - currentEEYawRad: the gripper's current world yaw in radians
//     (from YawFromOrientation on the same EE pose).
//   - targetXY: the world XY the base should swing toward. The arm
//     reaches *toward* this point; J0 rotates to make atan2(targetY,
//     targetX) the new gripper world angle.
//   - targetYawRad: the world yaw the wrist should end up at, in
//     radians.
//   - sign: the joint-to-world sign convention for this arm — see
//     SignConvention constants. Use SignConventionURStyle for UR-class
//     arms.
//
// Behavior:
//   - The world-angle delta is wrapped to [-π, π] so the base takes
//     the shortest path.
//   - J0 is wrapped to ±2π if the new value exceeds the joint's
//     standard ±2π range. Caller is responsible for any tighter
//     limits.
//   - J5 derives from the J0 delta + the requested yaw delta per
//     the yaw-consistency rule (gripper_world_yaw ≈ sign·(J0 + J5)).
//
// Returns a new []referenceframe.Input — currentJoints is not mutated.
func PreRotatedJoints(
	currentJoints []referenceframe.Input,
	currentEEXY r3.Vector,
	currentEEYawRad float64,
	targetXY r3.Vector,
	targetYawRad float64,
	sign SignConvention,
) []referenceframe.Input {
	out := make([]referenceframe.Input, len(currentJoints))
	copy(out, currentJoints)
	if len(out) < 6 {
		return out
	}

	currentWorldAngle := math.Atan2(currentEEXY.Y, currentEEXY.X)
	targetWorldAngle := math.Atan2(targetXY.Y, targetXY.X)
	deltaWorld := wrapToShortestPath(targetWorldAngle - currentWorldAngle)

	// J0 rotates by -delta (UR convention) or +delta (flipped).
	deltaJ0 := float64(sign) * deltaWorld
	newJ0 := wrapJointRotation(float64(currentJoints[0]) + deltaJ0)

	// J5: derive from the yaw-consistency rule. If
	// world_yaw = sign·(J0 + J5) + C, then a target world yaw of Y
	// requires J5 such that sign·(J0 + J5) = Y - C. Since the
	// constant C cancels when we work in deltas:
	//   target_yaw - current_yaw = sign·(deltaJ0 + deltaJ5)
	//   deltaJ5 = sign·(target_yaw - current_yaw) - deltaJ0
	deltaYaw := wrapToShortestPath(targetYawRad - currentEEYawRad)
	deltaJ5 := float64(sign)*deltaYaw - deltaJ0
	newJ5 := wrapJointRotation(float64(currentJoints[5]) + deltaJ5)

	out[0] = referenceframe.Input(newJ0)
	out[5] = referenceframe.Input(newJ5)
	return out
}

// AlignStartJointsToPlaceYaw takes a saved joint configuration (e.g.
// from an arm-position-saver switch's recorded joints) and rotates
// J5 so the gripper's world yaw matches targetYawRad. Holds all
// other joints constant — only the wrist moves.
//
// The common use case: a switch records the arm's joints at a
// hand-jogged pose. When the runtime later replays those joints, the
// gripper's yaw at the saved pose may not match the yaw the current
// task needs. This helper composes a J5 correction so the replay
// lands at the right yaw.
//
// Like PreRotatedJoints, this is pure — caller executes the result
// via arm.MoveToJointPositions.
//
// Parameters:
//
//   - savedJoints: the recorded joint configuration. Length must be
//     ≥ 6 (we only touch index 5).
//   - savedYawRad: the gripper's world yaw when the arm is at
//     savedJoints, in radians. Caller computes via FK
//     (motion.GetPose on the saved-joints arm pose → YawFromOrientation).
//   - targetYawRad: the world yaw the wrist should end up at.
//   - sign: joint-to-world sign convention — see SignConvention
//     constants.
//
// Returns a new []referenceframe.Input — savedJoints is not mutated.
func AlignStartJointsToPlaceYaw(
	savedJoints []referenceframe.Input,
	savedYawRad float64,
	targetYawRad float64,
	sign SignConvention,
) []referenceframe.Input {
	out := make([]referenceframe.Input, len(savedJoints))
	copy(out, savedJoints)
	if len(out) < 6 {
		return out
	}
	deltaYaw := wrapToShortestPath(targetYawRad - savedYawRad)
	deltaJ5 := float64(sign) * deltaYaw
	out[5] = referenceframe.Input(wrapJointRotation(float64(savedJoints[5]) + deltaJ5))
	return out
}

// wrapToShortestPath wraps an angle delta into [-π, π] so a base
// rotation takes the shortest path rather than the long way around.
func wrapToShortestPath(d float64) float64 {
	for d > math.Pi {
		d -= 2 * math.Pi
	}
	for d < -math.Pi {
		d += 2 * math.Pi
	}
	return d
}

// wrapJointRotation wraps a joint value into [-2π, 2π], the typical
// range for UR-class continuous-rotation joints. Helps avoid joint-
// limit failures when a cumulative rotation pushes past one revolution.
func wrapJointRotation(v float64) float64 {
	twoPi := 2 * math.Pi
	for v > twoPi {
		v -= twoPi
	}
	for v < -twoPi {
		v += twoPi
	}
	return v
}
