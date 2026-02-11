// Package navigation provides festival-project linking and navigation state management.
package navigation

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/id"
)

// FuzzyMatch represents a fuzzy match result
type FuzzyMatch struct {
	Path    string // Full path to the match
	Name    string // Display name
	Score   int    // Match score (higher is better)
	Indices []int  // Matched character positions (for highlighting)
}

// FuzzyTarget represents a target for fuzzy matching
type FuzzyTarget struct {
	Name string // Display name (used for matching)
	Path string // Full path (returned on match)
}

// FuzzyFinder provides fuzzy matching for festival navigation
type FuzzyFinder struct {
	targets   []FuzzyTarget // Available targets
	threshold int           // Minimum score threshold (0 = accept any)
}

// NewFuzzyFinder creates a finder for the given targets
func NewFuzzyFinder(targets []FuzzyTarget) *FuzzyFinder {
	return &FuzzyFinder{
		targets:   targets,
		threshold: 0,
	}
}

// WithThreshold sets the minimum score threshold
func (f *FuzzyFinder) WithThreshold(threshold int) *FuzzyFinder {
	f.threshold = threshold
	return f
}

// Find returns matches for the pattern, sorted by score descending
func (f *FuzzyFinder) Find(pattern string) []FuzzyMatch {
	// Handle multi-word patterns (AND logic)
	words := strings.Fields(pattern)
	if len(words) == 0 {
		return nil
	}

	// Score each target against all words
	var result []FuzzyMatch
	for _, target := range f.targets {
		// Target must match ALL words (AND logic)
		totalScore := 0
		var allIndices []int
		allMatch := true

		for _, word := range words {
			score, indices := Score(word, target.Name)
			if score == 0 {
				allMatch = false
				break
			}
			totalScore += score
			allIndices = append(allIndices, indices...)
		}

		if allMatch && totalScore >= f.threshold {
			result = append(result, FuzzyMatch{
				Path:    target.Path,
				Name:    target.Name,
				Score:   totalScore,
				Indices: allIndices,
			})
		}
	}

	// Sort by score descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result
}

// IsUnambiguous returns true if the top match is significantly better than alternatives
func IsUnambiguous(matches []FuzzyMatch) bool {
	if len(matches) <= 1 {
		return true
	}
	// Consider unambiguous if top score is 20% better than second
	threshold := float64(matches[0].Score) * 0.8
	return float64(matches[1].Score) < threshold
}

// CollectNavigationTargets gathers all possible navigation targets from a festivals directory.
// Limited to status directories and festival names only (not phases/sequences).
// Festival names are collected first (highest navigation priority), then status directories.
func CollectNavigationTargets(festivalsDir string) []FuzzyTarget {
	var targets []FuzzyTarget

	// Festival names first (most commonly navigated)
	primaryDirs := []string{"active", "ready", "planned"}
	for _, status := range primaryDirs {
		statusPath := filepath.Join(festivalsDir, status)
		entries, err := os.ReadDir(statusPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			festivalName := entry.Name()

			// Only include directories with valid festival ID suffix (e.g., -GS0001)
			if _, err := id.ExtractIDFromDirName(festivalName); err != nil {
				continue // Skip non-festival directories
			}

			festivalPath := filepath.Join(statusPath, festivalName)

			// Add festival by full name
			targets = append(targets, FuzzyTarget{
				Name: festivalName,
				Path: festivalPath,
			})
		}
	}

	// Primary status directories only (completed/dungeon available via "fgo completed <TAB>")
	statusDirs := []string{"active", "ready", "planned"}
	for _, status := range statusDirs {
		statusPath := filepath.Join(festivalsDir, status)
		if info, err := os.Stat(statusPath); err == nil && info.IsDir() {
			targets = append(targets, FuzzyTarget{
				Name: status,
				Path: statusPath,
			})
		}
	}

	return targets
}

// CollectFestivalsInStatus gathers festivals from a single status directory.
// Returns only directories with valid festival ID suffixes.
func CollectFestivalsInStatus(festivalsDir, status string) []FuzzyTarget {
	statusPath := filepath.Join(festivalsDir, status)
	entries, err := os.ReadDir(statusPath)
	if err != nil {
		return nil
	}

	var targets []FuzzyTarget
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, err := id.ExtractIDFromDirName(name); err != nil {
			continue
		}
		targets = append(targets, FuzzyTarget{
			Name: name,
			Path: filepath.Join(statusPath, name),
		})
	}
	return targets
}

// SortMatchesByScore sorts matches by score in descending order
func SortMatchesByScore(matches []FuzzyMatch) {
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})
}

// FormatMatchList formats matches for display in error messages
func FormatMatchList(matches []FuzzyMatch, limit int) []string {
	n := len(matches)
	if limit > 0 && n > limit {
		n = limit
	}
	result := make([]string, n)
	for i := 0; i < n; i++ {
		result[i] = matches[i].Name
	}
	return result
}
