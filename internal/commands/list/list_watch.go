package list

import (
	"context"
	"fmt"
	"io/fs"
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

	watchPaths := listWatchPaths(festivalsDir)
	var w *watch.Watcher
	w, err := watch.New(watch.Config{
		Paths:    watchPaths,
		Debounce: 100 * time.Millisecond,
		MaxWait:  watch.DefaultMaxWait,
		OnError: func(err error) {
			fmt.Fprintf(os.Stderr, "%s file watch error: %v\n", ui.Warning("Warning:"), err)
		},
	}, func() {
		// A create event on an already-watched parent can introduce a new phase,
		// sequence, or task directory. Reconcile before rendering so subsequent
		// writes inside that directory are observed too.
		addListWatchPaths(w, festivalsDir)
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
	session := &listFrameSession{}

	if err := session.render(ctx, festivalsDir, filterStatus, opts, campaignRoot, true); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Println()
			return nil
		case <-ticker.C:
			if err := session.render(ctx, festivalsDir, filterStatus, opts, campaignRoot, true); err != nil {
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

// listWatchPaths returns fsnotify paths for the festivals workspace and every
// existing directory below it. fsnotify directory watches are non-recursive,
// so task-file updates require watching their phase and sequence parents.
func listWatchPaths(festivalsDir string) []string {
	paths := make([]string, 0)
	seen := make(map[string]struct{})
	add := func(path string) {
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	add(festivalsDir)
	for _, status := range id.StatusDirectories {
		add(workspace.JoinStatus(festivalsDir, status))
	}
	add(workspace.JoinDungeon(festivalsDir))

	_ = filepath.WalkDir(festivalsDir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			add(path)
		}
		return nil
	})
	return paths
}

func addListWatchPaths(w *watch.Watcher, festivalsDir string) {
	if w == nil {
		return
	}
	for _, path := range listWatchPaths(festivalsDir) {
		if err := w.AddPath(path); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s could not watch %s: %v\n", ui.Warning("Warning:"), path, err)
		}
	}
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
