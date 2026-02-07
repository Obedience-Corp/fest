package loop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MarkersEvaluator scans for unfilled template markers.
type MarkersEvaluator struct{}

func (e *MarkersEvaluator) Evaluate(ctx context.Context, phasePath string, params map[string]any) (*ConditionResult, error) {
	scope := "phase"
	if s, ok := params["scope"].(string); ok {
		scope = s
	}

	searchPath := phasePath
	if scope == "festival" {
		searchPath = filepath.Dir(filepath.Dir(phasePath))
	}

	markers, err := findUnfilledMarkers(searchPath)
	if err != nil {
		return nil, fmt.Errorf("scan for markers: %w", err)
	}

	if len(markers) == 0 {
		return &ConditionResult{
			Pass:           true,
			FailureContext: "",
		}, nil
	}

	return &ConditionResult{
		Pass:           false,
		FailureContext: fmt.Sprintf("Found %d unfilled markers:\n%s", len(markers), strings.Join(markers, "\n")),
	}, nil
}

var markerPattern = regexp.MustCompile(`\[REPLACE:.*?\]|\[FILL:.*?\]`)

func findUnfilledMarkers(root string) ([]string, error) {
	var markers []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		matches := markerPattern.FindAllString(string(content), -1)
		for _, match := range matches {
			relPath, _ := filepath.Rel(root, path)
			markers = append(markers, fmt.Sprintf("%s: %s", relPath, match))
		}

		return nil
	})

	return markers, err
}
