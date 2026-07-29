// Package theme provides configurable huh themes for fest CLI.
// Supports multiple themes for different terminal backgrounds.
package theme

import (
	"context"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/ui"
	sharedbrand "github.com/Obedience-Corp/obey-shared/brand"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ThemeName represents available theme options.
type ThemeName string

const (
	ThemeAdaptive     ThemeName = "adaptive"      // Auto-detect light/dark (default)
	ThemeLight        ThemeName = "light"         // Optimized for light backgrounds
	ThemeDark         ThemeName = "dark"          // Optimized for dark backgrounds
	ThemeHighContrast ThemeName = "high-contrast" // Pure white + bright accents (any background)
)

// ValidThemes returns all valid theme names.
func ValidThemes() []string {
	return []string{
		string(ThemeAdaptive),
		string(ThemeLight),
		string(ThemeDark),
		string(ThemeHighContrast),
	}
}

// IsValidTheme checks if a theme name is valid.
func IsValidTheme(name string) bool {
	for _, t := range ValidThemes() {
		if t == name {
			return true
		}
	}
	return false
}

// palette holds the colors for a theme.
type palette struct {
	text        lipgloss.TerminalColor
	title       lipgloss.TerminalColor
	placeholder lipgloss.TerminalColor
	focus       lipgloss.TerminalColor
	error       lipgloss.TerminalColor
	selected    lipgloss.TerminalColor
	border      lipgloss.TerminalColor
	surface     lipgloss.TerminalColor
}

// getPalette adapts shared semantic roles to huh's field-level theme shape.
// The color values remain owned by obey-shared; this package only decides which
// role each Huh element consumes.
func getPalette(name ThemeName) palette {
	p := ui.ResolveBrandPalette(sharedbrand.ParseMode(string(name)))
	return palette{
		text:        lipgloss.Color(p.TextPrimary),
		title:       lipgloss.Color(p.Accent),
		placeholder: lipgloss.Color(p.TextMuted),
		focus:       lipgloss.Color(p.Focus),
		error:       lipgloss.Color(p.StatusError),
		selected:    lipgloss.Color(p.StatusSuccess),
		border:      lipgloss.Color(p.Border),
		surface:     lipgloss.Color(p.SurfaceBase),
	}
}

// FestTheme returns the default adaptive theme.
// Use GetTheme() or GetThemeFromConfig() for configurable themes.
func FestTheme() *huh.Theme {
	return GetTheme(ThemeAdaptive)
}

// GetTheme returns a huh theme for the given theme name.
func GetTheme(name ThemeName) *huh.Theme {
	p := getPalette(name)
	return buildTheme(p)
}

// GetThemeFromConfig loads the theme from user config.
func GetThemeFromConfig(ctx context.Context) *huh.Theme {
	cfg, err := config.Load(ctx, "")
	if err != nil {
		return FestTheme() // Fall back to default
	}
	return GetTheme(ThemeName(cfg.TUI.Theme))
}

// buildTheme constructs a huh.Theme from a palette.
func buildTheme(p palette) *huh.Theme {
	t := huh.ThemeBase()

	// === Focused field styles (active input) ===
	t.Focused.Title = lipgloss.NewStyle().
		Foreground(p.title).
		Bold(true)

	t.Focused.Description = lipgloss.NewStyle().
		Foreground(p.placeholder)

	t.Focused.TextInput.Text = lipgloss.NewStyle().
		Foreground(p.text)

	t.Focused.TextInput.Cursor = lipgloss.NewStyle().
		Foreground(p.focus)

	t.Focused.TextInput.Placeholder = lipgloss.NewStyle().
		Foreground(p.placeholder)

	t.Focused.TextInput.Prompt = lipgloss.NewStyle().
		Foreground(p.focus)

	t.Focused.SelectSelector = lipgloss.NewStyle().
		Foreground(p.focus).
		SetString("> ")

	t.Focused.SelectedOption = lipgloss.NewStyle().
		Foreground(p.selected).
		Bold(true)

	t.Focused.Option = lipgloss.NewStyle().
		Foreground(p.text)

	t.Focused.ErrorMessage = lipgloss.NewStyle().
		Foreground(p.error)

	t.Focused.ErrorIndicator = lipgloss.NewStyle().
		Foreground(p.error).
		SetString("! ")

	t.Focused.FocusedButton = lipgloss.NewStyle().
		Background(p.focus).
		Foreground(p.surface).
		Bold(true).
		Padding(0, 1)

	t.Focused.BlurredButton = lipgloss.NewStyle().
		Foreground(p.placeholder).
		Padding(0, 1)

	// Field Base is intentionally borderless. huh Group viewports MaxWidth-clip
	// at field width while lipgloss paints borders *outside* Width, so a
	// field-level rounded box always loses its right edge when full-width.
	// RunForm draws a responsive outer frame instead (FormFrameStyle).
	// BorderForeground is kept so FormFrameStyle can read the chrome color.
	focusedBox := lipgloss.NewStyle().
		BorderForeground(p.border).
		Padding(0, 1)
	t.Focused.Base = focusedBox
	t.Focused.Card = focusedBox

	// === Blurred field styles (inactive inputs) ===
	t.Blurred.Title = lipgloss.NewStyle().
		Foreground(p.placeholder)

	t.Blurred.Description = lipgloss.NewStyle().
		Foreground(p.placeholder).
		Faint(true)

	t.Blurred.TextInput.Text = lipgloss.NewStyle().
		Foreground(p.placeholder)

	t.Blurred.TextInput.Placeholder = lipgloss.NewStyle().
		Foreground(p.placeholder).
		Faint(true)

	t.Blurred.TextInput.Prompt = lipgloss.NewStyle().
		Foreground(p.placeholder)

	t.Blurred.Option = lipgloss.NewStyle().
		Foreground(p.placeholder)

	t.Blurred.SelectedOption = lipgloss.NewStyle().
		Foreground(p.placeholder)

	t.Blurred.SelectSelector = lipgloss.NewStyle().
		Foreground(p.placeholder).
		SetString("  ")

	// Blurred fields match focused chrome (borderless); outer form frame is shared.
	blurredBox := lipgloss.NewStyle().
		Padding(0, 1)
	t.Blurred.Base = blurredBox
	t.Blurred.Card = blurredBox

	return t
}
