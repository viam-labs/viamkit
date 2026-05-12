package fakes

import (
	"context"
	"sync"
	"sync/atomic"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Arm is an in-process fake of arm.Arm for unit tests. Methods
// default to safe no-ops: motion calls succeed without doing
// anything, JointPositions returns whatever was set via
// SetJointPositions (default: six zero joints), EndPosition returns
// SetEndPosition (default: origin pose).
//
// Every motion-side method records its arguments so tests can assert
// "MoveToJointPositions was called with these values."
type Arm struct {
	name resource.Name

	// Behavior overrides. Set per-test.
	EndPositionFn               func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error)
	JointPositionsFn            func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error)
	MoveToPositionFn            func(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error
	MoveToJointPositionsFn      func(ctx context.Context, positions []referenceframe.Input, extra map[string]interface{}) error
	MoveThroughJointPositionsFn func(ctx context.Context, positions [][]referenceframe.Input, opts *arm.MoveOptions, extra map[string]any) error
	StopFn                      func(ctx context.Context, extra map[string]interface{}) error
	KinematicsFn                func(ctx context.Context) (referenceframe.Model, error)

	mu             sync.Mutex
	jointPositions []referenceframe.Input
	endPosition    spatialmath.Pose

	endPositionCalls               atomic.Int32
	jointPositionsCalls            atomic.Int32
	moveToPositionCalls            atomic.Int32
	moveToJointPositionsCalls      atomic.Int32
	moveThroughJointPositionsCalls atomic.Int32
	stopCalls                      atomic.Int32

	// Last argument captures (under mu).
	lastMoveToPosition       spatialmath.Pose
	lastMoveToJointPositions []referenceframe.Input
}

// NewArm constructs an Arm fake with the given resource name.
// Defaults: six zero joints, origin pose.
func NewArm(name string) *Arm {
	return &Arm{
		name:           arm.Named(name),
		jointPositions: []referenceframe.Input{0, 0, 0, 0, 0, 0},
		endPosition:    spatialmath.NewZeroPose(),
	}
}

// SetJointPositions configures the value JointPositions returns when
// no JointPositionsFn override is set.
func (a *Arm) SetJointPositions(joints []referenceframe.Input) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.jointPositions = append([]referenceframe.Input{}, joints...)
}

// SetEndPosition configures the value EndPosition returns when no
// EndPositionFn override is set.
func (a *Arm) SetEndPosition(pose spatialmath.Pose) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.endPosition = pose
}

// LastMoveToPosition returns the pose passed to the most recent
// MoveToPosition call, or nil if there was none.
func (a *Arm) LastMoveToPosition() spatialmath.Pose {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastMoveToPosition
}

// LastMoveToJointPositions returns the joints passed to the most
// recent MoveToJointPositions call, or nil if there was none.
func (a *Arm) LastMoveToJointPositions() []referenceframe.Input {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastMoveToJointPositions == nil {
		return nil
	}
	out := make([]referenceframe.Input, len(a.lastMoveToJointPositions))
	copy(out, a.lastMoveToJointPositions)
	return out
}

// EndPositionCalls / JointPositionsCalls / etc. return invocation counts.
func (a *Arm) EndPositionCalls() int           { return int(a.endPositionCalls.Load()) }
func (a *Arm) JointPositionsCalls() int        { return int(a.jointPositionsCalls.Load()) }
func (a *Arm) MoveToPositionCalls() int        { return int(a.moveToPositionCalls.Load()) }
func (a *Arm) MoveToJointPositionsCalls() int  { return int(a.moveToJointPositionsCalls.Load()) }
func (a *Arm) MoveThroughJointPositionsCalls() int {
	return int(a.moveThroughJointPositionsCalls.Load())
}
func (a *Arm) StopCalls() int { return int(a.stopCalls.Load()) }

// Reset zeroes all call counters and clears captured arguments.
func (a *Arm) Reset() {
	a.endPositionCalls.Store(0)
	a.jointPositionsCalls.Store(0)
	a.moveToPositionCalls.Store(0)
	a.moveToJointPositionsCalls.Store(0)
	a.moveThroughJointPositionsCalls.Store(0)
	a.stopCalls.Store(0)
	a.mu.Lock()
	a.lastMoveToPosition = nil
	a.lastMoveToJointPositions = nil
	a.mu.Unlock()
}

// ---- resource.Resource ----

func (a *Arm) Name() resource.Name { return a.name }
func (a *Arm) Close(_ context.Context) error { return nil }
func (a *Arm) Reconfigure(_ context.Context, _ resource.Dependencies, _ resource.Config) error {
	return nil
}
func (a *Arm) DoCommand(_ context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	return nil, resource.ErrDoUnimplemented
}

// ---- resource.Shaped ----

func (a *Arm) Geometries(_ context.Context, _ map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, nil
}

// ---- resource.Actuator ----

func (a *Arm) IsMoving(_ context.Context) (bool, error) { return false, nil }
func (a *Arm) Stop(ctx context.Context, extra map[string]interface{}) error {
	a.stopCalls.Add(1)
	if a.StopFn != nil {
		return a.StopFn(ctx, extra)
	}
	return nil
}

// ---- framesystem.InputEnabled ----

func (a *Arm) Kinematics(ctx context.Context) (referenceframe.Model, error) {
	if a.KinematicsFn != nil {
		return a.KinematicsFn(ctx)
	}
	return nil, nil
}
func (a *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return a.JointPositions(ctx, nil)
}
func (a *Arm) GoToInputs(ctx context.Context, inputs ...[]referenceframe.Input) error {
	if len(inputs) == 0 {
		return nil
	}
	return a.MoveToJointPositions(ctx, inputs[len(inputs)-1], nil)
}

// ---- arm.Arm ----

func (a *Arm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	a.endPositionCalls.Add(1)
	if a.EndPositionFn != nil {
		return a.EndPositionFn(ctx, extra)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.endPosition, nil
}

func (a *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
	a.jointPositionsCalls.Add(1)
	if a.JointPositionsFn != nil {
		return a.JointPositionsFn(ctx, extra)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]referenceframe.Input, len(a.jointPositions))
	copy(out, a.jointPositions)
	return out, nil
}

func (a *Arm) MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error {
	a.moveToPositionCalls.Add(1)
	a.mu.Lock()
	a.lastMoveToPosition = pose
	a.mu.Unlock()
	if a.MoveToPositionFn != nil {
		return a.MoveToPositionFn(ctx, pose, extra)
	}
	return nil
}

func (a *Arm) MoveToJointPositions(ctx context.Context, positions []referenceframe.Input, extra map[string]interface{}) error {
	a.moveToJointPositionsCalls.Add(1)
	a.mu.Lock()
	a.lastMoveToJointPositions = append([]referenceframe.Input{}, positions...)
	// Successful move updates the "current" joints so subsequent
	// JointPositions reads reflect the new state, mirroring real arm
	// behavior.
	a.jointPositions = append([]referenceframe.Input{}, positions...)
	a.mu.Unlock()
	if a.MoveToJointPositionsFn != nil {
		return a.MoveToJointPositionsFn(ctx, positions, extra)
	}
	return nil
}

func (a *Arm) MoveThroughJointPositions(ctx context.Context, positions [][]referenceframe.Input, opts *arm.MoveOptions, extra map[string]any) error {
	a.moveThroughJointPositionsCalls.Add(1)
	if a.MoveThroughJointPositionsFn != nil {
		return a.MoveThroughJointPositionsFn(ctx, positions, opts, extra)
	}
	if len(positions) > 0 {
		return a.MoveToJointPositions(ctx, positions[len(positions)-1], nil)
	}
	return nil
}

func (a *Arm) Get3DModels(_ context.Context, _ map[string]interface{}) (map[string]*commonpb.Mesh, error) {
	return nil, nil
}
