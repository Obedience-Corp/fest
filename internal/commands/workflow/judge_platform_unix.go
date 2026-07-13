//go:build unix

package workflow

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func acquireJudgeStepLock(ctx context.Context, lockPath string) (func(), error) {
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	for {
		err = unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if err != unix.EWOULDBLOCK && err != unix.EAGAIN {
			_ = lockFile.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = lockFile.Close()
			return nil, ctx.Err()
		case <-time.After(judgeLockPoll):
		}
	}

	return func() {
		_ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
		_ = lockFile.Close()
	}, nil
}

func configureDetachedJudgeProcess(cmd *exec.Cmd) {
	// A new session lets the runner outlive the parent fest process.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// judgeProcessAlive reports whether the recorded judge PID still exists.
func judgeProcessAlive(pid int) bool {
	return pid > 0 && unix.Kill(pid, 0) == nil
}
