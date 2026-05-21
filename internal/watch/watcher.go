// Package watch provides file system watching utilities with debouncing support.
package watch

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDebounce is the default debounce duration for file changes.
const DefaultDebounce = 100 * time.Millisecond

// Config configures the file watcher behavior.
type Config struct {
	// Paths lists files and directories to watch.
	// Directories are watched non-recursively.
	Paths []string

	// Debounce is the duration to wait after the last change before triggering.
	// This coalesces rapid changes (e.g., editor save patterns) into a single event.
	// Defaults to DefaultDebounce if zero.
	Debounce time.Duration

	// FallbackPoll is the polling interval to use if fsnotify fails.
	// If zero, no fallback is used and errors are returned.
	FallbackPoll time.Duration

	// OnError receives non-fatal watcher errors reported by the OS.
	OnError func(error)
}

// Watcher monitors files for changes and calls a callback when changes occur.
type Watcher struct {
	config   Config
	onChange func()
	watcher  *fsnotify.Watcher

	mu      sync.Mutex
	timer   *time.Timer
	pending bool
	watched map[string]struct{}
}

// ErrNoWatchablePaths is returned when none of the specified paths can be watched.
var ErrNoWatchablePaths = os.ErrNotExist

// New creates a new file watcher that calls onChange when watched files change.
// The onChange callback is debounced according to config.Debounce.
// Returns ErrNoWatchablePaths if none of the specified paths exist or can be watched.
func New(cfg Config, onChange func()) (*Watcher, error) {
	if cfg.Debounce == 0 {
		cfg.Debounce = DefaultDebounce
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		config:   cfg,
		onChange: onChange,
		watcher:  fsw,
		watched:  make(map[string]struct{}),
	}

	// Add all paths to the watcher.
	watchedCount := 0
	for _, path := range cfg.Paths {
		if err := w.AddPath(path); err != nil {
			if !os.IsNotExist(err) {
				_ = fsw.Close()
				return nil, err
			}
			continue
		}
		if path != "" {
			watchedCount++
		}
	}

	// If no paths could be watched, return error to trigger fallback
	if watchedCount == 0 && len(cfg.Paths) > 0 {
		_ = fsw.Close()
		return nil, ErrNoWatchablePaths
	}

	return w, nil
}

// AddPath dynamically adds an existing file or directory to the watcher.
// Re-adding an already watched path is a no-op.
func (w *Watcher) AddPath(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.watched[path]; ok {
		return nil
	}
	if err := w.watcher.Add(path); err != nil {
		return err
	}
	w.watched[path] = struct{}{}
	return nil
}

// Watch blocks until the context is cancelled, calling onChange when files change.
// Returns nil on context cancellation, or an error if watching fails.
func (w *Watcher) Watch(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			w.cancelPendingTimer()
			return nil

		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				w.removePath(event.Name)
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				w.scheduleCallback()
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
			if w.config.OnError != nil {
				w.config.OnError(err)
			}
		}
	}
}

func (w *Watcher) removePath(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.watched, path)
}

// scheduleCallback schedules the onChange callback after the debounce period.
// Multiple calls within the debounce period are coalesced into a single callback.
func (w *Watcher) scheduleCallback() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// If there's already a pending callback, reset the timer
	if w.timer != nil {
		w.timer.Stop()
	}

	w.pending = true
	w.timer = time.AfterFunc(w.config.Debounce, func() {
		w.mu.Lock()
		w.pending = false
		w.timer = nil
		w.mu.Unlock()

		w.onChange()
	})
}

// cancelPendingTimer cancels any pending debounced callback.
func (w *Watcher) cancelPendingTimer() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
		w.pending = false
	}
}

// Close releases all resources associated with the watcher.
func (w *Watcher) Close() error {
	w.cancelPendingTimer()
	return w.watcher.Close()
}

// WatchWithFallback attempts file watching, falling back to polling if it fails.
// This is a convenience function that combines New, Watch, and polling fallback.
func WatchWithFallback(ctx context.Context, cfg Config, onChange func()) error {
	w, err := New(cfg, onChange)
	if err != nil {
		// Fallback to polling if configured
		if cfg.FallbackPoll > 0 {
			return runPolling(ctx, cfg.FallbackPoll, onChange)
		}
		return err
	}
	defer func() { _ = w.Close() }()

	return w.Watch(ctx)
}

// runPolling runs a simple polling loop as fallback.
func runPolling(ctx context.Context, interval time.Duration, onChange func()) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			onChange()
		}
	}
}
