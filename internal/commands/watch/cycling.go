package watch

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/Obedience-Corp/fest/internal/commands/promote"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"golang.org/x/term"
)

// runWatchCycle drives the arrow-key cycle over watchable festivals using the
// shared show.RunCycle engine, injecting watch's live renderer and its `p`
// promote key. The generic loop, raw-mode handling, and stale-path skipping all
// live in the show package.
func runWatchCycle(ctx context.Context, paths []string, startIndex int, opts options, deps commandDeps) error {
	if len(paths) == 0 {
		return errors.Validation("no watchable festivals found")
	}
	return show.RunCycle(ctx, paths, startIndex, show.CycleOptions{
		Detect: deps.detectFestival,
		Render: func(ctx context.Context, festival *show.FestivalInfo, cycling bool, frame *show.FrameState) error {
			watchOpts := cycleWatchOptions(opts, cycling)
			watchOpts.Frame = frame
			return deps.watch(ctx, festival, watchOpts)
		},
		RenderFallback: func(ctx context.Context, festival *show.FestivalInfo) error {
			return deps.watch(ctx, festival, showWatchOptions(opts))
		},
		ExtraKeys: map[byte]show.ExtraKeyHandler{
			'p': promoteFromWatch,
			'P': promoteFromWatch,
		},
	})
}

// promoteFromWatch runs the fest promote flow outside raw mode and returns the
// post-move path (empty if nothing moved) and whether raw mode was restored.
func promoteFromWatch(ctx context.Context, festival *show.FestivalInfo, rawState *term.State) (string, bool) {
	fd := int(os.Stdin.Fd())
	_ = term.Restore(fd, rawState)

	newPath, err := promote.PromoteResolved(ctx, festival)
	if err != nil {
		fmt.Printf("%s %v\n", ui.Warning("Promote failed:"), err)
	}
	if newPath == "" {
		fmt.Print("\nPress Enter to continue...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}

	if _, err := term.MakeRaw(fd); err != nil {
		return newPath, false
	}
	return newPath, true
}

func cycleWatchOptions(opts options, cycling bool) show.WatchOptions {
	wo := showWatchOptions(opts)
	wo.CycleHint = true
	wo.Cycling = cycling
	wo.CyclePromote = true
	return wo
}
