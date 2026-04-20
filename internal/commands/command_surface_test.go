package commands

import (
	"sort"
	"testing"
)

func TestCollectVisibleCommandPathsSorted(t *testing.T) {
	paths := collectVisibleCommandPaths(rootCmd)
	if !sort.StringsAreSorted(paths) {
		t.Fatal("command paths should be sorted")
	}
}

func TestCollectVisibleCommandPathsExcludesHiddenCommands(t *testing.T) {
	paths := collectVisibleCommandPaths(rootCmd)
	for _, hiddenPath := range []string{
		"fest __commands",
		"fest __manifest",
		"fest gendocs",
		"fest help",
	} {
		if containsCommandPath(paths, hiddenPath) {
			t.Fatalf("hidden command %q should not be listed", hiddenPath)
		}
	}
}

func containsCommandPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}
