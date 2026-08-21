//go:build !unix

package itestenv

import "os"

func lockFileNB(*os.File) error { return errLockUnsupported }

func unlockFile(*os.File) error { return nil }
