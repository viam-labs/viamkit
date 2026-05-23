package fakes

import (
	"context"
	"sync"
	"sync/atomic"

	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/resource"
)

// Switch is an in-process fake of toggleswitch.Switch for unit
// tests. Commonly used to fake arm-position-saver style switches
// (erh:vmodutils:arm-position-saver) where SetPosition replays a
// saved joint vector and DoCommand({"cfg":true}) returns the saved
// joints.
type Switch struct {
	name resource.Name

	// Behavior overrides.
	SetPositionFn          func(ctx context.Context, position uint32, extra map[string]interface{}) error
	GetPositionFn          func(ctx context.Context, extra map[string]interface{}) (uint32, error)
	GetNumberOfPositionsFn func(ctx context.Context, extra map[string]interface{}) (uint32, []string, error)
	DoCommandFn            func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)

	mu       sync.Mutex
	position uint32
	// numPositions, labels: default to (2, nil) — a typical two-state switch.
	numPositions uint32
	labels       []string

	setPositionCalls atomic.Int32
	getPositionCalls atomic.Int32
	doCommandCalls   atomic.Int32

	lastSetPosition uint32
}

// NewSwitch constructs a Switch fake with the given name. Default
// state: 2 positions, current position 0.
func NewSwitch(name string) *Switch {
	return &Switch{
		name:         toggleswitch.Named(name),
		numPositions: 2,
	}
}

// SetNumberOfPositions configures GetNumberOfPositions's return.
func (s *Switch) SetNumberOfPositions(n uint32, labels []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.numPositions = n
	s.labels = labels
}

// LastSetPosition returns the most recent position passed to SetPosition.
func (s *Switch) LastSetPosition() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSetPosition
}

// Call counters.
func (s *Switch) SetPositionCalls() int { return int(s.setPositionCalls.Load()) }
func (s *Switch) GetPositionCalls() int { return int(s.getPositionCalls.Load()) }
func (s *Switch) DoCommandCalls() int   { return int(s.doCommandCalls.Load()) }

// Reset clears call counters and the last-set-position capture.
func (s *Switch) Reset() {
	s.setPositionCalls.Store(0)
	s.getPositionCalls.Store(0)
	s.doCommandCalls.Store(0)
	s.mu.Lock()
	s.lastSetPosition = 0
	s.mu.Unlock()
}

// ---- resource.Resource ----

func (s *Switch) Name() resource.Name           { return s.name }
func (s *Switch) Close(_ context.Context) error { return nil }
func (s *Switch) Reconfigure(_ context.Context, _ resource.Dependencies, _ resource.Config) error {
	return nil
}
func (s *Switch) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	s.doCommandCalls.Add(1)
	if s.DoCommandFn != nil {
		return s.DoCommandFn(ctx, cmd)
	}
	return nil, resource.ErrDoUnimplemented
}

// Status satisfies resource.Resource; returns an empty map.
func (s *Switch) Status(_ context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

// ---- toggleswitch.Switch ----

func (s *Switch) SetPosition(ctx context.Context, position uint32, extra map[string]interface{}) error {
	s.setPositionCalls.Add(1)
	s.mu.Lock()
	s.lastSetPosition = position
	s.position = position
	s.mu.Unlock()
	if s.SetPositionFn != nil {
		return s.SetPositionFn(ctx, position, extra)
	}
	return nil
}

func (s *Switch) GetPosition(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	s.getPositionCalls.Add(1)
	if s.GetPositionFn != nil {
		return s.GetPositionFn(ctx, extra)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.position, nil
}

func (s *Switch) GetNumberOfPositions(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
	if s.GetNumberOfPositionsFn != nil {
		return s.GetNumberOfPositionsFn(ctx, extra)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.numPositions, s.labels, nil
}
