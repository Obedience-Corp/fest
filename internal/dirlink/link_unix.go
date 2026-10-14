//go:build !windows

package dirlink

import (
	"fmt"
	"os"
)

func create(linkPath, target string) error {
	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("create directory symlink: %w", err)
	}
	return nil
}

func readTarget(linkPath string) (string, error) {
	target, err := os.Readlink(linkPath)
	if err != nil {
		return "", fmt.Errorf("read directory symlink: %w", err)
	}
	return target, nil
}

func linkKind(linkPath string) (LinkKind, error) {
	info, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return KindNone, err
		}
		return KindNone, fmt.Errorf("stat link: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return KindSymlink, nil
	}
	return KindNone, nil
}
