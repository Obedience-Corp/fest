package ui

import (
	"os"
	"strings"
	"unicode/utf8"
)

var noColor bool

// ANSI color codes
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	// Cursor control
	HideCursor = "\033[?25l"
	ShowCursor = "\033[?25h"
	ClearLine  = "\033[2K"
	MoveUp     = "\033[A"
)

// Init initializes the UI package based on environment
func Init(noColorFlag bool) {
	noColor = noColorFlag || os.Getenv("NO_COLOR") != "" || os.Getenv("CI") != "" || !isatty()
}

// ColourEnabled returns true if colors should be used
func ColourEnabled() bool {
	return !noColor
}

// isatty checks if stdout is a terminal
func isatty() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// Center centers text to given width
func Center(text string, width int) string {
	textLen := VisualLength(text)
	if textLen >= width {
		return text
	}
	leftPad := (width - textLen) / 2
	rightPad := width - textLen - leftPad
	return strings.Repeat(" ", leftPad) + text + strings.Repeat(" ", rightPad)
}

// VisualLength calculates the visual width of text, accounting for ANSI codes and unicode
func VisualLength(text string) int {
	cleaned := StripANSI(text)
	width := 0
	for len(cleaned) > 0 {
		r, size := utf8.DecodeRuneInString(cleaned)
		if r == utf8.RuneError {
			width++
		} else {
			if r >= 0x1F000 {
				width += 2
			} else {
				width++
			}
		}
		cleaned = cleaned[size:]
	}
	return width
}

// StripANSI removes ANSI escape codes from text
func StripANSI(text string) string {
	result := text
	for _, code := range []string{Reset, Bold, Red, Green, Yellow, Cyan} {
		result = strings.ReplaceAll(result, code, "")
	}
	return result
}
