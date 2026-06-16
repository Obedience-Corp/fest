//go:build unix

package watch

import (
	"context"
	"os"

	"golang.org/x/sys/unix"
)

func readCycleKey(ctx context.Context, f *os.File) cycleDirection {
	fd := int(f.Fd())

	var pipe [2]int
	if err := unix.Pipe(pipe[:]); err != nil {
		return readCycleKeyBlocking(ctx, f)
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
			return cycleQuit
		}
		fds[0] = unix.PollFd{Fd: int32(fd), Events: unix.POLLIN}
		fds[1] = unix.PollFd{Fd: int32(readEnd), Events: unix.POLLIN}
		if _, err := unix.Poll(fds, -1); err != nil {
			if err == unix.EINTR {
				continue
			}
			return cycleQuit
		}
		if fds[1].Revents != 0 {
			return cycleQuit
		}
		if fds[0].Revents&unix.POLLIN != 0 {
			n, err := unix.Read(fd, buf)
			if err != nil {
				if err == unix.EINTR {
					continue
				}
				return cycleQuit
			}
			if n == 0 {
				return cycleQuit
			}
			if dir, ok := classifyCycleKey(buf[:n]); ok {
				return dir
			}
			continue
		}
		if fds[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			return cycleQuit
		}
	}
}

func readCycleKeyBlocking(ctx context.Context, f *os.File) cycleDirection {
	keyCh := make(chan cycleDirection, 1)
	go func() {
		buf := make([]byte, 3)
		for {
			n, err := f.Read(buf)
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
