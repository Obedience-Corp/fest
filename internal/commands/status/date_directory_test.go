package status

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCalculateDateDir(t *testing.T) {
	tests := []struct {
		name      string
		timestamp time.Time
		want      string
	}{
		// Standard cases
		{"january 2025", time.Date(2025, 1, 15, 0, 0, 0, 0, time.Local), "2025-01-15"},
		{"december 2024", time.Date(2024, 12, 31, 23, 59, 59, 0, time.Local), "2024-12-31"},
		{"mid month", time.Date(2025, 6, 15, 12, 0, 0, 0, time.Local), "2025-06-15"},

		// Month boundaries
		{"month boundary start", time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local), "2025-02-01"},
		{"month boundary end", time.Date(2025, 3, 31, 23, 59, 59, 0, time.Local), "2025-03-31"},
		{"last of january", time.Date(2025, 1, 31, 23, 59, 59, 999999999, time.Local), "2025-01-31"},

		// Year boundaries
		{"new years eve", time.Date(2024, 12, 31, 23, 59, 59, 0, time.Local), "2024-12-31"},
		{"new years day", time.Date(2025, 1, 1, 0, 0, 1, 0, time.Local), "2025-01-01"},
		{"new year 2026", time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local), "2026-01-01"},

		// Leap year
		{"leap year february", time.Date(2024, 2, 29, 12, 0, 0, 0, time.Local), "2024-02-29"},

		// Single digit months and days (verify zero-padding)
		{"single digit january", time.Date(2025, 1, 5, 0, 0, 0, 0, time.Local), "2025-01-05"},
		{"single digit september", time.Date(2025, 9, 3, 0, 0, 0, 0, time.Local), "2025-09-03"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateDateDir(tc.timestamp)
			if got != tc.want {
				t.Errorf("CalculateDateDir(%v) = %q, want %q",
					tc.timestamp, got, tc.want)
			}
		})
	}
}

func TestCalculateDateDirNow(t *testing.T) {
	now := time.Now()
	expected := now.Format("2006-01-02")
	got := CalculateDateDir(now)

	if got != expected {
		t.Errorf("CalculateDateDir(now) = %q, want %q", got, expected)
	}
}

func TestCalculateDateDir_UsesLocalTime(t *testing.T) {
	// Verify that the function uses the time as provided (local time)
	utcTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	localTime := utcTime.Local()

	utcResult := CalculateDateDir(utcTime)
	localResult := CalculateDateDir(localTime)

	// UTC Jan 1 00:00 should give 2025-01-01 in UTC
	if utcResult != "2025-01-01" {
		t.Errorf("UTC time: got %q, want %q", utcResult, "2025-01-01")
	}

	// Local result depends on timezone, but should match local time's date
	expectedLocal := localTime.Format("2006-01-02")
	if localResult != expectedLocal {
		t.Errorf("Local time: got %q, want %q", localResult, expectedLocal)
	}
}

func TestCreateDateDirectory(t *testing.T) {
	tests := []struct {
		name      string
		dateDir   string
		wantError bool
	}{
		{"valid date dir YYYY-MM-DD", "2025-01-15", false},
		{"valid date dir YYYY-MM", "2024-12", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseDir := t.TempDir()
			completedDir := filepath.Join(baseDir, "dungeon", "completed")

			err := CreateDateDirectory(completedDir, tc.dateDir)
			if (err != nil) != tc.wantError {
				t.Errorf("CreateDateDirectory() error = %v, wantError %v", err, tc.wantError)
			}

			if !tc.wantError {
				expectedPath := filepath.Join(completedDir, tc.dateDir)
				if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
					t.Errorf("Date directory was not created at %s", expectedPath)
				}
			}
		})
	}
}

func TestCreateDateDirectoryIdempotent(t *testing.T) {
	baseDir := t.TempDir()
	completedDir := filepath.Join(baseDir, "dungeon", "completed")
	dateDir := "2025-01-15"

	if err := CreateDateDirectory(completedDir, dateDir); err != nil {
		t.Fatalf("First CreateDateDirectory() failed: %v", err)
	}

	if err := CreateDateDirectory(completedDir, dateDir); err != nil {
		t.Errorf("Second CreateDateDirectory() should be idempotent, got error: %v", err)
	}
}

func TestMoveToDateDirectory(t *testing.T) {
	tests := []struct {
		name          string
		festivalName  string
		dateDir       string
		wantError     bool
		setupExisting bool // if true, create conflicting destination
	}{
		{"normal move", "my-festival", "2025-01-15", false, false},
		{"conflict exists", "my-festival", "2025-01-15", true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseDir := t.TempDir()

			// Create source festival
			activePath := filepath.Join(baseDir, "active", tc.festivalName)
			if err := os.MkdirAll(activePath, 0755); err != nil {
				t.Fatalf("Failed to create source: %v", err)
			}

			// Create a file to verify move
			testFile := filepath.Join(activePath, "FESTIVAL_OVERVIEW.md")
			if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			completedDir := filepath.Join(baseDir, "dungeon", "completed")
			if err := os.MkdirAll(completedDir, 0755); err != nil {
				t.Fatalf("Failed to create completed dir: %v", err)
			}

			if tc.setupExisting {
				existingPath := filepath.Join(completedDir, tc.dateDir, tc.festivalName)
				if err := os.MkdirAll(existingPath, 0755); err != nil {
					t.Fatalf("Failed to create existing path: %v", err)
				}
			}

			newPath, err := MoveToDateDirectory(activePath, completedDir, tc.dateDir)
			if (err != nil) != tc.wantError {
				t.Errorf("MoveToDateDirectory() error = %v, wantError %v", err, tc.wantError)
			}

			if !tc.wantError {
				if _, err := os.Stat(activePath); !os.IsNotExist(err) {
					t.Error("Source directory still exists after move")
				}
				if _, err := os.Stat(newPath); os.IsNotExist(err) {
					t.Error("Destination directory does not exist after move")
				}
				movedFile := filepath.Join(newPath, "FESTIVAL_OVERVIEW.md")
				if _, err := os.Stat(movedFile); os.IsNotExist(err) {
					t.Error("Test file was not moved with directory")
				}
			}
		})
	}
}

func TestMoveToDateDirectoryCreatesParent(t *testing.T) {
	baseDir := t.TempDir()

	sourcePath := filepath.Join(baseDir, "active", "test-festival")
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	completedDir := filepath.Join(baseDir, "dungeon", "completed")

	_, err := MoveToDateDirectory(sourcePath, completedDir, "2025-01-15")
	if err != nil {
		t.Errorf("MoveToDateDirectory() should create parent dirs, got error: %v", err)
	}
}

func TestGetCompletedPath(t *testing.T) {
	tests := []struct {
		name         string
		festivalName string
		dateDir      string
		want         string
	}{
		{"simple", "my-festival", "2025-01-15", "dungeon/completed/2025-01-15/my-festival"},
		{"with suffix", "my-project_AB0001", "2024-12-31", "dungeon/completed/2024-12-31/my-project_AB0001"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			festivalsRoot := "/festivals"
			got := GetCompletedPath(festivalsRoot, tc.festivalName, tc.dateDir)
			want := filepath.Join(festivalsRoot, tc.want)
			if got != want {
				t.Errorf("GetCompletedPath() = %q, want %q", got, want)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	t.Run("copies content and mode", func(t *testing.T) {
		root := resolvePath(t, t.TempDir())
		srcFile := filepath.Join(root, "source.txt")
		dstFile := filepath.Join(root, "dest.txt")

		content := []byte("hello world")
		mode := os.FileMode(0755)

		if err := os.WriteFile(srcFile, content, mode); err != nil {
			t.Fatalf("failed to write source: %v", err)
		}

		if err := copyFile(srcFile, dstFile, mode); err != nil {
			t.Fatalf("copyFile failed: %v", err)
		}

		got, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatalf("failed to read destination: %v", err)
		}
		if string(got) != string(content) {
			t.Errorf("content = %q, want %q", string(got), string(content))
		}

		info, err := os.Stat(dstFile)
		if err != nil {
			t.Fatalf("failed to stat destination: %v", err)
		}
		if info.Mode().Perm() != mode.Perm() {
			t.Errorf("mode = %v, want %v", info.Mode().Perm(), mode.Perm())
		}
	})

	t.Run("source not found returns error", func(t *testing.T) {
		root := resolvePath(t, t.TempDir())
		err := copyFile(
			filepath.Join(root, "nonexistent.txt"),
			filepath.Join(root, "dest.txt"),
			0644,
		)
		if err == nil {
			t.Fatal("expected error for nonexistent source")
		}
	})

	t.Run("invalid destination returns error", func(t *testing.T) {
		root := resolvePath(t, t.TempDir())
		srcFile := filepath.Join(root, "source.txt")
		os.WriteFile(srcFile, []byte("data"), 0644)

		badDest := filepath.Join(root, "nonexistent", "subdir", "dest.txt")
		err := copyFile(srcFile, badDest, 0644)
		if err == nil {
			t.Fatal("expected error for invalid destination path")
		}
	})
}

func TestLooksLikeDateDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"2025-01-15", true},
		{"2024-12-31", true},
		{"2025-01", true},
		{"2024-02", true},
		{"not-a-date", false},
		{"my-festival", false},
		{"2025", false},
		{"01-15", false},
		{"2025-1-15", false}, // missing zero padding
		{"2025-01-5", false}, // missing zero padding
		{"20250115", false},  // no hyphens
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LooksLikeDateDir(tc.name)
			if got != tc.want {
				t.Errorf("LooksLikeDateDir(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
