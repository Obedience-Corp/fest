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

type cycleDirection int

const (
	cycleNone cycleDirection = iota
	cycleNext
	cyclePrev
	cyclePromote
	cycleQuit
)

func runWatchCycle(ctx context.Context, paths []string, startIndex int, opts options, deps commandDeps) error {
	if len(paths) == 0 {
		return errors.Validation("no watchable festivals found")
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return watchFestivalAtPath(ctx, paths[startIndex], opts, deps)
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return watchFestivalAtPath(ctx, paths[startIndex], opts, deps)
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
		festival, err := deps.detectFestival(ctx, path)
		if err != nil {
			if isFestivalNotFound(err) {
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

		watchCtx, cancelWatch := context.WithCancel(ctx)
		watchDone := make(chan error, 1)
		go func() {
			watchDone <- deps.watch(watchCtx, festival, cycleWatchOptions(opts, cycling))
		}()

		dir := readCycleKey(ctx, os.Stdin)
		cancelWatch()
		watchErr := <-watchDone
		if watchErr != nil && watchCtx.Err() == nil {
			return watchErr
		}

		if ctx.Err() != nil {
			return nil
		}
		if dir == cycleQuit {
			return nil
		}
		if dir == cyclePromote {
			if newPath, ok := promoteFromWatch(ctx, festival, oldState); ok && newPath != "" {
				paths[index] = newPath
			} else if !ok {
				return nil
			}
			continue
		}
		if dir == cycleNext {
			index = (index + 1) % len(paths)
		} else {
			index = (index - 1 + len(paths)) % len(paths)
		}
	}
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

func watchFestivalAtPath(ctx context.Context, path string, opts options, deps commandDeps) error {
	festival, err := deps.detectFestival(ctx, path)
	if err != nil {
		return err
	}
	if festival == nil {
		return errors.NotFound("festival").WithField("path", path)
	}
	return watchFestival(ctx, festival, opts, deps)
}

func cycleWatchOptions(opts options, cycling bool) show.WatchOptions {
	wo := showWatchOptions(opts)
	wo.CycleHint = true
	wo.Cycling = cycling
	return wo
}

func classifyCycleKey(b []byte) (cycleDirection, bool) {
	if len(b) == 0 {
		return cycleNone, false
	}
	if b[0] == 0x03 {
		return cycleQuit, true
	}
	if b[0] == 'p' || b[0] == 'P' {
		return cyclePromote, true
	}
	if len(b) >= 3 && b[0] == 0x1b && b[1] == '[' {
		switch b[2] {
		case 'C':
			return cycleNext, true
		case 'D':
			return cyclePrev, true
		}
	}
	return cycleNone, false
}
