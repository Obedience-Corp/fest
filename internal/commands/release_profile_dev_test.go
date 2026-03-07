//go:build dev

package commands

import "testing"

func TestReleaseProfileDev_ExploreCommandRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"explore"})
	if err != nil {
		t.Fatalf("expected explore command in dev profile: %v", err)
	}
	if cmd == nil || cmd.Name() != "explore" {
		t.Fatalf("expected explore command, got %#v", cmd)
	}
	if got := cmd.Annotations[annotationReleaseChannel]; got != releaseChannelDevOnly {
		t.Fatalf("explore release_channel = %q, want %q", got, releaseChannelDevOnly)
	}
}
