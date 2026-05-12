// Package fakes provides in-process fake implementations of Viam SDK
// component and service interfaces, designed for Go unit tests.
//
// Each fake satisfies the corresponding SDK interface (e.g.
// gripper.Gripper) with sensible no-op defaults, plus overridable
// function fields (`GrabFn`, `OpenFn`, ...) for per-test behavior.
// Call counts are tracked atomically so tests can assert "Grab was
// called twice" without writing custom bookkeeping.
//
// How this differs from rdk:builtin:fake:
//
// The SDK's `rdk:builtin:fake` components are full Viam resources
// designed for "fake machine" integration tests — you wire them in
// a JSON config and bring up viam-server end-to-end. They're useful
// but they have fixed behavior (Move always succeeds, etc.), which
// makes them poor mocks for "test that doGrasp transitions to ERROR
// when IsHoldingSomething returns false three times in a row."
//
// viamkit/fakes is the other half: in-process, programmable, no
// viam-server needed. You construct one in a test, set its Fn fields
// to script the behavior you want, and pass it to your module code
// the same way the real wiring would.
//
// Typical use:
//
//	import "github.com/viam-labs/viamkit/fakes"
//
//	func TestDoGrasp(t *testing.T) {
//	    g := fakes.NewGripper("test-gripper")
//	    g.IsHoldingSomethingFn = func(_ context.Context, _ map[string]interface{}) (gripper.HoldingStatus, error) {
//	        return gripper.HoldingStatus{IsHoldingSomething: false}, nil
//	    }
//	    // ... construct module, call handler, assert state transition + g.OpenCalls() == 1
//	}
package fakes
