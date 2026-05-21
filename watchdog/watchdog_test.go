package watchdog

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

const tick = 5 * time.Millisecond

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", msg)
}

func TestHealthyKeepsPolling(t *testing.T) {
	var checks atomic.Int32
	wd := New(
		WithInterval(tick),
		WithCheck(func(_ context.Context) (Result, error) {
			checks.Add(1)
			return Healthy, nil
		}),
		WithOnFail(func(error) { t.Error("OnFail should not fire on healthy") }),
	)
	wd.Start(context.Background())
	waitFor(t, func() bool { return checks.Load() >= 3 }, "3 health checks")
	wd.Stop()
}

func TestLostFiresOnFailAndExits(t *testing.T) {
	want := errors.New("seal lost")
	var failed atomic.Int32
	wd := New(
		WithInterval(tick),
		WithCheck(func(_ context.Context) (Result, error) {
			return Lost, want
		}),
		WithOnFail(func(err error) {
			if !errors.Is(err, want) {
				t.Errorf("OnFail err: got %v, want %v", err, want)
			}
			failed.Add(1)
		}),
	)
	wd.Start(context.Background())
	waitFor(t, func() bool { return !wd.Running() }, "watchdog to exit")
	if failed.Load() != 1 {
		t.Errorf("OnFail fired %d times, want 1", failed.Load())
	}
}

func TestTransientLogsAndContinues(t *testing.T) {
	var checks atomic.Int32
	var transients atomic.Int32
	wd := New(
		WithInterval(tick),
		WithCheck(func(_ context.Context) (Result, error) {
			n := checks.Add(1)
			if n%2 == 0 {
				return Transient, errors.New("blip")
			}
			return Healthy, nil
		}),
		WithOnFail(func(error) { t.Error("OnFail should not fire on transient") }),
		WithOnTransient(func(_ error) { transients.Add(1) }),
	)
	wd.Start(context.Background())
	waitFor(t, func() bool { return checks.Load() >= 6 }, "6 checks")
	if transients.Load() < 2 {
		t.Errorf("expected at least 2 transients, got %d", transients.Load())
	}
	wd.Stop()
}

func TestShouldExitTerminatesCleanly(t *testing.T) {
	var checks atomic.Int32
	var failed atomic.Int32
	exited := make(chan struct{})
	var exitTriggered atomic.Bool

	wd := New(
		WithInterval(tick),
		WithCheck(func(_ context.Context) (Result, error) {
			checks.Add(1)
			return Healthy, nil
		}),
		WithShouldExit(exitTriggered.Load),
		WithOnFail(func(error) { failed.Add(1) }),
	)
	wd.Start(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		exitTriggered.Store(true)
		close(exited)
	}()
	<-exited
	waitFor(t, func() bool { return !wd.Running() }, "watchdog to exit cleanly")
	if failed.Load() != 0 {
		t.Errorf("ShouldExit should not fire OnFail; got %d", failed.Load())
	}
}

func TestParentCancelExits(t *testing.T) {
	wd := New(
		WithInterval(tick),
		WithCheck(func(_ context.Context) (Result, error) { return Healthy, nil }),
		WithOnFail(func(error) { t.Error("OnFail should not fire on parent cancel") }),
	)
	ctx, cancel := context.WithCancel(context.Background())
	wd.Start(ctx)
	cancel()
	waitFor(t, func() bool { return !wd.Running() }, "watchdog to exit on parent cancel")
}

func TestStopIsIdempotent(t *testing.T) {
	wd := New(
		WithInterval(tick),
		WithCheck(func(_ context.Context) (Result, error) { return Healthy, nil }),
		WithOnFail(func(error) {}),
	)
	wd.Stop() // before Start
	wd.Start(context.Background())
	wd.Stop()
	wd.Stop() // after Stop
}

func TestStartRestartsPriorGoroutine(t *testing.T) {
	var checks atomic.Int32
	wd := New(
		WithInterval(tick),
		WithCheck(func(_ context.Context) (Result, error) {
			checks.Add(1)
			return Healthy, nil
		}),
		WithOnFail(func(error) {}),
	)
	wd.Start(context.Background())
	waitFor(t, func() bool { return checks.Load() >= 2 }, "first goroutine running")
	prior := checks.Load()
	wd.Start(context.Background()) // restart
	waitFor(t, func() bool { return checks.Load() > prior+1 }, "second goroutine running")
	wd.Stop()
}

func TestNewWithNilCheckIsNoop(t *testing.T) {
	wd := New(WithInterval(tick))
	wd.Start(context.Background())
	if wd.Running() {
		t.Error("Start with no Check should be a no-op")
	}
}

func TestWithIntervalRejectsNonPositive(t *testing.T) {
	wd := New(WithInterval(0))
	if wd.interval != DefaultInterval {
		t.Errorf("zero interval should fall back to default; got %v", wd.interval)
	}
}
