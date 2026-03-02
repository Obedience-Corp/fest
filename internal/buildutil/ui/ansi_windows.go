//go:build windows

package ui

import (
	"fmt"
	"os"
)

// TermWidth returns the terminal width.
// On Windows, it reads the COLUMNS environment variable or falls back to 80.
func TermWidth() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		var width int
		fmt.Sscanf(cols, "%d", &width)
		if width > 0 {
			return width
		}
	}
	return 80
}
