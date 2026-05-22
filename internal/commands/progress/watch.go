// Package progress implements the fest progress command for tracking execution progress.
package progress

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/watch"
)

// runWatchMode watches for file changes and refreshes the progress display.
// Falls back to polling if file watching is not available.
func runWatchMode(ctx context.Context, mgr *progress.Manager, loc *show.LocationInfo, opts *progressOptions) error {
	festDir := loc.Festival.Path

	// Ensure .fest state directory exists so fsnotify has something to watch
	stateDir := filepath.Join(festDir, ".fest")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create state directory: %v\n", err)
	}

	// Watch paths include the festival root directory to detect structural changes
	// (new phases, fest.yaml modifications) in addition to state directory changes.
	// This ensures progress reflects the full festival state, not just progress events.
	watchPaths := []string{
		festDir,  // Festival root — detects structural changes (new phases, fest.yaml edits)
		stateDir, // State directory — detects progress state changes
		filepath.Join(stateDir, "progress_events.jsonl"), // Progress events file
	}

	// Initial render
	clearScreen()
	if err := showProgressOverview(ctx, mgr, loc, opts); err != nil {
		return err
	}
	printWatchFooter(false, opts.interval)

	// Attempt file watching with fallback to polling
	w, err := watch.New(watch.Config{
		Paths:    watchPaths,
		Debounce: 100 * time.Millisecond,
		OnError: func(err error) {
			fmt.Fprintf(os.Stderr, "%s file watch error: %v\n", ui.Warning("Warning:"), err)
		},
	}, func() {
		clearScreen()
		// Refresh progress manager to get latest data
		newMgr, err := progress.NewManager(ctx, festDir)
		if err == nil && newMgr != nil {
			mgr = newMgr
		}
		if err := showProgressOverview(ctx, mgr, loc, opts); err != nil {
			// Log error but don't fail - just show stale data
			fmt.Fprintf(os.Stderr, "%s could not refresh progress view: %v\n", ui.Warning("Warning:"), err)
		}
		printWatchFooter(false, opts.interval)
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "File watching unavailable (%v), using polling fallback\n", err)
		return runPollingMode(ctx, mgr, loc, opts)
	}
	defer func() { _ = w.Close() }()

	return w.Watch(ctx)
}

// runPollingMode continuously refreshes the progress display at the specified interval.
// Used as a fallback when file watching is not available.
func runPollingMode(ctx context.Context, mgr *progress.Manager, loc *show.LocationInfo, opts *progressOptions) error {
	ticker := time.NewTicker(opts.interval)
	defer ticker.Stop()

	// Re-render with polling indicator
	clearScreen()
	if err := showProgressOverview(ctx, mgr, loc, opts); err != nil {
		return err
	}
	printWatchFooter(true, opts.interval)

	for {
		select {
		case <-ctx.Done():
			fmt.Println() // Clean newline on exit
			return nil
		case <-ticker.C:
			clearScreen()

			// Refresh progress manager to get latest data
			newMgr, err := progress.NewManager(ctx, loc.Festival.Path)
			if err == nil && newMgr != nil {
				mgr = newMgr
			}

			if err := showProgressOverview(ctx, mgr, loc, opts); err != nil {
				return err
			}
			printWatchFooter(true, opts.interval)
		}
	}
}

// clearScreen clears the terminal screen using ANSI escape codes.
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

// printWatchFooter prints the watch mode footer with exit instructions.
func printWatchFooter(polling bool, interval time.Duration) {
	fmt.Println()
	if polling {
		fmt.Println(ui.Dim(fmt.Sprintf("Press Ctrl+C to exit • Polling every %s", interval)))
	} else {
		fmt.Println(ui.Dim("Press Ctrl+C to exit • Watching for changes"))
	}
}
