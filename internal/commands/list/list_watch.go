package list

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/watch"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// listPollingInterval is only used when filesystem watching is unavailable.
// Keep the fallback deliberately slow so degraded watch mode does not cause
// distracting terminal flashes.
const listPollingInterval = 30 * time.Second

// runListWatch refreshes the multi-festival list board when festival files
// change. It does not cycle between festivals (that behavior stays on fest
// watch).
//
// Recursive directory watches surface task progress and lifecycle changes. The
// frame cache redraws only when the rendered board actually changes, avoiding
// periodic or duplicate screen flashes.
func runListWatch(ctx context.Context, festivalsDir, filterStatus string, opts *listOptions, campaignRoot string) error {
	// Listing reads the dungeon; refuse against a both-spellings campaign so the
	// watch view cannot silently omit festivals filed under the other spelling.
	if err := workspace.CheckDungeonConflict(festivalsDir); err != nil {
		return err
	}
	session := &listFrameSession{}
	paint := func() error {
		return session.render(ctx, festivalsDir, filterStatus, opts, campaignRoot, false)
	}

	changes := make(chan struct{}, 1)
	w, err := watch.New(watch.Config{
		Paths:     []string{festivalsDir},
		Recursive: true,
		Debounce:  100 * time.Millisecond,
		MaxWait:   watch.DefaultMaxWait,
		OnError: func(err error) {
			fmt.Fprintf(os.Stderr, "%s file watch error: %v\n", ui.Warning("Warning:"), err)
		},
	}, func() {
		select {
		case changes <- struct{}{}:
		default:
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

	watchDone := make(chan error, 1)
	go func() {
		watchDone <- w.Watch(ctx)
	}()

	// This low-rate reconciliation recovers from missed filesystem events and
	// adds directories that appeared during an OS watcher race. The frame cache
	// means an unchanged board is never cleared or redrawn.
	ticker := time.NewTicker(listPollingInterval)
	defer ticker.Stop()

	return waitListWatchEvents(ctx, watchDone, changes, ticker.C, paint, func() error {
		if err := w.AddTree(festivalsDir); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s could not reconcile festival watches: %v\n", ui.Warning("Warning:"), err)
		}
		return paint()
	})
}

func runListPollingMode(ctx context.Context, festivalsDir, filterStatus string, opts *listOptions, campaignRoot string) error {
	ticker := time.NewTicker(listPollingInterval)
	defer ticker.Stop()
	session := &listFrameSession{}

	if err := session.render(ctx, festivalsDir, filterStatus, opts, campaignRoot, true); err != nil {
		return err
	}

	return waitListWatchEvents(ctx, nil, nil, ticker.C, nil, func() error {
		return session.render(ctx, festivalsDir, filterStatus, opts, campaignRoot, true)
	})
}

// waitListWatchEvents drives the list --watch select loop until ctx is
// cancelled or the filesystem watcher exits.
//
// A cancelled context is a user stop (SIGINT/SIGTERM wired at Execute): return
// nil so the process exits 0 instead of dying by signal.
func waitListWatchEvents(ctx context.Context, watchDone <-chan error, changes <-chan struct{}, tick <-chan time.Time, onChange, onTick func() error) error {
	for {
		select {
		case <-ctx.Done():
			fmt.Println()
			return nil
		case err := <-watchDone:
			if err != nil && ctx.Err() == nil {
				return err
			}
			fmt.Println()
			return nil
		case <-changes:
			if onChange == nil {
				continue
			}
			if err := onChange(); err != nil {
				return err
			}
		case <-tick:
			if onTick == nil {
				continue
			}
			if err := onTick(); err != nil {
				return err
			}
		}
	}
}

type listFrameSession struct {
	content  string
	rendered bool
}

func (s *listFrameSession) render(ctx context.Context, festivalsDir, filterStatus string, opts *listOptions, campaignRoot string, polling bool) error {
	content, err := formatListBoard(ctx, festivalsDir, filterStatus, opts, campaignRoot)
	if err != nil {
		return err
	}
	if s.rendered && s.content == content {
		return nil
	}

	clearListScreen()
	fmt.Print(content)
	printListWatchFooter(polling, festivalsDir)
	s.content = content
	s.rendered = true
	return nil
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
