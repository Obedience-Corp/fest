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

// Festival marker filenames. A directory carrying any of these is fest's.
const (
	FestivalGoalFile     = "FESTIVAL_GOAL.md"
	FestivalOverviewFile = "FESTIVAL_OVERVIEW.md"
	FestivalConfigFile   = "fest.yaml"
)

// DirKind classifies a directory found under a festivals/ lifecycle folder.
type DirKind int

const (
	// KindNeither is a directory with no festival markers and no workitem marker.
	KindNeither DirKind = iota
	// KindFestival is fest's own directory.
	KindFestival
	// KindResident is a workitem camp promoted onto the rail.
	KindResident
)

func (k DirKind) String() string {
	switch k {
	case KindFestival:
		return "festival"
	case KindResident:
		return "resident"
	default:
		return "neither"
	}
}

// IsFestivalDir reports whether dir carries a festival marker. This is the single
// implementation; scope and the command packages delegate here.
func IsFestivalDir(dir string) bool {
	for _, name := range []string{FestivalGoalFile, FestivalOverviewFile, FestivalConfigFile} {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// Classify decides what a lifecycle directory is. The workitem marker wins over
// festival markers: camp writes .workitem only when it has actually promoted the
// directory, whereas a stale fest.yaml proves nothing about the current owner.
func Classify(dir string) DirKind {
	if IsResident(dir) {
		return KindResident
	}
	if IsFestivalDir(dir) {
		return KindFestival
	}
	return KindNeither
}
