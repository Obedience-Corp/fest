// Package version provides build-time version information for fest.
// Variables are populated via ldflags during build.
package version

import "runtime"

var (
	// Version is the semantic version (set via ldflags)
	Version = "dev"

	// Commit is the git commit hash (set via ldflags)
	Commit = "unknown"

	// BuildDate is the build timestamp (set via ldflags)
	BuildDate = "unknown"
)

// Info contains all version information
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// Get returns the full version information
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}
