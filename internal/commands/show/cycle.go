package show

import (
	"context"
	stderrors "errors"
	"os"

	"github.com/Obedience-Corp/fest/internal/errors"
	"golang.org/x/term"
)

// CycleAction is the classified result of a keypress in the cycle loop.
type CycleAction int

const (
	// CycleNone is an unrecognized key that the reader keeps waiting past.
	CycleNone CycleAction = iota
	// CycleNext advances to the next festival.
	CycleNext
	// CyclePrev moves to the previous festival.
	CyclePrev
	// CycleQuit exits the cycle.
	CycleQuit
	// CycleExtra dispatches a caller-registered ExtraKeys handler.
	CycleExtra
)

// ExtraKeyHandler runs a custom action for a caller-registered key (e.g. watch's
// promote). It receives the festival currently displayed and the raw terminal
// state so it can drop and restore raw mode around an interactive sub-flow. It
// returns the possibly-updated path for the current index and whether the cycle
// should continue; returning cont=false exits the cycle cleanly.
type ExtraKeyHandler func(ctx context.Context, festival *FestivalInfo, rawState *term.State) (newPath string, cont bool)

// CycleOptions configures RunCycle. Detect resolves a festival from a path and
// Render draws one live-refreshing frame until its context is cancelled. ExtraKeys
// injects command-specific keys (watch registers promote; show registers quit)
// without the engine depending on those packages. RenderFallback, when set, is
// used instead of Render outside raw mode (non-TTY stdin or MakeRaw failure) so
// cycle-only output like the CRLF translation and key hints never reaches
// non-interactive consumers.
type CycleOptions struct {
	Detect         func(ctx context.Context, path string) (*FestivalInfo, error)
	Render         func(ctx context.Context, festival *FestivalInfo, cycling bool) error
	RenderFallback func(ctx context.Context, festival *FestivalInfo) error
	ExtraKeys      map[byte]ExtraKeyHandler
}

// RunCycle drives an arrow-key cycle over festivals at paths, rendering each with
// the live-refresh watch renderer. Left/right move between festivals, Ctrl+C
// quits, and registered ExtraKeys run custom actions. Outside an interactive
// terminal it renders paths[startIndex] once and returns.
func RunCycle(ctx context.Context, paths []string, startIndex int, opts CycleOptions) error {
	if len(paths) == 0 {
		return errors.Validation("no cyclable festivals found")
	}
	if opts.Detect == nil || opts.Render == nil {
		return errors.Validation("cycle requires detect and render delegates")
	}
	if startIndex < 0 || startIndex >= len(paths) {
		startIndex = 0
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return renderCycleFrameOnce(ctx, paths[startIndex], opts)
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return renderCycleFrameOnce(ctx, paths[startIndex], opts)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

	cycling := len(paths) > 1
	index := startIndex
	staleSince := -1
	for {
		if ctx.Err() != nil {
			return nil
		}

		path := paths[index]
		festival, err := opts.Detect(ctx, path)
		if err != nil {
			if isCycleFestivalNotFound(err) {
				if staleSince < 0 {
					staleSince = index
				} else if staleSince == index {
					return errors.Validation("all cycle targets are stale; no festival found").
						WithField("checked", len(paths))
				}
				index = (index + 1) % len(paths)
				continue
			}
			return err
		}
		if festival == nil {
			if staleSince < 0 {
				staleSince = index
			} else if staleSince == index {
				return errors.Validation("all cycle targets are stale; no festival found").
					WithField("checked", len(paths))
			}
			index = (index + 1) % len(paths)
			continue
		}
		staleSince = -1

		renderCtx, cancelRender := context.WithCancel(ctx)
		renderDone := make(chan error, 1)
		go func() {
			renderDone <- opts.Render(renderCtx, festival, cycling)
		}()

		action, key := readCycleAction(ctx, os.Stdin, opts.ExtraKeys)
		cancelRender()
		renderErr := <-renderDone
		if renderErr != nil && renderCtx.Err() == nil {
			return renderErr
		}

		if ctx.Err() != nil {
			return nil
		}
		switch action {
		case CycleQuit:
			return nil
		case CycleExtra:
			handler := opts.ExtraKeys[key]
			if handler == nil {
				continue
			}
			newPath, cont := handler(ctx, festival, oldState)
			if !cont {
				return nil
			}
			if newPath != "" {
				paths[index] = newPath
			}
		case CycleNext:
			index = (index + 1) % len(paths)
		case CyclePrev:
			index = (index - 1 + len(paths)) % len(paths)
		}
	}
}

func renderCycleFrameOnce(ctx context.Context, path string, opts CycleOptions) error {
	festival, err := opts.Detect(ctx, path)
	if err != nil {
		return err
	}
	if festival == nil {
		return errors.NotFound("festival").WithField("path", path)
	}
	if opts.RenderFallback != nil {
		return opts.RenderFallback(ctx, festival)
	}
	return opts.Render(ctx, festival, false)
}

// classifyCycleKey maps a raw key sequence to a cycle action. Ctrl+C quits and
// arrow keys move; any byte registered in extraKeys dispatches its handler.
func classifyCycleKey(b []byte, extraKeys map[byte]ExtraKeyHandler) (CycleAction, byte, bool) {
	if len(b) == 0 {
		return CycleNone, 0, false
	}
	if b[0] == 0x03 {
		return CycleQuit, 0, true
	}
	if _, ok := extraKeys[b[0]]; ok {
		return CycleExtra, b[0], true
	}
	if len(b) >= 3 && b[0] == 0x1b && b[1] == '[' {
		switch b[2] {
		case 'C':
			return CycleNext, 0, true
		case 'D':
			return CyclePrev, 0, true
		}
	}
	return CycleNone, 0, false
}

func isCycleFestivalNotFound(err error) bool {
	var structured *errors.Error
	return stderrors.As(err, &structured) && structured.Code == errors.ErrCodeNotFound
}
