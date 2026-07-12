package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// SetNoColor configures lipgloss to disable colors when requested.
func SetNoColor(noColor bool) {
	if noColor {
		lipgloss.SetColorProfile(termenv.Ascii)
		return
	}
	lipgloss.SetColorProfile(termenv.EnvColorProfile())
}

// MarkdownStyle returns an explicit glamour style ("dark" or "light") for
// markdown rendering, chosen from lipgloss's cached background value.
//
// This never queries the terminal: internal/bginit seeds the background before
// any render, so the "auto" style (which would send OSC 11 / DSR probes and
// stall on ptys that never answer) is avoided entirely.
func MarkdownStyle() string {
	if lipgloss.HasDarkBackground() {
		return "dark"
	}
	return "light"
}
