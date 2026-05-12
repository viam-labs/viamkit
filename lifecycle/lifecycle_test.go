package lifecycle

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewIsLive(t *testing.T) {
	l := New()
	defer l.Close()
	if err := l.Ctx().Err(); err != nil {
		t.Fatalf("fresh lifecycle Ctx should not be cancelled, got %v", err)
	}
}

func TestStopCancels(t *testing.T) {
	l := New()
	defer l.Close()
	ctx := l.Ctx()
	l.Stop()
	if err := ctx.Err(); err != context.Canceled {
		t.Errorf("after Stop, prior Ctx should be cancelled; got %v", err)
	}
	if err := l.Ctx().Err(); err != context.Canceled {
		t.Errorf("after Stop, current Ctx should be cancelled; got %v", err)
	}
}

func TestEnsureLiveRefreshesAfterStop(t *testing.T) {
	l := New()
	defer l.Close()
	l.Stop()
	if err := l.Ctx().Err(); err != context.Canceled {
		t.Fatalf("Stop should have cancelled Ctx")
	}
	fresh := l.EnsureLive()
	if err := fresh.Err(); err != nil {
		t.Errorf("EnsureLive should mint a fresh ctx; got Err()=%v", err)
	}
	if l.Ctx() != fresh {
		t.Errorf("Ctx() after EnsureLive should equal the refreshed ctx")
	}
}

func TestEnsureLiveIdempotent(t *testing.T) {
	l := New()
	defer l.Close()
	a := l.EnsureLive()
	b := l.EnsureLive()
	if a != b {
		t.Errorf("EnsureLive on a live ctx should return the same ctx, got two different")
	}
}

func TestStopIdempotent(t *testing.T) {
	l := New()
	defer l.Close()
	l.Stop()
	l.Stop() // should not panic
}

func TestCloseTerminates(t *testing.T) {
	l := New()
	l.Close()
	if err := l.Ctx().Err(); err != context.Canceled {
		t.Errorf("Close should cancel Ctx; got %v", err)
	}
	if !l.Closed() {
		t.Errorf("Closed() should be true after Close")
	}
	revived := l.EnsureLive()
	if err := revived.Err(); err != context.Canceled {
		t.Errorf("EnsureLive after Close must NOT resurrect; got %v", err)
	}
}

func TestCleanupCtxIndependent(t *testing.T) {
	l := New(WithCleanupTimeout(100 * time.Millisecond))
	defer l.Close()
	l.Stop()
	ctx, cancel := l.CleanupCtx()
	defer cancel()
	if err := ctx.Err(); err != nil {
		t.Errorf("CleanupCtx should be live even after Stop; got %v", err)
	}
	// And after Close.
	l.Close()
	ctx2, cancel2 := l.CleanupCtx()
	defer cancel2()
	if err := ctx2.Err(); err != nil {
		t.Errorf("CleanupCtx should still work after Close; got %v", err)
	}
}

func TestCleanupCtxTimesOut(t *testing.T) {
	l := New(WithCleanupTimeout(20 * time.Millisecond))
	defer l.Close()
	ctx, cancel := l.CleanupCtx()
	defer cancel()
	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded, got %v", ctx.Err())
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("CleanupCtx did not time out within 200ms")
	}
}

func TestCtxOrCleanupPicksLoopWhenLive(t *testing.T) {
	l := New()
	defer l.Close()
	loop := l.Ctx()
	got, cancel := l.CtxOrCleanup()
	defer cancel()
	if got != loop {
		t.Errorf("expected loop ctx when live, got a different ctx")
	}
}

func TestCtxOrCleanupPicksCleanupWhenStopped(t *testing.T) {
	l := New(WithCleanupTimeout(100 * time.Millisecond))
	defer l.Close()
	loop := l.Ctx()
	l.Stop()
	got, cancel := l.CtxOrCleanup()
	defer cancel()
	if got == loop {
		t.Errorf("expected fresh cleanup ctx when loop is cancelled, got loop ctx back")
	}
	if err := got.Err(); err != nil {
		t.Errorf("cleanup ctx should be live; got %v", err)
	}
}

func TestPackageLevelCleanupCtx(t *testing.T) {
	ctx, cancel := CleanupCtx(50 * time.Millisecond)
	defer cancel()
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		t.Errorf("package CleanupCtx should produce a context with a deadline")
	}
}

func TestConcurrentAccess(t *testing.T) {
	l := New()
	defer l.Close()
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	// Random mix of reads, stops, and refreshes — exercise the mutex.
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			switch i % 4 {
			case 0:
				_ = l.Ctx()
			case 1:
				l.Stop()
			case 2:
				_ = l.EnsureLive()
			case 3:
				ctx, cancel := l.CleanupCtx()
				cancel()
				_ = ctx
			}
		}(i)
	}
	wg.Wait()
}

func TestWithCleanupTimeoutZeroIgnored(t *testing.T) {
	// Passing zero should not override the default to 0.
	l := New(WithCleanupTimeout(0))
	defer l.Close()
	if l.cleanupTimeout != DefaultCleanupTimeout {
		t.Errorf("zero timeout should be ignored; got %v", l.cleanupTimeout)
	}
}
