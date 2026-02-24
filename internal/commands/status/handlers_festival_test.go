package status

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCwdInsideFestivalDetection(t *testing.T) {
	tests := []struct {
		name         string
		festivalPath string
		cwd          string
		wantInside   bool
	}{
		{
			name:         "cwd is festival root",
			festivalPath: "/tmp/festivals/active/my-fest",
			cwd:          "/tmp/festivals/active/my-fest",
			wantInside:   true,
		},
		{
			name:         "cwd is inside a phase",
			festivalPath: "/tmp/festivals/active/my-fest",
			cwd:          "/tmp/festivals/active/my-fest/001_PLAN",
			wantInside:   true,
		},
		{
			name:         "cwd is deeply nested",
			festivalPath: "/tmp/festivals/active/my-fest",
			cwd:          "/tmp/festivals/active/my-fest/001_PLAN/01_seq/artifacts",
			wantInside:   true,
		},
		{
			name:         "cwd is outside festival",
			festivalPath: "/tmp/festivals/active/my-fest",
			cwd:          "/tmp/festivals/active",
			wantInside:   false,
		},
		{
			name:         "cwd is sibling festival",
			festivalPath: "/tmp/festivals/active/my-fest",
			cwd:          "/tmp/festivals/active/other-fest",
			wantInside:   false,
		},
		{
			name:         "cwd is completely different path",
			festivalPath: "/tmp/festivals/active/my-fest",
			cwd:          "/home/user/projects",
			wantInside:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel, err := filepath.Rel(tt.festivalPath, tt.cwd)
			inside := err == nil && !strings.HasPrefix(rel, "..")
			if inside != tt.wantInside {
				t.Errorf("got inside=%v, want %v (rel=%q)", inside, tt.wantInside, rel)
			}
		})
	}
}

func TestComputeNewCwd(t *testing.T) {
	tests := []struct {
		name        string
		oldFestPath string
		cwd         string
		newFestPath string
		wantNewCwd  string
	}{
		{
			name:        "at festival root",
			oldFestPath: "/tmp/festivals/planning/my-fest",
			cwd:         "/tmp/festivals/planning/my-fest",
			newFestPath: "/tmp/festivals/active/my-fest",
			wantNewCwd:  "/tmp/festivals/active/my-fest",
		},
		{
			name:        "inside phase directory",
			oldFestPath: "/tmp/festivals/planning/my-fest",
			cwd:         "/tmp/festivals/planning/my-fest/001_PLAN/01_research",
			newFestPath: "/tmp/festivals/active/my-fest",
			wantNewCwd:  "/tmp/festivals/active/my-fest/001_PLAN/01_research",
		},
		{
			name:        "inside deeply nested path",
			oldFestPath: "/tmp/festivals/planning/my-fest",
			cwd:         "/tmp/festivals/planning/my-fest/002_IMPL/01_seq/artifacts/notes",
			newFestPath: "/tmp/festivals/active/my-fest",
			wantNewCwd:  "/tmp/festivals/active/my-fest/002_IMPL/01_seq/artifacts/notes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel, err := filepath.Rel(tt.oldFestPath, tt.cwd)
			if err != nil {
				t.Fatal(err)
			}
			newCwd := filepath.Join(tt.newFestPath, rel)
			if newCwd != tt.wantNewCwd {
				t.Errorf("got %q, want %q", newCwd, tt.wantNewCwd)
			}
		})
	}
}

func TestFestivalMoveDirectoryStructure(t *testing.T) {
	root := t.TempDir()
	planningDir := filepath.Join(root, "planning")
	activeDir := filepath.Join(root, "active")
	os.MkdirAll(planningDir, 0755)
	os.MkdirAll(activeDir, 0755)

	// Create a festival in planning
	festPath := filepath.Join(planningDir, "test-fest")
	os.MkdirAll(filepath.Join(festPath, "001_IMPL"), 0755)
	os.WriteFile(filepath.Join(festPath, "FESTIVAL_OVERVIEW.md"), []byte("# Test"), 0644)

	// Verify the festival directory exists before move
	if _, err := os.Stat(festPath); err != nil {
		t.Fatalf("festival dir should exist: %v", err)
	}

	// Simulate the move
	newPath := filepath.Join(activeDir, "test-fest")
	if err := os.Rename(festPath, newPath); err != nil {
		t.Fatalf("move failed: %v", err)
	}

	// Old location should be gone
	if _, err := os.Stat(festPath); err == nil {
		t.Error("festival should not exist at old path")
	}

	// New location should exist with contents
	if _, err := os.Stat(newPath); err != nil {
		t.Error("festival should exist at new path")
	}
	if _, err := os.Stat(filepath.Join(newPath, "001_IMPL")); err != nil {
		t.Error("phase directory should exist at new path")
	}
	if _, err := os.Stat(filepath.Join(newPath, "FESTIVAL_OVERVIEW.md")); err != nil {
		t.Error("overview file should exist at new path")
	}
}
