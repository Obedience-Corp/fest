package localstore

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// defaultFileMode is the mode for a store file written for the first time.
const defaultFileMode os.FileMode = 0o644

// writeFileAtomic replaces path with content through a synced temp file in the
// same directory and a rename, so a failed write leaves the previous file
// intact instead of truncated. An existing file keeps its permissions, and one
// the process cannot write is refused rather than replaced, as os.WriteFile
// would refuse it.
func writeFileAtomic(path string, content io.Reader) error {
	mode := defaultFileMode
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
		probe, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		_ = probe.Close()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	replaced := false
	defer func() {
		if !replaced {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, content); err != nil {
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	replaced = true
	return nil
}
