package cycle

import (
	"sync"
	"testing"
	"time"
)

func TestStartEndRecordsDuration(t *testing.T) {
	tr := New()
	tr.Start()
	time.Sleep(15 * time.Millisecond)
	tr.End()
	if tr.Count() != 1 {
		t.Errorf("Count: got %d, want 1", tr.Count())
	}
	s := tr.Stats()
	if s.Last < 10*time.Millisecond {
		t.Errorf("Last duration: got %v, want >= 10ms", s.Last)
	}
}

func TestEndWithoutStartIsNoop(t *testing.T) {
	tr := New()
	tr.End()
	if tr.Count() != 0 {
		t.Errorf("Count after stray End: got %d, want 0", tr.Count())
	}
}

func TestRunningReflectsState(t *testing.T) {
	tr := New()
	if tr.Running() {
		t.Error("fresh tracker should not be running")
	}
	tr.Start()
	if !tr.Running() {
		t.Error("after Start: should be running")
	}
	tr.End()
	if tr.Running() {
		t.Error("after End: should not be running")
	}
}

func TestElapsedTicks(t *testing.T) {
	tr := New()
	if tr.Elapsed() != 0 {
		t.Errorf("idle Elapsed: got %v, want 0", tr.Elapsed())
	}
	tr.Start()
	time.Sleep(10 * time.Millisecond)
	if e := tr.Elapsed(); e < 8*time.Millisecond {
		t.Errorf("running Elapsed: got %v, want >= 8ms", e)
	}
	tr.End()
	if tr.Elapsed() != 0 {
		t.Errorf("after End Elapsed: got %v, want 0", tr.Elapsed())
	}
}

func TestStartTwiceDiscardsPrior(t *testing.T) {
	tr := New()
	tr.Start()
	time.Sleep(20 * time.Millisecond)
	tr.Start() // discard the first
	time.Sleep(5 * time.Millisecond)
	tr.End()
	s := tr.Stats()
	if s.Last >= 18*time.Millisecond {
		t.Errorf("Start-twice should discard first; got Last=%v, want < 18ms", s.Last)
	}
}

func TestCancelPreservesNoData(t *testing.T) {
	tr := New()
	tr.Start()
	time.Sleep(10 * time.Millisecond)
	tr.Cancel()
	if tr.Count() != 0 {
		t.Errorf("Cancel should not record a cycle; Count=%d", tr.Count())
	}
	if tr.Running() {
		t.Error("Running should be false after Cancel")
	}
}

func TestStatsEmpty(t *testing.T) {
	tr := New()
	s := tr.Stats()
	if s.Count != 0 || s.WindowSize != 0 || s.Last != 0 {
		t.Errorf("empty Stats not zero: %+v", s)
	}
}

func TestRollingWindowEvictsOldest(t *testing.T) {
	tr := New(WithWindow(3))
	// Record 5 cycles with deterministic-ish durations via direct
	// time manipulation isn't easy; just use real sleeps and check
	// that only the last 3 are retained.
	for i := 0; i < 5; i++ {
		tr.Start()
		time.Sleep(1 * time.Millisecond)
		tr.End()
	}
	s := tr.Stats()
	if s.Count != 5 {
		t.Errorf("Count: got %d, want 5", s.Count)
	}
	if s.WindowSize != 3 {
		t.Errorf("WindowSize: got %d, want 3", s.WindowSize)
	}
}

func TestReset(t *testing.T) {
	tr := New()
	tr.Start()
	tr.End()
	if tr.Count() != 1 {
		t.Fatalf("Count: got %d, want 1", tr.Count())
	}
	tr.Reset()
	if tr.Count() != 0 {
		t.Errorf("Count after Reset: got %d, want 0", tr.Count())
	}
	s := tr.Stats()
	if s.WindowSize != 0 {
		t.Errorf("WindowSize after Reset: got %d, want 0", s.WindowSize)
	}
}

func TestWithWindowIgnoresNonPositive(t *testing.T) {
	tr := New(WithWindow(0))
	if tr.window != DefaultWindow {
		t.Errorf("zero window should fall back to default; got %d", tr.window)
	}
}

func TestPercentileSpot(t *testing.T) {
	// Synthetic sorted ms durations for spot-checking the percentile fn.
	sorted := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		100 * time.Millisecond,
	}
	if got := percentile(sorted, 0); got != 10*time.Millisecond {
		t.Errorf("p0: got %v, want 10ms", got)
	}
	if got := percentile(sorted, 1); got != 100*time.Millisecond {
		t.Errorf("p100: got %v, want 100ms", got)
	}
	// p50 of 5 samples: index 2 → 30ms (no interpolation needed).
	if got := percentile(sorted, 0.5); got != 30*time.Millisecond {
		t.Errorf("p50: got %v, want 30ms", got)
	}
	// p95 of 5 samples (n-1=4, idx=3.8): interpolates between sorted[3]=40 and sorted[4]=100.
	// frac=0.8 → 40 + 0.8*60 = 88ms.
	if got := percentile(sorted, 0.95); got != 88*time.Millisecond {
		t.Errorf("p95: got %v, want 88ms", got)
	}
}

func TestStatsOrdering(t *testing.T) {
	tr := New(WithWindow(10))
	// Push three cycles of increasing duration; verify min/max picks correctly.
	for _, d := range []time.Duration{5 * time.Millisecond, 1 * time.Millisecond, 10 * time.Millisecond} {
		tr.Start()
		time.Sleep(d)
		tr.End()
	}
	s := tr.Stats()
	if s.Min > 4*time.Millisecond {
		t.Errorf("Min: got %v, want roughly 1ms", s.Min)
	}
	if s.Max < 8*time.Millisecond {
		t.Errorf("Max: got %v, want roughly 10ms+", s.Max)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tr := New()
	var wg sync.WaitGroup
	wg.Add(20)
	for i := 0; i < 20; i++ {
		go func(i int) {
			defer wg.Done()
			switch i % 4 {
			case 0:
				tr.Start()
			case 1:
				tr.End()
			case 2:
				_ = tr.Stats()
			case 3:
				_ = tr.Elapsed()
			}
		}(i)
	}
	wg.Wait()
}
