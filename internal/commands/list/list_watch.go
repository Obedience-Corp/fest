package list

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/watch"
)

// listPollingInterval matches show/progress watch fallback cadence.
const listPollingInterval = 2 * time.Second

// runListWatch continuously refreshes the multi-festival list board.
// It does not cycle between festivals (that behavior stays on fest watch).
func runListWatch(ctx context.Context, festivalsDir, filterStatus string, opts *listOptions, campaignRoot string) error {
	render := func(polling bool) error {
		clearListScreen()
		content, err := formatListBoard(ctx, festivalsDir, filterStatus, opts, campaignRoot)
		if err != nil {
			return err
		}
		fmt.Print(content)
		printListWatchFooter(polling, festivalsDir)
		return nil
	}

	if err := render(false); err != nil {
		return err
	}

	watchPaths := listWatchPaths(festivalsDir)
	w, err := watch.New(watch.Config{
		Paths:    watchPaths,
		Debounce: 100 * time.Millisecond,
		MaxWait:  watch.DefaultMaxWait,
		OnError: func(err error) {
			fmt.Fprintf(os.Stderr, "%s file watch error: %v\n", ui.Warning("Warning:"), err)
		},
	}, func() {
		if err := render(false); err != nil {
			fmt.Fprintf(os.Stderr, "%s could not refresh list view: %v\n", ui.Warning("Warning:"), err)
		}
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "File watching unavailable (%v), using polling fallback\n", err)
		return runListPollingMode(ctx, festivalsDir, filterStatus, opts, campaignRoot)
	}
	defer func() { _ = w.Close() }()

	// Also poll so progress/stats updates inside festival trees surface even
	// though status-dir watches are non-recursive.
	ticker := time.NewTicker(listPollingInterval)
	defer ticker.Stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Watch(ctx)
	}()

	for {
		select {
		case <-ctx.Done():
			fmt.Println()
			return nil
		case err := <-errCh:
			if err != nil && ctx.Err() == nil {
				return err
			}
			fmt.Println()
			return nil
		case <-ticker.C:
			if err := render(false); err != nil {
				fmt.Fprintf(os.Stderr, "%s could not refresh list view: %v\n", ui.Warning("Warning:"), err)
			}
		}
	}
}

func runListPollingMode(ctx context.Context, festivalsDir, filterStatus string, opts *listOptions, campaignRoot string) error {
	ticker := time.NewTicker(listPollingInterval)
	defer ticker.Stop()

	if err := renderListFrame(ctx, festivalsDir, filterStatus, opts, campaignRoot, true); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Println()
			return nil
		case <-ticker.C:
			if err := renderListFrame(ctx, festivalsDir, filterStatus, opts, campaignRoot, true); err != nil {
				return err
			}
		}
	}
}

func renderListFrame(ctx context.Context, festivalsDir, filterStatus string, opts *listOptions, campaignRoot string, polling bool) error {
	clearListScreen()
	content, err := formatListBoard(ctx, festivalsDir, filterStatus, opts, campaignRoot)
	if err != nil {
		return err
	}
	fmt.Print(content)
	printListWatchFooter(polling, festivalsDir)
	return nil
}

// listWatchPaths returns non-recursive fsnotify paths covering status buckets
// where festivals appear, move, or leave. Deep progress file changes are
// covered by the polling interval in runListWatch.
func listWatchPaths(festivalsDir string) []string {
	paths := []string{festivalsDir}
	for _, status := range id.StatusDirectories {
		paths = append(paths, filepath.Join(festivalsDir, status))
	}
	// dungeon root so moves into/out of dungeon tree are noticed even when a
	// dated bucket path is new.
	paths = append(paths, filepath.Join(festivalsDir, "dungeon"))
	return paths
}

func clearListScreen() {
	fmt.Print("\033[H\033[2J")
}

func printListWatchFooter(polling bool, festivalsDir string) {
	fmt.Println()
	label := filepath.Base(festivalsDir)
	if label == "" || label == "." {
		label = "festivals"
	}
	if polling {
		fmt.Println(ui.Dim(fmt.Sprintf("watching %s — Ctrl+C to quit • Polling every %s", label, listPollingInterval)))
		return
	}
	fmt.Println(ui.Dim(fmt.Sprintf("watching %s — Ctrl+C to quit", label)))
}
