package explore

import (
	"os"
	"path/filepath"
	"strings"
)

const previewMaxLines = 30

// loadPreview reads the first N lines of a file for the preview pane.
func loadPreview(path string) string {
	if path == "" {
		return "No preview available"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "No preview available"
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > previewMaxLines {
		lines = lines[:previewMaxLines]
		lines = append(lines, "...")
	}

	return strings.Join(lines, "\n")
}

// goalFileForItem returns the path to the goal/preview file for a hierarchy item.
func goalFileForItem(item FestivalItem) string {
	switch item.Type {
	case ItemFestival:
		goal := filepath.Join(item.Path, "FESTIVAL_GOAL.md")
		if _, err := os.Stat(goal); err == nil {
			return goal
		}
		overview := filepath.Join(item.Path, "FESTIVAL_OVERVIEW.md")
		if _, err := os.Stat(overview); err == nil {
			return overview
		}
		return ""
	case ItemPhase:
		return filepath.Join(item.Path, "PHASE_GOAL.md")
	case ItemSequence:
		return filepath.Join(item.Path, "SEQUENCE_GOAL.md")
	case ItemTask:
		return item.Path
	default:
		return ""
	}
}
