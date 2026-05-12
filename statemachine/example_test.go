package statemachine_test

import (
	"context"
	"fmt"
	"time"

	"github.com/viam-labs/viamkit/statemachine"
)

// A tiny module showing the canonical pattern: typed state constants,
// handlers defined as methods, and registration as a single map
// literal — no repeated WithHandler() lines.
type pingPong struct {
	machine *statemachine.Machine[string]
	count   int
}

func newPingPong(limit int) *pingPong {
	pp := &pingPong{count: limit}
	pp.machine = statemachine.New("PING",
		statemachine.WithHandlers(map[string]statemachine.Handler[string]{
			"PING": pp.onPing,
			"PONG": pp.onPong,
		}),
		statemachine.WithTerminal("DONE"),
	)
	return pp
}

func (p *pingPong) onPing(_ context.Context) (string, error) {
	if p.count <= 0 {
		return "DONE", nil
	}
	p.count--
	return "PONG", nil
}

func (p *pingPong) onPong(_ context.Context) (string, error) {
	if p.count <= 0 {
		return "DONE", nil
	}
	p.count--
	return "PING", nil
}

// Example shows the canonical pattern: typed state constants, methods
// for handlers, a single WithHandlers map literal that reads as a
// state→handler table.
func Example() {
	pp := newPingPong(3)
	_ = pp.machine.Run(context.Background())
	fmt.Println("ended in:", pp.machine.Current())
	// Output:
	// ended in: DONE
}

// ExampleMachine_Step demonstrates step-through use, e.g. for a CLI
// or operator UI that wants to advance one state at a time.
func ExampleMachine_Step() {
	pp := newPingPong(2)
	for !pp.machine.IsTerminal(pp.machine.Current()) {
		_ = pp.machine.Step(context.Background())
	}
	fmt.Println("ended in:", pp.machine.Current())
	// Output:
	// ended in: DONE
}

// A canonical robotics-module shape: a cycle that can fail mid-state
// and needs to land in a recoverable ERROR state instead of bubbling
// the error up out of Run. The operator clears with Reset and resumes
// with another Run.
type cycleModule struct {
	machine   *statemachine.Machine[string]
	attempts  int
	failOnce  bool // set true to make the first GRASP fail
	cleanedUp bool
}

func newCycleModule() *cycleModule {
	cm := &cycleModule{}
	cm.machine = statemachine.New("PICKUP",
		statemachine.WithHandlers(map[string]statemachine.Handler[string]{
			"PICKUP": cm.doPickup,
			"GRASP":  cm.doGrasp,
			"PLACE":  cm.doPlace,
		}),
		statemachine.WithTerminal("DONE", "ERROR"),
		statemachine.WithErrorState("ERROR"),
	)
	return cm
}

func (cm *cycleModule) doPickup(_ context.Context) (string, error) {
	cm.attempts++
	return "GRASP", nil
}

func (cm *cycleModule) doGrasp(_ context.Context) (string, error) {
	if cm.failOnce {
		cm.failOnce = false
		return "", fmt.Errorf("suction failed")
	}
	return "PLACE", nil
}

func (cm *cycleModule) doPlace(_ context.Context) (string, error) {
	cm.cleanedUp = true
	return "DONE", nil
}

// ExampleWithErrorState shows the operator workflow: a failing handler
// routes the machine to ERROR (instead of propagating the error out of
// Run); the operator inspects LastError, resolves the underlying
// problem, calls Reset, and resumes with another Run.
func ExampleWithErrorState() {
	cm := newCycleModule()
	cm.failOnce = true

	_ = cm.machine.Run(context.Background())
	fmt.Println("first run ended in:", cm.machine.Current())
	fmt.Println("last error:", cm.machine.LastError())

	// Operator clears and resumes.
	_ = cm.machine.Reset()
	_ = cm.machine.Run(context.Background())
	fmt.Println("second run ended in:", cm.machine.Current())
	fmt.Println("cleanedUp:", cm.cleanedUp)
	// Output:
	// first run ended in: ERROR
	// last error: suction failed
	// second run ended in: DONE
	// cleanedUp: true
}

// A module demonstrating OnEntry / OnExit hooks for per-state setup
// and teardown. The hooks separate "what the state DOES" (in the
// handler) from "what it SETS UP and TEARS DOWN" (in the hooks).
type loggedCycle struct {
	machine *statemachine.Machine[string]
	log     []string
}

func newLoggedCycle() *loggedCycle {
	lc := &loggedCycle{}
	lc.machine = statemachine.New("IDLE",
		statemachine.WithHandlers(map[string]statemachine.Handler[string]{
			"IDLE": func(_ context.Context) (string, error) { return "WORKING", nil },
			"WORKING": func(_ context.Context) (string, error) {
				time.Sleep(5 * time.Millisecond)
				return "DONE", nil
			},
		}),
		statemachine.WithTerminal("DONE"),
		statemachine.WithOnEntry("WORKING", func(_ context.Context) error {
			lc.log = append(lc.log, "entered WORKING")
			return nil
		}),
		statemachine.WithOnExit("WORKING", func(_ context.Context, t time.Duration) error {
			// Don't print the exact duration (would break Output check);
			// just record that exit fired.
			if t > 0 {
				lc.log = append(lc.log, "exited WORKING")
			}
			return nil
		}),
	)
	return lc
}

// ExampleWithOnEntry shows the lifecycle hooks. Notice the hooks fire
// around the handler, not instead of it — the handler still owns the
// state's actual work and its next-state decision.
func ExampleWithOnEntry() {
	lc := newLoggedCycle()
	_ = lc.machine.Run(context.Background())
	for _, line := range lc.log {
		fmt.Println(line)
	}
	// Output:
	// entered WORKING
	// exited WORKING
}
