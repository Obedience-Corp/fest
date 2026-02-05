package navigation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupTestCampaign creates a temporary campaign directory structure for testing.
// Returns the campaign root path. Sets CAMP_ROOT env var.
func setupTestCampaign(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .campaign directory
	campaignDir := filepath.Join(tmpDir, ".campaign")
	if err := os.MkdirAll(campaignDir, 0755); err != nil {
		t.Fatalf("Failed to create .campaign dir: %v", err)
	}

	// Set CAMP_ROOT so DetectCampaign finds it
	t.Setenv("CAMP_ROOT", tmpDir)

	return tmpDir
}

func TestLoadNavigation_NewFile(t *testing.T) {
	setupTestCampaign(t)

	nav, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	if nav == nil {
		t.Fatal("LoadNavigation() returned nil")
	}

	if nav.Version != 1 {
		t.Errorf("Version = %d, want 1", nav.Version)
	}

	if nav.Links == nil {
		t.Error("Links map should not be nil")
	}

	if nav.Shortcuts == nil {
		t.Error("Shortcuts map should not be nil")
	}
}

func TestNavigation_SetAndGetLink(t *testing.T) {
	setupTestCampaign(t)

	nav, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	// Set a link
	nav.SetLink("test-festival", "/path/to/project")

	// Get the link
	link, found := nav.GetLink("test-festival")
	if !found {
		t.Fatal("GetLink() returned false, want true")
	}

	if link.Path != "/path/to/project" {
		t.Errorf("Link.Path = %q, want %q", link.Path, "/path/to/project")
	}

	// Check LinkedAt is recent
	if time.Since(link.LinkedAt) > time.Minute {
		t.Error("LinkedAt should be recent")
	}
}

func TestNavigation_SaveAndLoad(t *testing.T) {
	campaignRoot := setupTestCampaign(t)

	// Create and save navigation with a link using path within campaign
	nav1, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	projectPath := filepath.Join(campaignRoot, "projects", "my-project")
	nav1.SetLink("my-festival", projectPath)
	nav1.Shortcuts["p"] = "/path/to/shortcut"

	if err := nav1.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file was created in .campaign/fest/
	navPath := filepath.Join(campaignRoot, ".campaign", "fest", NavigationFileName)
	if _, err := os.Stat(navPath); os.IsNotExist(err) {
		t.Fatalf("Navigation file was not created at %s", navPath)
	}

	// Load navigation again
	nav2, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	// Verify link was preserved (should be absolute path in memory)
	link, found := nav2.GetLink("my-festival")
	if !found {
		t.Fatal("Link was not preserved after save/load")
	}

	if link.Path != projectPath {
		t.Errorf("Link.Path = %q, want %q", link.Path, projectPath)
	}

	// Verify shortcut was preserved
	if nav2.Shortcuts["p"] != "/path/to/shortcut" {
		t.Errorf("Shortcut not preserved: got %q", nav2.Shortcuts["p"])
	}
}

func TestNavigation_RemoveLink(t *testing.T) {
	setupTestCampaign(t)

	nav, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	// Set a link
	nav.SetLink("test-festival", "/path/to/project")

	// Verify link exists
	if _, found := nav.GetLink("test-festival"); !found {
		t.Fatal("Link should exist before removal")
	}

	// Remove the link
	removed := nav.RemoveLink("test-festival")
	if !removed {
		t.Error("RemoveLink() returned false, want true")
	}

	// Verify link is gone
	if _, found := nav.GetLink("test-festival"); found {
		t.Error("Link should not exist after removal")
	}

	// Remove non-existent link
	removed = nav.RemoveLink("non-existent")
	if removed {
		t.Error("RemoveLink() returned true for non-existent link")
	}
}

func TestNavigation_ListLinks(t *testing.T) {
	setupTestCampaign(t)

	nav, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	// Add multiple links
	nav.SetLink("festival-1", "/path/to/project1")
	nav.SetLink("festival-2", "/path/to/project2")
	nav.SetLink("festival-3", "/path/to/project3")

	links := nav.ListLinks()
	if len(links) != 3 {
		t.Errorf("ListLinks() returned %d links, want 3", len(links))
	}

	// Verify each link
	expectedLinks := map[string]string{
		"festival-1": "/path/to/project1",
		"festival-2": "/path/to/project2",
		"festival-3": "/path/to/project3",
	}

	for name, expectedPath := range expectedLinks {
		link, ok := links[name]
		if !ok {
			t.Errorf("Missing link for %q", name)
			continue
		}
		if link.Path != expectedPath {
			t.Errorf("Link[%q].Path = %q, want %q", name, link.Path, expectedPath)
		}
	}
}

func TestNavigationPath(t *testing.T) {
	campaignRoot := setupTestCampaign(t)

	path, err := NavigationPath()
	if err != nil {
		t.Fatalf("NavigationPath() error = %v", err)
	}

	expected := filepath.Join(campaignRoot, ".campaign", "fest", NavigationFileName)
	if path != expected {
		t.Errorf("NavigationPath() = %q, want %q", path, expected)
	}
}

func TestNavigationPath_NoCampaign(t *testing.T) {
	// Don't set up a campaign - should return error
	t.Setenv("CAMP_ROOT", "")

	// Change to a temp directory with no .campaign/
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	_, err := NavigationPath()
	if err == nil {
		t.Error("NavigationPath() should return error when not in a campaign")
	}
}

func TestNavigation_BidirectionalLinks(t *testing.T) {
	setupTestCampaign(t)

	nav, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	// Set a bidirectional link
	nav.SetLink("my-festival", "/path/to/project")

	// Test GetLinkedProject
	projectPath := nav.GetLinkedProject("my-festival")
	if projectPath != "/path/to/project" {
		t.Errorf("GetLinkedProject() = %q, want %q", projectPath, "/path/to/project")
	}

	// Test GetLinkedFestival (reverse lookup)
	festivalName := nav.GetLinkedFestival("/path/to/project")
	if festivalName != "my-festival" {
		t.Errorf("GetLinkedFestival() = %q, want %q", festivalName, "my-festival")
	}

	// Test non-existent lookups
	if nav.GetLinkedProject("nonexistent") != "" {
		t.Error("GetLinkedProject() should return empty for non-existent festival")
	}
	if nav.GetLinkedFestival("/nonexistent/path") != "" {
		t.Error("GetLinkedFestival() should return empty for non-existent path")
	}
}

func TestNavigation_FindFestivalForPath(t *testing.T) {
	campaignRoot := setupTestCampaign(t)

	// Create actual directories for the test
	projectRoot := filepath.Join(campaignRoot, "projects", "my-project")
	subDir := filepath.Join(projectRoot, "src", "components")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	nav, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	// Link festival to project root
	nav.SetLink("my-festival", projectRoot)

	tests := []struct {
		path     string
		expected string
	}{
		// Exact match
		{projectRoot, "my-festival"},
		// Subdirectory should find parent's linked festival
		{subDir, "my-festival"},
		// Unlinked path
		{filepath.Join(campaignRoot, "other"), ""},
	}

	for _, tc := range tests {
		result := nav.FindFestivalForPath(tc.path)
		if result != tc.expected {
			t.Errorf("FindFestivalForPath(%q) = %q, want %q", tc.path, result, tc.expected)
		}
	}
}

func TestNavigation_SetLinkUpdatesReverse(t *testing.T) {
	setupTestCampaign(t)

	nav, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	// First link: festival-a -> project-a
	nav.SetLink("festival-a", "/path/to/project-a")

	// Verify both directions
	if nav.GetLinkedProject("festival-a") != "/path/to/project-a" {
		t.Error("Forward link not set correctly")
	}
	if nav.GetLinkedFestival("/path/to/project-a") != "festival-a" {
		t.Error("Reverse link not set correctly")
	}

	// Now relink festival-a to a different project
	nav.SetLink("festival-a", "/path/to/project-b")

	// Old reverse link should be gone
	if nav.GetLinkedFestival("/path/to/project-a") != "" {
		t.Error("Old reverse link should be removed when festival is relinked")
	}

	// New links should be correct
	if nav.GetLinkedProject("festival-a") != "/path/to/project-b" {
		t.Error("Forward link not updated correctly")
	}
	if nav.GetLinkedFestival("/path/to/project-b") != "festival-a" {
		t.Error("New reverse link not set correctly")
	}
}

// TestNavigation_RelinkProjectToDifferentFestival tests the specific bug scenario:
// 1. Link Project P to Festival A
// 2. Re-link Project P to Festival B (from Festival B)
// 3. Verify `fgo project` works from Festival B
func TestNavigation_RelinkProjectToDifferentFestival(t *testing.T) {
	setupTestCampaign(t)

	nav, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	projectPath := "/path/to/shared-project"

	// Step 1: Link project to Festival A
	nav.SetLink("festival-a", projectPath)

	// Verify initial state
	if nav.GetLinkedProject("festival-a") != projectPath {
		t.Error("Step 1: Forward link festival-a -> project not set")
	}
	if nav.GetLinkedFestival(projectPath) != "festival-a" {
		t.Error("Step 1: Reverse link project -> festival-a not set")
	}

	// Step 2: Re-link same project to Festival B (simulating user in Festival B)
	nav.SetLink("festival-b", projectPath)

	// Step 3: Verify the bug - can we navigate from Festival B to project?
	linkedProject := nav.GetLinkedProject("festival-b")
	if linkedProject != projectPath {
		t.Errorf("Bug: GetLinkedProject('festival-b') = %q, want %q", linkedProject, projectPath)
	}

	// Verify Festival A's link was properly removed
	if nav.GetLinkedProject("festival-a") != "" {
		t.Error("Festival A's forward link should be removed when project relinked")
	}

	// Verify reverse lookup returns Festival B
	linkedFestival := nav.GetLinkedFestival(projectPath)
	if linkedFestival != "festival-b" {
		t.Errorf("GetLinkedFestival(project) = %q, want 'festival-b'", linkedFestival)
	}

	// Verify only one entry in Links map
	if len(nav.Links) != 1 {
		t.Errorf("Expected 1 link, got %d", len(nav.Links))
	}

	// Verify only one entry in ProjectLinks map
	if len(nav.ProjectLinks) != 1 {
		t.Errorf("Expected 1 project link, got %d", len(nav.ProjectLinks))
	}
}

// TestNavigation_SetLinkWithPath tests that festival path is stored correctly
func TestNavigation_SetLinkWithPath(t *testing.T) {
	setupTestCampaign(t)

	nav, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	projectPath := "/path/to/project"
	festivalPath := "/path/to/festivals/active/my-festival"

	// Set link with festival path
	nav.SetLinkWithPath("my-festival", projectPath, festivalPath)

	// Verify forward link has festival path
	link, found := nav.GetLink("my-festival")
	if !found {
		t.Fatal("Link should exist")
	}
	if link.Path != projectPath {
		t.Errorf("Link.Path = %q, want %q", link.Path, projectPath)
	}
	if link.FestivalPath != festivalPath {
		t.Errorf("Link.FestivalPath = %q, want %q", link.FestivalPath, festivalPath)
	}

	// Save and reload
	if err := nav.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	nav2, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	link2, found := nav2.GetLink("my-festival")
	if !found {
		t.Fatal("Link should exist after reload")
	}
	if link2.FestivalPath != festivalPath {
		t.Errorf("After reload, Link.FestivalPath = %q, want %q", link2.FestivalPath, festivalPath)
	}
}

// TestNavigation_RelativePathStorage tests that paths are stored as relative in YAML
func TestNavigation_RelativePathStorage(t *testing.T) {
	campaignRoot := setupTestCampaign(t)

	nav, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	// Set link with absolute path within campaign
	projectPath := filepath.Join(campaignRoot, "projects", "my-project")
	festivalPath := filepath.Join(campaignRoot, "festivals", "active", "my-festival")
	nav.SetLinkWithPath("my-festival", projectPath, festivalPath)

	if err := nav.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read raw YAML to verify paths are relative
	navPath := filepath.Join(campaignRoot, ".campaign", "fest", NavigationFileName)
	data, err := os.ReadFile(navPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	yamlContent := string(data)

	// Paths should be relative (not contain campaign root)
	if strings.Contains(yamlContent, campaignRoot) {
		t.Errorf("YAML should contain relative paths, but found campaign root:\n%s", yamlContent)
	}

	// Should contain relative paths
	if !strings.Contains(yamlContent, "projects/my-project") {
		t.Errorf("YAML should contain relative path 'projects/my-project':\n%s", yamlContent)
	}
	if !strings.Contains(yamlContent, "festivals/active/my-festival") {
		t.Errorf("YAML should contain relative path 'festivals/active/my-festival':\n%s", yamlContent)
	}

	// Should NOT contain project_links (not serialized)
	if strings.Contains(yamlContent, "project_links") {
		t.Errorf("YAML should NOT contain project_links section:\n%s", yamlContent)
	}

	// Reload and verify paths are expanded to absolute
	nav2, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	link, found := nav2.GetLink("my-festival")
	if !found {
		t.Fatal("Link should exist after reload")
	}

	// In-memory paths should be absolute
	if link.Path != projectPath {
		t.Errorf("After reload, Link.Path = %q, want %q", link.Path, projectPath)
	}
	if link.FestivalPath != festivalPath {
		t.Errorf("After reload, Link.FestivalPath = %q, want %q", link.FestivalPath, festivalPath)
	}

	// Verify ProjectLinks is rebuilt correctly
	if nav2.GetLinkedFestival(projectPath) != "my-festival" {
		t.Error("ProjectLinks should be rebuilt from Links after load")
	}
}

// TestNavigation_PathsOutsideCampaign tests that paths outside campaign remain absolute
func TestNavigation_PathsOutsideCampaign(t *testing.T) {
	campaignRoot := setupTestCampaign(t)

	nav, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	// Set link with absolute path OUTSIDE campaign
	outsidePath := "/some/external/path"
	nav.SetLink("external-festival", outsidePath)

	if err := nav.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read raw YAML to verify path is kept absolute
	navPath := filepath.Join(campaignRoot, ".campaign", "fest", NavigationFileName)
	data, err := os.ReadFile(navPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	yamlContent := string(data)

	// Path outside campaign should remain absolute
	if !strings.Contains(yamlContent, outsidePath) {
		t.Errorf("YAML should keep absolute path for path outside campaign:\n%s", yamlContent)
	}

	// Reload and verify path is preserved
	nav2, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	link, found := nav2.GetLink("external-festival")
	if !found {
		t.Fatal("Link should exist after reload")
	}

	if link.Path != outsidePath {
		t.Errorf("After reload, Link.Path = %q, want %q", link.Path, outsidePath)
	}
}

// TestNavigation_RelinkWithSaveLoadCycle tests the relinking scenario with persistence
func TestNavigation_RelinkWithSaveLoadCycle(t *testing.T) {
	setupTestCampaign(t)

	projectPath := "/path/to/shared-project"

	// Step 1: Link project to Festival A and save
	nav1, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}
	nav1.SetLink("festival-a", projectPath)
	if err := nav1.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Step 2: Load fresh, re-link project to Festival B, and save
	nav2, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}
	nav2.SetLink("festival-b", projectPath)
	if err := nav2.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Step 3: Load fresh and verify state
	nav3, err := LoadNavigation()
	if err != nil {
		t.Fatalf("LoadNavigation() error = %v", err)
	}

	// Forward link from Festival B should work
	if nav3.GetLinkedProject("festival-b") != projectPath {
		t.Error("Forward link festival-b -> project not persisted")
	}

	// Forward link from Festival A should be gone
	if nav3.GetLinkedProject("festival-a") != "" {
		t.Error("Festival A's forward link should have been removed")
	}

	// Reverse lookup should return Festival B
	if nav3.GetLinkedFestival(projectPath) != "festival-b" {
		t.Error("Reverse link project -> festival-b not persisted correctly")
	}

	// Verify maps have correct size
	if len(nav3.Links) != 1 {
		t.Errorf("After persist, expected 1 link, got %d", len(nav3.Links))
	}
	if len(nav3.ProjectLinks) != 1 {
		t.Errorf("After persist, expected 1 project link, got %d", len(nav3.ProjectLinks))
	}
}
