// Package dirlink creates and removes Fest-managed directory links.
//
// On Unix, links are directory symlinks. On Windows, local targets use junctions
// (IO_REPARSE_TAG_MOUNT_POINT); UNC/network targets use directory symlinks when
// permitted. Safe removal deletes only the link entry, never the target tree.
package dirlink

import (
	"errors"
	"fmt"
	"os"
)

// ErrNotLink means the path is not a supported directory link.
var ErrNotLink = errors.New("not a directory link")

// LinkKind classifies a managed directory link.
type LinkKind int

const (
	KindNone LinkKind = iota
	KindSymlink
	KindJunction
)

// Create creates a directory link at linkPath pointing at target.
// Target may be relative to linkPath's directory.
func Create(linkPath, target string) error {
	if linkPath == "" || target == "" {
		return fmt.Errorf("link path and target are required")
	}
	return create(linkPath, target)
}

// Remove deletes linkPath when it is a supported directory link.
func Remove(linkPath string) error {
	kind, err := Kind(linkPath)
	if err != nil {
		return err
	}
	if kind == KindNone {
		return fmt.Errorf("%w: %s (refusing to delete a plain directory)", ErrNotLink, linkPath)
	}
	if err := os.Remove(linkPath); err != nil {
		return fmt.Errorf("remove directory link %s: %w", linkPath, err)
	}
	return nil
}

// ReadTarget returns the stored link target (relative or absolute as written).
func ReadTarget(linkPath string) (string, error) {
	return readTarget(linkPath)
}

// Kind reports whether path is a Fest-managed directory link.
func Kind(linkPath string) (LinkKind, error) {
	return linkKind(linkPath)
}
