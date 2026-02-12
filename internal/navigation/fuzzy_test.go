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

func TestCollectFestivalsInStatus(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := tmpDir

	// Create active/ with two valid festivals and one non-festival dir
	activeDir := filepath.Join(festivalsDir, "active")
	os.MkdirAll(filepath.Join(activeDir, "my-feature-MF0001"), 0755)
	os.MkdirAll(filepath.Join(activeDir, "bug-fix-BF0002"), 0755)
	os.MkdirAll(filepath.Join(activeDir, "random-dir"), 0755) // no valid ID suffix

	// Create planning/ with one festival
	planningDir := filepath.Join(festivalsDir, "planning")
	os.MkdirAll(filepath.Join(planningDir, "next-task-NT0003"), 0755)

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
		os.MkdirAll(filepath.Join(festivalsDir, "dungeon"), 0755)
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
