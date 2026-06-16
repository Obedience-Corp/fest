// Package show implements the fest show command for displaying festival information.
package show

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/watch"
)

// crlfWriter translates a bare "\n" into "\r\n" before writing. The watch cycle
// reader puts the terminal in raw mode (term.MakeRaw) to capture arrow keys,
// which clears OPOST/ONLCR so the terminal no longer maps "\n" to "\r\n" on
// output. Without this, every rendered line starts where the previous one ended
// (a diagonal staircase). It reports the original input length per io.Writer.
type crlfWriter struct{ w io.Writer }

func (c crlfWriter) Write(p []byte) (int, error) {
	var b bytes.Buffer
	b.Grow(len(p) + bytes.Count(p, []byte{'\n'}))
	for i := 0; i < len(p); i++ {
		if p[i] == '\n' && (i == 0 || p[i-1] != '\r') {
			b.WriteByte('\r')
		}
		b.WriteByte(p[i])
	}
	translated := b.Bytes()
	written, err := c.w.Write(translated)
	if written == len(translated) && err == nil {
		return len(p), nil
	}

	n, remaining := 0, written
	for i := 0; i < len(p); i++ {
		cost := 1
		if p[i] == '\n' && (i == 0 || p[i-1] != '\r') {
			cost = 2
		}
		if remaining < cost {
			break
		}
		remaining -= cost
		n++
	}
	if err == nil && n < len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

// watchWriter returns the stdout writer for a watch frame. In cycle mode the
// terminal is in raw mode, so newlines must be translated to keep lines
// column-aligned.
func watchWriter(cycleHint bool) io.Writer {
	return cycleOutput(cycleHint, os.Stdout)
}

// watchErrWriter returns the writer for warnings and errors emitted during a
// watch frame. term.MakeRaw stays active for the entire cycle session, so
// stderr output must also translate newlines or it produces the same staircase
// as the rendered tree.
func watchErrWriter(cycleHint bool) io.Writer {
	return cycleOutput(cycleHint, os.Stderr)
}

func cycleOutput(cycleHint bool, w io.Writer) io.Writer {
	if cycleHint {
		return crlfWriter{w: w}
	}
	return w
}

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
	out := watchWriter(cycleHint)
	errOut := watchErrWriter(cycleHint)

	stateDir := filepath.Join(festival.Path, ".fest")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		_, _ = fmt.Fprintf(errOut, "Warning: could not create state directory: %v\n", err)
	}

	watchPaths := []string{
		festival.Path,
		stateDir,
	}

	clearScreen(out)
	if err := renderFestivalView(ctx, festival, opts, out); err != nil {
		return err
	}
	printWatchFooter(out, false, cycleHint)

	w, err := watch.New(watch.Config{
		Paths:    watchPaths,
		Debounce: 100 * time.Millisecond,
		OnError: func(err error) {
			_, _ = fmt.Fprintf(errOut, "%s file watch error: %v\n", ui.Warning("Warning:"), err)
		},
	}, func() {
		clearScreen(out)
		refreshed, err := DetectCurrentFestival(ctx, festival.Path, "")
		if err == nil && refreshed != nil {
			festival = refreshed
		}
		if err := renderFestivalView(ctx, festival, opts, out); err != nil {
			_, _ = fmt.Fprintf(errOut, "%s could not refresh festival view: %v\n", ui.Warning("Warning:"), err)
		}
		printWatchFooter(out, false, cycleHint)
	})

	if err != nil {
		_, _ = fmt.Fprintf(errOut, "File watching unavailable (%v), using polling fallback\n", err)
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

	out := watchWriter(cycleHint)
	clearScreen(out)
	if err := renderFestivalView(ctx, festival, opts, out); err != nil {
		return err
	}
	printWatchFooter(out, true, cycleHint)

	for {
		select {
		case <-ctx.Done():
			_, _ = fmt.Fprintln(out)
			return nil
		case <-ticker.C:
			clearScreen(out)

			refreshed, err := DetectCurrentFestival(ctx, festival.Path, "")
			if err == nil && refreshed != nil {
				festival = refreshed
			}

			if err := renderFestivalView(ctx, festival, opts, out); err != nil {
				return err
			}
			printWatchFooter(out, true, cycleHint)
		}
	}
}

// renderFestivalView renders the appropriate view for the festival.
func renderFestivalView(ctx context.Context, festival *FestivalInfo, opts *showOptions, out io.Writer) error {
	verbose := shared.IsVerbose()

	// Use tree view by default, summary view with --summary flag
	if opts.summary {
		_, _ = fmt.Fprintln(out, FormatFestivalDetails(festival, verbose, ""))
		return nil
	}

	// Build and render tree view
	tree, err := BuildFestivalTree(ctx, festival.Path)
	if err != nil {
		// Fall back to summary view on error
		_, _ = fmt.Fprintln(out, FormatFestivalDetails(festival, verbose, ""))
		return nil
	}

	// Render progress bar at the top
	if festival.Stats != nil {
		progressBar := renderProgressBar(int(festival.Stats.Progress))
		_, _ = fmt.Fprintf(out, "%s %s %s\n\n",
			ui.Value(fmt.Sprintf("%.0f%%", festival.Stats.Progress)),
			progressBar,
			ui.Dim(fmt.Sprintf("(%d/%d tasks)", festival.Stats.Tasks.Completed, festival.Stats.Tasks.Total)))
	}

	treeOpts := DefaultTreeOptions()
	treeOpts.ShowGoals = opts.goals
	treeOpts.Collapsed = opts.collapsed
	treeOpts.InProgress = opts.inProgress
	_, _ = fmt.Fprintln(out, RenderTree(tree, treeOpts))
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
func clearScreen(out io.Writer) {
	_, _ = fmt.Fprint(out, "\033[H\033[2J")
}

// printWatchFooter prints the watch mode footer with exit instructions.
func printWatchFooter(out io.Writer, polling bool, cycleHint bool) {
	_, _ = fmt.Fprintln(out)
	suffix := "Watching for changes"
	if polling {
		suffix = "Polling for changes"
	}
	if cycleHint {
		_, _ = fmt.Fprintln(out, ui.Dim("Ctrl+C to exit • ← → cycle festivals • "+suffix))
	} else {
		_, _ = fmt.Fprintln(out, ui.Dim("Press Ctrl+C to exit • "+suffix))
	}
}
