package festival

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
)

// writeMinimalCoreTemplates writes the four required core festival templates
// (OVERVIEW, GOAL, RULES, TODO) into .festival/templates/festival/ under the
// given festivals root. Tests that exercise the real create pipeline need these
// because missing core templates are now a hard error (fest#139).
func writeMinimalCoreTemplates(t *testing.T, festivalsRoot string) {
	t.Helper()
	dir := filepath.Join(festivalsRoot, ".festival", "templates", "festival")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create festival templates dir: %v", err)
	}
	for _, name := range []string{"OVERVIEW.md", "GOAL.md", "RULES.md", "TODO.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n"), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}
}

// TestCreateFestival_DirectoryNaming verifies that festival directories
// are created with the format {slug}-{ID} where ID is XX0001 format.
func TestCreateFestival_DirectoryNaming(t *testing.T) {
	tests := []struct {
		name           string
		festivalName   string
		expectedPrefix string // Expected 2-letter prefix
	}{
		{
			name:           "two word name",
			festivalName:   "my project",
			expectedPrefix: "MP",
		},
		{
			name:           "hyphenated name",
			festivalName:   "guild-usable",
			expectedPrefix: "GU",
		},
		{
			name:           "single word",
			festivalName:   "onboarding",
			expectedPrefix: "ON",
		},
		{
			name:           "three word name",
			festivalName:   "fest node ids",
			expectedPrefix: "FN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create festivals directory structure
			festivalsRoot := filepath.Join(tmpDir, "festivals")
			for _, status := range []string{"planning", "active", "dungeon"} {
				if err := os.MkdirAll(filepath.Join(festivalsRoot, status), 0755); err != nil {
					t.Fatalf("Failed to create status dir: %v", err)
				}
			}
			// Create dungeon/completed subdirectory
			if err := os.MkdirAll(filepath.Join(festivalsRoot, "dungeon", "completed"), 0755); err != nil {
				t.Fatalf("Failed to create dungeon/completed dir: %v", err)
			}

			// Create minimal .festival/templates directory with core templates
			writeMinimalCoreTemplates(t, festivalsRoot)

			// Change to festivals directory
			origDir, _ := os.Getwd()
			if err := os.Chdir(festivalsRoot); err != nil {
				t.Fatalf("Failed to chdir: %v", err)
			}
			defer func() { _ = os.Chdir(origDir) }()

			// Run create festival
			opts := &CreateFestivalOptions{
				Name:        tt.festivalName,
				Dest:        "planning",
				SkipMarkers: true,
				JSONOutput:  true, // Suppress console output
			}

			err := RunCreateFestival(context.Background(), opts)
			if err != nil {
				t.Fatalf("RunCreateFestival failed: %v", err)
			}

			// Verify directory was created with ID suffix
			slug := Slugify(tt.festivalName)
			planningDir := filepath.Join(festivalsRoot, "planning")
			entries, err := os.ReadDir(planningDir)
			if err != nil {
				t.Fatalf("Failed to read planning dir: %v", err)
			}

			if len(entries) != 1 {
				t.Fatalf("Expected 1 entry in planning/, got %d", len(entries))
			}

			dirName := entries[0].Name()

			// Directory should start with slug
			if !strings.HasPrefix(dirName, slug) {
				t.Errorf("Directory %q should start with slug %q", dirName, slug)
			}

			// Directory should have hyphen separator before ID suffix
			// Note: slug may contain hyphens, so we look for the ID pattern at the end
			if !strings.Contains(dirName, "-") {
				t.Errorf("Directory %q should contain hyphen separator", dirName)
			}

			// Extract the ID suffix (last 6 chars after final hyphen with uppercase letters)
			// Format: slug-XX0001, where XX are uppercase letters
			lastHyphen := strings.LastIndex(dirName, "-")
			if lastHyphen == -1 || lastHyphen == len(dirName)-1 {
				t.Fatalf("Directory %q should have format {slug}-{id}", dirName)
			}
			idPart := dirName[lastHyphen+1:]

			// Verify it looks like an ID (starts with 2 uppercase letters)
			if len(idPart) < 2 || idPart[0] < 'A' || idPart[0] > 'Z' {
				t.Fatalf("Directory %q should end with ID in XX0001 format", dirName)
			}

			// ID should be 6 characters: 2 letters + 4 digits
			if len(idPart) != 6 {
				t.Errorf("ID %q should be 6 characters (XX0001 format)", idPart)
			}

			// First two characters should be the expected prefix
			if !strings.HasPrefix(idPart, tt.expectedPrefix) {
				t.Errorf("ID %q should start with prefix %q", idPart, tt.expectedPrefix)
			}

			// Last 4 characters should be digits (0001)
			counter := idPart[2:]
			if counter != "0001" {
				t.Errorf("First festival should have counter 0001, got %q", counter)
			}
		})
	}
}

// TestCreateFestival_MetadataPopulation verifies that fest.yaml
// includes the metadata section with ID, UUID, name, and timestamps.
func TestCreateFestival_MetadataPopulation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals directory structure
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	for _, status := range []string{"planning", "active", "dungeon"} {
		if err := os.MkdirAll(filepath.Join(festivalsRoot, status), 0755); err != nil {
			t.Fatalf("Failed to create status dir: %v", err)
		}
	}
	// Create dungeon/completed subdirectory
	if err := os.MkdirAll(filepath.Join(festivalsRoot, "dungeon", "completed"), 0755); err != nil {
		t.Fatalf("Failed to create dungeon/completed dir: %v", err)
	}

	// Create minimal .festival/templates directory with core templates
	writeMinimalCoreTemplates(t, festivalsRoot)

	// Change to festivals directory
	origDir, _ := os.Getwd()
	if err := os.Chdir(festivalsRoot); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Run create festival
	opts := &CreateFestivalOptions{
		Name:        "test festival",
		Goal:        "Test the metadata population",
		Dest:        "planning",
		SkipMarkers: true,
		JSONOutput:  true,
	}

	err := RunCreateFestival(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunCreateFestival failed: %v", err)
	}

	// Find the created directory
	planningDir := filepath.Join(festivalsRoot, "planning")
	entries, err := os.ReadDir(planningDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("Expected 1 entry in planning/")
	}

	festivalDir := filepath.Join(planningDir, entries[0].Name())

	// Load the fest.yaml
	cfg, err := config.LoadFestivalConfig(festivalDir, "")
	if err != nil {
		t.Fatalf("Failed to load fest.yaml: %v", err)
	}

	// Verify metadata fields
	if cfg.Metadata.ID == "" {
		t.Error("Metadata.ID should not be empty")
	}

	if cfg.Metadata.UUID == "" {
		t.Error("Metadata.UUID should not be empty")
	}

	if cfg.Metadata.Name == "" {
		t.Error("Metadata.Name should not be empty")
	}

	if cfg.Metadata.Name != "test festival" {
		t.Errorf("Metadata.Name = %q, want %q", cfg.Metadata.Name, "test festival")
	}

	if cfg.Metadata.CreatedAt.IsZero() {
		t.Error("Metadata.CreatedAt should not be zero")
	}

	// Verify status history
	if len(cfg.Metadata.StatusHistory) == 0 {
		t.Error("Metadata.StatusHistory should have at least one entry")
	}

	if len(cfg.Metadata.StatusHistory) > 0 {
		firstChange := cfg.Metadata.StatusHistory[0]
		if firstChange.Status != "planning" {
			t.Errorf("First status should be 'planning', got %q", firstChange.Status)
		}
		if firstChange.Timestamp.IsZero() {
			t.Error("Status change timestamp should not be zero")
		}
	}
}

// TestCreateFestival_UniqueIDs verifies that multiple festivals
// get unique IDs with incrementing counters.
func TestCreateFestival_UniqueIDs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals directory structure
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	for _, status := range []string{"planning", "active", "dungeon"} {
		if err := os.MkdirAll(filepath.Join(festivalsRoot, status), 0755); err != nil {
			t.Fatalf("Failed to create status dir: %v", err)
		}
	}
	// Create dungeon/completed subdirectory
	if err := os.MkdirAll(filepath.Join(festivalsRoot, "dungeon", "completed"), 0755); err != nil {
		t.Fatalf("Failed to create dungeon/completed dir: %v", err)
	}

	// Create minimal .festival/templates directory with core templates
	writeMinimalCoreTemplates(t, festivalsRoot)

	// Change to festivals directory
	origDir, _ := os.Getwd()
	if err := os.Chdir(festivalsRoot); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create first festival with "GU" prefix
	opts1 := &CreateFestivalOptions{
		Name:        "guild usable",
		Dest:        "planning",
		SkipMarkers: true,
		JSONOutput:  true,
	}
	if err := RunCreateFestival(context.Background(), opts1); err != nil {
		t.Fatalf("First RunCreateFestival failed: %v", err)
	}

	// Create second festival with same "GU" prefix
	opts2 := &CreateFestivalOptions{
		Name:        "guild ui",
		Dest:        "planning",
		SkipMarkers: true,
		JSONOutput:  true,
	}
	if err := RunCreateFestival(context.Background(), opts2); err != nil {
		t.Fatalf("Second RunCreateFestival failed: %v", err)
	}

	// Create third festival with different prefix
	opts3 := &CreateFestivalOptions{
		Name:        "fest node",
		Dest:        "planning",
		SkipMarkers: true,
		JSONOutput:  true,
	}
	if err := RunCreateFestival(context.Background(), opts3); err != nil {
		t.Fatalf("Third RunCreateFestival failed: %v", err)
	}

	// Verify directory names — all 3 should be in planning/
	planningEntries, _ := os.ReadDir(filepath.Join(festivalsRoot, "planning"))

	if len(planningEntries) != 3 {
		t.Errorf("Expected 3 entries in planning/, got %d", len(planningEntries))
	}

	// Collect all IDs (extract from end of directory name after last hyphen)
	ids := make(map[string]bool)
	for _, e := range planningEntries {
		name := e.Name()
		lastHyphen := strings.LastIndex(name, "-")
		if lastHyphen == -1 {
			continue
		}
		id := name[lastHyphen+1:]
		if ids[id] {
			t.Errorf("Duplicate ID found: %s", id)
		}
		ids[id] = true
	}

	// Verify we have GU0001, GU0002, FN0001
	expectedIDs := []string{"GU0001", "GU0002", "FN0001"}
	for _, expectedID := range expectedIDs {
		if !ids[expectedID] {
			t.Errorf("Expected ID %s not found in %v", expectedID, ids)
		}
	}
}

// TestCreateFestival_BackwardsCompatibility verifies that old festivals
// without IDs in their directory names still work.
func TestCreateFestival_BackwardsCompatibility(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals directory structure
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	for _, status := range []string{"planning", "active", "dungeon"} {
		if err := os.MkdirAll(filepath.Join(festivalsRoot, status), 0755); err != nil {
			t.Fatalf("Failed to create status dir: %v", err)
		}
	}
	// Create dungeon/completed subdirectory
	if err := os.MkdirAll(filepath.Join(festivalsRoot, "dungeon", "completed"), 0755); err != nil {
		t.Fatalf("Failed to create dungeon/completed dir: %v", err)
	}

	// Create an old-style festival without ID suffix
	oldFestivalDir := filepath.Join(festivalsRoot, "active", "old-festival")
	if err := os.MkdirAll(oldFestivalDir, 0755); err != nil {
		t.Fatalf("Failed to create old festival dir: %v", err)
	}

	// Create minimal fest.yaml without metadata
	oldConfig := &config.FestivalConfig{
		Version: "1.0",
		QualityGates: config.QualityGatesConfig{
			Enabled: true,
		},
	}
	if err := config.SaveFestivalConfig(oldFestivalDir, "", oldConfig); err != nil {
		t.Fatalf("Failed to save old fest.yaml: %v", err)
	}

	// Create minimal .festival/templates directory with core templates
	writeMinimalCoreTemplates(t, festivalsRoot)

	// Change to festivals directory
	origDir, _ := os.Getwd()
	if err := os.Chdir(festivalsRoot); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create a new festival - should still work even with old festival present
	opts := &CreateFestivalOptions{
		Name:        "new festival",
		Dest:        "planning",
		SkipMarkers: true,
		JSONOutput:  true,
	}

	err := RunCreateFestival(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunCreateFestival failed: %v", err)
	}

	// Verify old festival still exists in active/
	activeEntries, _ := os.ReadDir(filepath.Join(festivalsRoot, "active"))
	if len(activeEntries) != 1 {
		t.Errorf("Expected 1 entry in active/ (old festival), got %d", len(activeEntries))
	}
	oldExists := false
	for _, e := range activeEntries {
		if e.Name() == "old-festival" {
			oldExists = true
		}
	}
	if !oldExists {
		t.Error("Old festival directory should still exist in active/")
	}

	// Verify new festival was created in planning/
	planningEntries, _ := os.ReadDir(filepath.Join(festivalsRoot, "planning"))
	if len(planningEntries) != 1 {
		t.Errorf("Expected 1 entry in planning/ (new festival), got %d", len(planningEntries))
	}
}
