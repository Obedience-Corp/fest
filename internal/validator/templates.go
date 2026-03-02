package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

// templateMarkers is the canonical set of unfilled template markers.
// Both ValidateTemplateMarkers and CheckTemplatesFilled use this set.
var templateMarkers = []string{"[FILL:", "[REPLACE:", "[GUIDANCE:", "{{ "}

// stripInlineCode removes text inside backticks from a line.
// This prevents markers inside inline code examples from being counted.
func stripInlineCode(line string) string {
	result := strings.Builder{}
	inBacktick := false
	for _, ch := range line {
		if ch == '`' {
			inBacktick = !inBacktick
			continue
		}
		if !inBacktick {
			result.WriteRune(ch)
		}
	}
	return result.String()
}

// MarkerFileResult holds per-file marker scan results.
type MarkerFileResult struct {
	RelPath     string
	MarkerCount int
	MarkerTypes []string
	Level       string
}

// ValidateTemplateMarkers scans .md files for unfilled markers with phase-aware severity.
// Implementation/review/deployment phases produce errors; planning/research produce warnings.
func ValidateTemplateMarkers(festivalPath string) ([]Issue, error) {
	var issues []Issue

	_ = filepath.Walk(festivalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, _ := filepath.Rel(festivalPath, path)
		if strings.HasPrefix(rel, ".") || strings.Contains(rel, "/.") {
			return nil
		}
		// Skip gates/ directory - these are intentional template files
		if strings.HasPrefix(rel, "gates/") || strings.HasPrefix(rel, "gates"+string(filepath.Separator)) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Scan line-by-line, skipping code blocks
		lines := strings.Split(string(content), "\n")
		inCodeBlock := false
		fileMarkerCount := 0
		foundMarkers := make(map[string]bool)

		for _, line := range lines {
			// Toggle fence state on ``` lines (handles ```go, ```yaml, etc.)
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			// Skip markers inside code blocks - they're documentation examples
			if inCodeBlock {
				continue
			}
			// Strip inline code (backticks) before checking for markers
			lineWithoutCode := stripInlineCode(line)
			for _, m := range templateMarkers {
				count := strings.Count(lineWithoutCode, m)
				if count > 0 {
					fileMarkerCount += count
					foundMarkers[m] = true
				}
			}
		}

		if fileMarkerCount > 0 {
			markerTypes := make([]string, 0, len(foundMarkers))
			for m := range foundMarkers {
				markerTypes = append(markerTypes, m)
			}

			// Phase-aware severity
			level := resolveMarkerLevel(festivalPath, rel)

			issues = append(issues, Issue{
				Level:   level,
				Code:    CodeUnfilledTemplate,
				Path:    rel,
				Message: fmt.Sprintf("File contains %d unfilled template markers (%s)", fileMarkerCount, strings.Join(markerTypes, ", ")),
				Fix:     "Edit file and replace template markers with actual content",
			})
		}

		return nil
	})
	return issues, nil
}

// CheckTemplatesFilled returns true if no unfilled template markers remain.
func CheckTemplatesFilled(festivalPath string) bool {
	filled := true
	_ = filepath.Walk(festivalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, _ := filepath.Rel(festivalPath, path)
		if strings.HasPrefix(rel, ".") || strings.Contains(rel, "/.") {
			return nil
		}
		// Skip gates/ directory - these are intentional template files
		if strings.HasPrefix(rel, "gates/") || strings.HasPrefix(rel, "gates"+string(filepath.Separator)) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		// Scan line-by-line, skipping code blocks
		lines := strings.Split(string(b), "\n")
		inCodeBlock := false
		for _, line := range lines {
			// Toggle fence state on ``` lines
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			// Skip markers inside code blocks - they're documentation examples
			if inCodeBlock {
				continue
			}
			// Strip inline code (backticks) before checking for markers
			lineWithoutCode := stripInlineCode(line)
			for _, m := range templateMarkers {
				if strings.Contains(lineWithoutCode, m) {
					filled = false
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	return filled
}

// resolveMarkerLevel determines the severity level for template markers
// based on the phase type containing the file.
// Implementation/review/deployment phases → error; planning/research → warning.
func resolveMarkerLevel(festivalPath, relPath string) string {
	phaseType := resolvePhaseType(festivalPath, relPath)
	switch phaseType {
	case frontmatter.PhaseTypeImplementation, frontmatter.PhaseTypeReview, frontmatter.PhaseTypeDeployment:
		return LevelError
	default:
		return LevelWarning
	}
}

// resolvePhaseType reads the PHASE_GOAL.md frontmatter to determine the phase type.
// Returns the PhaseType for the phase containing the given relative path.
// Defaults to implementation if frontmatter is missing or unreadable.
func resolvePhaseType(festivalPath, relPath string) frontmatter.PhaseType {
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) < 1 {
		return frontmatter.PhaseTypeImplementation
	}
	goalPath := filepath.Join(festivalPath, parts[0], "PHASE_GOAL.md")
	content, err := os.ReadFile(goalPath)
	if err != nil {
		return frontmatter.PhaseTypeImplementation // default assumption
	}
	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		return frontmatter.PhaseTypeImplementation
	}
	if fm.PhaseType == "" {
		return frontmatter.PhaseTypeImplementation
	}
	return fm.PhaseType
}
