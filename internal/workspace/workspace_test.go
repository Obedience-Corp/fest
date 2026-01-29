package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMarkerPath(t *testing.T) {
	festivalsDir := "/path/to/festivals"
	expected := "/path/to/festivals/.festival/.workspace"
	result := MarkerPath(festivalsDir)
	if result != expected {
		t.Errorf("MarkerPath(%q) = %q, want %q", festivalsDir, result, expected)
	}
}

func TestHasMarker(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals structure without marker
	festivalsDir := filepath.Join(tmpDir, "festivals")
	dotFestival := filepath.Join(festivalsDir, ".festival")
	if err := os.MkdirAll(dotFestival, 0755); err != nil {
		t.Fatal(err)
	}

	// Test: no marker exists
	if HasMarker(festivalsDir) {
		t.Error("HasMarker returned true when marker does not exist")
	}

	// Create marker file
	markerPath := filepath.Join(dotFestival, ".workspace")
	if err := os.WriteFile(markerPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test: marker exists
	if !HasMarker(festivalsDir) {
		t.Error("HasMarker returned false when marker exists")
	}
}

func TestReadMarker(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals structure with marker
	festivalsDir := filepath.Join(tmpDir, "festivals")
	dotFestival := filepath.Join(festivalsDir, ".festival")
	if err := os.MkdirAll(dotFestival, 0755); err != nil {
		t.Fatal(err)
	}

	// Create marker file with valid JSON
	marker := Marker{
		Workspace:  "test-workspace",
		Registered: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	}
	data, _ := json.Marshal(marker)
	markerPath := filepath.Join(dotFestival, ".workspace")
	if err := os.WriteFile(markerPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Test: read marker
	result, err := ReadMarker(festivalsDir)
	if err != nil {
		t.Fatalf("ReadMarker failed: %v", err)
	}
	if result.Workspace != "test-workspace" {
		t.Errorf("Workspace = %q, want %q", result.Workspace, "test-workspace")
	}

	// Test: read non-existent marker
	emptyDir := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}
	_, err = ReadMarker(emptyDir)
	if err == nil {
		t.Error("ReadMarker should fail for non-existent marker")
	}
}

func TestRegisterFestivals(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure: parent/festivals/
	parentDir := filepath.Join(tmpDir, "my-project")
	festivalsDir := filepath.Join(parentDir, "festivals")
	dotFestival := filepath.Join(festivalsDir, ".festival")
	if err := os.MkdirAll(dotFestival, 0755); err != nil {
		t.Fatal(err)
	}

	// Register
	if err := RegisterFestivals(festivalsDir); err != nil {
		t.Fatalf("RegisterFestivals failed: %v", err)
	}

	// Verify marker was created
	if !HasMarker(festivalsDir) {
		t.Error("Marker should exist after registration")
	}

	// Verify workspace name is derived from parent directory
	marker, err := ReadMarker(festivalsDir)
	if err != nil {
		t.Fatalf("Failed to read marker: %v", err)
	}
	if marker.Workspace != "my-project" {
		t.Errorf("Workspace = %q, want %q", marker.Workspace, "my-project")
	}
}

func TestUnregisterFestivals(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and register festivals
	parentDir := filepath.Join(tmpDir, "my-project")
	festivalsDir := filepath.Join(parentDir, "festivals")
	dotFestival := filepath.Join(festivalsDir, ".festival")
	if err := os.MkdirAll(dotFestival, 0755); err != nil {
		t.Fatal(err)
	}
	if err := RegisterFestivals(festivalsDir); err != nil {
		t.Fatal(err)
	}

	// Verify marker exists
	if !HasMarker(festivalsDir) {
		t.Fatal("Marker should exist before unregistration")
	}

	// Unregister
	if err := UnregisterFestivals(festivalsDir); err != nil {
		t.Fatalf("UnregisterFestivals failed: %v", err)
	}

	// Verify marker was removed
	if HasMarker(festivalsDir) {
		t.Error("Marker should not exist after unregistration")
	}

	// Unregistering again should not error
	if err := UnregisterFestivals(festivalsDir); err != nil {
		t.Errorf("Unregistering non-existent marker should not error: %v", err)
	}
}

func TestFindMarkedFestivals(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested structure:
	// tmpDir/
	//   outer-project/
	//     festivals/          <- has marker
	//       .festival/
	//     inner-project/
	//       festivals/        <- no marker
	//         .festival/
	//       src/
	//         code/

	outerProject := filepath.Join(tmpDir, "outer-project")
	outerFestivals := filepath.Join(outerProject, "festivals")
	outerDotFestival := filepath.Join(outerFestivals, ".festival")

	innerProject := filepath.Join(outerProject, "inner-project")
	innerFestivals := filepath.Join(innerProject, "festivals")
	innerDotFestival := filepath.Join(innerFestivals, ".festival")

	codeDir := filepath.Join(innerProject, "src", "code")

	// Create all directories
	for _, dir := range []string{outerDotFestival, innerDotFestival, codeDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Register only outer festivals
	if err := RegisterFestivals(outerFestivals); err != nil {
		t.Fatal(err)
	}

	// Test: from codeDir, should find outer festivals (skips inner because no marker)
	result, err := FindMarkedFestivals(codeDir)
	if err != nil {
		t.Fatalf("FindMarkedFestivals failed: %v", err)
	}
	if result != outerFestivals {
		t.Errorf("FindMarkedFestivals = %q, want %q", result, outerFestivals)
	}

	// Test: from inner project root
	result, err = FindMarkedFestivals(innerProject)
	if err != nil {
		t.Fatalf("FindMarkedFestivals failed: %v", err)
	}
	if result != outerFestivals {
		t.Errorf("FindMarkedFestivals = %q, want %q", result, outerFestivals)
	}

	// Test: from outer festivals directly
	result, err = FindMarkedFestivals(outerFestivals)
	if err != nil {
		t.Fatalf("FindMarkedFestivals failed: %v", err)
	}
	if result != outerFestivals {
		t.Errorf("FindMarkedFestivals = %q, want %q", result, outerFestivals)
	}
}

func TestFindMarkedFestivalsNoMarker(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals without marker
	festivalsDir := filepath.Join(tmpDir, "project", "festivals")
	dotFestival := filepath.Join(festivalsDir, ".festival")
	if err := os.MkdirAll(dotFestival, 0755); err != nil {
		t.Fatal(err)
	}

	// Test: no marker anywhere
	result, err := FindMarkedFestivals(festivalsDir)
	if err != nil {
		t.Fatalf("FindMarkedFestivals failed: %v", err)
	}
	if result != "" {
		t.Errorf("FindMarkedFestivals should return empty string when no marker found, got %q", result)
	}
}

func TestFindAllMarkedFestivals(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested structure with multiple markers:
	// tmpDir/
	//   level1/
	//     festivals/          <- has marker
	//     level2/
	//       festivals/        <- has marker
	//       level3/
	//         code/

	level1 := filepath.Join(tmpDir, "level1")
	level1Festivals := filepath.Join(level1, "festivals")
	level1DotFestival := filepath.Join(level1Festivals, ".festival")

	level2 := filepath.Join(level1, "level2")
	level2Festivals := filepath.Join(level2, "festivals")
	level2DotFestival := filepath.Join(level2Festivals, ".festival")

	codeDir := filepath.Join(level2, "level3", "code")

	// Create all directories
	for _, dir := range []string{level1DotFestival, level2DotFestival, codeDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Register both
	if err := RegisterFestivals(level1Festivals); err != nil {
		t.Fatal(err)
	}
	if err := RegisterFestivals(level2Festivals); err != nil {
		t.Fatal(err)
	}

	// Test: from codeDir, should find both markers (level2 first, then level1)
	results, err := FindAllMarkedFestivals(codeDir)
	if err != nil {
		t.Fatalf("FindAllMarkedFestivals failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("FindAllMarkedFestivals returned %d results, want 2", len(results))
	}
	// First should be level2 (closest)
	if len(results) > 0 && results[0] != level2Festivals {
		t.Errorf("First result = %q, want %q", results[0], level2Festivals)
	}
	// Second should be level1
	if len(results) > 1 && results[1] != level1Festivals {
		t.Errorf("Second result = %q, want %q", results[1], level1Festivals)
	}
}

func TestFindNearestFestivals(t *testing.T) {
	tmpDir := t.TempDir()

	// Create structure without markers
	project := filepath.Join(tmpDir, "project")
	festivalsDir := filepath.Join(project, "festivals")
	dotFestival := filepath.Join(festivalsDir, ".festival")
	codeDir := filepath.Join(project, "src", "code")

	for _, dir := range []string{dotFestival, codeDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Test: finds nearest festivals even without marker
	result, err := FindNearestFestivals(codeDir)
	if err != nil {
		t.Fatalf("FindNearestFestivals failed: %v", err)
	}
	if result != festivalsDir {
		t.Errorf("FindNearestFestivals = %q, want %q", result, festivalsDir)
	}

	// Test: from festivals directory itself
	result, err = FindNearestFestivals(festivalsDir)
	if err != nil {
		t.Fatalf("FindNearestFestivals failed: %v", err)
	}
	if result != festivalsDir {
		t.Errorf("FindNearestFestivals = %q, want %q", result, festivalsDir)
	}
}

func TestFindNearestFestivalsRequiresDotFestival(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals directory WITHOUT .festival subdirectory
	project := filepath.Join(tmpDir, "project")
	festivalsDir := filepath.Join(project, "festivals")
	codeDir := filepath.Join(project, "src")

	for _, dir := range []string{festivalsDir, codeDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Test: should NOT find festivals without .festival subdirectory
	result, err := FindNearestFestivals(codeDir)
	if err != nil {
		t.Fatalf("FindNearestFestivals failed: %v", err)
	}
	if result != "" {
		t.Errorf("FindNearestFestivals should return empty when .festival missing, got %q", result)
	}
}

func TestFindFestivals(t *testing.T) {
	tmpDir := t.TempDir()

	// Create structure with marker
	project := filepath.Join(tmpDir, "project")
	festivalsDir := filepath.Join(project, "festivals")
	dotFestival := filepath.Join(festivalsDir, ".festival")
	codeDir := filepath.Join(project, "src")

	for _, dir := range []string{dotFestival, codeDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Test without marker: should use fallback
	result, err := FindFestivals(codeDir)
	if err != nil {
		t.Fatalf("FindFestivals failed: %v", err)
	}
	if result != festivalsDir {
		t.Errorf("FindFestivals (no marker) = %q, want %q", result, festivalsDir)
	}

	// Register and test with marker
	if err := RegisterFestivals(festivalsDir); err != nil {
		t.Fatal(err)
	}

	result, err = FindFestivals(codeDir)
	if err != nil {
		t.Fatalf("FindFestivals failed: %v", err)
	}
	if result != festivalsDir {
		t.Errorf("FindFestivals (with marker) = %q, want %q", result, festivalsDir)
	}
}

func TestFindFestivalsPrefersMark(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested structure where only outer has marker
	// tmpDir/
	//   outer/
	//     festivals/          <- has marker
	//       .festival/
	//     inner/
	//       festivals/        <- no marker (like source repo)
	//         .festival/
	//       code/

	outer := filepath.Join(tmpDir, "outer")
	outerFestivals := filepath.Join(outer, "festivals")
	outerDotFestival := filepath.Join(outerFestivals, ".festival")

	inner := filepath.Join(outer, "inner")
	innerFestivals := filepath.Join(inner, "festivals")
	innerDotFestival := filepath.Join(innerFestivals, ".festival")

	codeDir := filepath.Join(inner, "code")

	for _, dir := range []string{outerDotFestival, innerDotFestival, codeDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Register only outer
	if err := RegisterFestivals(outerFestivals); err != nil {
		t.Fatal(err)
	}

	// Test: from code dir, should find outer (marked) not inner (unmarked)
	result, err := FindFestivals(codeDir)
	if err != nil {
		t.Fatalf("FindFestivals failed: %v", err)
	}
	if result != outerFestivals {
		t.Errorf("FindFestivals should prefer marked directory, got %q, want %q", result, outerFestivals)
	}
}

// normalizePath resolves symlinks to handle macOS /var -> /private/var differences
func normalizePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

func TestFindWorkspace(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		wantType WorkspaceType
		wantErr  error
	}{
		{
			name: "campaign from root",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.MkdirAll(filepath.Join(dir, ".campaign"), 0755)
				os.MkdirAll(filepath.Join(dir, "festivals", ".festival"), 0755)
				return dir
			},
			wantType: WorkspaceTypeCampaign,
		},
		{
			name: "campaign from nested project",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.MkdirAll(filepath.Join(dir, ".campaign"), 0755)
				os.MkdirAll(filepath.Join(dir, "festivals", ".festival"), 0755)
				projectDir := filepath.Join(dir, "projects", "myproject")
				os.MkdirAll(projectDir, 0755)
				return projectDir
			},
			wantType: WorkspaceTypeCampaign,
		},
		{
			name: "fallback to marker",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.MkdirAll(filepath.Join(dir, "festivals", ".festival"), 0755)
				os.WriteFile(filepath.Join(dir, "festivals", ".festival", ".workspace"), []byte(`{"workspace":"test"}`), 0644)
				return dir
			},
			wantType: WorkspaceTypeStandalone,
		},
		{
			name: "fallback to nearest festivals",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.MkdirAll(filepath.Join(dir, "festivals", ".festival"), 0755)
				return dir
			},
			wantType: WorkspaceTypeStandalone,
		},
		{
			name: "no workspace",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: ErrNoWorkspace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			ws, err := FindWorkspace(context.Background(), dir)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("got error %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ws.Type != tt.wantType {
				t.Errorf("got type %v, want %v", ws.Type, tt.wantType)
			}
		})
	}
}

func TestFindWorkspace_CampaignPriority(t *testing.T) {
	// Create a directory with BOTH campaign marker AND workspace marker
	// Campaign should take priority
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".campaign"), 0755)
	os.MkdirAll(filepath.Join(dir, "festivals", ".festival"), 0755)
	os.WriteFile(filepath.Join(dir, "festivals", ".festival", ".workspace"), []byte(`{"workspace":"test"}`), 0644)

	ws, err := FindWorkspace(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.Type != WorkspaceTypeCampaign {
		t.Errorf("campaign should take priority over marker, got type %v", ws.Type)
	}
}

func TestFindWorkspace_CAMP_ROOT(t *testing.T) {
	campRoot := t.TempDir()
	os.MkdirAll(filepath.Join(campRoot, ".campaign"), 0755)
	os.MkdirAll(filepath.Join(campRoot, "festivals", ".festival"), 0755)

	t.Setenv("CAMP_ROOT", campRoot)

	// Search from a completely different directory
	otherDir := t.TempDir()

	ws, err := FindWorkspace(context.Background(), otherDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.Type != WorkspaceTypeCampaign {
		t.Errorf("got type %v, want campaign", ws.Type)
	}
	// Verify the correct root was used
	if normalizePath(ws.Root) != normalizePath(campRoot) {
		t.Errorf("got root %q, want %q", ws.Root, campRoot)
	}
}

func TestFindWorkspace_CAMP_ROOT_Invalid(t *testing.T) {
	// Set CAMP_ROOT to a directory without .campaign/
	invalidDir := t.TempDir()
	t.Setenv("CAMP_ROOT", invalidDir)

	// Create a valid standalone workspace to fall back to
	testDir := t.TempDir()
	os.MkdirAll(filepath.Join(testDir, "festivals", ".festival"), 0755)

	ws, err := FindWorkspace(context.Background(), testDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to standalone detection
	if ws.Type != WorkspaceTypeStandalone {
		t.Errorf("should fall back to standalone when CAMP_ROOT is invalid, got type %v", ws.Type)
	}
}

func TestFindWorkspace_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FindWorkspace(ctx, t.TempDir())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestFindWorkspace_EmptyStartDir(t *testing.T) {
	// When startDir is empty, should use current working directory
	// Create a campaign structure in a temp dir and cd to it
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".campaign"), 0755)
	os.MkdirAll(filepath.Join(dir, "festivals", ".festival"), 0755)

	oldCwd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldCwd)

	ws, err := FindWorkspace(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.Type != WorkspaceTypeCampaign {
		t.Errorf("got type %v, want campaign", ws.Type)
	}
}

func TestDetectCampaign(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr error
	}{
		{
			name: "finds campaign from root",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.MkdirAll(filepath.Join(dir, ".campaign"), 0755)
				return dir
			},
			wantErr: nil,
		},
		{
			name: "finds campaign from nested directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.MkdirAll(filepath.Join(dir, ".campaign"), 0755)
				nested := filepath.Join(dir, "projects", "myproject", "src")
				os.MkdirAll(nested, 0755)
				return nested
			},
			wantErr: nil,
		},
		{
			name: "no campaign",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: ErrNoCampaign,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			_, err := detectCampaign(context.Background(), dir)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("got error %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDetectCampaign_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := detectCampaign(ctx, t.TempDir())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestWorkspaceType_String(t *testing.T) {
	tests := []struct {
		wt   WorkspaceType
		want string
	}{
		{WorkspaceTypeCampaign, "campaign"},
		{WorkspaceTypeStandalone, "standalone"},
		{WorkspaceType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.wt.String(); got != tt.want {
				t.Errorf("WorkspaceType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCampaignRoot(t *testing.T) {
	t.Run("true when .campaign exists", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, ".campaign"), 0755)
		if !IsCampaignRoot(dir) {
			t.Error("IsCampaignRoot should return true when .campaign exists")
		}
	})

	t.Run("false when .campaign does not exist", func(t *testing.T) {
		dir := t.TempDir()
		if IsCampaignRoot(dir) {
			t.Error("IsCampaignRoot should return false when .campaign does not exist")
		}
	})

	t.Run("false when .campaign is a file", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".campaign"), []byte("not a dir"), 0644)
		if IsCampaignRoot(dir) {
			t.Error("IsCampaignRoot should return false when .campaign is a file")
		}
	})
}
