package version

import "testing"

func TestIsDevBuild(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"default dev sentinel", "dev", true},
		{"pre-release dev suffix", "v0.2.0-dev.3", true},
		{"dev suffix with zeroes", "v0.0.1-dev.0", true},
		{"stable release", "v0.2.0", false},
		{"stable patch release", "v1.0.3", false},
		{"rc build (not dev)", "v0.2.0-rc.1", false},
		{"empty version", "", false},
	}

	original := Version
	defer func() { Version = original }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.version
			got := IsDevBuild()
			if got != tt.want {
				t.Errorf("IsDevBuild() = %v, want %v (Version=%q)", got, tt.want, tt.version)
			}
		})
	}
}

func TestDefaultChannel(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"dev sentinel returns dev channel", "dev", "dev"},
		{"pre-release dev returns dev channel", "v0.2.0-dev.3", "dev"},
		{"stable returns stable channel", "v0.2.0", "stable"},
		{"stable patch returns stable channel", "v1.0.3", "stable"},
		{"rc returns stable channel (not a dev build)", "v0.2.0-rc.1", "stable"},
	}

	original := Version
	defer func() { Version = original }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.version
			got := DefaultChannel()
			if got != tt.want {
				t.Errorf("DefaultChannel() = %q, want %q (Version=%q)", got, tt.want, tt.version)
			}
		})
	}
}
