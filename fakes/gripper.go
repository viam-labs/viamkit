package fakes

import (
	"context"
	"sync/atomic"

	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Gripper is an in-process fake of gripper.Gripper for unit tests.
// The Fn fields are nil by default — calls to the corresponding
// methods take a sensible default path (Grab returns true, Open is
// a no-op, IsHoldingSomething returns true, etc.). Set the Fn for
// any method whose behavior your test needs to control.
//
// All call counts are tracked atomically and safe to read from any
// goroutine. Reset() clears them between sub-tests.
type Gripper struct {
	name resource.Name

	// Behavior overrides. Set in tests to customize.
	GrabFn               func(ctx context.Context, extra map[string]interface{}) (bool, error)
	OpenFn               func(ctx context.Context, extra map[string]interface{}) error
	IsHoldingSomethingFn func(ctx context.Context, extra map[string]interface{}) (gripper.HoldingStatus, error)
	StopFn               func(ctx context.Context, extra map[string]interface{}) error
	DoCommandFn          func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)

	grabCalls               atomic.Int32
	openCalls               atomic.Int32
	isHoldingSomethingCalls atomic.Int32
	stopCalls               atomic.Int32
	doCommandCalls          atomic.Int32
}

// NewGripper constructs a Gripper fake with the given resource name.
func NewGripper(name string) *Gripper {
	return &Gripper{name: gripper.Named(name)}
}

// ---- resource.Resource ----

// Name returns the resource name.
func (g *Gripper) Name() resource.Name { return g.name }

// Close is a no-op for the fake.
func (g *Gripper) Close(_ context.Context) error { return nil }

// Reconfigure is a no-op for the fake.
func (g *Gripper) Reconfigure(_ context.Context, _ resource.Dependencies, _ resource.Config) error {
	return nil
}

// DoCommand defers to DoCommandFn if set, otherwise returns
// resource.ErrDoUnimplemented.
func (g *Gripper) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	g.doCommandCalls.Add(1)
	if g.DoCommandFn != nil {
		return g.DoCommandFn(ctx, cmd)
	}
	return nil, resource.ErrDoUnimplemented
}

// Status satisfies resource.Resource; returns an empty map.
func (g *Gripper) Status(_ context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

// ---- resource.Actuator ----

// IsMoving returns false unless your test overrides it.
func (g *Gripper) IsMoving(_ context.Context) (bool, error) { return false, nil }

// Stop defers to StopFn if set.
func (g *Gripper) Stop(ctx context.Context, extra map[string]interface{}) error {
	g.stopCalls.Add(1)
	if g.StopFn != nil {
		return g.StopFn(ctx, extra)
	}
	return nil
}

// ---- resource.Shaped ----

// Geometries returns nil; override if your test inspects gripper geometry.
func (g *Gripper) Geometries(_ context.Context, _ map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, nil
}

// ---- framesystem.InputEnabled ----

// Kinematics returns nil; gripper kinematics are usually a no-op.
func (g *Gripper) Kinematics(_ context.Context) (referenceframe.Model, error) {
	return nil, nil
}

// CurrentInputs returns nil; gripper has no joints.
func (g *Gripper) CurrentInputs(_ context.Context) ([]referenceframe.Input, error) {
	return nil, nil
}

// GoToInputs is a no-op for the fake.
func (g *Gripper) GoToInputs(_ context.Context, _ ...[]referenceframe.Input) error {
	return nil
}

// ---- gripper.Gripper ----

// Grab defers to GrabFn if set, otherwise returns (true, nil)
// (success: grabbed something).
func (g *Gripper) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	g.grabCalls.Add(1)
	if g.GrabFn != nil {
		return g.GrabFn(ctx, extra)
	}
	return true, nil
}

// Open defers to OpenFn if set, otherwise no-ops.
func (g *Gripper) Open(ctx context.Context, extra map[string]interface{}) error {
	g.openCalls.Add(1)
	if g.OpenFn != nil {
		return g.OpenFn(ctx, extra)
	}
	return nil
}

// IsHoldingSomething defers to IsHoldingSomethingFn if set, otherwise
// returns {IsHoldingSomething: true} — the optimistic default,
// matching what a healthy grasp would look like.
func (g *Gripper) IsHoldingSomething(ctx context.Context, extra map[string]interface{}) (gripper.HoldingStatus, error) {
	g.isHoldingSomethingCalls.Add(1)
	if g.IsHoldingSomethingFn != nil {
		return g.IsHoldingSomethingFn(ctx, extra)
	}
	return gripper.HoldingStatus{IsHoldingSomething: true}, nil
}

// ---- Call counters ----

// GrabCalls returns the total Grab invocations.
func (g *Gripper) GrabCalls() int { return int(g.grabCalls.Load()) }

// OpenCalls returns the total Open invocations.
func (g *Gripper) OpenCalls() int { return int(g.openCalls.Load()) }

// IsHoldingSomethingCalls returns the total IsHoldingSomething invocations.
func (g *Gripper) IsHoldingSomethingCalls() int { return int(g.isHoldingSomethingCalls.Load()) }

// StopCalls returns the total Stop invocations.
func (g *Gripper) StopCalls() int { return int(g.stopCalls.Load()) }

// DoCommandCalls returns the total DoCommand invocations.
func (g *Gripper) DoCommandCalls() int { return int(g.doCommandCalls.Load()) }

// Reset zeroes all call counters. Useful between sub-tests when
// reusing the same Gripper instance.
func (g *Gripper) Reset() {
	g.grabCalls.Store(0)
	g.openCalls.Store(0)
	g.isHoldingSomethingCalls.Store(0)
	g.stopCalls.Store(0)
	g.doCommandCalls.Store(0)
}
