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
		"fest research",
		"fest research create",
		"fest research summary",
		"fest research link",
		"fest templates",
		"fest templates create",
		"fest templates apply",
		"fest templates list",
		"fest index write",
		"fest index validate",
		"fest index show",
		"fest index tree",
		"fest index diff",
		"fest config theme",
		"fest config theme show",
		"fest config theme set",
		"fest config theme test",
		"fest understand chains",
		"fest understand context",
		"fest understand lifecycle",
		"fest understand loop",
		"fest understand nodeids",
		"fest understand planning",
		"fest understand resources",
		"fest understand rituals",
		"fest understand roles",
	} {
		if containsCommandPath(paths, hiddenPath) {
			t.Fatalf("hidden command %q should not be listed", hiddenPath)
		}
	}
}

func TestCollectVisibleCommandPathsKeepsIndexParentAndHotUnderstandChildren(t *testing.T) {
	paths := collectVisibleCommandPaths(rootCmd)
	for _, wantPath := range []string{
		"fest index",
		"fest understand tasks",
		"fest understand structure",
		"fest understand rules",
		"fest understand templates",
	} {
		if !containsCommandPath(paths, wantPath) {
			t.Fatalf("expected command %q to remain visible", wantPath)
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
