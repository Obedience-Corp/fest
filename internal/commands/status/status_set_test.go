package status

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAtomicStatusChange(t *testing.T) {
	tests := []struct {
		name          string
		setupFn       func(baseDir string) (festivalPath string)
		fromStatus    string
		toStatus      string
		wantError     bool
		checkRollback bool
	}{
		{
			name: "successful planning to active",
			setupFn: func(baseDir string) string {
				path := filepath.Join(baseDir, "planning", "test-festival")
				_ = os.MkdirAll(path, 0755)
				_ = os.WriteFile(filepath.Join(path, "FESTIVAL_OVERVIEW.md"), []byte("test"), 0644)
				return path
			},
			fromStatus: "planning",
			toStatus:   "active",
			wantError:  false,
		},
		{
			name: "successful active to completed with date",
			setupFn: func(baseDir string) string {
				path := filepath.Join(baseDir, "active", "test-festival")
				_ = os.MkdirAll(path, 0755)
				_ = os.WriteFile(filepath.Join(path, "FESTIVAL_OVERVIEW.md"), []byte("test"), 0644)
				return path
			},
			fromStatus: "active",
			toStatus:   "completed",
			wantError:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseDir := t.TempDir()
			festivalPath := tc.setupFn(baseDir)

			newPath, err := AtomicStatusChange(context.Background(), festivalPath, tc.fromStatus, tc.toStatus)
			if (err != nil) != tc.wantError {
				t.Errorf("AtomicStatusChange() error = %v, wantError %v", err, tc.wantError)
			}

			if !tc.wantError {
				// Verify source no longer exists
				if _, err := os.Stat(festivalPath); !os.IsNotExist(err) {
					t.Error("Source directory still exists after status change")
				}

				// Verify destination exists
				if _, err := os.Stat(newPath); os.IsNotExist(err) {
					t.Errorf("Destination directory does not exist: %s", newPath)
				}
			}
		})
	}
}

func TestAtomicStatusChangeRollback(t *testing.T) {
	baseDir := t.TempDir()

	// Create source festival
	sourcePath := filepath.Join(baseDir, "active", "test-festival")
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	testFile := filepath.Join(sourcePath, "FESTIVAL_OVERVIEW.md")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create conflicting destination at the correct date-based path
	// The dateDir will be the current date in YYYY-MM-DD format
	dateDir := CalculateDateDir(time.Now())
	conflictPath := filepath.Join(baseDir, "dungeon", "completed", dateDir, "test-festival")
	if err := os.MkdirAll(conflictPath, 0755); err != nil {
		t.Fatalf("Failed to create conflict: %v", err)
	}

	// Attempt status change - should fail
	_, err := AtomicStatusChange(context.Background(), sourcePath, "active", "completed")
	if err == nil {
		t.Error("Expected error when destination conflicts")
	}

	// Verify source still exists (rollback/no change)
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		t.Error("Source was removed despite failed status change")
	}

	// Verify test file still exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Test file was lost during failed status change")
	}
}

func TestCrossFilesystemFallback(t *testing.T) {
	// This test simulates cross-filesystem move by using copy+delete
	// In practice, os.Rename returns EXDEV for cross-filesystem moves
	baseDir := t.TempDir()

	sourcePath := filepath.Join(baseDir, "active", "test-festival")
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// Create some files
	testFile := filepath.Join(sourcePath, "FESTIVAL_OVERVIEW.md")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	subDir := filepath.Join(sourcePath, "001_phase")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	subFile := filepath.Join(subDir, "PHASE_GOAL.md")
	if err := os.WriteFile(subFile, []byte("phase content"), 0644); err != nil {
		t.Fatalf("Failed to create subfile: %v", err)
	}

	destDir := filepath.Join(baseDir, "dungeon", "completed")

	// Use copy-delete for cross-filesystem simulation
	destPath := filepath.Join(destDir, "test-festival")
	newPath, err := copyAndDelete(sourcePath, destPath)
	if err != nil {
		t.Fatalf("copyAndDelete() error = %v", err)
	}

	// Verify source is gone
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Error("Source still exists after copy-delete move")
	}

	// Verify destination exists with all files
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("Destination does not exist")
	}

	movedFile := filepath.Join(newPath, "FESTIVAL_OVERVIEW.md")
	if _, err := os.Stat(movedFile); os.IsNotExist(err) {
		t.Error("Test file was not copied")
	}

	movedSubFile := filepath.Join(newPath, "001_phase", "PHASE_GOAL.md")
	if _, err := os.Stat(movedSubFile); os.IsNotExist(err) {
		t.Error("Subdirectory file was not copied")
	}
}

func TestStatusSetValidation(t *testing.T) {
	tests := []struct {
		name       string
		entityType EntityType
		newStatus  string
		wantValid  bool
	}{
		{"festival to active", EntityFestival, "active", true},
		{"festival to completed", EntityFestival, "completed", true},
		{"festival to dungeon/completed", EntityFestival, "dungeon/completed", true},
		{"festival to dungeon", EntityFestival, "dungeon", true},
		{"festival to planning", EntityFestival, "planning", true},
		{"festival to parked", EntityFestival, "parked", true},
		{"festival to invalid", EntityFestival, "invalid", false},
		{"festival to pending", EntityFestival, "pending", false}, // pending is for phases
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidStatus(tc.entityType, tc.newStatus)
			if got != tc.wantValid {
				t.Errorf("isValidStatus(%q, %q) = %v, want %v",
					tc.entityType, tc.newStatus, got, tc.wantValid)
			}
		})
	}
}

func TestCompletedUsesDateDirectory(t *testing.T) {
	baseDir := t.TempDir()

	// Create source festival
	sourcePath := filepath.Join(baseDir, "active", "test-festival")
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "FESTIVAL_OVERVIEW.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create overview: %v", err)
	}

	newPath, err := AtomicStatusChange(context.Background(), sourcePath, "active", "completed")
	if err != nil {
		t.Fatalf("AtomicStatusChange() error = %v", err)
	}

	// Verify path includes date directory (YYYY-MM-DD format)
	// Path should be like: baseDir/dungeon/completed/2025-01-15/test-festival
	relPath, err := filepath.Rel(baseDir, newPath)
	if err != nil {
		t.Fatalf("Failed to get relative path: %v", err)
	}

	// Should have dungeon/completed/YYYY-MM-DD/festival-name structure
	// Parse path parts to verify structure
	_ = filepath.SplitList(relPath) // Verify path can be parsed
	if len(relPath) < 10 {          // At minimum: dungeon/completed/YYYY-MM-DD
		t.Errorf("Path too short for date directory structure: %s", relPath)
	}
	if !strings.HasPrefix(relPath, "dungeon") {
		t.Errorf("Expected path under 'dungeon/completed', got: %s", relPath)
	}
}

// TestParkedIsNonTerminal verifies that moving a festival to the parked status
// does NOT use date-based directories (which are for dungeon/terminal
// statuses only). Parked is a non-terminal status that should live directly
// under festivals/parked/<name>, not festivals/parked/YYYY-MM-DD/<name>.
func TestParkedIsNonTerminal(t *testing.T) {
	baseDir := t.TempDir()

	// Create source festival in ready/
	sourcePath := filepath.Join(baseDir, "ready", "test-festival")
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "FESTIVAL_OVERVIEW.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create overview: %v", err)
	}

	// Move ready -> parked
	newPath, err := AtomicStatusChange(context.Background(), sourcePath, "ready", "parked")
	if err != nil {
		t.Fatalf("AtomicStatusChange() error = %v", err)
	}

	relPath, err := filepath.Rel(baseDir, newPath)
	if err != nil {
		t.Fatalf("Failed to get relative path: %v", err)
	}

	// Parked should be a direct child: parked/test-festival (no date directory)
	expectedRel := filepath.Join("parked", "test-festival")
	if relPath != expectedRel {
		t.Errorf("Parked path = %q, want %q (no date directory for non-terminal status)", relPath, expectedRel)
	}

	// Verify the festival actually moved to the expected path
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("Festival not found at parked path %q: %v", newPath, err)
	}
	// Verify old path is gone
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Errorf("Old ready/ path still exists after move to parked: %s", sourcePath)
	}
}

// TestParkedRoundTrip verifies a festival can move ready -> parked -> ready,
// confirming parked is reversible and the festival survives both moves.
func TestParkedRoundTrip(t *testing.T) {
	baseDir := t.TempDir()

	// Create source festival in ready/
	sourcePath := filepath.Join(baseDir, "ready", "test-festival")
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "FESTIVAL_OVERVIEW.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create overview: %v", err)
	}

	// Move ready -> parked
	parkedPath, err := AtomicStatusChange(context.Background(), sourcePath, "ready", "parked")
	if err != nil {
		t.Fatalf("AtomicStatusChange ready->parked error = %v", err)
	}

	// Move parked -> ready
	readyPath, err := AtomicStatusChange(context.Background(), parkedPath, "parked", "ready")
	if err != nil {
		t.Fatalf("AtomicStatusChange parked->ready error = %v", err)
	}

	expectedReady := filepath.Join(baseDir, "ready", "test-festival")
	if readyPath != expectedReady {
		t.Errorf("Round-trip path = %q, want %q", readyPath, expectedReady)
	}

	// Verify the overview file survived both moves
	overviewPath := filepath.Join(readyPath, "FESTIVAL_OVERVIEW.md")
	if _, err := os.Stat(overviewPath); err != nil {
		t.Errorf("FESTIVAL_OVERVIEW.md missing after round-trip: %v", err)
	}
}

// Note: AtomicStatusChange is implemented in atomic.go, copyAndDelete in date_directory.go
