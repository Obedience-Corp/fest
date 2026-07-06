package navigation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createPickerTestWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	festivalsDir := filepath.Join(root, "festivals")

	for _, status := range []string{"active", "planning", "dungeon/completed"} {
		_ = os.MkdirAll(filepath.Join(festivalsDir, status), 0755)
	}

	_ = os.MkdirAll(filepath.Join(festivalsDir, "active", "deploy-DS0001"), 0755)
	_ = os.MkdirAll(filepath.Join(festivalsDir, "active", "auth-AI0001"), 0755)
	_ = os.MkdirAll(filepath.Join(festivalsDir, "planning", "search-SR0001"), 0755)
	_ = os.MkdirAll(filepath.Join(festivalsDir, "dungeon", "completed", "old-OL0001"), 0755)

	return festivalsDir
}

func TestCollectPickerItems(t *testing.T) {
	festivalsDir := createPickerTestWorkspace(t)

	items := collectPickerItems(festivalsDir)

	// 3 status dirs (active/, planning/, dungeon/completed/) + 4 festivals = 7
	if len(items) != 7 {
		t.Errorf("got %d items, want 7", len(items))
	}

	// The status label lives in Prefix; the festival name in Name (empty for a
	// status directory). Reconstruct the combined label for lookups.
	labels := make(map[string]string, len(items))
	for _, item := range items {
		label := item.Prefix
		if item.Name != "" {
			label = item.Prefix + " " + item.Name
		}
		labels[label] = item.Value
	}

	for _, dir := range []string{"[active]/", "[planning]/", "[dungeon/completed]/"} {
		if _, ok := labels[dir]; !ok {
			t.Errorf("missing directory entry: %s", dir)
		}
	}

	wantFests := map[string]string{
		"[active] deploy-DS0001":         filepath.Join(festivalsDir, "active", "deploy-DS0001"),
		"[active] auth-AI0001":           filepath.Join(festivalsDir, "active", "auth-AI0001"),
		"[planning] search-SR0001":       filepath.Join(festivalsDir, "planning", "search-SR0001"),
		"[dungeon/completed] old-OL0001": filepath.Join(festivalsDir, "dungeon", "completed", "old-OL0001"),
	}
	for label, wantPath := range wantFests {
		gotPath, ok := labels[label]
		if !ok {
			t.Errorf("missing festival entry: %s", label)
			continue
		}
		if gotPath != wantPath {
			t.Errorf("item %q path = %q, want %q", label, gotPath, wantPath)
		}
	}
}

func TestCollectPickerItems_EmptyWorkspace(t *testing.T) {
	root := t.TempDir()
	festivalsDir := filepath.Join(root, "festivals")

	_ = os.MkdirAll(filepath.Join(festivalsDir, "active"), 0755)
	_ = os.MkdirAll(filepath.Join(festivalsDir, "planning"), 0755)

	items := collectPickerItems(festivalsDir)

	for _, item := range items {
		if !strings.HasSuffix(item.Prefix, "/") {
			t.Errorf("expected only directory entries in empty workspace, got prefix %q name %q", item.Prefix, item.Name)
		}
	}
}

func TestCollectPickerItems_NonexistentDir(t *testing.T) {
	items := collectPickerItems("/nonexistent/festivals")
	if len(items) != 0 {
		t.Errorf("expected empty list for nonexistent dir, got %d items", len(items))
	}
}

func TestCollectPickerItems_SingleFestival(t *testing.T) {
	root := t.TempDir()
	festivalsDir := filepath.Join(root, "festivals")
	_ = os.MkdirAll(filepath.Join(festivalsDir, "active", "only-fest-OF0001"), 0755)

	items := collectPickerItems(festivalsDir)

	festCount := 0
	for _, item := range items {
		if item.Prefix == "[active]" && item.Name == "only-fest-OF0001" {
			festCount++
		}
	}
	if festCount != 1 {
		t.Errorf("expected 1 festival item, got %d", festCount)
	}
}

func TestCollectPickerItems_SkipsFiles(t *testing.T) {
	root := t.TempDir()
	festivalsDir := filepath.Join(root, "festivals")
	_ = os.MkdirAll(filepath.Join(festivalsDir, "active"), 0755)
	// Create a regular file (should be skipped)
	_ = os.WriteFile(filepath.Join(festivalsDir, "active", "README.md"), []byte("hi"), 0644)
	// Create a festival directory (should be included)
	_ = os.MkdirAll(filepath.Join(festivalsDir, "active", "real-fest-RF0001"), 0755)

	items := collectPickerItems(festivalsDir)

	for _, item := range items {
		if item.Name == "README.md" {
			t.Error("regular files should be skipped by collectPickerItems")
		}
	}
}
