package shared

import (
	"os"
	"path/filepath"
	"strings"
)

// HasSequenceDirs returns true if the phase directory contains numbered subdirectories (sequences).
func HasSequenceDirs(phasePath string) bool {
	entries, err := os.ReadDir(phasePath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && IsNumberedDir(entry.Name()) {
			return true
		}
	}
	return false
}

// IsPhaseMarkedComplete checks PHASE_GOAL.md frontmatter for fest_status: completed.
func IsPhaseMarkedComplete(phasePath string) bool {
	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")
	data, err := os.ReadFile(goalPath)
	if err != nil {
		return false
	}
	content := string(data)
	// Frontmatter is between --- delimiters at the start of the file
	if !strings.HasPrefix(content, "---") {
		return false
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return false
	}
	fm := content[3 : 3+end]
	return strings.Contains(fm, "fest_status: completed")
}

// IsNumberedDir checks if a directory name starts with a digit.
func IsNumberedDir(name string) bool {
	if len(name) < 2 {
		return false
	}
	return name[0] >= '0' && name[0] <= '9'
}
