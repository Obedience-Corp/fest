// Package ui provides UI components and styling for the fest CLI.
// This file implements a configurable color palette that respects the user's theme setting.
package ui

import (
	"context"
	"sync"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/charmbracelet/lipgloss"
)

var (
	currentPalette Palette
	paletteOnce    sync.Once
	paletteInit    bool
)

// Palette holds all the colors used throughout the CLI.
// Colors are loaded from the user's theme setting at startup.
type Palette struct {
	// Status colors for festival states
	Active    lipgloss.TerminalColor
	Ready     lipgloss.TerminalColor
	Planned   lipgloss.TerminalColor
	Completed lipgloss.TerminalColor
	Archived  lipgloss.TerminalColor
	Dungeon   lipgloss.TerminalColor

	// Entity colors
	Festival lipgloss.TerminalColor
	Phase    lipgloss.TerminalColor
	Sequence lipgloss.TerminalColor
	Task     lipgloss.TerminalColor
	Gate     lipgloss.TerminalColor

	// State colors
	Pending    lipgloss.TerminalColor
	InProgress lipgloss.TerminalColor
	Blocked    lipgloss.TerminalColor
	Success    lipgloss.TerminalColor
	Warning    lipgloss.TerminalColor
	Error      lipgloss.TerminalColor

	// Structural colors
	Border   lipgloss.TerminalColor
	Value    lipgloss.TerminalColor
	Metadata lipgloss.TerminalColor
	Dim      lipgloss.TerminalColor
}

// InitPalette loads the color palette from user config.
// Must be called early in main() before any UI output.
func InitPalette(ctx context.Context) {
	paletteOnce.Do(func() {
		cfg, err := config.Load(ctx, "")
		if err != nil {
			currentPalette = defaultDarkPalette()
			paletteInit = true
			return
		}

		switch cfg.TUI.Theme {
		case "light":
			currentPalette = lightPalette()
		case "dark":
			currentPalette = defaultDarkPalette()
		case "high-contrast":
			currentPalette = highContrastPalette()
		default:
			// "adaptive" or unknown - use dark as safe default
			currentPalette = defaultDarkPalette()
		}
		paletteInit = true
	})
}

// Current returns the current palette.
// If InitPalette hasn't been called, returns dark palette as fallback.
func Current() *Palette {
	if !paletteInit {
		// Fallback for code paths that don't go through main()
		return defaultDarkPalettePtr()
	}
	return &currentPalette
}

// defaultDarkPalette returns colors optimized for dark terminal backgrounds.
// Uses lighter greys that are readable on dark but non-black backgrounds.
func defaultDarkPalette() Palette {
	return Palette{
		// Status colors - bright and visible
		Active:    lipgloss.Color("42"),  // Bright green
		Ready:     lipgloss.Color("33"),  // Blue (same as planned)
		Planned:   lipgloss.Color("33"),  // Blue
		Completed: lipgloss.Color("205"), // Magenta/pink
		Archived:  lipgloss.Color("250"), // Light grey (readable!)
		Dungeon:   lipgloss.Color("248"), // Light grey (readable!)

		// Entity colors
		Festival: lipgloss.Color("42"),  // Green (same as active)
		Phase:    lipgloss.Color("33"),  // Blue (same as planned)
		Sequence: lipgloss.Color("51"),  // Cyan
		Task:     lipgloss.Color("141"), // Purple
		Gate:     lipgloss.Color("214"), // Orange

		// State colors
		Pending:    lipgloss.Color("250"), // Light grey
		InProgress: lipgloss.Color("220"), // Yellow/amber
		Blocked:    lipgloss.Color("196"), // Red
		Success:    lipgloss.Color("42"),  // Green
		Warning:    lipgloss.Color("220"), // Yellow
		Error:      lipgloss.Color("196"), // Red

		// Structural colors
		Border:   lipgloss.Color("244"), // Medium grey
		Value:    lipgloss.Color("255"), // White
		Metadata: lipgloss.Color("250"), // Light grey
		Dim:      lipgloss.Color("248"), // Light grey
	}
}

var defaultDarkPaletteInstance *Palette

func defaultDarkPalettePtr() *Palette {
	if defaultDarkPaletteInstance == nil {
		p := defaultDarkPalette()
		defaultDarkPaletteInstance = &p
	}
	return defaultDarkPaletteInstance
}

// lightPalette returns colors optimized for light terminal backgrounds.
// Uses darker colors that are readable on white/light backgrounds.
func lightPalette() Palette {
	return Palette{
		// Status colors - darker versions
		Active:    lipgloss.Color("28"),  // Dark green
		Ready:     lipgloss.Color("25"),  // Dark blue (same as planned)
		Planned:   lipgloss.Color("25"),  // Dark blue
		Completed: lipgloss.Color("128"), // Dark purple
		Archived:  lipgloss.Color("241"), // Dark grey
		Dungeon:   lipgloss.Color("238"), // Darker grey

		// Entity colors
		Festival: lipgloss.Color("28"),  // Dark green
		Phase:    lipgloss.Color("25"),  // Dark blue
		Sequence: lipgloss.Color("30"),  // Dark cyan
		Task:     lipgloss.Color("91"),  // Dark purple
		Gate:     lipgloss.Color("166"), // Dark orange

		// State colors
		Pending:    lipgloss.Color("241"), // Dark grey
		InProgress: lipgloss.Color("172"), // Dark yellow/orange
		Blocked:    lipgloss.Color("124"), // Dark red
		Success:    lipgloss.Color("28"),  // Dark green
		Warning:    lipgloss.Color("172"), // Dark yellow
		Error:      lipgloss.Color("124"), // Dark red

		// Structural colors
		Border:   lipgloss.Color("244"), // Medium grey
		Value:    lipgloss.Color("232"), // Near black
		Metadata: lipgloss.Color("241"), // Dark grey
		Dim:      lipgloss.Color("244"), // Medium grey
	}
}

// highContrastPalette returns maximum visibility colors.
// Works on any background with bright, saturated colors.
func highContrastPalette() Palette {
	return Palette{
		// Status colors - maximum brightness
		Active:    lipgloss.Color("46"),  // Bright green
		Ready:     lipgloss.Color("39"),  // Bright blue (same as planned)
		Planned:   lipgloss.Color("39"),  // Bright blue
		Completed: lipgloss.Color("201"), // Bright magenta
		Archived:  lipgloss.Color("255"), // White
		Dungeon:   lipgloss.Color("255"), // White

		// Entity colors - bright versions
		Festival: lipgloss.Color("46"),  // Bright green
		Phase:    lipgloss.Color("39"),  // Bright blue
		Sequence: lipgloss.Color("51"),  // Bright cyan
		Task:     lipgloss.Color("177"), // Bright purple
		Gate:     lipgloss.Color("214"), // Bright orange

		// State colors - maximum contrast
		Pending:    lipgloss.Color("252"), // Very light grey
		InProgress: lipgloss.Color("226"), // Bright yellow
		Blocked:    lipgloss.Color("196"), // Bright red
		Success:    lipgloss.Color("46"),  // Bright green
		Warning:    lipgloss.Color("226"), // Bright yellow
		Error:      lipgloss.Color("196"), // Bright red

		// Structural colors - high visibility
		Border:   lipgloss.Color("255"), // White
		Value:    lipgloss.Color("255"), // White
		Metadata: lipgloss.Color("252"), // Very light grey
		Dim:      lipgloss.Color("250"), // Light grey
	}
}

// ResetPalette resets the palette state (for testing only).
func ResetPalette() {
	paletteOnce = sync.Once{}
	paletteInit = false
	currentPalette = Palette{}
}
