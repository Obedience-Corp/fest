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

// Semantic text functions for festival concepts.
// These provide consistent styling for entity names throughout the CLI.

// Festival styles a festival name with the festival color.
func Festival(name string) string {
	return lipgloss.NewStyle().Foreground(Current().Festival).Bold(true).Render(name)
}

// Phase styles a phase name with the phase color.
func Phase(name string) string {
	return lipgloss.NewStyle().Foreground(Current().Phase).Bold(true).Render(name)
}

// Sequence styles a sequence name with the sequence color.
func Sequence(name string) string {
	return lipgloss.NewStyle().Foreground(Current().Sequence).Bold(true).Render(name)
}

// Task styles a task name with the task color.
func Task(name string) string {
	return lipgloss.NewStyle().Foreground(Current().Task).Bold(true).Render(name)
}

// Gate styles a gate name with the gate color.
func Gate(name string) string {
	return lipgloss.NewStyle().Foreground(Current().Gate).Bold(true).Render(name)
}

// Accent styles accent text (cyan/sequence color) for commands and highlights.
func Accent(text string) string {
	return lipgloss.NewStyle().Foreground(Current().Sequence).Bold(true).Render(text)
}

// Category styles category/section headers (blue/phase color).
func Category(text string) string {
	return lipgloss.NewStyle().Foreground(Current().Phase).Bold(true).Render(text)
}

// BoldText renders bold text without color.
func BoldText(text string) string {
	return lipgloss.NewStyle().Bold(true).Render(text)
}

// StyleHelpText styles help text by highlighting section headers.
// Lines that are all-caps (with optional trailing colon or parenthetical) are styled as category headers.
// This is useful for cobra Long descriptions to add visual structure.
func StyleHelpText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isSectionHeader(trimmed) {
			// Preserve leading whitespace, style the text
			leadingSpace := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = leadingSpace + Category(trimmed)
		}
	}
	return strings.Join(lines, "\n")
}

// isSectionHeader checks if a line looks like a section header.
// Matches patterns like "GETTING STARTED:", "NAVIGATION (using cgo):", "Examples:"
func isSectionHeader(line string) bool {
	if len(line) < 3 {
		return false
	}

	// Must end with : (possibly after parenthetical)
	if !strings.HasSuffix(line, ":") {
		return false
	}

	// Remove trailing : and any parenthetical for checking
	check := strings.TrimSuffix(line, ":")
	if idx := strings.Index(check, "("); idx > 0 {
		check = strings.TrimSpace(check[:idx])
	}

	// Check if primarily uppercase letters (allowing spaces)
	// Also allow capitalized single words like "Examples:"
	hasUpper := false
	hasLower := false
	for _, r := range check {
		if r >= 'A' && r <= 'Z' {
			hasUpper = true
		} else if r >= 'a' && r <= 'z' {
			hasLower = true
		}
	}

	// All caps (with spaces) = header
	if hasUpper && !hasLower {
		return true
	}

	// Capitalized single word like "Examples:" = header
	if hasUpper && hasLower && !strings.Contains(check, " ") {
		firstRune := rune(check[0])
		return firstRune >= 'A' && firstRune <= 'Z'
	}

	return false
}
