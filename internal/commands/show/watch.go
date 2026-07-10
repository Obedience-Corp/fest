// Package show implements the fest show command for displaying festival information.
package show

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/watch"
	"golang.org/x/term"
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
	Cycling    bool
	// CyclePromote controls the cycle footer: when true the "p promote" hint is
	// shown (watch); when false the footer advertises "q/Ctrl+C" exit instead
	// (show's cycle has no promote affordance).
	CyclePromote bool
	// Frame, when set by the cycle engine, enables viewport scrolling of the
	// rendered content (up/down arrows, space/b). Nil outside cycle mode.
	Frame *FrameState
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
	}, opts.CycleHint, opts.Cycling, opts.CyclePromote, opts.Frame)
}

// watchFrameSession draws watch frames, applying the cycle viewport when a
// FrameState is attached. Content is cached so scroll-triggered redraws never
// rebuild the festival tree.
type watchFrameSession struct {
	mu          sync.Mutex
	out         io.Writer
	frame       *FrameState
	cycleHint   bool
	cycling     bool
	promoteHint bool
	polling     bool
	content     string
	// height returns the terminal row count; overridden in tests.
	height func() int
}

func newWatchFrameSession(out io.Writer, frame *FrameState, cycleHint, cycling, promoteHint, polling bool) *watchFrameSession {
	return &watchFrameSession{
		out:         out,
		frame:       frame,
		cycleHint:   cycleHint,
		cycling:     cycling,
		promoteHint: promoteHint,
		polling:     polling,
		height:      terminalRows,
	}
}

func terminalRows() int {
	_, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || rows <= 0 {
		return 0
	}
	return rows
}

// setContent replaces the cached frame content and draws it.
func (s *watchFrameSession) setContent(content string) {
	s.mu.Lock()
	s.content = content
	s.mu.Unlock()
	s.draw()
}

// draw paints the cached content, sliced to the scroll viewport when a frame
// is attached and the terminal size is known.
func (s *watchFrameSession) draw() {
	s.mu.Lock()
	defer s.mu.Unlock()

	clearScreen(s.out)
	rows := s.height()
	if s.frame == nil || rows <= footerRows {
		_, _ = fmt.Fprint(s.out, s.content)
		s.printFooter(0, 0, 0, false)
		return
	}

	lines := strings.Split(strings.TrimRight(s.content, "\n"), "\n")
	visible, from, to, total, overflow := s.frame.Viewport(lines, rows-footerRows)
	_, _ = fmt.Fprintln(s.out, strings.Join(visible, "\n"))
	s.printFooter(from, to, total, overflow)
}

// footerRows is the screen space reserved for the footer (blank line + hint).
const footerRows = 2

func (s *watchFrameSession) printFooter(from, to, total int, overflow bool) {
	_, _ = fmt.Fprintln(s.out)
	suffix := "Watching for changes"
	if s.polling {
		suffix = "Polling for changes"
	}
	if !s.cycleHint {
		_, _ = fmt.Fprintln(s.out, ui.Dim("Press Ctrl+C to exit • "+suffix))
		return
	}
	hint := "q/Ctrl+C exit • "
	if s.cycling {
		hint += "← → cycle • "
	}
	if s.promoteHint {
		hint += "p promote • "
	}
	if overflow {
		hint += fmt.Sprintf("↑↓/spc/b scroll (%d-%d/%d) • ", from, to, total)
	}
	hint += suffix
	_, _ = fmt.Fprintln(s.out, ui.Dim(hint))
}

// render rebuilds the festival content and draws a fresh frame.
func (s *watchFrameSession) render(ctx context.Context, festival *FestivalInfo, opts *showOptions) error {
	var buf bytes.Buffer
	if err := renderFestivalView(ctx, festival, opts, &buf); err != nil {
		return err
	}
	s.setContent(buf.String())
	return nil
}

// drainRedraw repaints the cached content whenever the cycle key loop scrolls,
// until ctx ends. No-op when no frame is attached.
func (s *watchFrameSession) drainRedraw(ctx context.Context) {
	if s.frame == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.frame.Redraw():
				s.draw()
			}
		}
	}()
}

// printWatchFooter prints the plain (non-cycle) watch footer. Used by views
// that never run inside the cycle key loop, e.g. standalone workflow watch.
func printWatchFooter(out io.Writer, polling bool) {
	_, _ = fmt.Fprintln(out)
	suffix := "Watching for changes"
	if polling {
		suffix = "Polling for changes"
	}
	_, _ = fmt.Fprintln(out, ui.Dim("Press Ctrl+C to exit • "+suffix))
}

// runWatchMode watches for file changes and refreshes the festival display.
// Falls back to polling if file watching is not available.
func runWatchMode(ctx context.Context, festival *FestivalInfo, opts *showOptions, cycleHint bool, cycling bool, promoteHint bool, frame *FrameState) error {
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

	session := newWatchFrameSession(out, frame, cycleHint, cycling, promoteHint, false)
	if err := session.render(ctx, festival, opts); err != nil {
		return err
	}

	w, err := watch.New(watch.Config{
		Paths:    watchPaths,
		Debounce: 100 * time.Millisecond,
		MaxWait:  watch.DefaultMaxWait,
		OnError: func(err error) {
			_, _ = fmt.Fprintf(errOut, "%s file watch error: %v\n", ui.Warning("Warning:"), err)
		},
	}, func() {
		refreshed, err := DetectCurrentFestival(ctx, festival.Path, "")
		if err == nil && refreshed != nil {
			festival = refreshed
		}
		if err := session.render(ctx, festival, opts); err != nil {
			_, _ = fmt.Fprintf(errOut, "%s could not refresh festival view: %v\n", ui.Warning("Warning:"), err)
		}
	})

	if err != nil {
		_, _ = fmt.Fprintf(errOut, "File watching unavailable (%v), using polling fallback\n", err)
		return runPollingMode(ctx, festival, opts, cycleHint, cycling, promoteHint, frame)
	}
	defer func() { _ = w.Close() }()

	session.drainRedraw(ctx)
	return w.Watch(ctx)
}

// runPollingMode continuously refreshes the festival display at the specified interval.
// Used as a fallback when file watching is not available.
func runPollingMode(ctx context.Context, festival *FestivalInfo, opts *showOptions, cycleHint bool, cycling bool, promoteHint bool, frame *FrameState) error {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	out := watchWriter(cycleHint)
	session := newWatchFrameSession(out, frame, cycleHint, cycling, promoteHint, true)
	if err := session.render(ctx, festival, opts); err != nil {
		return err
	}
	session.drainRedraw(ctx)

	for {
		select {
		case <-ctx.Done():
			_, _ = fmt.Fprintln(out)
			return nil
		case <-ticker.C:
			refreshed, err := DetectCurrentFestival(ctx, festival.Path, "")
			if err == nil && refreshed != nil {
				festival = refreshed
			}

			if err := session.render(ctx, festival, opts); err != nil {
				return err
			}
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

	if festival.Status != "" {
		statusValue := ui.GetStatusStyle(festival.Status).Render(festival.Status)
		if festival.StatusDate != "" {
			statusValue += " " + ui.Dim("("+festival.StatusDate+")")
		}
		_, _ = fmt.Fprintf(out, "%s %s\n", ui.Label("Status"), statusValue)
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

