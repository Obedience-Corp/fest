package status

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/scope"
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
	_ = os.MkdirAll(planningDir, 0755)
	_ = os.MkdirAll(activeDir, 0755)

	// Create a festival in planning
	festPath := filepath.Join(planningDir, "test-fest")
	_ = os.MkdirAll(filepath.Join(festPath, "001_IMPL"), 0755)
	_ = os.WriteFile(filepath.Join(festPath, "FESTIVAL_OVERVIEW.md"), []byte("# Test"), 0644)

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

// initTestRepo creates a git repo in dir with an initial commit.
func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}
}

func TestAutoCommitStatusChange_SelectiveStaging(t *testing.T) {
	root := t.TempDir()
	initTestRepo(t, root)

	// Create festival directories simulating a move from active/ to dungeon/completed/
	activeDir := filepath.Join(root, "festivals", "active", "my-fest")
	completedDir := filepath.Join(root, "festivals", "dungeon", "completed", "my-fest")
	if err := os.MkdirAll(activeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(completedDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Write a file in the completed destination (simulates the moved festival)
	if err := os.WriteFile(filepath.Join(completedDir, "FESTIVAL_GOAL.md"), []byte("# Goal"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an unrelated dirty file at the workspace root
	unrelatedFile := filepath.Join(root, "unrelated.txt")
	if err := os.WriteFile(unrelatedFile, []byte("should not be committed"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set up context with workspace info
	ctx := context.Background()
	ws := &scope.WorkspaceInfo{
		Root:          root,
		FestivalsPath: filepath.Join(root, "festivals"),
		Type:          scope.WorkspaceTypeStandalone,
	}
	ctx = scope.WithWorkspace(ctx, ws)

	// Call AutoCommitStatusChange with only the festival paths
	changedPaths := []string{activeDir, completedDir}
	hash, err := AutoCommitStatusChange(ctx, "my-fest", "MF001", "active", "dungeon/completed", changedPaths)
	if err != nil {
		t.Fatalf("AutoCommitStatusChange failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected a commit hash, got empty string")
	}

	// Verify: unrelated file should NOT be staged or committed.
	// Check git status — the unrelated file should still be untracked.
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status failed: %v\n%s", err, out)
	}

	statusOutput := string(out)
	if !strings.Contains(statusOutput, "?? unrelated.txt") {
		t.Errorf("unrelated.txt should remain untracked, got status:\n%s", statusOutput)
	}

	// Also verify the festival files were committed by checking git log --name-only
	cmd = exec.Command("git", "log", "-1", "--name-only", "--pretty=format:")
	cmd.Dir = root
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log failed: %v\n%s", err, out)
	}
	committedFiles := string(out)
	if strings.Contains(committedFiles, "unrelated.txt") {
		t.Errorf("unrelated.txt should NOT appear in committed files:\n%s", committedFiles)
	}
}
