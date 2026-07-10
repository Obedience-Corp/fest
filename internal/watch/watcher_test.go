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
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	var callCount atomic.Int32

	cfg := Config{
		Paths:    []string{tmpFile},
		Debounce: 100 * time.Millisecond,
		MaxWait:  250 * time.Millisecond,
	}

	w, err := New(cfg, func() {
		callCount.Add(1)
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = w.Close() }()

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

	err := runPolling(ctx, 50*time.Millisecond, func() {
		callCount.Add(1)
	})

	if err != nil {
		t.Errorf("runPolling() error = %v", err)
	}

	// Should have been called 2-3 times (at 50ms and 100ms, maybe 150ms)
	if count := callCount.Load(); count < 2 {
		t.Errorf("expected at least 2 polling callbacks, got %d", count)
	}
}
