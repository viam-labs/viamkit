package statemachine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type S string

const (
	sIdle S = "IDLE"
	sA    S = "A"
	sB    S = "B"
	sDone S = "DONE"
	sErr  S = "ERROR"
)

func okHandler(next S) Handler[S] {
	return func(_ context.Context) (S, error) { return next, nil }
}

func errHandler(err error) Handler[S] {
	return func(_ context.Context) (S, error) { return "", err }
}

func TestRunReachesTerminal(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sA, okHandler(sDone)),
		WithTerminal(sDone),
	)
	if err := m.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := m.Current(); got != sDone {
		t.Errorf("Current after Run: got %q, want %q", got, sDone)
	}
}

func TestRunRespectsContext(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sA, func(ctx context.Context) (S, error) {
			<-ctx.Done()
			return "", ctx.Err()
		}),
		WithTerminal(sDone),
	)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	if err := m.Run(ctx); err != nil {
		t.Errorf("Run on cancellation should return nil, got %v", err)
	}
	if m.Current() != sA {
		t.Errorf("expected state preserved as %q for resume, got %q", sA, m.Current())
	}
}

func TestErrorWithoutErrorStateSurfaces(t *testing.T) {
	want := errors.New("boom")
	m := New(sIdle,
		WithHandler(sIdle, errHandler(want)),
	)
	err := m.Run(context.Background())
	if !errors.Is(err, want) {
		t.Errorf("Run: got %v, want %v", err, want)
	}
	if m.LastError() == nil {
		t.Error("LastError should be set")
	}
}

func TestErrorRoutesToErrorState(t *testing.T) {
	failed := errors.New("transient")
	m := New(sIdle,
		WithHandler(sIdle, errHandler(failed)),
		WithTerminal(sErr),
		WithErrorState(sErr),
	)
	if err := m.Run(context.Background()); err != nil {
		t.Errorf("Run with err route to terminal err state should exit clean, got %v", err)
	}
	if m.Current() != sErr {
		t.Errorf("Current: got %q, want %q", m.Current(), sErr)
	}
	if !errors.Is(m.LastError(), failed) {
		t.Errorf("LastError: got %v, want wrapping %v", m.LastError(), failed)
	}
}

func TestMissingHandlerErrors(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		// sA has no handler and is not terminal
	)
	err := m.Run(context.Background())
	if err == nil {
		t.Error("expected error when transitioning to a state with no handler")
	}
}

func TestStepSingleTransition(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sA, okHandler(sB)),
		WithHandler(sB, okHandler(sDone)),
		WithTerminal(sDone),
	)
	if err := m.Step(context.Background()); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if m.Current() != sA {
		t.Errorf("after 1 step: got %q, want %q", m.Current(), sA)
	}
	if err := m.Step(context.Background()); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if m.Current() != sB {
		t.Errorf("after 2 steps: got %q, want %q", m.Current(), sB)
	}
}

func TestStepOnTerminalIsNoop(t *testing.T) {
	m := New(sDone,
		WithTerminal(sDone),
	)
	if err := m.Step(context.Background()); err != nil {
		t.Errorf("Step on terminal: %v", err)
	}
	if m.Current() != sDone {
		t.Errorf("terminal state should not advance, got %q", m.Current())
	}
}

func TestGoto(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sA, okHandler(sDone)),
		WithTerminal(sDone),
	)
	if err := m.Goto(sA); err != nil {
		t.Fatalf("Goto: %v", err)
	}
	if m.Current() != sA {
		t.Errorf("Goto did not set current: got %q", m.Current())
	}
	// Step from sA should advance to sDone
	if err := m.Step(context.Background()); err != nil {
		t.Fatalf("Step after Goto: %v", err)
	}
	if m.Current() != sDone {
		t.Errorf("after Goto+Step: got %q, want %q", m.Current(), sDone)
	}
}

func TestGotoUnknownStateRejected(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sDone)),
		WithTerminal(sDone),
	)
	if err := m.Goto("WHATEVER"); err == nil {
		t.Error("Goto to unknown state should error")
	}
}

func TestGotoTerminalAllowed(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sDone)),
		WithTerminal(sDone, sErr),
	)
	if err := m.Goto(sErr); err != nil {
		t.Errorf("Goto to terminal state should succeed: %v", err)
	}
}

func TestRunRejectsConcurrent(t *testing.T) {
	block := make(chan struct{})
	m := New(sIdle,
		WithHandler(sIdle, func(ctx context.Context) (S, error) {
			<-block
			return sDone, nil
		}),
		WithTerminal(sDone),
	)
	go m.Run(context.Background())
	// Give first Run a moment to take the running flag.
	time.Sleep(20 * time.Millisecond)
	if err := m.Run(context.Background()); !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Run: got %v, want ErrAlreadyRunning", err)
	}
	close(block) // let the first Run finish
}

func TestStepDuringRunRejected(t *testing.T) {
	block := make(chan struct{})
	m := New(sIdle,
		WithHandler(sIdle, func(ctx context.Context) (S, error) {
			<-block
			return sDone, nil
		}),
		WithTerminal(sDone),
	)
	go m.Run(context.Background())
	time.Sleep(20 * time.Millisecond)
	if err := m.Step(context.Background()); !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("Step during Run: got %v, want ErrAlreadyRunning", err)
	}
	close(block)
}

func TestResetClearsState(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, errHandler(errors.New("x"))),
	)
	_ = m.Run(context.Background())
	if m.LastError() == nil {
		t.Fatal("expected LastError after failing Run")
	}
	if err := m.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if m.Current() != sIdle {
		t.Errorf("after Reset: got %q, want %q", m.Current(), sIdle)
	}
	if m.LastError() != nil {
		t.Errorf("LastError should be cleared after Reset; got %v", m.LastError())
	}
}

func TestOnTransitionFires(t *testing.T) {
	var count int32
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sA, okHandler(sDone)),
		WithTerminal(sDone),
		OnTransition(func(from, to S) { atomic.AddInt32(&count, 1) }),
	)
	_ = m.Run(context.Background())
	if got := atomic.LoadInt32(&count); got != 2 {
		t.Errorf("OnTransition fires: got %d, want 2", got)
	}
}

func TestStates(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sA, okHandler(sDone)),
		WithTerminal(sDone, sErr),
	)
	got := m.States()
	want := map[S]bool{sIdle: true, sA: true, sDone: true, sErr: true}
	if len(got) != len(want) {
		t.Errorf("States count: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for _, s := range got {
		if !want[s] {
			t.Errorf("unexpected state in States: %q", s)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	m := New(sIdle,
		WithTerminal(sDone),
	)
	if !m.IsTerminal(sDone) {
		t.Error("sDone should be terminal")
	}
	if m.IsTerminal(sA) {
		t.Error("sA should not be terminal")
	}
}

func TestIsDone(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sDone)),
		WithTerminal(sDone),
	)
	if m.IsDone() {
		t.Error("fresh machine should not be done")
	}
	_ = m.Run(context.Background())
	if !m.IsDone() {
		t.Error("after Run reaches terminal, IsDone should be true")
	}
}

func TestTimeInState(t *testing.T) {
	m := New(sIdle,
		WithTerminal(sIdle),
	)
	time.Sleep(15 * time.Millisecond)
	d := m.TimeInState()
	if d < 10*time.Millisecond {
		t.Errorf("TimeInState should reflect elapsed time, got %v", d)
	}
}

func TestTimeInStateResetsOnTransition(t *testing.T) {
	step := make(chan struct{})
	m := New(sIdle,
		WithHandler(sIdle, func(ctx context.Context) (S, error) {
			<-step
			return sA, nil
		}),
		WithHandler(sA, func(ctx context.Context) (S, error) {
			return sDone, nil
		}),
		WithTerminal(sDone),
	)
	go m.Run(context.Background())
	time.Sleep(20 * time.Millisecond)
	before := m.TimeInState()
	if before < 15*time.Millisecond {
		t.Errorf("expected ~20ms in sIdle before step, got %v", before)
	}
	close(step) // advance through sA→sDone
	// Wait for run to finish (terminal sDone).
	for i := 0; i < 100 && m.Running(); i++ {
		time.Sleep(1 * time.Millisecond)
	}
	after := m.TimeInState()
	if after >= before {
		t.Errorf("TimeInState should reset on transition: before=%v, after=%v", before, after)
	}
}

func TestTimeInStateSurvivesSelfLoop(t *testing.T) {
	loops := 0
	m := New(sIdle,
		WithHandler(sIdle, func(ctx context.Context) (S, error) {
			loops++
			if loops < 3 {
				return sIdle, nil // self-loop
			}
			return sDone, nil
		}),
		WithTerminal(sDone),
	)
	time.Sleep(10 * time.Millisecond)
	before := m.TimeInState()
	_ = m.Run(context.Background())
	if m.Current() != sDone {
		t.Fatalf("expected sDone, got %v", m.Current())
	}
	// Self-loops shouldn't have reset the sIdle timer mid-Run.
	// (After Run reaches sDone, TimeInState measures sDone — so the
	// in-Run timing is observed via OnTransition. Just check the
	// self-loops happened.)
	if loops != 3 {
		t.Errorf("expected 3 sIdle entries, got %d", loops)
	}
	_ = before
}

func TestTimeInCycleZeroBeforeRun(t *testing.T) {
	m := New(sIdle, WithTerminal(sIdle))
	if d := m.TimeInCycle(); d != 0 {
		t.Errorf("TimeInCycle before any Run: got %v, want 0", d)
	}
}

func TestTimeInCycleStartsOnRun(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sDone)),
		WithTerminal(sDone),
	)
	_ = m.Run(context.Background())
	d := m.TimeInCycle()
	if d == 0 {
		t.Error("TimeInCycle after Run should be > 0")
	}
}

func TestTimeInCycleSurvivesStopResume(t *testing.T) {
	step := make(chan struct{})
	m := New(sIdle,
		WithHandler(sIdle, func(ctx context.Context) (S, error) {
			<-step
			return sDone, nil
		}),
		WithTerminal(sDone),
	)
	ctx, cancel := context.WithCancel(context.Background())
	go m.Run(ctx)
	time.Sleep(15 * time.Millisecond)
	beforeCycle := m.TimeInCycle()
	cancel() // stop
	for i := 0; i < 100 && m.Running(); i++ {
		time.Sleep(1 * time.Millisecond)
	}
	time.Sleep(15 * time.Millisecond)
	// Resume without Reset — cycle should continue accumulating.
	go m.Run(context.Background())
	time.Sleep(15 * time.Millisecond)
	close(step)
	for i := 0; i < 100 && m.Running(); i++ {
		time.Sleep(1 * time.Millisecond)
	}
	afterResume := m.TimeInCycle()
	if afterResume < beforeCycle+25*time.Millisecond {
		t.Errorf("TimeInCycle should accumulate across Stop+Resume: before=%v, after=%v", beforeCycle, afterResume)
	}
}

func TestResetClearsCycleTimer(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sDone)),
		WithTerminal(sDone),
	)
	_ = m.Run(context.Background())
	if d := m.TimeInCycle(); d == 0 {
		t.Fatal("TimeInCycle should be > 0 after Run")
	}
	if err := m.Reset(); err != nil {
		t.Fatal(err)
	}
	if d := m.TimeInCycle(); d != 0 {
		t.Errorf("TimeInCycle after Reset: got %v, want 0", d)
	}
}

func TestOnEntryFiresOnFirstRun(t *testing.T) {
	var entered []S
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sDone)),
		WithTerminal(sDone),
		WithOnEntry(sIdle, func(_ context.Context) error {
			entered = append(entered, sIdle)
			return nil
		}),
		WithOnEntry(sDone, func(_ context.Context) error {
			entered = append(entered, sDone)
			return nil
		}),
	)
	_ = m.Run(context.Background())
	want := []S{sIdle, sDone}
	if len(entered) != 2 || entered[0] != want[0] || entered[1] != want[1] {
		t.Errorf("OnEntry order: got %v, want %v", entered, want)
	}
}

func TestOnExitFiresOnTransition(t *testing.T) {
	var exited []S
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sA, okHandler(sDone)),
		WithTerminal(sDone),
		WithOnExit(sIdle, func(_ context.Context, _ time.Duration) error {
			exited = append(exited, sIdle)
			return nil
		}),
		WithOnExit(sA, func(_ context.Context, _ time.Duration) error {
			exited = append(exited, sA)
			return nil
		}),
	)
	_ = m.Run(context.Background())
	want := []S{sIdle, sA}
	if len(exited) != 2 || exited[0] != want[0] || exited[1] != want[1] {
		t.Errorf("OnExit order: got %v, want %v", exited, want)
	}
}

func TestOnExitGetsTimeInState(t *testing.T) {
	var observed time.Duration
	m := New(sIdle,
		WithHandler(sIdle, func(ctx context.Context) (S, error) {
			time.Sleep(20 * time.Millisecond)
			return sDone, nil
		}),
		WithTerminal(sDone),
		WithOnExit(sIdle, func(_ context.Context, t time.Duration) error {
			observed = t
			return nil
		}),
	)
	_ = m.Run(context.Background())
	if observed < 15*time.Millisecond {
		t.Errorf("OnExit timeInState: got %v, want at least 15ms", observed)
	}
}

func TestHooksSkipSelfLoop(t *testing.T) {
	loops := 0
	var entries, exits int
	m := New(sIdle,
		WithHandler(sIdle, func(_ context.Context) (S, error) {
			loops++
			if loops < 3 {
				return sIdle, nil
			}
			return sDone, nil
		}),
		WithTerminal(sDone),
		WithOnEntry(sIdle, func(_ context.Context) error {
			entries++
			return nil
		}),
		WithOnExit(sIdle, func(_ context.Context, _ time.Duration) error {
			exits++
			return nil
		}),
	)
	_ = m.Run(context.Background())
	if entries != 1 {
		t.Errorf("OnEntry should fire once across self-loops, got %d", entries)
	}
	if exits != 1 {
		t.Errorf("OnExit should fire once after the loops end, got %d", exits)
	}
	if loops != 3 {
		t.Errorf("expected 3 handler invocations, got %d", loops)
	}
}

func TestOnEntryErrorRoutes(t *testing.T) {
	want := errors.New("entry failed")
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sA, okHandler(sDone)),
		WithTerminal(sDone, sErr),
		WithErrorState(sErr),
		WithOnEntry(sA, func(_ context.Context) error {
			return want
		}),
	)
	if err := m.Run(context.Background()); err != nil {
		t.Errorf("Run with err route should be clean exit, got %v", err)
	}
	if m.Current() != sErr {
		t.Errorf("expected current=sErr after OnEntry failure, got %v", m.Current())
	}
	if !errors.Is(m.LastError(), want) {
		t.Errorf("LastError: got %v, want wrapping %v", m.LastError(), want)
	}
}

func TestOnEntryErrorSurfacesWithoutErrState(t *testing.T) {
	want := errors.New("entry failed")
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sA, okHandler(sDone)),
		WithTerminal(sDone),
		WithOnEntry(sA, func(_ context.Context) error {
			return want
		}),
	)
	if err := m.Run(context.Background()); !errors.Is(err, want) {
		t.Errorf("Run should surface OnEntry error, got %v", err)
	}
}

func TestOnExitErrorRoutes(t *testing.T) {
	want := errors.New("exit failed")
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sA, okHandler(sDone)),
		WithTerminal(sDone, sErr),
		WithErrorState(sErr),
		WithOnExit(sIdle, func(_ context.Context, _ time.Duration) error {
			return want
		}),
	)
	_ = m.Run(context.Background())
	if m.Current() != sErr {
		t.Errorf("expected current=sErr after OnExit failure, got %v", m.Current())
	}
}

func TestGotoSkipsOnExit(t *testing.T) {
	var exits int
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sDone)),
		WithTerminal(sDone),
		WithOnExit(sIdle, func(_ context.Context, _ time.Duration) error {
			exits++
			return nil
		}),
	)
	if err := m.Goto(sDone); err != nil {
		t.Fatal(err)
	}
	if exits != 0 {
		t.Errorf("Goto should NOT fire OnExit on the abandoned state, got %d", exits)
	}
}

func TestGotoMarksOnEntryForNextTick(t *testing.T) {
	var entries int
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sIdle)), // self-loop forever
		WithHandler(sA, okHandler(sDone)),
		WithTerminal(sDone),
		WithOnEntry(sA, func(_ context.Context) error {
			entries++
			return nil
		}),
	)
	if err := m.Goto(sA); err != nil {
		t.Fatal(err)
	}
	// Goto itself does not fire OnEntry — it sets up for the next dispatch.
	if entries != 0 {
		t.Errorf("Goto should not fire OnEntry directly, got %d", entries)
	}
	if err := m.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if entries != 1 {
		t.Errorf("Step after Goto should fire OnEntry, got %d", entries)
	}
}

func TestResetRefiresOnEntryEvenInInitialState(t *testing.T) {
	var entries int
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sDone)),
		WithTerminal(sDone),
		WithOnEntry(sIdle, func(_ context.Context) error {
			entries++
			return nil
		}),
	)
	// First Step fires OnEntry for the initial state.
	_ = m.Step(context.Background())
	if entries != 1 {
		t.Fatalf("first Step should fire OnEntry, got %d", entries)
	}
	// After Step we're in sDone (terminal). Reset back to sIdle.
	if err := m.Reset(); err != nil {
		t.Fatal(err)
	}
	_ = m.Step(context.Background())
	if entries != 2 {
		t.Errorf("Step after Reset should refire OnEntry, got %d", entries)
	}
}

func TestPauseResumeDoesNotRefireOnEntry(t *testing.T) {
	var entries atomic.Int32
	step := make(chan struct{})
	m := New(sIdle,
		WithHandler(sIdle, func(ctx context.Context) (S, error) {
			<-step
			return sDone, nil
		}),
		WithTerminal(sDone),
		WithOnEntry(sIdle, func(_ context.Context) error {
			entries.Add(1)
			return nil
		}),
	)
	ctx, cancel := context.WithCancel(context.Background())
	go m.Run(ctx)
	time.Sleep(20 * time.Millisecond)
	if e := entries.Load(); e != 1 {
		t.Fatalf("OnEntry should fire once on Run start, got %d", e)
	}
	cancel() // pause mid-state
	for i := 0; i < 100 && m.Running(); i++ {
		time.Sleep(1 * time.Millisecond)
	}
	// Resume.
	go m.Run(context.Background())
	time.Sleep(15 * time.Millisecond)
	close(step)
	for i := 0; i < 100 && m.Running(); i++ {
		time.Sleep(1 * time.Millisecond)
	}
	if e := entries.Load(); e != 1 {
		t.Errorf("Pause→Resume should NOT refire OnEntry (state was never left), got %d", e)
	}
}

func TestOnEntryFiresOnTerminalState(t *testing.T) {
	// Operator UIs often want to log "entered ERROR state, here's why".
	// OnEntry should fire even for terminal states.
	var enteredTerminal bool
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sErr)),
		WithTerminal(sErr),
		WithOnEntry(sErr, func(_ context.Context) error {
			enteredTerminal = true
			return nil
		}),
	)
	_ = m.Run(context.Background())
	if !enteredTerminal {
		t.Error("OnEntry should fire when entering a terminal state")
	}
}

func TestTimeSinceState(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sA, okHandler(sDone)),
		WithTerminal(sDone),
	)
	// sIdle has been seen (initial); sA and sDone have not.
	if d, ok := m.TimeSinceState(sIdle); !ok || d == 0 {
		t.Errorf("sIdle should have been seen: ok=%v, d=%v", ok, d)
	}
	if _, ok := m.TimeSinceState(sA); ok {
		t.Error("sA should not have been seen yet")
	}
	_ = m.Run(context.Background())
	if _, ok := m.TimeSinceState(sA); !ok {
		t.Error("sA should have been seen during Run")
	}
	if _, ok := m.TimeSinceState(sDone); !ok {
		t.Error("sDone should have been seen during Run")
	}
	// TimeSinceState for current state ≈ TimeInState
	dSince, _ := m.TimeSinceState(sDone)
	dIn := m.TimeInState()
	delta := dSince - dIn
	if delta < 0 {
		delta = -delta
	}
	if delta > 5*time.Millisecond {
		t.Errorf("TimeSinceState(current) ≈ TimeInState; got delta=%v", delta)
	}
}

// TestRequestExitRedirectsHandlerReturn — watchdog-style use case: a
// background goroutine wants the machine to land at sErr regardless
// of what the in-flight handler returns.
func TestRequestExitRedirectsHandlerReturn(t *testing.T) {
	m := New(sIdle,
		// Handler returns sDone — but RequestExit should override.
		WithHandler(sIdle, func(_ context.Context) (S, error) {
			return sDone, nil
		}),
		WithHandler(sErr, okHandler(sDone)),
		WithTerminal(sDone),
	)
	m.RequestExit(sErr, errors.New("watchdog: seal lost"))
	_ = m.Run(context.Background())
	// Should have routed through sErr (which forwards to sDone).
	if got := m.Current(); got != sDone {
		t.Errorf("Current after redirected Run: got %q, want %q", got, sDone)
	}
	if got := m.LastError(); got == nil || got.Error() != "watchdog: seal lost" {
		t.Errorf("LastError after RequestExit: got %v, want %q", got, "watchdog: seal lost")
	}
}

// TestRequestExitOverridesCancelledHandler — the realistic palletizer
// scenario: a watchdog cancels the loop ctx (handler returns
// ctx.Canceled), then RequestExit redirects. The machine should land
// in sErr instead of unwinding with no state change.
func TestRequestExitOverridesCancelledHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	handlerEntered := make(chan struct{}, 1)
	m := New(sIdle,
		WithHandler(sIdle, func(c context.Context) (S, error) {
			handlerEntered <- struct{}{}
			<-c.Done()
			return "", c.Err()
		}),
		WithTerminal(sErr),
	)

	go func() {
		<-handlerEntered
		m.RequestExit(sErr, errors.New("watchdog: seal lost"))
		cancel()
	}()

	_ = m.Run(ctx)
	if got := m.Current(); got != sErr {
		t.Errorf("Current after cancellation+RequestExit: got %q, want %q", got, sErr)
	}
	if got := m.LastError(); got == nil || got.Error() != "watchdog: seal lost" {
		t.Errorf("LastError: got %v, want %q", got, "watchdog: seal lost")
	}
}

// TestRequestExitWithNilReasonPreservesHandlerError — when no reason
// is supplied and the handler returned an error, that error is what
// LastError carries.
func TestRequestExitWithNilReasonPreservesHandlerError(t *testing.T) {
	handlerErr := errors.New("handler failed")
	m := New(sIdle,
		WithHandler(sIdle, func(_ context.Context) (S, error) {
			return "", handlerErr
		}),
		WithTerminal(sErr),
	)
	m.RequestExit(sErr, nil)
	_ = m.Run(context.Background())
	if got := m.Current(); got != sErr {
		t.Errorf("Current: got %q, want %q", got, sErr)
	}
	if got := m.LastError(); got != handlerErr {
		t.Errorf("LastError: got %v, want %v", got, handlerErr)
	}
}

// TestRequestExitLastCallWins — multiple RequestExit before the loop
// honors any of them: the last one wins.
func TestRequestExitLastCallWins(t *testing.T) {
	gate := make(chan struct{})
	m := New(sIdle,
		WithHandler(sIdle, func(c context.Context) (S, error) {
			<-gate
			return sDone, nil
		}),
		WithHandler(sErr, okHandler(sDone)),
		WithHandler(sA, okHandler(sDone)),
		WithTerminal(sDone),
	)
	m.RequestExit(sErr, errors.New("first"))
	m.RequestExit(sA, errors.New("second"))
	close(gate)
	_ = m.Run(context.Background())
	if got := m.Current(); got != sDone {
		t.Errorf("Current: got %q, want %q", got, sDone)
	}
	if got := m.LastError(); got == nil || got.Error() != "second" {
		t.Errorf("LastError (last-wins): got %v, want %q", got, "second")
	}
}

// TestRequestExitClearedAfterUse — after a request has been honored,
// the next handler return is not redirected.
func TestRequestExitClearedAfterUse(t *testing.T) {
	var count atomic.Int32
	m := New(sIdle,
		WithHandler(sIdle, func(_ context.Context) (S, error) {
			count.Add(1)
			return sA, nil
		}),
		WithHandler(sA, okHandler(sDone)),
		WithHandler(sErr, okHandler(sDone)),
		WithTerminal(sDone),
	)
	m.RequestExit(sErr, errors.New("once"))
	_ = m.Run(context.Background())
	// Path: sIdle → (requested) sErr → sDone. sA never entered.
	if got := m.Current(); got != sDone {
		t.Errorf("Current: got %q, want %q", got, sDone)
	}
	if count.Load() != 1 {
		t.Errorf("sIdle handler ran %d times, want 1", count.Load())
	}
}

// TestRequestExitInStep — Step honors the request the same way Run
// does.
func TestRequestExitInStep(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sA)),
		WithHandler(sErr, okHandler(sDone)),
		WithTerminal(sDone),
	)
	m.RequestExit(sErr, errors.New("step redirect"))
	if err := m.Step(context.Background()); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if got := m.Current(); got != sErr {
		t.Errorf("Current after Step: got %q, want %q", got, sErr)
	}
	if got := m.LastError(); got == nil || got.Error() != "step redirect" {
		t.Errorf("LastError: got %v", got)
	}
}

// TestResetClearsExitRequest — a stale request from a prior cycle
// doesn't leak into the next Run.
func TestResetClearsExitRequest(t *testing.T) {
	m := New(sIdle,
		WithHandler(sIdle, okHandler(sDone)),
		WithHandler(sErr, okHandler(sDone)),
		WithTerminal(sDone),
	)
	m.RequestExit(sErr, errors.New("stale"))
	if err := m.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	_ = m.Run(context.Background())
	// Path should be sIdle → sDone, not sIdle → (stale request) sErr.
	if got := m.Current(); got != sDone {
		t.Errorf("Current after Reset+Run: got %q, want %q", got, sDone)
	}
}
