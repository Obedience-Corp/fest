// Package show implements the fest show command for displaying festival information.
package show

import (
	"context"
	"fmt"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// ProgressBarWidth defines the number of characters in the progress bar
const ProgressBarWidth = 20

// runWatchMode continuously refreshes the festival display at the specified interval.
func runWatchMode(ctx context.Context, festival *FestivalInfo, opts *showOptions) error {
	ticker := time.NewTicker(opts.interval)
	defer ticker.Stop()

	// Initial render
	clearScreen()
	if err := renderFestivalView(ctx, festival, opts); err != nil {
		return err
	}
	printWatchFooter(opts.interval)

	for {
		select {
		case <-ctx.Done():
			fmt.Println() // Clean newline on exit
			return nil
		case <-ticker.C:
			clearScreen()

			// Refresh festival info to get latest data
			refreshed, err := DetectCurrentFestival(ctx, festival.Path)
			if err == nil && refreshed != nil {
				festival = refreshed
			}

			if err := renderFestivalView(ctx, festival, opts); err != nil {
				return err
			}
			printWatchFooter(opts.interval)
		}
	}
}

// renderFestivalView renders the appropriate view for the festival.
func renderFestivalView(ctx context.Context, festival *FestivalInfo, opts *showOptions) error {
	verbose := shared.IsVerbose()

	// Use tree view by default, summary view with --summary flag
	if opts.summary {
		fmt.Println(FormatFestivalDetails(festival, verbose))
		return nil
	}

	// Build and render tree view
	tree, err := BuildFestivalTree(ctx, festival.Path)
	if err != nil {
		// Fall back to summary view on error
		fmt.Println(FormatFestivalDetails(festival, verbose))
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
func printWatchFooter(interval time.Duration) {
	fmt.Println()
	fmt.Println(ui.Dim(fmt.Sprintf("Press Ctrl+C to exit • Refreshing every %s", interval)))
}
