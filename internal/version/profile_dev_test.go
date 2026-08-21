//go:build dev

package version

import "testing"

func TestProfileDev(t *testing.T) {
	if Profile != "dev" {
		t.Fatalf("Profile = %q, want %q", Profile, "dev")
	}
	if Get().Profile != "dev" {
		t.Fatalf("Get().Profile = %q, want %q", Get().Profile, "dev")
	}
}

func TestDevProfileForcesDevChannelOnStableVersion(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	Version = "v0.6.2-5-gabcdef0"
	if !IsDevBuild() {
		t.Fatalf("IsDevBuild() = false, want true for a dev-profile build")
	}
	if got := DefaultChannel(); got != "dev" {
		t.Fatalf("DefaultChannel() = %q, want %q", got, "dev")
	}
}
