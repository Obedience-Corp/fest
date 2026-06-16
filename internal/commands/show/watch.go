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
	CycleHint  bool
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
	}, opts.CycleHint)
}

// runWatchMode watches for file changes and refreshes the festival display.
// Falls back to polling if file watching is not available.
func runWatchMode(ctx context.Context, festival *FestivalInfo, opts *showOptions, cycleHint bool) error {
	stateDir := filepath.Join(festival.Path, ".fest")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create state directory: %v\n", err)
	}

	watchPaths := []string{
		festival.Path,
		stateDir,
	}

	clearScreen()
	if err := renderFestivalView(ctx, festival, opts); err != nil {
		return err
	}
	printWatchFooter(false, cycleHint)

	w, err := watch.New(watch.Config{
		Paths:    watchPaths,
		Debounce: 100 * time.Millisecond,
		OnError: func(err error) {
			fmt.Fprintf(os.Stderr, "%s file watch error: %v\n", ui.Warning("Warning:"), err)
		},
	}, func() {
		clearScreen()
		refreshed, err := DetectCurrentFestival(ctx, festival.Path, "")
		if err == nil && refreshed != nil {
			festival = refreshed
		}
		if err := renderFestivalView(ctx, festival, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s could not refresh festival view: %v\n", ui.Warning("Warning:"), err)
		}
		printWatchFooter(false, cycleHint)
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "File watching unavailable (%v), using polling fallback\n", err)
		return runPollingMode(ctx, festival, opts, cycleHint)
	}
	defer func() { _ = w.Close() }()

	return w.Watch(ctx)
}

// runPollingMode continuously refreshes the festival display at the specified interval.
// Used as a fallback when file watching is not available.
func runPollingMode(ctx context.Context, festival *FestivalInfo, opts *showOptions, cycleHint bool) error {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	clearScreen()
	if err := renderFestivalView(ctx, festival, opts); err != nil {
		return err
	}
	printWatchFooter(true, cycleHint)

	for {
		select {
		case <-ctx.Done():
			fmt.Println()
			return nil
		case <-ticker.C:
			clearScreen()

			refreshed, err := DetectCurrentFestival(ctx, festival.Path, "")
			if err == nil && refreshed != nil {
				festival = refreshed
			}

			if err := renderFestivalView(ctx, festival, opts); err != nil {
				return err
			}
			printWatchFooter(true, cycleHint)
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
func printWatchFooter(polling bool, cycleHint bool) {
	fmt.Println()
	suffix := "Watching for changes"
	if polling {
		suffix = "Polling for changes"
	}
	if cycleHint {
		fmt.Println(ui.Dim("Ctrl+C to exit • ← → cycle festivals • " + suffix))
	} else {
		fmt.Println(ui.Dim("Press Ctrl+C to exit • " + suffix))
	}
}
