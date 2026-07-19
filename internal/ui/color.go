package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// noColorOverride records the explicit --no-color flag separately from the
// ambient environment so palette resolution can apply the same policy before
// Lip Gloss renders its first frame.
var noColorOverride bool

// SetNoColor configures lipgloss to disable colors when requested.
func SetNoColor(noColor bool) {
	noColorOverride = noColor
	if noColor {
		lipgloss.SetColorProfile(termenv.Ascii)
		return
	}
	lipgloss.SetColorProfile(termenv.EnvColorProfile())
}

// ColorSequence converts a shared semantic color into an ANSI-256 foreground
// sequence for explicitly colorized shell completion output. The source color
// remains a shared hex token; termenv performs the terminal-safe conversion.
func ColorSequence(color lipgloss.TerminalColor) string {
	if !CurrentBrandPalette().ColorEnabled || noColorOverride {
		return ""
	}

	value, ok := color.(lipgloss.Color)
	if !ok || value == "" {
		return ""
	}
	converted := termenv.ANSI256.Color(string(value))
	if converted == nil {
		return ""
	}
	return "\x1b[" + converted.Sequence(false) + "m"
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
