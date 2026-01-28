// Package theme provides configurable huh themes for fest CLI.
// Supports multiple themes for different terminal backgrounds.
package theme

import (
	"context"

	"github.com/Obedience-Corp/fest/internal/config"
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
}

// adaptivePalette returns colors that auto-detect terminal background.
func adaptivePalette() palette {
	return palette{
		text:        lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"},
		title:       lipgloss.AdaptiveColor{Light: "#005FAF", Dark: "#00D7FF"},
		placeholder: lipgloss.AdaptiveColor{Light: "#808080", Dark: "#949494"},
		focus:       lipgloss.AdaptiveColor{Light: "#FF8700", Dark: "#FFD700"},
		error:       lipgloss.AdaptiveColor{Light: "#D70000", Dark: "#FF5F87"},
		selected:    lipgloss.AdaptiveColor{Light: "#00AF00", Dark: "#00FF5F"},
		border:      lipgloss.AdaptiveColor{Light: "#005FAF", Dark: "#00D7FF"},
	}
}

// lightPalette returns colors optimized for light backgrounds.
func lightPalette() palette {
	return palette{
		text:        lipgloss.Color("#000000"),
		title:       lipgloss.Color("#005FAF"),
		placeholder: lipgloss.Color("#666666"),
		focus:       lipgloss.Color("#D75F00"),
		error:       lipgloss.Color("#D70000"),
		selected:    lipgloss.Color("#008700"),
		border:      lipgloss.Color("#005FAF"),
	}
}

// darkPalette returns colors optimized for dark backgrounds.
func darkPalette() palette {
	return palette{
		text:        lipgloss.Color("#FFFFFF"),
		title:       lipgloss.Color("#00D7FF"),
		placeholder: lipgloss.Color("#949494"),
		focus:       lipgloss.Color("#FFD700"),
		error:       lipgloss.Color("#FF5F87"),
		selected:    lipgloss.Color("#00FF5F"),
		border:      lipgloss.Color("#00D7FF"),
	}
}

// highContrastPalette returns maximum contrast colors for any background.
// Uses pure white text with bright accent colors that work on colored backgrounds.
func highContrastPalette() palette {
	return palette{
		text:        lipgloss.Color("#FFFFFF"), // Pure white
		title:       lipgloss.Color("#00FFFF"), // Bright cyan
		placeholder: lipgloss.Color("#AAAAAA"), // Light grey (visible on colors)
		focus:       lipgloss.Color("#FFFF00"), // Bright yellow
		error:       lipgloss.Color("#FF5555"), // Bright red
		selected:    lipgloss.Color("#00FF00"), // Bright green
		border:      lipgloss.Color("#00FFFF"), // Bright cyan
	}
}

// getPalette returns the color palette for a theme name.
func getPalette(name ThemeName) palette {
	switch name {
	case ThemeLight:
		return lightPalette()
	case ThemeDark:
		return darkPalette()
	case ThemeHighContrast:
		return highContrastPalette()
	default:
		return adaptivePalette()
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
		Foreground(lipgloss.Color("#000000")).
		Bold(true).
		Padding(0, 1)

	t.Focused.BlurredButton = lipgloss.NewStyle().
		Foreground(p.placeholder).
		Padding(0, 1)

	t.Focused.Base = lipgloss.NewStyle().
		BorderForeground(p.border).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, 1)

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

	return t
}
