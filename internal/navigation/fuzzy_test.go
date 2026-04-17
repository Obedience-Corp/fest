package navigation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFuzzyFinder_Find(t *testing.T) {
	tests := []struct {
		name    string
		targets []FuzzyTarget
		pattern string
		wantLen int
		wantTop string // expected top match name
	}{
		{
			name: "exact match",
			targets: []FuzzyTarget{
				{Name: "fest-improvements-FI0002", Path: "/path/to/fest"},
				{Name: "other-festival", Path: "/path/to/other"},
			},
			pattern: "fest-improvements",
			wantLen: 1,
			wantTop: "fest-improvements-FI0002",
		},
		{
			name: "fuzzy match",
			targets: []FuzzyTarget{
				{Name: "001_CRITICAL_BUGS_AND_TRACKING", Path: "/path/001"},
				{Name: "002_IMPLEMENT", Path: "/path/002"},
				{Name: "003_FOUNDATION", Path: "/path/003"},
			},
			pattern: "impl",
			wantLen: 1,
			wantTop: "002_IMPLEMENT",
		},
		{
			name: "multi-word pattern",
			targets: []FuzzyTarget{
				{Name: "002_IMPLEMENT/01_api", Path: "/path/001"},
				{Name: "002_IMPLEMENT/02_service", Path: "/path/002"},
				{Name: "003_FOUNDATION/01_api", Path: "/path/003"},
			},
			pattern: "impl api",
			wantLen: 1,
			wantTop: "002_IMPLEMENT/01_api",
		},
		{
			name: "no match",
			targets: []FuzzyTarget{
				{Name: "one", Path: "/path/one"},
				{Name: "two", Path: "/path/two"},
			},
			pattern: "xyz",
			wantLen: 0,
		},
		{
			name:    "empty targets",
			targets: []FuzzyTarget{},
			pattern: "test",
			wantLen: 0,
		},
		{
			name: "empty pattern",
			targets: []FuzzyTarget{
				{Name: "test", Path: "/path"},
			},
			pattern: "",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder := NewFuzzyFinder(tt.targets)
			matches := finder.Find(tt.pattern)

			assert.Len(t, matches, tt.wantLen)
			if tt.wantLen > 0 && tt.wantTop != "" {
				assert.Equal(t, tt.wantTop, matches[0].Name)
			}
		})
	}
}

func TestIsUnambiguous(t *testing.T) {
	tests := []struct {
		name    string
		matches []FuzzyMatch
		want    bool
	}{
		{
			name:    "empty matches",
			matches: []FuzzyMatch{},
			want:    true,
		},
		{
			name: "single match",
			matches: []FuzzyMatch{
				{Name: "test", Score: 100},
			},
			want: true,
		},
		{
			name: "clearly better top match",
			matches: []FuzzyMatch{
				{Name: "test1", Score: 100},
				{Name: "test2", Score: 50},
			},
			want: true,
		},
		{
			name: "ambiguous matches",
			matches: []FuzzyMatch{
				{Name: "test1", Score: 100},
				{Name: "test2", Score: 95},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUnambiguous(tt.matches)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Note: isPhaseDir and isSequenceDir tests removed as these functions
// are no longer used in fuzzy matching. Phase/sequence navigation uses
// numeric shortcuts (e.g., "fest go 1/1") instead of fuzzy matching.

func TestFormatMatchList(t *testing.T) {
	matches := []FuzzyMatch{
		{Name: "one"},
		{Name: "two"},
		{Name: "three"},
		{Name: "four"},
		{Name: "five"},
	}

	// Test with limit
	got := FormatMatchList(matches, 3)
	assert.Equal(t, []string{"one", "two", "three"}, got)

	// Test with 0 limit (returns all - no limit)
	got = FormatMatchList(matches, 0)
	assert.Len(t, got, 5)

	// Test with negative limit (returns all - no limit)
	got = FormatMatchList(matches, -1)
	assert.Len(t, got, 5)

	// Test with limit > len
	got = FormatMatchList(matches, 10)
	assert.Len(t, got, 5)
}

func TestCollectNavigationTargets_IncludesRitual(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := tmpDir

	// Active festival (baseline — must still be found)
	_ = os.MkdirAll(filepath.Join(festivalsDir, "active", "my-feature-MF0001"), 0755)

	// Ritual festival definition — follows the documented on-disk format
	// (see docs/ritual.md): festivals/ritual/{name}-RI-{XX}{NNNN}
	ritualName := "weekly-review-RI-WR0001"
	_ = os.MkdirAll(filepath.Join(festivalsDir, "ritual", ritualName), 0755)

	// Non-festival dir in ritual/ — must be filtered out by ID suffix check.
	_ = os.MkdirAll(filepath.Join(festivalsDir, "ritual", "not-a-festival"), 0755)

	targets := CollectNavigationTargets(festivalsDir)

	names := make(map[string]string) // name -> path
	for _, tgt := range targets {
		names[tgt.Name] = tgt.Path
	}

	// Regression guard: active festivals still collected.
	assert.Contains(t, names, "my-feature-MF0001",
		"active festival must remain in navigation targets")

	// The actual fix under test: ritual festivals appear in fuzzy targets.
	assert.Contains(t, names, ritualName,
		"ritual festival must be included so fgo fuzzy navigation can reach it")

	// ritual/ status directory itself should also be navigable by name
	// (parity with active/ready/planning appearing as status targets).
	assert.Contains(t, names, "ritual",
		"ritual status directory must be a navigable target")

	// Non-festival dirs in ritual/ must be filtered out.
	assert.NotContains(t, names, "not-a-festival",
		"directories without a valid ID suffix must not appear")

	// Path correctness: ritual festival path points inside festivals/ritual/.
	assert.Equal(t,
		filepath.Join(festivalsDir, "ritual", ritualName),
		names[ritualName],
		"ritual festival path should point to festivals/ritual/{name}")
}

func TestCollectNavigationTargets_IncludesActiveRitualRun(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := tmpDir

	runName := "daily-job-search-RI-DJ0001-0001"
	runPath := filepath.Join(festivalsDir, "active", runName)
	if err := os.MkdirAll(runPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runPath, "fest.yaml"), []byte("metadata:\n  name: daily-job-search\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	targets := CollectNavigationTargets(festivalsDir)

	names := make(map[string]string)
	for _, tgt := range targets {
		names[tgt.Name] = tgt.Path
	}

	assert.Contains(t, names, runName,
		"active ritual runs must be included so fgo fuzzy navigation can reach current ritual work")
	assert.Equal(t, runPath, names[runName],
		"active ritual run path should point to festivals/active/{name}")
}

func TestFuzzyFinder_PrefersActiveRunOverRitualTemplate(t *testing.T) {
	finder := NewFuzzyFinder([]FuzzyTarget{
		{
			Name:     "daily-job-search-RI-DJ0001",
			Path:     "/ritual/daily-job-search-RI-DJ0001",
			Priority: 0,
		},
		{
			Name:     "daily-job-search-RI-DJ0001-0001",
			Path:     "/active/daily-job-search-RI-DJ0001-0001",
			Priority: 40,
		},
	})

	matches := finder.Find("daily-job")
	if assert.Len(t, matches, 2) {
		assert.Equal(t, "/active/daily-job-search-RI-DJ0001-0001", matches[0].Path)
		assert.True(t, IsUnambiguous(matches),
			"active ritual run should outrank the ritual template strongly enough for non-interactive fgo")
	}
}

func TestCollectFestivalsInStatus(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := tmpDir

	// Create active/ with two valid festivals and one non-festival dir
	activeDir := filepath.Join(festivalsDir, "active")
	_ = os.MkdirAll(filepath.Join(activeDir, "my-feature-MF0001"), 0755)
	_ = os.MkdirAll(filepath.Join(activeDir, "bug-fix-BF0002"), 0755)
	_ = os.MkdirAll(filepath.Join(activeDir, "random-dir"), 0755) // no valid ID suffix

	// Create planning/ with one festival
	planningDir := filepath.Join(festivalsDir, "planning")
	_ = os.MkdirAll(filepath.Join(planningDir, "next-task-NT0003"), 0755)

	t.Run("active returns valid festivals only", func(t *testing.T) {
		targets := CollectFestivalsInStatus(festivalsDir, "active")
		assert.Len(t, targets, 2)
		names := make(map[string]bool)
		for _, tgt := range targets {
			names[tgt.Name] = true
		}
		assert.True(t, names["my-feature-MF0001"])
		assert.True(t, names["bug-fix-BF0002"])
		assert.False(t, names["random-dir"])
	})

	t.Run("planning returns its festivals", func(t *testing.T) {
		targets := CollectFestivalsInStatus(festivalsDir, "planning")
		assert.Len(t, targets, 1)
		assert.Equal(t, "next-task-NT0003", targets[0].Name)
	})

	t.Run("nonexistent status returns nil", func(t *testing.T) {
		targets := CollectFestivalsInStatus(festivalsDir, "nonexistent")
		assert.Nil(t, targets)
	})

	t.Run("empty status dir returns nil", func(t *testing.T) {
		_ = os.MkdirAll(filepath.Join(festivalsDir, "dungeon"), 0755)
		targets := CollectFestivalsInStatus(festivalsDir, "dungeon")
		assert.Nil(t, targets)
	})

	t.Run("paths are correct", func(t *testing.T) {
		targets := CollectFestivalsInStatus(festivalsDir, "active")
		for _, tgt := range targets {
			assert.Equal(t, filepath.Join(activeDir, tgt.Name), tgt.Path)
		}
	})
}
