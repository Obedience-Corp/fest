//go:build windows

package dirlink

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func create(linkPath, target string) error {
	relative := !filepath.IsAbs(target)
	resolvedTarget := target
	if relative {
		resolvedTarget = filepath.Join(filepath.Dir(linkPath), target)
	}
	absTarget, err := filepath.Abs(resolvedTarget)
	if err != nil {
		return fmt.Errorf("resolve link target: %w", err)
	}
	absTarget = filepath.Clean(absTarget)

	if classifyAbsTarget(absTarget) == targetNetwork || isRemoteDrive(absTarget) {
		return createDirectorySymlink(linkPath, target, relative)
	}
	return createJunction(linkPath, absTarget)
}

func readTarget(linkPath string) (string, error) {
	target, err := os.Readlink(linkPath)
	if err != nil {
		return "", fmt.Errorf("read directory link: %w", err)
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
	if info.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0 {
		return KindNone, nil
	}
	tag, err := reparseTag(linkPath)
	if err != nil {
		return KindNone, fmt.Errorf("read reparse tag: %w", err)
	}
	kind := linkKindFromTag(tag)
	if kind == KindNone {
		return KindNone, nil
	}
	return kind, nil
}

func isRemoteDrive(absTarget string) bool {
	vol := filepath.VolumeName(absTarget)
	if vol == "" {
		return false
	}
	if strings.EqualFold(vol, `\\`) {
		return true
	}
	vol = strings.TrimSuffix(vol, `\`)
	if len(vol) < 2 || vol[1] != ':' {
		return false
	}
	pathPtr, err := syscall.UTF16PtrFromString(vol + `\`)
	if err != nil {
		return false
	}
	driveType := windows.GetDriveType(pathPtr)
	return driveType == windows.DRIVE_REMOTE
}
