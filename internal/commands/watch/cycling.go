package watch

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"golang.org/x/term"
)

type cycleDirection int

const (
	cycleNext cycleDirection = iota
	cyclePrev
	cycleQuit
)

const cycleKeyPollInterval = 150 * time.Millisecond

func runWatchCycle(ctx context.Context, paths []string, startIndex int, opts options, deps commandDeps) error {
	if len(paths) == 0 {
		return errors.Validation("no watchable festivals found")
	}
	if len(paths) == 1 {
		return watchFestivalAtPath(ctx, paths[0], opts, deps)
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return watchFestivalAtPath(ctx, paths[startIndex], opts, deps)
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return watchFestivalAtPath(ctx, paths[startIndex], opts, deps)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

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
			watchDone <- deps.watch(watchCtx, festival, cycleWatchOptions(opts))
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
		if dir == cycleNext {
			index = (index + 1) % len(paths)
		} else {
			index = (index - 1 + len(paths)) % len(paths)
		}
	}
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

func cycleWatchOptions(opts options) show.WatchOptions {
	wo := showWatchOptions(opts)
	wo.CycleHint = true
	return wo
}

type deadlineReader interface {
	io.Reader
	SetReadDeadline(t time.Time) error
}

func readCycleKey(ctx context.Context, r io.Reader) cycleDirection {
	if dr, ok := r.(deadlineReader); ok {
		if dir, supported := readCycleKeyWithDeadline(ctx, dr); supported {
			return dir
		}
	}
	return readCycleKeyBlocking(ctx, r)
}

func readCycleKeyWithDeadline(ctx context.Context, r deadlineReader) (cycleDirection, bool) {
	deadlineSet := false
	defer func() {
		if deadlineSet {
			_ = r.SetReadDeadline(time.Time{})
		}
	}()

	buf := make([]byte, 3)
	for {
		if ctx.Err() != nil {
			return cycleQuit, true
		}
		if err := r.SetReadDeadline(time.Now().Add(cycleKeyPollInterval)); err != nil {
			return 0, false
		}
		deadlineSet = true

		n, err := r.Read(buf)
		if err != nil {
			if os.IsTimeout(err) {
				continue
			}
			return cycleQuit, true
		}
		if n == 0 {
			return cycleQuit, true
		}
		if dir, ok := classifyCycleKey(buf[:n]); ok {
			return dir, true
		}
	}
}

func readCycleKeyBlocking(ctx context.Context, r io.Reader) cycleDirection {
	buf := make([]byte, 3)
	keyCh := make(chan cycleDirection, 1)

	go func() {
		for {
			n, err := r.Read(buf)
			if err != nil || n == 0 {
				keyCh <- cycleQuit
				return
			}
			if dir, ok := classifyCycleKey(buf[:n]); ok {
				keyCh <- dir
				return
			}
		}
	}()

	select {
	case dir := <-keyCh:
		return dir
	case <-ctx.Done():
		return cycleQuit
	}
}

func classifyCycleKey(b []byte) (cycleDirection, bool) {
	if len(b) == 0 {
		return 0, false
	}
	if b[0] == 0x03 {
		return cycleQuit, true
	}
	if len(b) >= 3 && b[0] == 0x1b && b[1] == '[' {
		switch b[2] {
		case 'C':
			return cycleNext, true
		case 'D':
			return cyclePrev, true
		}
	}
	return 0, false
}
