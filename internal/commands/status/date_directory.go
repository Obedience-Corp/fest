// Package status provides date-based directory management for festival dungeon statuses.
package status

import (
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// CalculateDateDir returns the YYYY-MM-DD formatted date directory
// for organizing dungeon festivals by date.
func CalculateDateDir(t time.Time) string {
	return t.Format("2006-01-02")
}

// CreateDateDirectory creates a date-organized directory under the completed folder.
// It's idempotent - calling multiple times with the same arguments is safe.
func CreateDateDirectory(completedDir, dateDir string) error {
	path := filepath.Join(completedDir, dateDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return errors.IO("creating date directory", err).WithField("path", path)
	}
	return nil
}

// MoveToDateDirectory moves a festival to a date-organized dungeon directory.
// It returns the new path on success.
func MoveToDateDirectory(sourcePath, completedDir, dateDir string) (string, error) {
	festivalName := filepath.Base(sourcePath)
	destPath := filepath.Join(completedDir, dateDir, festivalName)

	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		return "", errors.Validation("destination already exists").
			WithField("destination", destPath)
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Join(completedDir, dateDir), 0755); err != nil {
		return "", errors.IO("creating date directory", err)
	}

	// Attempt atomic rename first
	if err := os.Rename(sourcePath, destPath); err != nil {
		if !isCrossDevice(err) {
			return "", errors.IO("moving festival to date directory", err).
				WithField("source", sourcePath).
				WithField("destination", destPath)
		}
		return copyAndDelete(sourcePath, destPath)
	}

	return destPath, nil
}

// isCrossDevice reports whether a rename failed solely because source and
// destination live on different filesystems.
func isCrossDevice(err error) bool {
	var linkErr *os.LinkError
	if stderrors.As(err, &linkErr) {
		return stderrors.Is(linkErr.Err, syscall.EXDEV)
	}
	return stderrors.Is(err, syscall.EXDEV)
}

// GetCompletedPath returns the full path for a completed festival with date organization.
// Festivals are stored under dungeon/completed with date-based subdirectories.
func GetCompletedPath(festivalsRoot, festivalName, dateDir string) string {
	return filepath.Join(festivalsRoot, "dungeon", "completed", dateDir, festivalName)
}

// copyAndDelete performs a copy+delete move across filesystems. It stages the
// copy into a sibling ".partial" directory it owns and renames that into place
// only after a complete copy, so a failure never removes a destination this
// call did not create.
func copyAndDelete(sourcePath, destPath string) (string, error) {
	stagePath := destPath + ".partial"
	_ = os.RemoveAll(stagePath)

	if err := os.MkdirAll(stagePath, 0755); err != nil {
		return "", errors.IO("creating staging directory", err)
	}

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(stagePath, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath, info.Mode())
	})
	if err != nil {
		_ = os.RemoveAll(stagePath)
		return "", errors.Wrap(err, "copying festival files")
	}

	if _, err := os.Stat(destPath); err == nil {
		_ = os.RemoveAll(stagePath)
		return "", errors.Validation("destination already exists").
			WithField("destination", destPath)
	}

	if err := os.Rename(stagePath, destPath); err != nil {
		_ = os.RemoveAll(stagePath)
		return "", errors.IO("finalizing festival move", err).
			WithField("destination", destPath)
	}

	if err := os.RemoveAll(sourcePath); err != nil {
		return "", errors.IO("removing source after copy", err)
	}

	return destPath, nil
}

// copyFile copies a single file preserving permissions.
func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
