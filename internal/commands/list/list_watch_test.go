package list

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/id"
)

func TestListWatchPathsIncludeStatusBuckets(t *testing.T) {
	root := "/tmp/festivals-workspace"
	paths := listWatchPaths(root)

	want := map[string]bool{
		root: true,
		filepath.Join(root, "dungeon"): true,
	}
	for _, status := range id.StatusDirectories {
		want[filepath.Join(root, status)] = true
	}

	got := make(map[string]bool, len(paths))
	for _, p := range paths {
		got[p] = true
	}
	for p := range want {
		if !got[p] {
			t.Errorf("listWatchPaths missing %q", p)
		}
	}
}

func TestFormatAllHumanEmpty(t *testing.T) {
	out := formatAllHuman(nil, nil, nil, 0, false)
	if !strings.Contains(out, "No festivals found") {
		t.Fatalf("expected empty-board message, got %q", out)
	}
	if !strings.Contains(out, "fest create festival") {
		t.Fatalf("expected create hint, got %q", out)
	}
}

func TestFormatDungeonHumanEmpty(t *testing.T) {
	out := formatDungeonHuman(nil, nil, nil, 0, false)
	if !strings.Contains(out, "No festivals in dungeon") {
		t.Fatalf("expected empty dungeon message, got %q", out)
	}
}
