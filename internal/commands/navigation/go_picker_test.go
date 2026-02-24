package navigation

import (
	"os"
	"path/filepath"
	"testing"
)

func createPickerTestWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	festivalsDir := filepath.Join(root, "festivals")

	for _, status := range []string{"active", "planning", "dungeon/completed"} {
		os.MkdirAll(filepath.Join(festivalsDir, status), 0755)
	}

	os.MkdirAll(filepath.Join(festivalsDir, "active", "deploy-DS0001"), 0755)
	os.MkdirAll(filepath.Join(festivalsDir, "active", "auth-AI0001"), 0755)
	os.MkdirAll(filepath.Join(festivalsDir, "planning", "search-SR0001"), 0755)
	os.MkdirAll(filepath.Join(festivalsDir, "dungeon", "completed", "old-OL0001"), 0755)

	return festivalsDir
}

func TestCollectPickerItems(t *testing.T) {
	festivalsDir := createPickerTestWorkspace(t)

	items := collectPickerItems(festivalsDir)

	// 3 status dirs (active/, planning/, dungeon/completed/) + 4 festivals = 7
	if len(items) != 7 {
		names := make([]string, len(items))
		for i, item := range items {
			names[i] = item.Name
		}
		t.Errorf("got %d items %v, want 7", len(items), names)
	}

	// Verify status directory entries
	wantDirs := map[string]bool{
		"[active]/":            false,
		"[planning]/":          false,
		"[dungeon/completed]/": false,
	}
	for _, item := range items {
		if _, ok := wantDirs[item.Name]; ok {
			wantDirs[item.Name] = true
		}
	}
	for name, found := range wantDirs {
		if !found {
			t.Errorf("missing directory entry: %s", name)
		}
	}

	// Verify festival entries have correct format and paths
	wantFests := map[string]string{
		"[active] deploy-DS0001":          filepath.Join(festivalsDir, "active", "deploy-DS0001"),
		"[active] auth-AI0001":            filepath.Join(festivalsDir, "active", "auth-AI0001"),
		"[planning] search-SR0001":        filepath.Join(festivalsDir, "planning", "search-SR0001"),
		"[dungeon/completed] old-OL0001":  filepath.Join(festivalsDir, "dungeon", "completed", "old-OL0001"),
	}
	for _, item := range items {
		if expectedPath, ok := wantFests[item.Name]; ok {
			if item.Value != expectedPath {
				t.Errorf("item %q path = %q, want %q", item.Name, item.Value, expectedPath)
			}
			delete(wantFests, item.Name)
		}
	}
	for name := range wantFests {
		t.Errorf("missing festival entry: %s", name)
	}
}

func TestCollectPickerItems_EmptyWorkspace(t *testing.T) {
	root := t.TempDir()
	festivalsDir := filepath.Join(root, "festivals")

	os.MkdirAll(filepath.Join(festivalsDir, "active"), 0755)
	os.MkdirAll(filepath.Join(festivalsDir, "planning"), 0755)

	items := collectPickerItems(festivalsDir)

	for _, item := range items {
		if item.Name[len(item.Name)-1] != '/' {
			t.Errorf("expected only directory entries in empty workspace, got %q", item.Name)
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
	os.MkdirAll(filepath.Join(festivalsDir, "active", "only-fest-OF0001"), 0755)

	items := collectPickerItems(festivalsDir)

	festCount := 0
	for _, item := range items {
		if item.Name == "[active] only-fest-OF0001" {
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
	os.MkdirAll(filepath.Join(festivalsDir, "active"), 0755)
	// Create a regular file (should be skipped)
	os.WriteFile(filepath.Join(festivalsDir, "active", "README.md"), []byte("hi"), 0644)
	// Create a festival directory (should be included)
	os.MkdirAll(filepath.Join(festivalsDir, "active", "real-fest-RF0001"), 0755)

	items := collectPickerItems(festivalsDir)

	for _, item := range items {
		if item.Name == "[active] README.md" {
			t.Error("regular files should be skipped by collectPickerItems")
		}
	}
}
