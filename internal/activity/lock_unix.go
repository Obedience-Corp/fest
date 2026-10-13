//go:build unix

package activity

import (
	"os"

	"golang.org/x/sys/unix"
)

func defaultLockActivityFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX)
}

var lockActivityFile = defaultLockActivityFile

func unlockActivityFile(f *os.File) {
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
