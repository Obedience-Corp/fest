//go:build windows

package workflow

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

func acquireJudgeStepLock(ctx context.Context, lockPath string) (func(), error) {
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	handle := windows.Handle(lockFile.Fd())
	overlapped := new(windows.Overlapped)
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	for {
		err = windows.LockFileEx(handle, flags, 0, 1, 0, overlapped)
		if err == nil {
			break
		}
		if err != windows.ERROR_LOCK_VIOLATION {
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
		_ = windows.UnlockFileEx(handle, 0, 1, 0, overlapped)
		_ = lockFile.Close()
	}, nil
}

func configureDetachedJudgeProcess(cmd *exec.Cmd) {
	// Windows processes normally survive their parent. A detached process group
	// also prevents console control events from coupling the judge to fest.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
		HideWindow:    true,
	}
}

func judgeProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	result, err := windows.WaitForSingleObject(handle, 0)
	return err == nil && result == uint32(windows.WAIT_TIMEOUT)
}
