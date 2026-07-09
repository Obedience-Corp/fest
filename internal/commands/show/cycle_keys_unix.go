//go:build unix

package show

import (
	"context"
	"os"

	"golang.org/x/sys/unix"
)

func readCycleAction(ctx context.Context, f *os.File, extraKeys map[byte]ExtraKeyHandler) (CycleAction, byte) {
	fd := int(f.Fd())

	var pipe [2]int
	if err := unix.Pipe(pipe[:]); err != nil {
		return readCycleActionBlocking(ctx, f, extraKeys)
	}
	readEnd, writeEnd := pipe[0], pipe[1]
	defer func() { _ = unix.Close(readEnd) }()

	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
		case <-stop:
		}
		_ = unix.Close(writeEnd)
	}()
	defer close(stop)

	buf := make([]byte, 3)
	fds := make([]unix.PollFd, 2)
	for {
		if ctx.Err() != nil {
			return CycleQuit, 0
		}
		fds[0] = unix.PollFd{Fd: int32(fd), Events: unix.POLLIN}
		fds[1] = unix.PollFd{Fd: int32(readEnd), Events: unix.POLLIN}
		if _, err := unix.Poll(fds, -1); err != nil {
			if err == unix.EINTR {
				continue
			}
			return CycleQuit, 0
		}
		if fds[1].Revents != 0 {
			return CycleQuit, 0
		}
		if fds[0].Revents&unix.POLLIN != 0 {
			n, err := unix.Read(fd, buf)
			if err != nil {
				if err == unix.EINTR {
					continue
				}
				return CycleQuit, 0
			}
			if n == 0 {
				return CycleQuit, 0
			}
			if action, key, ok := classifyCycleKey(buf[:n], extraKeys); ok {
				return action, key
			}
			continue
		}
		if fds[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			return CycleQuit, 0
		}
	}
}

func readCycleActionBlocking(ctx context.Context, f *os.File, extraKeys map[byte]ExtraKeyHandler) (CycleAction, byte) {
	type keyResult struct {
		action CycleAction
		key    byte
	}
	keyCh := make(chan keyResult, 1)
	go func() {
		buf := make([]byte, 3)
		for {
			n, err := f.Read(buf)
			if err != nil || n == 0 {
				keyCh <- keyResult{CycleQuit, 0}
				return
			}
			if action, key, ok := classifyCycleKey(buf[:n], extraKeys); ok {
				keyCh <- keyResult{action, key}
				return
			}
		}
	}()
	select {
	case r := <-keyCh:
		return r.action, r.key
	case <-ctx.Done():
		return CycleQuit, 0
	}
}
