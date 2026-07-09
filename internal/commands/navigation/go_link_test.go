package navigation

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	nav "github.com/Obedience-Corp/fest/internal/navigation"
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
	planningFests := []string{"fest-c"}

	for _, f := range activeFests {
		if err := os.MkdirAll(filepath.Join(festivalsDir, "active", f), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range planningFests {
		if err := os.MkdirAll(filepath.Join(festivalsDir, "planning", f), 0755); err != nil {
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
	planningCount := 0
	for _, f := range festivals {
		switch f.status {
		case "active":
			activeCount++
		case "planning":
			planningCount++
		}
	}

	if activeCount != 2 {
		t.Errorf("Expected 2 active festivals, got %d", activeCount)
	}
	if planningCount != 1 {
		t.Errorf("Expected 1 planning festival, got %d", planningCount)
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

	// Should only contain the 3 non-hidden project directories (no monorepo sub-entries)
	want := map[string]string{
		filepath.Join(tmpDir, "projects", "camp"):        "camp",
		filepath.Join(tmpDir, "projects", "fest"):        "fest",
		filepath.Join(tmpDir, "projects", "obey-daemon"): "obey-daemon",
	}

	if len(got) != len(want) {
		t.Fatalf("collectProjectDirectories() returned %d entries, want %d\ngot: %v", len(got), len(want), got)
	}

	for _, entry := range got {
		expectedLabel, ok := want[entry.path]
		if !ok {
			t.Errorf("unexpected entry: path=%s label=%s", entry.path, entry.label)
			continue
		}
		if entry.label != expectedLabel {
			t.Errorf("entry path=%s: label=%q, want %q", entry.path, entry.label, expectedLabel)
		}
	}
}

func TestCollectProjectDirectoriesMonorepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a standalone project
	if err := os.MkdirAll(filepath.Join(tmpDir, "projects", "camp"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a monorepo with submodules
	monorepoDir := filepath.Join(tmpDir, "projects", "my-monorepo")
	if err := os.MkdirAll(monorepoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize git and create .gitmodules
	if err := exec.Command("git", "init", monorepoDir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	for _, sub := range []string{"obey", "festui"} {
		if err := exec.Command("git", "-C", monorepoDir, "config", "-f", ".gitmodules",
			"submodule."+sub+".path", sub).Run(); err != nil {
			t.Fatalf("git config submodule path: %v", err)
		}
		if err := exec.Command("git", "-C", monorepoDir, "config", "-f", ".gitmodules",
			"submodule."+sub+".url", "https://example.com/"+sub+".git").Run(); err != nil {
			t.Fatalf("git config submodule url: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(monorepoDir, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := collectProjectDirectories(tmpDir)
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	// Expect: camp, my-monorepo (root), my-monorepo@obey, my-monorepo@festui
	want := map[string]string{
		filepath.Join(tmpDir, "projects", "camp"):                  "camp",
		filepath.Join(tmpDir, "projects", "my-monorepo"):           "my-monorepo",
		filepath.Join(tmpDir, "projects", "my-monorepo", "obey"):   "my-monorepo@obey",
		filepath.Join(tmpDir, "projects", "my-monorepo", "festui"): "my-monorepo@festui",
	}

	if len(got) != len(want) {
		var labels []string
		for _, e := range got {
			labels = append(labels, e.label)
		}
		t.Fatalf("got %d entries %v, want %d", len(got), labels, len(want))
	}

	for _, entry := range got {
		expectedLabel, ok := want[entry.path]
		if !ok {
			t.Errorf("unexpected entry: path=%s label=%s", entry.path, entry.label)
			continue
		}
		if entry.label != expectedLabel {
			t.Errorf("path=%s: label=%q, want %q", entry.path, entry.label, expectedLabel)
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
			"/home/user/campaign/festivals/planning/my-fest",
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
	planningFest := filepath.Join(festivalsDir, "planning", "my-planning-fest")
	completedFest := filepath.Join(festivalsDir, "dungeon", "completed", "my-completed-fest")

	for _, f := range []string{activeFest, planningFest, completedFest} {
		if err := os.MkdirAll(f, 0755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name     string
		expected string
	}{
		{"my-active-fest", activeFest},
		{"my-planning-fest", planningFest},
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

func TestGuardProjectConflict(t *testing.T) {
	projectPath := "/campaign/projects/camp"

	tests := []struct {
		name             string
		existingFestival string
		existingPath     string
		force            bool
		wantErr          bool
		wantEvictable    string
	}{
		{
			name: "no existing link permits linking",
		},
		{
			name:             "active festival link blocks without force",
			existingFestival: "active-fest-AF0001",
			existingPath:     "/campaign/festivals/active/active-fest-AF0001",
			wantErr:          true,
		},
		{
			name:             "planning festival link blocks without force",
			existingFestival: "planning-fest-PF0001",
			existingPath:     "/campaign/festivals/planning/planning-fest-PF0001",
			wantErr:          true,
		},
		{
			name:             "legacy link without festival path blocks without force",
			existingFestival: "legacy-fest-LF0001",
			wantErr:          true,
		},
		{
			name:             "force permits takeover and names the evicted festival",
			existingFestival: "planning-fest-PF0001",
			existingPath:     "/campaign/festivals/planning/planning-fest-PF0001",
			force:            true,
			wantEvictable:    "planning-fest-PF0001",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := newTestNav()
			if tc.existingFestival != "" {
				n.SetLinkWithPath(tc.existingFestival, projectPath, tc.existingPath)
			}

			existing, err := guardProjectConflict(n, "new-fest-NF0001", projectPath, tc.force)
			if tc.wantErr {
				if err == nil {
					t.Fatal("guardProjectConflict() error = nil, want conflict error")
				}
				return
			}
			if err != nil {
				t.Fatalf("guardProjectConflict() error: %v", err)
			}
			if existing != tc.wantEvictable {
				t.Fatalf("guardProjectConflict() existing = %q, want %q", existing, tc.wantEvictable)
			}
		})
	}
}

func TestGuardProjectConflict_SameFestivalRelinks(t *testing.T) {
	n := newTestNav()
	n.SetLinkWithPath("my-fest-MF0001", "/campaign/projects/camp", "/campaign/festivals/planning/my-fest-MF0001")

	existing, err := guardProjectConflict(n, "my-fest-MF0001", "/campaign/projects/camp", false)
	if err != nil {
		t.Fatalf("relinking the same festival should not conflict: %v", err)
	}
	if existing != "" {
		t.Fatalf("existing = %q, want empty for same-festival relink", existing)
	}
}

func newTestNav() *nav.Navigation {
	return &nav.Navigation{
		Version:      1,
		UpdatedAt:    time.Now().UTC(),
		Links:        make(map[string]*nav.Link),
		ProjectLinks: make(map[string]string),
		Shortcuts:    make(map[string]string),
	}
}

func TestLinkProjectToFestival_HijackGuardBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "projects", "camp")
	activeFestPath := filepath.Join(tmpDir, "festivals", "active", "active-fest-AF0001")
	planningFestPath := filepath.Join(tmpDir, "festivals", "planning", "planning-fest-PF0001")

	for _, d := range []string{projectPath, activeFestPath, planningFestPath} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	n := newTestNav()
	n.SetLinkWithPath("active-fest-AF0001", projectPath, activeFestPath)

	if _, err := guardProjectConflict(n, "planning-fest-PF0001", projectPath, false); err == nil {
		t.Fatal("guardProjectConflict() should block re-linking a project held by another festival")
	}

	n2 := newTestNav()
	n2.SetLinkWithPath("active-fest-AF0001", projectPath, activeFestPath)
	evicted := n2.SetLinkWithPath("planning-fest-PF0001", projectPath, planningFestPath)

	if evicted != "active-fest-AF0001" {
		t.Errorf("SetLinkWithPath() evicted = %q, want active-fest-AF0001 so callers can report the takeover", evicted)
	}
	link, ok := n2.GetLink("active-fest-AF0001")
	if ok && link.Path == projectPath {
		t.Error("SetLinkWithPath must re-point the project to exactly one festival")
	}

	n3 := newTestNav()
	n3.SetLinkWithPath("active-fest-AF0001", filepath.Join(tmpDir, "projects", "other"), activeFestPath)
	if _, _, hasConflict := n3.ProjectConflict("planning-fest-PF0001", projectPath); hasConflict {
		t.Error("ProjectConflict() should not report conflict when project is not yet linked")
	}
}
