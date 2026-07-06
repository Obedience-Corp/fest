package navigation

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFindCompletedFestivals(t *testing.T) {
	tmpDir := t.TempDir()
	completedDir := filepath.Join(tmpDir, "festivals", "dungeon", "completed")

	// Create test structure with date directories
	festivals := map[string][]string{
		"2024-11": {"fest-alpha", "fest-beta"},
		"2024-12": {"fest-gamma"},
		"2025-01": {"fest-delta", "fest-epsilon"},
	}

	for dateDir, fests := range festivals {
		for _, fest := range fests {
			path := filepath.Join(completedDir, dateDir, fest)
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatal(err)
			}
			// Create FESTIVAL_OVERVIEW.md to make it a valid festival
			if err := os.WriteFile(filepath.Join(path, "FESTIVAL_OVERVIEW.md"), []byte("test"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	tests := []struct {
		name      string
		prefix    string
		wantCount int
	}{
		{"all festivals", "", 5},
		{"fest- prefix", "fest-", 5},
		{"alpha prefix", "alpha", 0},
		{"fest-a prefix", "fest-a", 1}, // fest-alpha
		{"fest-d prefix", "fest-d", 1}, // fest-delta
		{"fest-e prefix", "fest-e", 1}, // fest-epsilon
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findCompletedFestivals(completedDir, tt.prefix)
			if len(got) != tt.wantCount {
				t.Errorf("findCompletedFestivals() returned %d, want %d: %v", len(got), tt.wantCount, got)
			}
		})
	}
}

func TestFindCompletedFestivals_MixedStructure(t *testing.T) {
	tmpDir := t.TempDir()
	completedDir := filepath.Join(tmpDir, "festivals", "dungeon", "completed")

	// Old flat structure (legacy festivals)
	legacyFests := []string{"old-fest-1", "old-fest-2"}
	for _, fest := range legacyFests {
		path := filepath.Join(completedDir, fest)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		// Create FESTIVAL_OVERVIEW.md to make it a valid festival
		if err := os.WriteFile(filepath.Join(path, "FESTIVAL_OVERVIEW.md"), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// New date-based structure
	dateDir := "2025-01"
	newFest := "new-fest-1"
	path := filepath.Join(completedDir, dateDir, newFest)
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "FESTIVAL_OVERVIEW.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	got := findCompletedFestivals(completedDir, "")

	// Should find both legacy and new festivals
	if len(got) != 3 {
		t.Errorf("Expected 3 festivals, got %d: %v", len(got), got)
	}

	// Verify specific festivals are found
	foundOld1, foundOld2, foundNew := false, false, false
	for _, f := range got {
		switch f {
		case "old-fest-1":
			foundOld1 = true
		case "old-fest-2":
			foundOld2 = true
		case "new-fest-1":
			foundNew = true
		}
	}

	if !foundOld1 || !foundOld2 {
		t.Error("Legacy festivals not found")
	}
	if !foundNew {
		t.Error("New date-based festival not found")
	}
}

func TestFindCompletedFestivals_EmptyDateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	completedDir := filepath.Join(tmpDir, "festivals", "dungeon", "completed")

	// Create an empty date directory
	if err := os.MkdirAll(filepath.Join(completedDir, "2025-01"), 0755); err != nil {
		t.Fatal(err)
	}

	got := findCompletedFestivals(completedDir, "")

	if len(got) != 0 {
		t.Errorf("Expected 0 festivals from empty directory, got %d: %v", len(got), got)
	}
}

func TestFindCompletedFestivals_NoCompletedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	completedDir := filepath.Join(tmpDir, "festivals", "dungeon", "completed")
	// Don't create the directory

	got := findCompletedFestivals(completedDir, "")

	if len(got) != 0 {
		t.Errorf("Expected 0 festivals from nonexistent directory, got %d: %v", len(got), got)
	}
}

func TestIsDateDirectory(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"2025-01", true},
		{"2024-12", true},
		{"2020-06", true},
		{"my-festival", false},
		{"2025-1", false}, // Not zero-padded
		{"25-01", false},  // Short year
		{"202501", false}, // No dash
		{"2025-13", true}, // Month validation not enforced by pattern
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDateDirectory(tt.name)
			if got != tt.want {
				t.Errorf("isDateDirectory(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func BenchmarkFindCompletedFestivals(b *testing.B) {
	tmpDir := b.TempDir()
	completedDir := filepath.Join(tmpDir, "festivals", "dungeon", "completed")

	// Create 72 date directories (6 years * 12 months) with 10 festivals each = 720 festivals
	for year := 2020; year <= 2025; year++ {
		for month := 1; month <= 12; month++ {
			dateDir := fmt.Sprintf("%d-%02d", year, month)
			for i := 0; i < 10; i++ {
				festName := fmt.Sprintf("fest-%d-%02d-%d", year, month, i)
				path := filepath.Join(completedDir, dateDir, festName)
				_ = os.MkdirAll(path, 0755)
				_ = os.WriteFile(filepath.Join(path, "FESTIVAL_OVERVIEW.md"), []byte("test"), 0644)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findCompletedFestivals(completedDir, "")
	}
}

func TestStatusFromPath(t *testing.T) {
	festivalsDir := "/home/user/campaigns/festivals"

	tests := []struct {
		name         string
		path         string
		festivalsDir string
		want         string
	}{
		{
			name:         "active festival",
			path:         filepath.Join(festivalsDir, "active", "my-fest-AB0001"),
			festivalsDir: festivalsDir,
			want:         "active",
		},
		{
			name:         "planning festival",
			path:         filepath.Join(festivalsDir, "planning", "other-fest-CD0002"),
			festivalsDir: festivalsDir,
			want:         "planning",
		},
		{
			name:         "completed festival",
			path:         filepath.Join(festivalsDir, "dungeon", "completed", "done-fest-EF0003"),
			festivalsDir: festivalsDir,
			want:         "dungeon/completed",
		},
		{
			name:         "same directory returns festival",
			path:         festivalsDir,
			festivalsDir: festivalsDir,
			want:         "festival",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusFromPath(tt.path, tt.festivalsDir)
			if got != tt.want {
				t.Errorf("statusFromPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompleteGoTarget_WithFestivals(t *testing.T) {
	// Create a temp festivals directory with valid festival dirs
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")

	// Create .festival/.state/.workspace marker so FindFestivals works
	statePath := filepath.Join(festivalsDir, ".festival", ".state")
	if err := os.MkdirAll(statePath, 0755); err != nil {
		t.Fatal(err)
	}
	marker := `{"workspace":"test","registered":"2026-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(statePath, ".workspace"), []byte(marker), 0644); err != nil {
		t.Fatal(err)
	}

	// Create status directories
	for _, status := range []string{"active", "planning", "ritual", "dungeon"} {
		if err := os.MkdirAll(filepath.Join(festivalsDir, status), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Create dungeon/completed subdirectory
	if err := os.MkdirAll(filepath.Join(festivalsDir, "dungeon", "completed"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create festivals with valid ID suffixes (required by CollectNavigationTargets)
	activeFest := filepath.Join(festivalsDir, "active", "my-feature-MF0001")
	activeRitualRun := filepath.Join(festivalsDir, "active", "daily-job-search-RI-DJ0001-0001")
	planningFest := filepath.Join(festivalsDir, "planning", "next-task-NT0002")
	ritualFest := filepath.Join(festivalsDir, "ritual", "daily-job-search-RI-DJ0001")
	for _, dir := range []string{activeFest, planningFest, ritualFest} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(activeRitualRun, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(activeRitualRun, "fest.yaml"), []byte("metadata:\n  name: daily-job-search\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to tmpDir so FindFestivals can discover the festivals directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Test with no partial input - should return all targets
	results, directive := CompleteGoTarget(nil, nil, "")
	_ = directive // cobra.ShellCompDirectiveNoFileComp = 4; value recorded but not asserted

	// Should include status directories AND festival names
	resultSet := make(map[string]bool)
	for _, r := range results {
		resultSet[r] = true
	}

	// Primary status directories should be present (completed/dungeon excluded from default)
	for _, status := range []string{"active", "planning", "ritual"} {
		if !resultSet[status] {
			t.Errorf("expected status directory %q in completions, got: %v", status, results)
		}
	}

	// Archival status directories should NOT be present in default completions
	for _, status := range []string{"dungeon/completed", "dungeon"} {
		if resultSet[status] {
			t.Errorf("unexpected archival status directory %q in default completions, got: %v", status, results)
		}
	}

	// Festival names should be present
	if !resultSet["my-feature-MF0001"] {
		t.Errorf("expected active festival 'my-feature-MF0001' in completions, got: %v", results)
	}
	if !resultSet["daily-job-search-RI-DJ0001-0001"] {
		t.Errorf("expected active ritual run in completions, got: %v", results)
	}
	if !resultSet["daily-job-search-RI-DJ0001"] {
		t.Errorf("expected ritual template in completions, got: %v", results)
	}
	if !resultSet["next-task-NT0002"] {
		t.Errorf("expected planning festival 'next-task-NT0002' in completions, got: %v", results)
	}

	// Test with partial input - should fuzzy filter
	filtered, _ := CompleteGoTarget(nil, nil, "my-feat")
	if len(filtered) == 0 {
		t.Error("expected fuzzy match for 'my-feat' but got no results")
	}
	foundMyFeature := false
	for _, r := range filtered {
		if r == "my-feature-MF0001" {
			foundMyFeature = true
		}
	}
	if !foundMyFeature {
		t.Errorf("expected 'my-feature-MF0001' in fuzzy results for 'my-feat', got: %v", filtered)
	}

	ritualFiltered, _ := CompleteGoTarget(nil, nil, "daily-job")
	if len(ritualFiltered) == 0 {
		t.Fatal("expected fuzzy match for 'daily-job' but got no results")
	}
	if ritualFiltered[0] != "daily-job-search-RI-DJ0001-0001" {
		t.Fatalf("expected active ritual run to be the top fuzzy completion, got %v", ritualFiltered)
	}
}

// Note: isDateDirectory, findCompletedFestivals, and isValidFestivalDir
// are implemented in completions.go
