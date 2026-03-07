//go:build !dev

package commands

import "testing"

func TestReleaseProfileStable_ExploreCommandNotRegistered(t *testing.T) {
	t.Helper()
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "explore" {
			t.Fatal("explore command should not be registered in stable profile")
		}
	}
}
