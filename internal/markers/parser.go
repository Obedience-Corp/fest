package markers

import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// Marker patterns for different types of unfilled content
var (
	replaceRegex = regexp.MustCompile(`\[REPLACE:\s*([^\]]+)\]`)
	fillRegex    = regexp.MustCompile(`\[FILL:\s*([^\]]+)\]`)
	todoRegex    = regexp.MustCompile(`\[TODO:\s*([^\]]+)\]`)
	// Legacy support for older pattern
	markerRegex = replaceRegex
)

// MarkerType identifies the kind of unfilled marker
type MarkerType string

const (
	MarkerTypeReplace MarkerType = "REPLACE"
	MarkerTypeFill    MarkerType = "FILL"
	MarkerTypeTodo    MarkerType = "TODO"
)

// Parse extracts all REPLACE markers from content.
func Parse(content string) []Marker {
	var markers []Marker

	matches := markerRegex.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		fullMatch := content[match[0]:match[1]]
		hint := strings.TrimSpace(content[match[2]:match[3]])

		// Calculate line number (1-indexed)
		lineNumber := strings.Count(content[:match[0]], "\n") + 1

		markers = append(markers, Marker{
			FullMatch:   fullMatch,
			Hint:        hint,
			LineNumber:  lineNumber,
			StartOffset: match[0],
			EndOffset:   match[1],
		})
	}

	return markers
}

// ParseFile reads a file and extracts all REPLACE markers.
func ParseFile(ctx context.Context, filePath string) ([]Marker, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled").WithOp("markers.ParseFile")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "reading file").
			WithCode(errors.ErrCodeIO).
			WithField("path", filePath)
	}

	return Parse(string(content)), nil
}

// ExtractHints returns just the hint strings from a slice of markers.
func ExtractHints(markers []Marker) []string {
	hints := make([]string, len(markers))
	for i, m := range markers {
		hints[i] = m.Hint
	}
	return hints
}

// ScanAllMarkers extracts all unfilled markers (REPLACE, FILL, TODO) from content.
func ScanAllMarkers(content string) []Marker {
	var markers []Marker

	// Scan for REPLACE markers
	markers = append(markers, scanMarkersWithType(content, replaceRegex, MarkerTypeReplace)...)

	// Scan for FILL markers
	markers = append(markers, scanMarkersWithType(content, fillRegex, MarkerTypeFill)...)

	// Scan for TODO markers
	markers = append(markers, scanMarkersWithType(content, todoRegex, MarkerTypeTodo)...)

	return markers
}

// scanMarkersWithType extracts markers of a specific type from content.
func scanMarkersWithType(content string, regex *regexp.Regexp, markerType MarkerType) []Marker {
	var markers []Marker

	matches := regex.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		fullMatch := content[match[0]:match[1]]
		hint := strings.TrimSpace(content[match[2]:match[3]])

		// Calculate line number (1-indexed)
		lineNumber := strings.Count(content[:match[0]], "\n") + 1

		markers = append(markers, Marker{
			FullMatch:   fullMatch,
			Hint:        hint,
			Type:        markerType,
			LineNumber:  lineNumber,
			StartOffset: match[0],
			EndOffset:   match[1],
		})
	}

	return markers
}

// ScanFileForMarkers reads a file and returns all unfilled markers.
func ScanFileForMarkers(ctx context.Context, filePath string) ([]Marker, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled").WithOp("markers.ScanFileForMarkers")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "reading file").
			WithCode(errors.ErrCodeIO).
			WithField("path", filePath)
	}

	return ScanAllMarkers(string(content)), nil
}

// HasUnfilledMarkers checks if a file contains any unfilled markers.
func HasUnfilledMarkers(ctx context.Context, filePath string) (bool, []Marker, error) {
	markers, err := ScanFileForMarkers(ctx, filePath)
	if err != nil {
		return false, nil, err
	}
	return len(markers) > 0, markers, nil
}
