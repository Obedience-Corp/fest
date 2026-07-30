// Package resident provides a read-only reader for camp's .workitem marker.
//
// Decision D007: the .workitem marker is owned by camp. Fest reads it to
// recognize a lifecycle resident but never writes it, mirroring camp reading
// fest's .workflow/ runtime read-only. Camp and fest are separate Go modules, so
// this is a thin duplicate of the fields fest needs; camp is the source of truth
// for the schema, which is why the version is not validated here.
package resident

import (
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"gopkg.in/yaml.v3"
)

// MarkerFilename is camp's workitem marker sidecar.
const MarkerFilename = ".workitem"

const markerKind = "workitem"

// Marker is the subset of camp's .workitem that fest needs to recognize and
// label a resident. Unknown fields are ignored so a newer camp schema does not
// break fest.
type Marker struct {
	Kind  string `yaml:"kind"`
	ID    string `yaml:"id"`
	Type  string `yaml:"type"`
	Title string `yaml:"title"`
}

// Read loads <dir>/.workitem. Returns (nil, nil) when the marker is absent or
// carries some other kind, and an error only when the file exists but cannot be
// read or parsed.
func Read(dir string) (*Marker, error) {
	path := filepath.Join(dir, MarkerFilename)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.IO("reading workitem marker", err).WithField("path", path)
	}

	var m Marker
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, errors.Wrap(err, "parsing workitem marker").WithField("path", path)
	}
	if m.Kind != markerKind {
		return nil, nil
	}
	return &m, nil
}

// IsResident reports whether dir carries a camp .workitem marker. An unreadable
// marker reports false; callers that need to surface the problem use Read.
func IsResident(dir string) bool {
	m, err := Read(dir)
	return err == nil && m != nil
}
