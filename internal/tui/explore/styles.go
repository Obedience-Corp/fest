// Package explore provides the BubbleTea TUI model for the festival explorer.
package explore

import "github.com/charmbracelet/lipgloss"

// Adaptive colors for status (auto-detect light/dark terminal).
var (
	colorActive    = lipgloss.AdaptiveColor{Light: "#00AF00", Dark: "#00FF5F"}
	colorPlanned   = lipgloss.AdaptiveColor{Light: "#AF8700", Dark: "#FFD700"}
	colorCompleted = lipgloss.AdaptiveColor{Light: "#005FAF", Dark: "#00AFFF"}
	colorDungeon   = lipgloss.AdaptiveColor{Light: "#808080", Dark: "#949494"}
	colorText      = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
	colorDim       = lipgloss.AdaptiveColor{Light: "#808080", Dark: "#949494"}
	colorFocus     = lipgloss.AdaptiveColor{Light: "#FF8700", Dark: "#FFD700"}
	colorBorder    = lipgloss.AdaptiveColor{Light: "#005FAF", Dark: "#00D7FF"}
	colorDimBorder = lipgloss.AdaptiveColor{Light: "#808080", Dark: "#585858"}
)

// StatusStyle returns the lipgloss style for a given festival status.
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "active":
		return lipgloss.NewStyle().Foreground(colorActive).Bold(true)
	case "planned":
		return lipgloss.NewStyle().Foreground(colorPlanned)
	case "completed":
		return lipgloss.NewStyle().Foreground(colorCompleted)
	case "dungeon":
		return lipgloss.NewStyle().Foreground(colorDungeon).Faint(true)
	default:
		return lipgloss.NewStyle().Foreground(colorText)
	}
}

var (
	selectedStyle   = lipgloss.NewStyle().Foreground(colorFocus).Bold(true)
	normalStyle     = lipgloss.NewStyle().Foreground(colorText)
	dimStyle        = lipgloss.NewStyle().Foreground(colorDim)
	cursorStyle     = lipgloss.NewStyle().Foreground(colorFocus)
	headerStyle     = lipgloss.NewStyle().Foreground(colorBorder).Bold(true)
	helpStyle       = lipgloss.NewStyle().Foreground(colorDim).Faint(true)
	borderStyle     = lipgloss.NewStyle().Foreground(colorBorder)
	breadcrumbStyle = lipgloss.NewStyle().Foreground(colorDim)
	previewTitle    = lipgloss.NewStyle().Foreground(colorBorder).Bold(true)
)

// panelBorder returns a border style for a panel with the given focus state.
func panelBorder(focused bool) lipgloss.Style {
	bc := colorDimBorder
	if focused {
		bc = colorBorder
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(bc)
}
