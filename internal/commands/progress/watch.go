// Package progress implements the fest progress command for tracking execution progress.
package progress

import (
	"context"
	"fmt"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// runWatchMode continuously refreshes the progress display at the specified interval.
func runWatchMode(ctx context.Context, mgr *progress.Manager, loc *show.LocationInfo, opts *progressOptions) error {
	ticker := time.NewTicker(opts.interval)
	defer ticker.Stop()

	// Initial render
	clearScreen()
	if err := showProgressOverview(ctx, mgr, loc, opts); err != nil {
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

			// Refresh progress manager to get latest data
			newMgr, err := progress.NewManager(ctx, loc.Festival.Path)
			if err == nil && newMgr != nil {
				mgr = newMgr
			}

			if err := showProgressOverview(ctx, mgr, loc, opts); err != nil {
				return err
			}
			printWatchFooter(opts.interval)
		}
	}
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
