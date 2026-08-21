//go:build windows

package progress

import (
	"context"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

const progressLockSupported = true

func lockProgressFile(ctx context.Context, lockPath string) (func(), error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	handle := windows.Handle(f.Fd())
	overlapped := new(windows.Overlapped)
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	for {
		err = windows.LockFileEx(handle, flags, 0, 1, 0, overlapped)
		if err == nil {
			break
		}
		if err != windows.ERROR_LOCK_VIOLATION {
			_ = f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(progressLockPoll):
		}
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = windows.UnlockFileEx(handle, 0, 1, 0, overlapped)
			_ = f.Close()
		})
	}, nil
}
