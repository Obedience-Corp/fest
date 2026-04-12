package navigation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsPhaseShortcut(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1", true},
		{"01", true},
		{"001", true},
		{"2", true},
		{"12", true},
		{"123", true},
		{"", false},
		{"1234", false},     // Too long
		{"abc", false},      // Not numeric
		{"1a", false},       // Mixed
		{"a1", false},       // Mixed
		{"001_PLAN", false}, // Full phase name
	}

	for _, tc := range tests {
		result := isPhaseShortcut(tc.input)
		if result != tc.expected {
			t.Errorf("isPhaseShortcut(%q) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}

func TestIsSequenceShortcut(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1", true},
		{"01", true},
		{"12", true},
		{"", false},
		{"123", false},      // Too long for sequence (max 2 digits)
		{"abc", false},      // Not numeric
		{"01_setup", false}, // Full sequence name
		{"1a", false},       // Mixed
	}

	for _, tc := range tests {
		result := isSequenceShortcut(tc.input)
		if result != tc.expected {
			t.Errorf("isSequenceShortcut(%q) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}

func TestResolvePhaseShortcut(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals structure with phases
	festivalsDir := filepath.Join(tmpDir, "festivals")
	activeDir := filepath.Join(festivalsDir, "active")
	planningDir := filepath.Join(festivalsDir, "planning")

	// Create some phases
	phases := []string{
		filepath.Join(activeDir, "my-festival", "001_PLAN"),
		filepath.Join(activeDir, "my-festival", "002_IMPLEMENT"),
		filepath.Join(activeDir, "my-festival", "003_REVIEW"),
		filepath.Join(planningDir, "another", "001_DISCOVERY"),
	}

	for _, p := range phases {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		shortcut string
		expected string
		wantErr  bool
	}{
		{"1", filepath.Join(activeDir, "my-festival", "001_PLAN"), false},
		{"01", filepath.Join(activeDir, "my-festival", "001_PLAN"), false},
		{"001", filepath.Join(activeDir, "my-festival", "001_PLAN"), false},
		{"2", filepath.Join(activeDir, "my-festival", "002_IMPLEMENT"), false},
		{"3", filepath.Join(activeDir, "my-festival", "003_REVIEW"), false},
		{"999", "", true}, // Non-existent phase
	}

	for _, tc := range tests {
		result, err := resolvePhaseShortcut(tc.shortcut, filepath.Join(activeDir, "my-festival"))
		if tc.wantErr {
			if err == nil {
				t.Errorf("resolvePhaseShortcut(%q) expected error, got nil", tc.shortcut)
			}
		} else {
			if err != nil {
				t.Errorf("resolvePhaseShortcut(%q) unexpected error: %v", tc.shortcut, err)
			} else if result != tc.expected {
				t.Errorf("resolvePhaseShortcut(%q) = %q, want %q", tc.shortcut, result, tc.expected)
			}
		}
	}
}

func TestResolveSequenceShortcut(t *testing.T) {
	tmpDir := t.TempDir()

	// Create phase with sequences
	phaseDir := filepath.Join(tmpDir, "001_PLAN")
	sequences := []string{
		filepath.Join(phaseDir, "01_requirements"),
		filepath.Join(phaseDir, "02_design"),
		filepath.Join(phaseDir, "03_review"),
	}

	for _, s := range sequences {
		if err := os.MkdirAll(s, 0755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		shortcut string
		expected string
		wantErr  bool
	}{
		{"1", filepath.Join(phaseDir, "01_requirements"), false},
		{"01", filepath.Join(phaseDir, "01_requirements"), false},
		{"2", filepath.Join(phaseDir, "02_design"), false},
		{"3", filepath.Join(phaseDir, "03_review"), false},
		{"99", "", true}, // Non-existent sequence
	}

	for _, tc := range tests {
		result, err := resolveSequenceShortcut(tc.shortcut, phaseDir)
		if tc.wantErr {
			if err == nil {
				t.Errorf("resolveSequenceShortcut(%q) expected error, got nil", tc.shortcut)
			}
		} else {
			if err != nil {
				t.Errorf("resolveSequenceShortcut(%q) unexpected error: %v", tc.shortcut, err)
			} else if result != tc.expected {
				t.Errorf("resolveSequenceShortcut(%q) = %q, want %q", tc.shortcut, result, tc.expected)
			}
		}
	}
}

func TestResolveGoTarget(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals structure - phases should be in active/
	festivalsDir := filepath.Join(tmpDir, "festivals")
	activeDir := filepath.Join(festivalsDir, "active")

	// Create phases directly in active/ (not nested in another festival dir)
	phase1 := filepath.Join(activeDir, "001_PLAN")
	seq1 := filepath.Join(phase1, "01_requirements")

	for _, d := range []string{seq1, filepath.Join(festivalsDir, "custom", "path")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		target   string
		expected string
		wantErr  bool
	}{
		// Phase shortcuts - resolvePhaseShortcut searches active/, planning/, completed/
		{"1", phase1, false},
		{"001", phase1, false},

		// Relative paths
		{"custom/path", filepath.Join(festivalsDir, "custom", "path"), false},
		{"active", activeDir, false},

		// Non-existent
		{"nonexistent", "", true},
	}

	for _, tc := range tests {
		result, err := resolveGoTarget(tc.target, festivalsDir)
		if tc.wantErr {
			if err == nil {
				t.Errorf("resolveGoTarget(%q) expected error, got nil", tc.target)
			}
		} else {
			if err != nil {
				t.Errorf("resolveGoTarget(%q) unexpected error: %v", tc.target, err)
			} else if result != tc.expected {
				t.Errorf("resolveGoTarget(%q) = %q, want %q", tc.target, result, tc.expected)
			}
		}
	}
}

func TestResolveGoTargetWithSequence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals structure with phase and sequence
	// Phases should be in active/ directly
	festivalsDir := filepath.Join(tmpDir, "festivals")
	activeDir := filepath.Join(festivalsDir, "active")
	phase1 := filepath.Join(activeDir, "001_PLAN")
	seq1 := filepath.Join(phase1, "01_requirements")
	seq2 := filepath.Join(phase1, "02_design")

	for _, d := range []string{seq1, seq2} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		target   string
		expected string
		wantErr  bool
	}{
		// Phase/sequence shortcuts
		{"1/1", seq1, false},
		{"1/01", seq1, false},
		{"001/01", seq1, false},
		{"1/2", seq2, false},

		// Invalid sequence
		{"1/99", "", true},
	}

	for _, tc := range tests {
		result, err := resolveGoTarget(tc.target, festivalsDir)
		if tc.wantErr {
			if err == nil {
				t.Errorf("resolveGoTarget(%q) expected error, got nil", tc.target)
			}
		} else {
			if err != nil {
				t.Errorf("resolveGoTarget(%q) unexpected error: %v", tc.target, err)
			} else if result != tc.expected {
				t.Errorf("resolveGoTarget(%q) = %q, want %q", tc.target, result, tc.expected)
			}
		}
	}
}

func TestResolveFestivalByName(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")

	// Create festivals in different status directories
	activeFest := filepath.Join(festivalsDir, "active", "fest-cli")
	planningFest := filepath.Join(festivalsDir, "planning", "new-feature")
	completedFest := filepath.Join(festivalsDir, "dungeon", "completed", "old-project")

	for _, f := range []string{activeFest, planningFest, completedFest} {
		if err := os.MkdirAll(f, 0755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name     string
		expected string
	}{
		{"fest-cli", activeFest},
		{"new-feature", planningFest},
		{"old-project", completedFest},
		{"nonexistent", ""},
	}

	for _, tc := range tests {
		result := resolveFestivalByName(tc.name, festivalsDir)
		if result != tc.expected {
			t.Errorf("resolveFestivalByName(%q) = %q, want %q", tc.name, result, tc.expected)
		}
	}
}

func TestResolveGoTargetDungeonAliases(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")

	// Create dungeon status directories
	dungeonDirs := []string{
		filepath.Join(festivalsDir, "dungeon", "completed"),
		filepath.Join(festivalsDir, "dungeon", "archived"),
		filepath.Join(festivalsDir, "dungeon", "someday"),
	}
	for _, d := range dungeonDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		target   string
		expected string
		wantErr  bool
	}{
		{"completed", filepath.Join(festivalsDir, "dungeon", "completed"), false},
		{"archived", filepath.Join(festivalsDir, "dungeon", "archived"), false},
		{"someday", filepath.Join(festivalsDir, "dungeon", "someday"), false},
	}

	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			result, err := resolveGoTarget(tc.target, festivalsDir)
			if tc.wantErr {
				if err == nil {
					t.Errorf("resolveGoTarget(%q) expected error, got nil", tc.target)
				}
			} else {
				if err != nil {
					t.Errorf("resolveGoTarget(%q) unexpected error: %v", tc.target, err)
				} else if result != tc.expected {
					t.Errorf("resolveGoTarget(%q) = %q, want %q", tc.target, result, tc.expected)
				}
			}
		})
	}
}

func TestResolveFestivalByName_FindsDungeonDateBucket(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")

	// Mirror the real on-disk layout: dungeon/<substatus>/YYYY-MM-DD/<name>
	target := filepath.Join(festivalsDir, "dungeon", "completed", "2026-02-10",
		"feb-9th-gathered-improvements-FG0001")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}

	got := resolveFestivalByName("feb-9th-gathered-improvements-FG0001", festivalsDir)
	if got != target {
		t.Errorf("resolveFestivalByName() = %q, want %q", got, target)
	}
}

func TestResolveFestivalByName_FindsDungeonDateBucket_Someday(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")

	target := filepath.Join(festivalsDir, "dungeon", "someday", "2026-04-01",
		"maybe-later-ML0001")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}

	got := resolveFestivalByName("maybe-later-ML0001", festivalsDir)
	if got != target {
		t.Errorf("resolveFestivalByName() = %q, want %q", got, target)
	}
}

func TestResolveFestivalByName_PicksNewestBucketOnCollision(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")
	name := "cloned-fest-CF0001"

	// Same festival name exists in two date buckets — the resolver should
	// return the newest one to match sortByStatusDate's newest-first order.
	olderPath := filepath.Join(festivalsDir, "dungeon", "completed", "2026-01-10", name)
	newerPath := filepath.Join(festivalsDir, "dungeon", "completed", "2026-03-15", name)
	for _, d := range []string{olderPath, newerPath} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	got := resolveFestivalByName(name, festivalsDir)
	if got != newerPath {
		t.Errorf("resolveFestivalByName() = %q, want %q (newest bucket should win)", got, newerPath)
	}
}

func TestResolveFestivalByName_PrefersActiveOverDungeon(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")
	name := "shared-name-SH0001"

	activePath := filepath.Join(festivalsDir, "active", name)
	dungeonPath := filepath.Join(festivalsDir, "dungeon", "completed", "2026-02-01", name)
	for _, d := range []string{activePath, dungeonPath} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	got := resolveFestivalByName(name, festivalsDir)
	if got != activePath {
		t.Errorf("resolveFestivalByName() = %q, want %q (active should win over dungeon)", got, activePath)
	}
}

func TestResolveFestivalByName_IgnoresNonDateDirsInDungeon(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")

	// A non-date directory (e.g., a README-shaped folder) should not be
	// descended as if it were a bucket.
	notes := filepath.Join(festivalsDir, "dungeon", "completed", "notes", "my-fest")
	if err := os.MkdirAll(notes, 0755); err != nil {
		t.Fatal(err)
	}

	if got := resolveFestivalByName("my-fest", festivalsDir); got != "" {
		t.Errorf("resolveFestivalByName() = %q, want empty (non-date-dir subdirs should not be descended)", got)
	}
}

func TestResolveGoTargetWithFestivalName(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")

	// Create a festival
	festPath := filepath.Join(festivalsDir, "active", "my-festival")
	if err := os.MkdirAll(festPath, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		target   string
		expected string
		wantErr  bool
	}{
		// Festival name resolution
		{"my-festival", festPath, false},

		// Non-existent festival
		{"nonexistent", "", true},
	}

	for _, tc := range tests {
		result, err := resolveGoTarget(tc.target, festivalsDir)
		if tc.wantErr {
			if err == nil {
				t.Errorf("resolveGoTarget(%q) expected error, got nil", tc.target)
			}
		} else {
			if err != nil {
				t.Errorf("resolveGoTarget(%q) unexpected error: %v", tc.target, err)
			} else if result != tc.expected {
				t.Errorf("resolveGoTarget(%q) = %q, want %q", tc.target, result, tc.expected)
			}
		}
	}
}
