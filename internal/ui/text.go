package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Note: Text styling functions now use the palette system for theme-aware colors.
// The Faint modifier has been removed as it makes text unreadable on non-black backgrounds.

// Label styles a short label for key/value pairs or section metadata.
// Uses the palette's Metadata color with bold styling.
func Label(text string) string {
	return lipgloss.NewStyle().Foreground(Current().Metadata).Bold(true).Render(text)
}

// Value styles a primary value. Pass an optional color override.
// Uses the palette's Value color with bold styling.
func Value(text string, colors ...lipgloss.Color) string {
	if len(colors) > 0 && colors[0] != "" {
		return lipgloss.NewStyle().Foreground(colors[0]).Bold(true).Render(text)
	}
	return lipgloss.NewStyle().Foreground(Current().Value).Bold(true).Render(text)
}

// Dim styles secondary metadata (paths, IDs, timestamps).
// Uses the palette's Dim color WITHOUT the Faint modifier for readability.
func Dim(text string) string {
	return lipgloss.NewStyle().Foreground(Current().Dim).Render(text)
}

// ColoredText renders text in a specific color without other styling.
func ColoredText(text string, color lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(color).Render(text)
}

// Success styles a success message fragment.
// Uses the palette's Success color with bold styling.
func Success(text string) string {
	return lipgloss.NewStyle().Foreground(Current().Success).Bold(true).Render(text)
}

// Warning styles a warning message fragment.
// Uses the palette's Warning color with bold styling.
func Warning(text string) string {
	return lipgloss.NewStyle().Foreground(Current().Warning).Bold(true).Render(text)
}

// Error styles an error message fragment.
// Uses the palette's Error color with bold styling.
func Error(text string) string {
	return lipgloss.NewStyle().Foreground(Current().Error).Bold(true).Render(text)
}

// Info styles informational message fragments without strong emphasis.
// Uses the palette's Metadata color without bold styling.
func Info(text string) string {
	return lipgloss.NewStyle().Foreground(Current().Metadata).Render(text)
}

// StateIcon returns a colored symbol representing a progress state.
func StateIcon(state string) string {
	switch normalizeState(state) {
	case "completed", "complete", "done":
		return ColoredText("✓", SuccessColor)
	case "in_progress", "inprogress", "active":
		return ColoredText("●", InProgressColor)
	case "blocked", "error", "failed":
		return ColoredText("■", BlockedColor)
	case "pending", "todo", "queued":
		return ColoredText("○", PendingColor)
	default:
		return ColoredText("•", MetadataColor)
	}
}

func normalizeState(state string) string {
	normalized := strings.ToLower(strings.TrimSpace(state))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

// WriteLabelValue writes a labeled value line to a builder.
func WriteLabelValue(sb *strings.Builder, label, value string) {
	if sb == nil {
		return
	}
	sb.WriteString(Label(label))
	sb.WriteByte(' ')
	sb.WriteString(value)
	sb.WriteByte('\n')
}
