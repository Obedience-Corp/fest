package dirlink

import (
	"fmt"
	"os"
	"path/filepath"
)

type linkSnapshot struct {
	exists bool
	target string
}

func snapshotLink(linkPath string) (linkSnapshot, error) {
	var snap linkSnapshot
	if _, err := os.Lstat(linkPath); err != nil {
		if os.IsNotExist(err) {
			return snap, nil
		}
		return snap, fmt.Errorf("stat existing link: %w", err)
	}
	kind, err := Kind(linkPath)
	if err != nil {
		return snap, err
	}
	if kind == KindNone {
		return snap, fmt.Errorf("%w: %s (refusing to replace a plain directory)", ErrNotLink, linkPath)
	}
	target, err := ReadTarget(linkPath)
	if err != nil {
		return snap, fmt.Errorf("read existing link target: %w", err)
	}
	snap.exists = true
	snap.target = target
	return snap, nil
}

func restoreSnapshot(linkPath string, snap linkSnapshot) error {
	_ = Remove(linkPath)
	if !snap.exists {
		return nil
	}
	if err := Create(linkPath, snap.target); err != nil {
		return fmt.Errorf("restore previous link: %w", err)
	}
	return nil
}

// Replace prepares a new link, switches linkPath to target, runs finalize,
// and restores the previous link when switching or finalization fails.
func Replace(linkPath, target string, finalize func() error) error {
	prev, err := snapshotLink(linkPath)
	if err != nil {
		return err
	}

	parent := filepath.Dir(linkPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create link parent directory: %w", err)
	}

	tempPath := linkPath + ".fest-link-new"
	_ = Remove(tempPath)

	if err := Create(tempPath, target); err != nil {
		return err
	}

	if prev.exists {
		if err := Remove(linkPath); err != nil {
			_ = Remove(tempPath)
			return err
		}
	}

	if err := os.Rename(tempPath, linkPath); err != nil {
		_ = Remove(tempPath)
		if restoreErr := restoreSnapshot(linkPath, prev); restoreErr != nil {
			return fmt.Errorf("rename new link: %w (restore failed: %v)", err, restoreErr)
		}
		return fmt.Errorf("rename new link: %w", err)
	}

	if finalize == nil {
		return nil
	}
	if err := finalize(); err != nil {
		if restoreErr := restoreSnapshot(linkPath, prev); restoreErr != nil {
			return fmt.Errorf("%w (restore previous link failed: %v)", err, restoreErr)
		}
		return err
	}
	return nil
}
