package watch

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	// Create temp file to watch
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	cfg := Config{
		Paths:    []string{tmpFile},
		Debounce: 50 * time.Millisecond,
	}

	w, err := New(cfg, func() {})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	if w.watcher == nil {
		t.Error("watcher should not be nil")
	}
}

func TestNewAllowsEmptyPathSet(t *testing.T) {
	w, err := New(Config{}, func() {})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = w.Close() }()
}

func TestWatcherAddPathTracksDynamicPath(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(Config{}, func() {})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.AddPath(tmpDir); err != nil {
		t.Fatalf("AddPath() error = %v", err)
	}
	if err := w.AddPath(tmpDir); err != nil {
		t.Fatalf("AddPath() duplicate error = %v", err)
	}
	if got := len(w.watched); got != 1 {
		t.Fatalf("watched path count = %d, want 1", got)
	}
}

func TestWatcherRemovePathAllowsReAdd(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(Config{}, func() {})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.AddPath(tmpDir); err != nil {
		t.Fatalf("AddPath() error = %v", err)
	}
	w.removePath(tmpDir)
	if _, ok := w.watched[tmpDir]; ok {
		t.Fatalf("removePath(%q) left stale cache entry", tmpDir)
	}
	if err := w.AddPath(tmpDir); err != nil {
		t.Fatalf("AddPath() after remove error = %v", err)
	}
	if _, ok := w.watched[tmpDir]; !ok {
		t.Fatalf("AddPath(%q) did not restore cache entry", tmpDir)
	}
}

func TestWatcher_Watch_DetectsChanges(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	var callCount atomic.Int32

	cfg := Config{
		Paths:    []string{tmpFile},
		Debounce: 50 * time.Millisecond,
	}

	w, err := New(cfg, func() {
		callCount.Add(1)
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watching in goroutine
	done := make(chan error, 1)
	go func() {
		done <- w.Watch(ctx)
	}()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(tmpFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	// Wait for debounce + some buffer
	time.Sleep(150 * time.Millisecond)

	if count := callCount.Load(); count != 1 {
		t.Errorf("expected 1 callback, got %d", count)
	}

	// Cancel and verify clean shutdown
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Watch() returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Error("Watch() did not return after context cancellation")
	}
}

func TestWatcher_Debouncing(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	var callCount atomic.Int32

	cfg := Config{
		Paths:    []string{tmpFile},
		Debounce: 100 * time.Millisecond,
	}

	w, err := New(cfg, func() {
		callCount.Add(1)
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Watch(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Make multiple rapid changes
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(tmpFile, []byte("change"+string(rune('0'+i))), 0644); err != nil {
			t.Fatalf("failed to modify file: %v", err)
		}
		time.Sleep(20 * time.Millisecond) // Less than debounce
	}

	// Wait for debounce to fire
	time.Sleep(200 * time.Millisecond)

	// Should have coalesced to 1 callback
	if count := callCount.Load(); count != 1 {
		t.Errorf("expected 1 callback (debounced), got %d", count)
	}

	cancel()
}

func TestWatcher_MaxWaitFiresDuringSustainedChanges(t *testing.T) {
	var callCount atomic.Int32

	cfg := Config{
		Debounce: 100 * time.Millisecond,
		MaxWait:  250 * time.Millisecond,
	}

	w := &Watcher{
		config: cfg,
		dispatcher: newDispatcher(func() {
			callCount.Add(1)
		}),
	}
	defer w.cancelPendingTimer()

	// Sustained events every 20ms reset the trailing debounce timer forever;
	// only the MaxWait cap lets the callback fire while the storm is running.
	storm := 700 * time.Millisecond
	deadline := time.Now().Add(storm)
	for time.Now().Before(deadline) {
		w.scheduleCallback()
		time.Sleep(20 * time.Millisecond)
	}
	duringStorm := callCount.Load()

	if duringStorm < 1 {
		t.Errorf("expected at least 1 callback during a %v storm with MaxWait=%v, got %d", storm, cfg.MaxWait, duringStorm)
	}

	time.Sleep(300 * time.Millisecond)
	if total := callCount.Load(); total <= duringStorm {
		t.Errorf("expected a trailing callback after the storm, got %d during and %d total", duringStorm, total)
	}
}

func TestWatcher_OnChangeNeverOverlaps(t *testing.T) {
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	// plainCounter is deliberately unsynchronized: if two onChange calls ever
	// overlap, the read-modify-write races and -race fails the test.
	var plainCounter int

	cfg := Config{
		Debounce: 5 * time.Millisecond,
		MaxWait:  10 * time.Millisecond,
	}

	w := &Watcher{config: cfg}
	w.dispatcher = newDispatcher(func() {
		n := inFlight.Add(1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		plainCounter++
		// Render slower than the fire cadence so a following timer fires while
		// this callback is still running, exercising the serialization path.
		time.Sleep(15 * time.Millisecond)
		inFlight.Add(-1)
	})

	// Sustained events reset the trailing debounce forever; MaxWait keeps firing
	// callbacks mid-storm, so without serialization these would overlap.
	storm := 400 * time.Millisecond
	deadline := time.Now().Add(storm)
	for time.Now().Before(deadline) {
		w.scheduleCallback()
		time.Sleep(2 * time.Millisecond)
	}

	// cancelPendingTimer waits on dispatchMu for any in-flight onChange and
	// neutralizes pending timers, so once it returns no callback runs or will
	// run. That release/acquire also orders every onChange write to plainCounter
	// before the reads below, keeping the canary honest under -race.
	w.cancelPendingTimer()

	if got := maxInFlight.Load(); got > 1 {
		t.Fatalf("onChange overlapped: max concurrent invocations = %d, want <= 1", got)
	}
	if plainCounter == 0 {
		t.Fatal("expected at least one onChange during the storm, got none")
	}
}

func TestWatcher_DispatchIgnoresStaleGeneration(t *testing.T) {
	var calls atomic.Int32

	// Debounce is long enough that no AfterFunc timer fires on its own; the test
	// drives dispatch directly to exercise the generation guard deterministically.
	w := &Watcher{
		config:     Config{Debounce: time.Hour},
		dispatcher: newDispatcher(func() { calls.Add(1) }),
	}

	w.scheduleCallback() // gen == 1
	w.scheduleCallback() // gen == 2, supersedes gen 1

	// A stale timer func from generation 1 must be inert.
	w.dispatch(1)
	if got := calls.Load(); got != 0 {
		t.Fatalf("stale dispatch fired onChange: calls = %d, want 0", got)
	}

	// The current generation dispatches exactly once.
	w.dispatch(2)
	if got := calls.Load(); got != 1 {
		t.Fatalf("current dispatch: calls = %d, want 1", got)
	}

	// After cancel, even a formerly-current generation is inert (gen bumped and
	// stopped set), so no callback can fire once the watcher is torn down.
	w.scheduleCallback()   // gen == 3
	w.cancelPendingTimer() // gen == 4, stopped == true
	w.dispatch(3)
	if got := calls.Load(); got != 1 {
		t.Fatalf("post-cancel dispatch fired onChange: calls = %d, want 1", got)
	}
}

func TestWatcher_WatchDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	var callCount atomic.Int32

	cfg := Config{
		Paths:    []string{tmpDir},
		Debounce: 50 * time.Millisecond,
	}

	w, err := New(cfg, func() {
		callCount.Add(1)
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Watch(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Create a new file in the directory
	newFile := filepath.Join(tmpDir, "new.txt")
	if err := os.WriteFile(newFile, []byte("new file"), 0644); err != nil {
		t.Fatalf("failed to create new file: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	if count := callCount.Load(); count < 1 {
		t.Errorf("expected at least 1 callback for new file, got %d", count)
	}

	cancel()
}

func TestWatchWithFallback_UsesPolling(t *testing.T) {
	var callCount atomic.Int32

	cfg := Config{
		Paths:        []string{"/nonexistent/path/that/does/not/exist"},
		Debounce:     50 * time.Millisecond,
		FallbackPoll: 50 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := WatchWithFallback(ctx, cfg, func() {
		callCount.Add(1)
	})

	if err != nil {
		t.Errorf("WatchWithFallback() error = %v", err)
	}

	// Should have been called at least once via polling
	if count := callCount.Load(); count < 1 {
		t.Errorf("expected at least 1 polling callback, got %d", count)
	}
}

func TestRunPolling(t *testing.T) {
	var callCount atomic.Int32

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	err := runPolling(ctx, 50*time.Millisecond, newDispatcher(func() {
		callCount.Add(1)
	}))

	if err != nil {
		t.Errorf("runPolling() error = %v", err)
	}

	// Should have been called 2-3 times (at 50ms and 100ms, maybe 150ms)
	if count := callCount.Load(); count < 2 {
		t.Errorf("expected at least 2 polling callbacks, got %d", count)
	}
}

func TestRunPolling_ReturnsOnContextCancel(t *testing.T) {
	var callCount atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runPolling(ctx, 10*time.Millisecond, newDispatcher(func() {
			callCount.Add(1)
		}))
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runPolling() returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runPolling() did not return after context cancellation")
	}

	// runPolling dispatches inline on its own goroutine, so its return means no
	// callback is in flight and the ticker is stopped: the count must be frozen.
	settled := callCount.Load()
	time.Sleep(50 * time.Millisecond)
	if got := callCount.Load(); got != settled {
		t.Errorf("polling fired %d more callbacks after returning, want 0", got-settled)
	}
}

func TestRunPolling_StoppedDispatcherSilencesTicks(t *testing.T) {
	var callCount atomic.Int32

	d := newDispatcher(func() { callCount.Add(1) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- runPolling(ctx, 10*time.Millisecond, d)
	}()

	time.Sleep(60 * time.Millisecond)
	d.stop()
	stoppedAt := callCount.Load()
	if stoppedAt == 0 {
		t.Fatal("expected polling callbacks before stop, got none")
	}

	// Ticks keep arriving, but a stopped dispatcher must swallow every one, the
	// same gate Close relies on for the fsnotify path.
	time.Sleep(60 * time.Millisecond)
	if got := callCount.Load(); got != stoppedAt {
		t.Errorf("polling fired %d callbacks after the dispatcher stopped, want 0", got-stoppedAt)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runPolling() did not return after context cancellation")
	}
}

func TestRunPolling_SharedDispatcherNeverOverlapsWatcher(t *testing.T) {
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	// plainCounter is deliberately unsynchronized: if a polling tick and a
	// watcher debounce timer ever run the callback concurrently, the
	// read-modify-write races and -race fails the test.
	var plainCounter int

	d := newDispatcher(func() {
		n := inFlight.Add(1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		plainCounter++
		// Run slower than either scheduler's cadence so the other one fires while
		// this callback is still in the critical section.
		time.Sleep(15 * time.Millisecond)
		inFlight.Add(-1)
	})

	// The watcher and the polling fallback share one dispatcher, which is the
	// arrangement WatchWithFallback hands out and the case the old free-function
	// runPolling could not serialize.
	w := &Watcher{
		config:     Config{Debounce: 5 * time.Millisecond, MaxWait: 10 * time.Millisecond},
		dispatcher: d,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	polling := make(chan error, 1)
	go func() {
		polling <- runPolling(ctx, 2*time.Millisecond, d)
	}()

	storm := 400 * time.Millisecond
	deadline := time.Now().Add(storm)
	for time.Now().Before(deadline) {
		w.scheduleCallback()
		time.Sleep(2 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-polling:
		if err != nil {
			t.Fatalf("runPolling() returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runPolling() did not return after context cancellation")
	}

	// cancelPendingTimer waits on the dispatcher for any in-flight callback and
	// neutralizes pending timers, so once it returns nothing runs or will run.
	// That release/acquire also orders every callback write to plainCounter
	// before the reads below, keeping the canary honest under -race.
	w.cancelPendingTimer()

	if got := maxInFlight.Load(); got > 1 {
		t.Fatalf("callback overlapped: max concurrent invocations = %d, want <= 1", got)
	}
	if plainCounter == 0 {
		t.Fatal("expected at least one callback during the storm, got none")
	}
}

func TestDispatcher(t *testing.T) {
	tests := []struct {
		name      string
		run       func(d *dispatcher)
		wantCalls int32
	}{
		{
			name:      "dispatch runs the callback",
			run:       func(d *dispatcher) { d.dispatch() },
			wantCalls: 1,
		},
		{
			name: "repeated dispatch runs the callback each time",
			run: func(d *dispatcher) {
				d.dispatch()
				d.dispatch()
			},
			wantCalls: 2,
		},
		{
			name: "dispatch after stop is inert",
			run: func(d *dispatcher) {
				d.stop()
				d.dispatch()
			},
			wantCalls: 0,
		},
		{
			name: "stop is idempotent",
			run: func(d *dispatcher) {
				d.stop()
				d.stop()
				d.dispatch()
			},
			wantCalls: 0,
		},
		{
			name: "stop mid-sequence silences only later dispatches",
			run: func(d *dispatcher) {
				d.dispatch()
				d.stop()
				d.dispatch()
			},
			wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			d := newDispatcher(func() { calls.Add(1) })

			tt.run(d)

			if got := calls.Load(); got != tt.wantCalls {
				t.Errorf("callback invocations = %d, want %d", got, tt.wantCalls)
			}
		})
	}
}
