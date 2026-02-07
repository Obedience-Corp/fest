package loop

import (
	"os"
	"path/filepath"
)

// ValidationIssue represents a LOOP.yaml validation problem.
type ValidationIssue struct {
	File    string
	Message string
	Level   string // "error", "warning"
}

// ValidateLoopConfig checks LOOP.yaml in the given phase path for required fields.
func ValidateLoopConfig(phasePath string) []ValidationIssue {
	loopPath := filepath.Join(phasePath, "LOOP.yaml")

	// LOOP.yaml is optional
	if _, err := os.Stat(loopPath); os.IsNotExist(err) {
		return nil
	}

	_, err := ParseLoopConfig(loopPath)
	if err != nil {
		return []ValidationIssue{
			{
				File:    "LOOP.yaml",
				Message: err.Error(),
				Level:   "error",
			},
		}
	}

	return nil
}
