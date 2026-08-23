//go:build !dev

package commands

import "testing"

func TestCollectVisibleCommandPathsStableOmitsDevOnlyCommands(t *testing.T) {
	paths := collectVisibleCommandPaths(rootCmd)

	if !containsCommandPath(paths, "fest create") {
		t.Fatal("expected stable command surface to include fest create")
	}

	if containsCommandPath(paths, "fest explore") {
		t.Fatal("stable command surface should not include fest explore")
	}
	if !containsCommandPath(paths, "fest watch") {
		t.Fatal("stable command surface should include fest watch")
	}
	if !containsCommandPath(paths, "fest run") {
		t.Fatal("stable command surface should include fest run")
	}
}
