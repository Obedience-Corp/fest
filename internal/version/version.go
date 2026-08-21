// Package version provides build-time version information for fest.
// Variables are populated via ldflags during build.
package version

import (
	"runtime"
	"strings"

	"github.com/Obedience-Corp/fest/internal/features"
)

var (
	// Version is the semantic version (set via ldflags)
	Version = "dev"

	// Commit is the git commit hash (set via ldflags)
	Commit = "unknown"

	// BuildDate is the build timestamp (set via ldflags)
	BuildDate = "unknown"

	// Bundle is the festival suite version this binary shipped in (set via
	// ldflags by the festival release build). It is empty for binaries built
	// outside a festival bundle.
	Bundle = ""
)

// Info contains all version information
type Info struct {
	Version   string   `json:"version"`
	Bundle    string   `json:"bundle,omitempty"`
	Commit    string   `json:"commit"`
	BuildDate string   `json:"buildDate"`
	GoVersion string   `json:"goVersion"`
	Platform  string   `json:"platform"`
	Profile   string   `json:"profile"`
	Features  []string `json:"features"`
}

// IsDevBuild reports whether this is a development build.
// A dev build is compiled with the dev build profile, or has Version equal to
// "dev", or has a version containing "-dev." (e.g. v0.2.0-dev.3).
func IsDevBuild() bool {
	return Profile == "dev" || Version == "dev" || strings.Contains(Version, "-dev.")
}

// DefaultChannel returns the default release channel for this build.
// Dev builds default to "dev"; stable builds default to "stable".
func DefaultChannel() string {
	if IsDevBuild() {
		return "dev"
	}
	return "stable"
}

// Get returns the full version information
func Get() Info {
	return Info{
		Version:   Version,
		Bundle:    Bundle,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		Profile:   Profile,
		Features:  features.Supported(),
	}
}
