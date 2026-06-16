package show

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	filewatch "github.com/Obedience-Corp/fest/internal/watch"
)

// WatchStandaloneWorkflow watches and refreshes a standalone WORKFLOW.md view.
func WatchStandaloneWorkflow(ctx context.Context, info *StandaloneWorkflowInfo, opts WatchOptions) error {
	if info == nil {
		return errors.Validation("standalone workflow is required")
	}
	startDir := info.StartDir
	if startDir == "" {
		startDir = filepath.Dir(info.WorkflowDoc)
	}
	return runStandaloneWatchMode(ctx, startDir, opts)
}

func runStandaloneWatchMode(ctx context.Context, startDir string, opts WatchOptions) error {
	session := &standaloneWatchSession{startDir: startDir, opts: opts}
	w, err := filewatch.New(filewatch.Config{
		Debounce: filewatch.DefaultDebounce,
		OnError:  session.reportWatchError,
	}, func() {
		session.renderOrWarn(ctx, false)
	})
	if err != nil {
		wrapped := errors.IO("creating standalone workflow watcher", err)
		fmt.Fprintf(os.Stderr, "File watching unavailable (%v), using polling fallback\n", wrapped)
		return runStandalonePollingMode(ctx, startDir, opts)
	}
	defer func() { _ = w.Close() }()

	session.watcher = w
	if err := session.render(ctx, false); err != nil {
		return err
	}
	return w.Watch(ctx)
}

func runStandalonePollingMode(ctx context.Context, startDir string, opts WatchOptions) error {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	session := &standaloneWatchSession{startDir: startDir, opts: opts}
	if err := session.render(ctx, true); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			fmt.Println()
			return nil
		case <-ticker.C:
			session.renderOrWarn(ctx, true)
		}
	}
}

type standaloneWatchSession struct {
	startDir string
	opts     WatchOptions
	watcher  *filewatch.Watcher
}

func (s *standaloneWatchSession) render(ctx context.Context, polling bool) error {
	info, err := ResolveStandaloneWorkflow(ctx, s.startDir)
	if err != nil {
		return err
	}
	if info == nil {
		return errors.NotFound("standalone workflow").WithField("path", s.startDir)
	}

	clearScreen(os.Stdout)
	fmt.Print(formatStandaloneWorkflowProgress(info, standaloneRenderOptionsFromWatch(s.opts)))
	printWatchFooter(os.Stdout, polling, false)
	s.addWatchPaths(info)
	return nil
}

func (s *standaloneWatchSession) renderOrWarn(ctx context.Context, polling bool) {
	if err := s.render(ctx, polling); err != nil {
		fmt.Fprintf(os.Stderr, "%s could not refresh standalone workflow view: %v\n", ui.Warning("Warning:"), err)
	}
}

func (s *standaloneWatchSession) reportWatchError(err error) {
	fmt.Fprintf(os.Stderr, "%s file watch error: %v\n", ui.Warning("Warning:"), err)
}

func (s *standaloneWatchSession) addWatchPaths(info *StandaloneWorkflowInfo) {
	s.addWatchPath(filepath.Dir(info.WorkflowDoc))
	s.addWatchPath(info.WorkflowDoc)
	s.addWatchPath(info.RuntimeDir)
	if info.RuntimeDir != "" {
		s.addWatchPath(filepath.Join(info.RuntimeDir, "runs"))
	}
	if info.RuntimeDir != "" && info.RunID != "" {
		s.addWatchPath(filepath.Join(info.RuntimeDir, "runs", info.RunID))
	}
}

func (s *standaloneWatchSession) addWatchPath(path string) {
	if path == "" || s.watcher == nil {
		return
	}
	stat, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s could not inspect %s: %v\n", ui.Warning("Warning:"), path, err)
		}
		return
	}
	if !stat.IsDir() && filepath.Base(path) != "WORKFLOW.md" {
		return
	}
	if err := s.watcher.AddPath(path); err != nil {
		fmt.Fprintf(os.Stderr, "%s could not watch %s: %v\n", ui.Warning("Warning:"), path, err)
	}
}
