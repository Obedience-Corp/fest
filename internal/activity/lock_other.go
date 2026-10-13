//go:build !unix && !windows

package activity

import "os"

func defaultLockActivityFile(f *os.File) error {
	return nil
}

var lockActivityFile = defaultLockActivityFile

func unlockActivityFile(f *os.File) {}
