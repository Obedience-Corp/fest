//go:build dev

package commands

import "testing"

func TestCollectVisibleCommandPathsDevIncludesDevOnlyCommands(t *testing.T) {
	paths := collectVisibleCommandPaths(rootCmd)

	if !containsCommandPath(paths, "fest explore") {
		t.Fatal("dev command surface should include fest explore")
	}
}
