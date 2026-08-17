// Package bundled materializes the methodology scaffold embedded in the fest
// binary onto disk, for machines that cannot reach GitHub.
package bundled

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/methodology"
)

const (
	dirPerm  = 0o755
	filePerm = 0o644
)

// Seed writes the embedded methodology scaffold into targetDir, creating it if
// needed. Existing files are overwritten, so a caller that wants to preserve
// local edits must check before calling.
//
// Embedded files carry no mode bits, so directories are created 0755 and files
// written 0644 — the scaffold is documents and templates, nothing executable.
func Seed(ctx context.Context, targetDir string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "seeding bundled methodology")
	}
	if targetDir == "" {
		return errors.Validation("seeding bundled methodology: no target directory")
	}

	return fs.WalkDir(methodology.FS, methodology.Root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		rel, err := filepath.Rel(methodology.Root, path)
		if err != nil {
			return errors.Wrap(err, "resolving bundled path").WithField("path", path)
		}
		dest := filepath.Join(targetDir, rel)

		if d.IsDir() {
			if err := os.MkdirAll(dest, dirPerm); err != nil {
				return errors.IO("creating directory", err).WithField("path", dest)
			}
			return nil
		}

		data, err := methodology.FS.ReadFile(path)
		if err != nil {
			return errors.IO("reading bundled file", err).WithField("path", path)
		}
		// A file can precede its parent only if the walk is reordered, but
		// creating the parent here costs nothing and removes the assumption.
		if err := os.MkdirAll(filepath.Dir(dest), dirPerm); err != nil {
			return errors.IO("creating directory", err).WithField("path", filepath.Dir(dest))
		}
		if err := os.WriteFile(dest, data, filePerm); err != nil {
			return errors.IO("writing bundled file", err).WithField("path", dest)
		}
		return nil
	})
}
