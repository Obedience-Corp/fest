// Package show implements the fest show command for displaying festival information.
package show

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/watch"
)

// ProgressBarWidth defines the number of characters in the progress bar
const ProgressBarWidth = 20

// pollingInterval is the fixed interval for the polling fallback when fsnotify is unavailable.
const pollingInterval = 2 * time.Second

// WatchOptions contains the public options needed to run the show watch renderer.
type WatchOptions struct {
	Summary    bool
	Goals      bool
	Collapsed  bool
	InProgress bool
}

// WatchFestival watches a festival using the existing show watch renderer.
func WatchFestival(ctx context.Context, festival *FestivalInfo, opts WatchOptions) error {
	if festival == nil {
		return errors.Validation("festival is required")
	}
	return runWatchMode(ctx, festival, &showOptions{
		summary:    opts.Summary,
		watch:      true,
		goals:      opts.Goals,
		collapsed:  opts.Collapsed,
		inProgress: opts.InProgress,
	})
}

// runWatchMode watches for file changes and refreshes the festival display.
// Falls back to polling if file watching is not available.
func runWatchMode(ctx context.Context, festival *FestivalInfo, opts *showOptions) error {
	// Ensure .fest state directory exists so fsnotify has something to watch
	stateDir := filepath.Join(festival.Path, ".fest")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create state directory: %v\n", err)
	}

	// Watch paths include the festival root directory to detect structural changes
	// (new phases, fest.yaml modifications) in addition to state directory changes.
	// This ensures `fest show --watch` reflects the full festival state.
	watchPaths := []string{
		festival.Path, // Festival root — detects structural changes
		stateDir,      // State directory — detects progress/state changes
	}

	// Initial render
	clearScreen()
	if err := renderFestivalView(ctx, festival, opts); err != nil {
		return err
	}
	printWatchFooter(false)

	// Attempt file watching with fallback to polling
	w, err := watch.New(watch.Config{
		Paths:    watchPaths,
		Debounce: 100 * time.Millisecond,
		OnError: func(err error) {
			fmt.Fprintf(os.Stderr, "%s file watch error: %v\n", ui.Warning("Warning:"), err)
		},
	}, func() {
		clearScreen()
		// Refresh festival info to get latest data
		refreshed, err := DetectCurrentFestival(ctx, festival.Path, "")
		if err == nil && refreshed != nil {
			festival = refreshed
		}
		if err := renderFestivalView(ctx, festival, opts); err != nil {
			// Log error but don't fail - just show stale data
			fmt.Fprintf(os.Stderr, "%s could not refresh festival view: %v\n", ui.Warning("Warning:"), err)
		}
		printWatchFooter(false)
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "File watching unavailable (%v), using polling fallback\n", err)
		return runPollingMode(ctx, festival, opts)
	}
	defer func() { _ = w.Close() }()

	return w.Watch(ctx)
}

// runPollingMode continuously refreshes the festival display at the specified interval.
// Used as a fallback when file watching is not available.
func runPollingMode(ctx context.Context, festival *FestivalInfo, opts *showOptions) error {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	// Re-render with polling indicator
	clearScreen()
	if err := renderFestivalView(ctx, festival, opts); err != nil {
		return err
	}
	printWatchFooter(true)

	for {
		select {
		case <-ctx.Done():
			fmt.Println() // Clean newline on exit
			return nil
		case <-ticker.C:
			clearScreen()

			// Refresh festival info to get latest data
			refreshed, err := DetectCurrentFestival(ctx, festival.Path, "")
			if err == nil && refreshed != nil {
				festival = refreshed
			}

			if err := renderFestivalView(ctx, festival, opts); err != nil {
				return err
			}
			printWatchFooter(true)
		}
	}
}

// renderFestivalView renders the appropriate view for the festival.
func renderFestivalView(ctx context.Context, festival *FestivalInfo, opts *showOptions) error {
	verbose := shared.IsVerbose()

	// Use tree view by default, summary view with --summary flag
	if opts.summary {
		fmt.Println(FormatFestivalDetails(festival, verbose, ""))
		return nil
	}

	// Build and render tree view
	tree, err := BuildFestivalTree(ctx, festival.Path)
	if err != nil {
		// Fall back to summary view on error
		fmt.Println(FormatFestivalDetails(festival, verbose, ""))
		return nil
	}

	// Render progress bar at the top
	if festival.Stats != nil {
		progressBar := renderProgressBar(int(festival.Stats.Progress))
		fmt.Printf("%s %s %s\n\n",
			ui.Value(fmt.Sprintf("%.0f%%", festival.Stats.Progress)),
			progressBar,
			ui.Dim(fmt.Sprintf("(%d/%d tasks)", festival.Stats.Tasks.Completed, festival.Stats.Tasks.Total)))
	}

	treeOpts := DefaultTreeOptions()
	treeOpts.ShowGoals = opts.goals
	treeOpts.Collapsed = opts.collapsed
	treeOpts.InProgress = opts.inProgress
	fmt.Println(RenderTree(tree, treeOpts))
	return nil
}

// renderProgressBar creates a progress bar for the given percentage.
func renderProgressBar(percentage int) string {
	opts := ui.DefaultProgressBarOptions()
	opts.Current = percentage
	opts.Total = 100
	opts.Width = ProgressBarWidth
	opts.ShowPercentage = false
	opts.ShowFraction = false
	opts.FilledColor = ui.SuccessColor
	opts.EmptyColor = ui.BorderColor
	return ui.RenderProgressBar(opts)
}

// clearScreen clears the terminal screen using ANSI escape codes.
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

// printWatchFooter prints the watch mode footer with exit instructions.
func printWatchFooter(polling bool) {
	fmt.Println()
	if polling {
		fmt.Println(ui.Dim("Press Ctrl+C to exit • Polling for changes"))
	} else {
		fmt.Println(ui.Dim("Press Ctrl+C to exit • Watching for changes"))
	}
}
