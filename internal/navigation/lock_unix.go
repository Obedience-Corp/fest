//go:build unix

package navigation

import (
	"os"

	"golang.org/x/sys/unix"
)

// lockNavigationFile takes an exclusive advisory lock guarding navigation
// state mutations. It blocks until the lock is available and returns a
// release function.
func lockNavigationFile(lockPath string) (func(), error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}
