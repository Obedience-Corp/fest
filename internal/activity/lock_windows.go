//go:build windows

package activity

import (
	"os"
	"sync"

	"golang.org/x/sys/windows"
)

var (
	activityLockMu sync.Mutex
	activityLocks  = map[int]*windows.Overlapped{}
)

func defaultLockActivityFile(f *os.File) error {
	handle := windows.Handle(f.Fd())
	ov := &windows.Overlapped{}
	err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, ov)
	if err != nil {
		return err
	}
	activityLockMu.Lock()
	activityLocks[int(f.Fd())] = ov
	activityLockMu.Unlock()
	return nil
}

var lockActivityFile = defaultLockActivityFile

func unlockActivityFile(f *os.File) {
	fd := int(f.Fd())
	activityLockMu.Lock()
	ov := activityLocks[fd]
	delete(activityLocks, fd)
	activityLockMu.Unlock()
	if ov == nil {
		return
	}
	_ = windows.UnlockFileEx(windows.Handle(fd), 0, 1, 0, ov)
}
