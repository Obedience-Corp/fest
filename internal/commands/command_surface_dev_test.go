//go:build dev

package commands

import "testing"

func TestCollectVisibleCommandPathsDevIncludesDevOnlyCommands(t *testing.T) {
	paths := collectVisibleCommandPaths(rootCmd)

	if !containsCommandPath(paths, "fest explore") {
		t.Fatal("dev command surface should include fest explore")
	}
	if !containsCommandPath(paths, "fest watch") {
		t.Fatal("dev command surface should include stable fest watch")
	}
}
