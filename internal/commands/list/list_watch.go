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
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// listPollingInterval is only used when filesystem watching is unavailable.
// Keep the fallback deliberately slow so degraded watch mode does not cause
// distracting terminal flashes.
const listPollingInterval = 30 * time.Second

// runListWatch refreshes the multi-festival list board when festival lifecycle
// statuses change. It does not cycle between festivals (that behavior stays on
// fest watch).
//
// The board redraws only when a festival moves into or out of a lifecycle
// status directory. Task progress changes are visible on the next lifecycle
// change (or the next invocation) rather than causing periodic screen flashes.
func runListWatch(ctx context.Context, festivalsDir, filterStatus string, opts *listOptions, campaignRoot string) error {
	// Listing reads the dungeon; refuse against a both-spellings campaign so the
	// watch view cannot silently omit festivals filed under the other spelling.
	if err := workspace.CheckDungeonConflict(festivalsDir); err != nil {
		return err
	}
	paint := func() error {
		return renderListFrame(ctx, festivalsDir, filterStatus, opts, campaignRoot, false)
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
		if err := paint(); err != nil {
			fmt.Fprintf(os.Stderr, "%s could not refresh list view: %v\n", ui.Warning("Warning:"), err)
		}
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "File watching unavailable (%v), using polling fallback\n", err)
		return runListPollingMode(ctx, festivalsDir, filterStatus, opts, campaignRoot)
	}
	defer func() { _ = w.Close() }()

	if err := paint(); err != nil {
		return err
	}

	err = w.Watch(ctx)
	if err != nil && ctx.Err() == nil {
		return err
	}
	fmt.Println()
	return nil
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
// where festivals appear, move, or leave.
func listWatchPaths(festivalsDir string) []string {
	paths := []string{festivalsDir}
	for _, status := range id.StatusDirectories {
		paths = append(paths, workspace.JoinStatus(festivalsDir, status))
	}
	// dungeon root so moves into/out of dungeon tree are noticed even when a
	// dated bucket path is new.
	paths = append(paths, workspace.JoinDungeon(festivalsDir))
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
