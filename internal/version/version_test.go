package version

import (
	"encoding/json"
	"strings"
	"testing"
)

// devProfile reports whether this test binary was compiled with the dev build
// profile, so the table cases below assert the same behaviour under both
// `go test ./...` and `go test -tags dev ./...`.
func devProfile() bool { return Profile == "dev" }

func TestIsDevBuild(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		wantWhenStable bool
	}{
		{"empty version", "", false},
		{"stable release", "v0.2.0", false},
		{"stable patch release", "v1.0.3", false},
		{"rc build (not dev)", "v0.2.0-rc.1", false},
		{"describe output off an untagged commit", "v0.6.2-5-gabcdef0", false},
		{"describe output with dirty worktree", "v0.6.2-5-gabcdef0-dirty", false},
		{"describe output with no tags in range", "abcdef0", false},
		{"default dev sentinel", "dev", true},
		{"pre-release dev suffix", "v0.2.0-dev.3", true},
		{"dev suffix with zeroes", "v0.0.1-dev.0", true},
		{"describe output off a dev tag", "v0.2.0-dev.3-5-gabcdef0", true},
	}

	original := Version
	defer func() { Version = original }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.version
			want := tt.wantWhenStable || devProfile()
			if got := IsDevBuild(); got != want {
				t.Errorf("IsDevBuild() = %v, want %v (Version=%q, Profile=%q)", got, want, tt.version, Profile)
			}
		})
	}
}

func TestDefaultChannel(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		wantWhenStable string
	}{
		{"empty version returns stable channel", "", "stable"},
		{"stable returns stable channel", "v0.2.0", "stable"},
		{"stable patch returns stable channel", "v1.0.3", "stable"},
		{"rc returns stable channel (not a dev build)", "v0.2.0-rc.1", "stable"},
		{"describe output returns stable channel", "v0.6.2-5-gabcdef0", "stable"},
		{"dirty describe output returns stable channel", "v0.6.2-5-gabcdef0-dirty", "stable"},
		{"dev sentinel returns dev channel", "dev", "dev"},
		{"pre-release dev returns dev channel", "v0.2.0-dev.3", "dev"},
		{"describe off a dev tag returns dev channel", "v0.2.0-dev.3-5-gabcdef0", "dev"},
	}

	original := Version
	defer func() { Version = original }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.version
			want := tt.wantWhenStable
			if devProfile() {
				want = "dev"
			}
			if got := DefaultChannel(); got != want {
				t.Errorf("DefaultChannel() = %q, want %q (Version=%q, Profile=%q)", got, want, tt.version, Profile)
			}
		})
	}
}

func TestGetCarriesBundleAndProfile(t *testing.T) {
	tests := []struct {
		name   string
		bundle string
	}{
		{"unstamped bundle stays empty", ""},
		{"whitespace bundle is carried verbatim", " "},
		{"festival release bundle", "v0.2.17"},
	}

	original := Bundle
	defer func() { Bundle = original }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Bundle = tt.bundle
			info := Get()
			if info.Bundle != tt.bundle {
				t.Errorf("Get().Bundle = %q, want %q", info.Bundle, tt.bundle)
			}
			if info.Profile != Profile {
				t.Errorf("Get().Profile = %q, want %q", info.Profile, Profile)
			}
		})
	}
}

func TestInfoJSONOmitsEmptyBundle(t *testing.T) {
	tests := []struct {
		name          string
		bundle        string
		wantBundleKey bool
	}{
		{"empty bundle is omitted", "", false},
		{"stamped bundle is present", "v0.2.17", true},
	}

	original := Bundle
	defer func() { Bundle = original }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Bundle = tt.bundle
			encoded, err := json.Marshal(Get())
			if err != nil {
				t.Fatalf("marshal version info: %v", err)
			}

			var decoded map[string]any
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("unmarshal version info: %v", err)
			}

			if _, ok := decoded["bundle"]; ok != tt.wantBundleKey {
				t.Errorf("bundle key present = %v, want %v (json=%s)", ok, tt.wantBundleKey, encoded)
			}
			if tt.wantBundleKey && decoded["bundle"] != tt.bundle {
				t.Errorf("bundle = %v, want %q", decoded["bundle"], tt.bundle)
			}

			profile, ok := decoded["profile"]
			if !ok {
				t.Fatalf("profile key missing from %s", encoded)
			}
			if profile != Profile {
				t.Errorf("profile = %v, want %q", profile, Profile)
			}
			if !strings.Contains(string(encoded), `"version"`) {
				t.Errorf("version key missing from %s", encoded)
			}
		})
	}
}
