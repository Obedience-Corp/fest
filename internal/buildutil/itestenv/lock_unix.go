//go:build unix

package itestenv

import (
	"errors"
	"os"
	"syscall"
)

// lockFileNB takes an exclusive advisory lock without blocking. flock is used
// rather than a pid file because the kernel releases it when the holder exits.
func lockFileNB(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.EWOULDBLOCK):
		return errLockBusy
	default:
		return err
	}
}

func unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
