package chain

import (
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"gopkg.in/yaml.v3"
)

// Write marshals a Chain to YAML and writes it atomically to path. It renders a
// clean document from the struct (dropping any explanatory comments the create
// template added) and replaces the destination via a temp file in the same
// directory followed by an atomic rename, so a failed write never leaves a
// partial or corrupt chain file in place.
func Write(path string, c *Chain) error {
	if c == nil {
		return errors.Validation("cannot write nil chain")
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return errors.Wrap(err, "marshaling chain").WithCode(errors.ErrCodeParse)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".chain-*.yaml.tmp")
	if err != nil {
		return errors.IO("creating temp chain file", err).WithField("dir", dir)
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return errors.IO("writing temp chain file", err).WithField("path", tmpPath)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return errors.IO("syncing temp chain file", err).WithField("path", tmpPath)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return errors.IO("closing temp chain file", err).WithField("path", tmpPath)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return errors.IO("replacing chain file", err).WithField("path", path)
	}

	return nil
}
