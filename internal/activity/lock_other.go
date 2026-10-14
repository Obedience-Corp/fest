//go:build !unix && !windows

package activity

import "os"

func activityFileOpenFlags() int { return os.O_APPEND | os.O_CREATE | os.O_WRONLY }

func defaultLockActivityFile(f *os.File) error {
	return nil
}

var lockActivityFile = defaultLockActivityFile

func unlockActivityFile(f *os.File) {}
