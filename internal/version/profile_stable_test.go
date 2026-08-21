//go:build !dev

package version

import "testing"

func TestProfileStable(t *testing.T) {
	if Profile != "stable" {
		t.Fatalf("Profile = %q, want %q", Profile, "stable")
	}
	if Get().Profile != "stable" {
		t.Fatalf("Get().Profile = %q, want %q", Get().Profile, "stable")
	}
}

func TestStableProfileKeepsDevTagsOnDevChannel(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	Version = "v0.2.0-dev.3"
	if !IsDevBuild() {
		t.Fatalf("IsDevBuild() = false, want true for a -dev. version")
	}
	if got := DefaultChannel(); got != "dev" {
		t.Fatalf("DefaultChannel() = %q, want %q", got, "dev")
	}
}
