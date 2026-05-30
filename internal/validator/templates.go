package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	RelPath     string                     `json:"file"`
	MarkerCount int                        `json:"count"`
	MarkerTypes []string                   `json:"marker_types,omitempty"`
	Level       string                     `json:"level,omitempty"`
	Markers     []TemplateMarkerOccurrence `json:"markers,omitempty"`
}

// TemplateMarkerOccurrence describes a single unfilled marker location.
type TemplateMarkerOccurrence struct {
	Line       int    `json:"line"`
	MarkerType string `json:"marker_type"`
	Content    string `json:"content"`
}

// ValidateTemplateMarkers scans .md files for unfilled markers with phase-aware severity.
// Implementation/review/deployment phases produce errors; planning/research produce warnings.
func ValidateTemplateMarkers(festivalPath string) ([]Issue, error) {
	var issues []Issue

	results, err := ScanTemplateMarkers(festivalPath)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		issues = append(issues, Issue{
			Level:   result.Level,
			Code:    CodeUnfilledTemplate,
			Path:    result.RelPath,
			Message: fmt.Sprintf("File contains %d unfilled template markers (%s)", result.MarkerCount, strings.Join(result.MarkerTypes, ", ")),
			Fix:     "Edit file and replace template markers with actual content",
		})
	}
	return issues, nil
}

// ScanTemplateMarkers scans .md files for unfilled markers and returns
// per-file detail suitable for validation output and agent JSON responses.
func ScanTemplateMarkers(festivalPath string) ([]MarkerFileResult, error) {
	var results []MarkerFileResult

	err := filepath.Walk(festivalPath, func(path string, info os.FileInfo, err error) error {
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

		fileMarkers, markerTypes := scanTemplateMarkersInContent(string(content))
		if len(fileMarkers) == 0 {
			return nil
		}

		results = append(results, MarkerFileResult{
			RelPath:     rel,
			MarkerCount: len(fileMarkers),
			MarkerTypes: markerTypes,
			Level:       resolveMarkerLevel(festivalPath, rel),
			Markers:     fileMarkers,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].RelPath < results[j].RelPath
	})
	return results, nil
}

func scanTemplateMarkersInContent(content string) ([]TemplateMarkerOccurrence, []string) {
	lines := strings.Split(content, "\n")
	inCodeBlock := false
	markerTypes := make(map[string]bool)
	var occurrences []TemplateMarkerOccurrence

	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		lineWithoutCode := stripInlineCode(line)
		for _, marker := range templateMarkers {
			for _, markerText := range extractMarkerContents(lineWithoutCode, marker) {
				markerTypes[marker] = true
				occurrences = append(occurrences, TemplateMarkerOccurrence{
					Line:       i + 1,
					MarkerType: marker,
					Content:    markerText,
				})
			}
		}
	}

	types := make([]string, 0, len(markerTypes))
	for markerType := range markerTypes {
		types = append(types, markerType)
	}
	sort.Strings(types)

	return occurrences, types
}

func extractMarkerContents(line, marker string) []string {
	var contents []string
	endMarker := "]"
	if marker == "{{ " {
		endMarker = "}}"
	}

	offset := 0
	for {
		idx := strings.Index(line[offset:], marker)
		if idx == -1 {
			break
		}
		start := offset + idx
		end := strings.Index(line[start:], endMarker)
		if end == -1 {
			contents = append(contents, strings.TrimSpace(line[start:]))
			offset = start + len(marker)
			continue
		}
		end += start + len(endMarker)
		contents = append(contents, strings.TrimSpace(line[start:end]))
		offset = end
	}

	return contents
}

// CheckTemplatesFilled returns true if no unfilled template markers remain.
func CheckTemplatesFilled(festivalPath string) bool {
	results, err := ScanTemplateMarkers(festivalPath)
	return err == nil && len(results) == 0
}

// resolveMarkerLevel determines the severity level for template markers
// based on the phase type containing the file.
// Implementation/review/deployment phases → error; planning/research → warning.
func resolveMarkerLevel(festivalPath, relPath string) string {
	phaseType := resolvePhaseType(festivalPath, relPath)
	switch phaseType {
	case frontmatter.PhaseTypeImplementation, frontmatter.PhaseTypeReview, frontmatter.PhaseTypeNonCodingAction:
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
	if len(parts) < 2 {
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
