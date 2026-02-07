package navigation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsInsideFestival(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals structure
	festivalPath := filepath.Join(tmpDir, "festivals", "active", "my-fest")
	festivalPhase := filepath.Join(festivalPath, "001_PLAN")
	festivalTask := filepath.Join(festivalPhase, "tasks")
	projectPath := filepath.Join(tmpDir, "projects", "my-project")
	projectSubdir := filepath.Join(projectPath, "src", "components")

	// Create all directories
	for _, d := range []string{festivalTask, projectSubdir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"festival root", festivalPath, true},
		{"festival phase", festivalPhase, true},
		{"festival nested task", festivalTask, true},
		{"project root", projectPath, false},
		{"project subdir", projectSubdir, false},
		{"festivals parent dir", filepath.Join(tmpDir, "festivals"), true},
		{"random dir", tmpDir, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isInsideFestival(tc.path)
			if result != tc.expected {
				t.Errorf("isInsideFestival(%q) = %v, want %v", tc.path, result, tc.expected)
			}
		})
	}
}

func TestCollectFestivals(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")

	// Create some festivals
	activeFests := []string{"fest-a", "fest-b"}
	plannedFests := []string{"fest-c"}

	for _, f := range activeFests {
		if err := os.MkdirAll(filepath.Join(festivalsDir, "active", f), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range plannedFests {
		if err := os.MkdirAll(filepath.Join(festivalsDir, "planned", f), 0755); err != nil {
			t.Fatal(err)
		}
	}

	festivals, err := collectFestivals(festivalsDir)
	if err != nil {
		t.Fatalf("collectFestivals() error = %v", err)
	}

	if len(festivals) != 3 {
		t.Errorf("collectFestivals() returned %d festivals, want 3", len(festivals))
	}

	// Verify festivals are categorized correctly
	activeCount := 0
	plannedCount := 0
	for _, f := range festivals {
		switch f.status {
		case "active":
			activeCount++
		case "planned":
			plannedCount++
		}
	}

	if activeCount != 2 {
		t.Errorf("Expected 2 active festivals, got %d", activeCount)
	}
	if plannedCount != 1 {
		t.Errorf("Expected 1 planned festival, got %d", plannedCount)
	}
}

func TestCollectProjectDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Build a realistic campaign structure
	dirs := []string{
		"projects/camp",
		"projects/fest",
		"projects/obey-daemon",
		"projects/.hidden-project",
		"docs/guides",
		"workflow/pipelines",
		"festivals/active/some-fest",
		"ai_docs/research",
		"dungeon/old-stuff",
	}
	// Also add a regular file inside projects/
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Create a file in projects/ to ensure it's skipped
	if err := os.WriteFile(filepath.Join(tmpDir, "projects", "README.md"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := collectProjectDirectories(tmpDir)
	if err != nil {
		t.Fatalf("collectProjectDirectories() error = %v", err)
	}

	// Should only contain the 3 non-hidden project directories
	want := map[string]bool{
		filepath.Join(tmpDir, "projects", "camp"):        true,
		filepath.Join(tmpDir, "projects", "fest"):        true,
		filepath.Join(tmpDir, "projects", "obey-daemon"): true,
	}

	if len(got) != len(want) {
		t.Fatalf("collectProjectDirectories() returned %d dirs, want %d\ngot: %v", len(got), len(want), got)
	}

	for _, dir := range got {
		if !want[dir] {
			t.Errorf("unexpected directory in results: %s", dir)
		}
	}
}

func TestCollectProjectDirectoriesNoProjectsDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Campaign root with no projects/ directory
	_, err := collectProjectDirectories(tmpDir)
	if err == nil {
		t.Fatal("expected error when projects/ directory doesn't exist")
	}
}

func TestCampaignRootFromFestival(t *testing.T) {
	tests := []struct {
		name         string
		festivalPath string
		wantRoot     string
	}{
		{
			"active festival",
			"/home/user/campaign/festivals/active/my-fest",
			"/home/user/campaign",
		},
		{
			"planned festival",
			"/home/user/campaign/festivals/planned/my-fest",
			"/home/user/campaign",
		},
		{
			"completed festival",
			"/home/user/campaign/festivals/completed/my-fest",
			"/home/user/campaign",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := campaignRootFromFestival(tc.festivalPath)
			if got != tc.wantRoot {
				t.Errorf("campaignRootFromFestival(%q) = %q, want %q", tc.festivalPath, got, tc.wantRoot)
			}
		})
	}
}

func TestLinkRoutingUsesPhysicalPath(t *testing.T) {
	// This test verifies that link routing is based on physical filesystem
	// location (isInsideFestival), NOT navigation-link-aware detection.
	// A project directory that is linked to a festival via navigation.yaml
	// must still route to the "select festival" TUI, not the "select project" TUI.
	tmpDir := t.TempDir()

	// Create both festival and project dirs
	festivalDir := filepath.Join(tmpDir, "festivals", "active", "my-fest")
	projectDir := filepath.Join(tmpDir, "projects", "camp")
	for _, d := range []string{festivalDir, projectDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name           string
		path           string
		wantInsideFest bool
		description    string
	}{
		{
			"festival dir routes to project picker",
			festivalDir,
			true,
			"inside festival → should pick a project to link",
		},
		{
			"project dir routes to festival picker",
			projectDir,
			false,
			"inside project → should pick a festival to link",
		},
		{
			"project subdir routes to festival picker",
			filepath.Join(projectDir, "src"),
			false,
			"inside project subdir → should pick a festival to link",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isInsideFestival(tc.path)
			if got != tc.wantInsideFest {
				t.Errorf("isInsideFestival(%q) = %v, want %v (%s)",
					tc.path, got, tc.wantInsideFest, tc.description)
			}
		})
	}
}

func TestResolveFestivalPath(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")

	// Create festivals in different statuses
	activeFest := filepath.Join(festivalsDir, "active", "my-active-fest")
	plannedFest := filepath.Join(festivalsDir, "planned", "my-planned-fest")
	completedFest := filepath.Join(festivalsDir, "completed", "my-completed-fest")

	for _, f := range []string{activeFest, plannedFest, completedFest} {
		if err := os.MkdirAll(f, 0755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name     string
		expected string
	}{
		{"my-active-fest", activeFest},
		{"my-planned-fest", plannedFest},
		{"my-completed-fest", completedFest},
		{"nonexistent", ""},
	}

	for _, tc := range tests {
		result := resolveFestivalPath(festivalsDir, tc.name)
		if result != tc.expected {
			t.Errorf("resolveFestivalPath(%q) = %q, want %q", tc.name, result, tc.expected)
		}
	}
}
