package activity

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
)

const activityPathLockDirEnv = "FEST_ACTIVITY_PATH_LOCK_DIR"

// activityLockPath returns a stable per-user lock file for the given activity
// log path. Locks live under the user cache (or FEST_ACTIVITY_PATH_LOCK_DIR)
// so fest activity never leaves *.lock siblings in the campaign workspace.
func activityLockPath(logPath string) (string, error) {
	canonical, err := canonicalActivityLogPath(logPath)
	if err != nil {
		return "", errors.IO("canonicalizing activity log path", err).WithField("path", logPath)
	}
	root, err := activityPathLockRoot()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(canonical))
	name := hex.EncodeToString(sum[:]) + ".lock"
	return filepath.Join(root, name), nil
}

func activityPathLockRoot() (string, error) {
	if dir := os.Getenv(activityPathLockDirEnv); dir != "" {
		return dir, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", errors.IO("resolving user cache for activity path locks", err)
	}
	return filepath.Join(cache, "fest", "activity-path-locks"), nil
}

func canonicalActivityLogPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if _, err := os.Stat(path); err == nil {
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			return resolved, nil
		}
	}
	dir := filepath.Dir(abs)
	if dir != "" && dir != "." {
		if _, err := os.Stat(dir); err == nil {
			if resolvedDir, err := filepath.EvalSymlinks(dir); err == nil {
				return filepath.Join(resolvedDir, filepath.Base(abs)), nil
			}
		}
	}
	return abs, nil
}
