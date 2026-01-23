// Package ui provides UI components and styling for the fest CLI.
// This file defines the status color scheme used throughout the fest CLI
// to provide visual distinction between different festival statuses.
//
// Colors are now loaded from the user's theme configuration via the palette system.
// Call InitPalette() early in main() to load the user's theme preference.
package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Status color getters - use palette colors based on user's theme setting.
// These replace the old hardcoded color variables.

// GetActiveColor returns the color for active/executing items.
func GetActiveColor() lipgloss.TerminalColor { return Current().Active }

// GetPlannedColor returns the color for planned/future items.
func GetPlannedColor() lipgloss.TerminalColor { return Current().Planned }

// GetCompletedColor returns the color for completed items.
func GetCompletedColor() lipgloss.TerminalColor { return Current().Completed }

// GetArchivedColor returns the color for archived/paused items.
func GetArchivedColor() lipgloss.TerminalColor { return Current().Archived }

// GetDungeonColor returns the color for dungeon (deep archive) items.
func GetDungeonColor() lipgloss.TerminalColor { return Current().Dungeon }

// Legacy color variables for backward compatibility.
// These are used by code that hasn't been updated to use the palette yet.
// NOTE: These use the dark theme defaults and won't change with theme setting.
// New code should use the Get*Color() functions instead.
var (
	ActiveColor    = lipgloss.Color("42")  // Green - currently executing
	PlannedColor   = lipgloss.Color("33")  // Blue - future work
	CompletedColor = lipgloss.Color("205") // Purple/Magenta - finished successfully
	ArchivedColor  = lipgloss.Color("250") // Light grey - deprioritized/paused (updated from 245)
	DungeonColor   = lipgloss.Color("248") // Light grey - deep archived (updated from 240)
)

// Entity type colors for visual hierarchy in fest CLI output.
// Each entity type gets a distinct color to make nested structures easier to scan.
var (
	FestivalColor = ActiveColor           // Green (42) - reuse active color for top-level entities
	PhaseColor    = PlannedColor          // Blue (33) - reuse planned color for major divisions
	SequenceColor = lipgloss.Color("51")  // Cyan - distinct color for mid-level groupings
	TaskColor     = lipgloss.Color("141") // Purple - for individual work items
	GateColor     = lipgloss.Color("214") // Orange - for quality gates and checkpoints
)

// State colors for progress and status indication across all entity types.
var (
	PendingColor    = lipgloss.Color("250") // Light grey - not yet started (updated from 245)
	InProgressColor = lipgloss.Color("220") // Yellow/amber - actively being worked
	BlockedColor    = lipgloss.Color("196") // Red - blocked/waiting
)

// Structural element colors for UI components and formatting.
var (
	BorderColor   = lipgloss.Color("244") // Medium grey - borders, separators (updated from 240)
	ValueColor    = lipgloss.Color("255") // Bright white - emphasized values
	MetadataColor = lipgloss.Color("250") // Light grey - paths, IDs, secondary info (updated from 245)
	SuccessColor  = ActiveColor           // Green (42) - reuse active color for success
	WarningColor  = InProgressColor       // Yellow (220) - reuse in-progress color for warnings
	ErrorColor    = BlockedColor          // Red (196) - reuse blocked color for errors
)

// GetStatusStyle returns the appropriate lipgloss style for a given status string.
// Now uses the palette for theme-aware colors.
func GetStatusStyle(status string) lipgloss.Style {
	p := Current()
	switch strings.ToLower(status) {
	case "active":
		return lipgloss.NewStyle().Foreground(p.Active).Bold(true)
	case "planned":
		return lipgloss.NewStyle().Foreground(p.Planned).Bold(true)
	case "completed":
		return lipgloss.NewStyle().Foreground(p.Completed).Bold(true)
	case "archived":
		return lipgloss.NewStyle().Foreground(p.Archived).Bold(true)
	case "dungeon":
		return lipgloss.NewStyle().Foreground(p.Dungeon).Bold(true)
	default:
		return lipgloss.NewStyle()
	}
}

// GetStatusColor returns the appropriate color for a given status string.
// Now uses the palette for theme-aware colors.
func GetStatusColor(status string) lipgloss.TerminalColor {
	p := Current()
	switch strings.ToLower(status) {
	case "active":
		return p.Active
	case "planned":
		return p.Planned
	case "completed":
		return p.Completed
	case "archived":
		return p.Archived
	case "dungeon":
		return p.Dungeon
	default:
		return lipgloss.Color("")
	}
}

// GetStateColor returns the appropriate color for a workflow state string.
// Now uses the palette for theme-aware colors.
func GetStateColor(state string) lipgloss.TerminalColor {
	p := Current()
	switch normalizeState(state) {
	case "pending", "todo", "queued":
		return p.Pending
	case "in_progress", "inprogress", "active":
		return p.InProgress
	case "blocked", "error", "failed":
		return p.Blocked
	case "completed", "complete", "done":
		return p.Success
	default:
		return lipgloss.Color("")
	}
}

// GetStateStyle returns a bold style for a workflow state string.
func GetStateStyle(state string) lipgloss.Style {
	color := GetStateColor(state)
	if color == nil || color == lipgloss.Color("") {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true)
}

// Legacy styles - these use the static color variables.
// For theme-aware styling, use GetStatusStyle() instead.
var (
	ActiveStyle    = lipgloss.NewStyle().Foreground(ActiveColor).Bold(true)
	PlannedStyle   = lipgloss.NewStyle().Foreground(PlannedColor).Bold(true)
	CompletedStyle = lipgloss.NewStyle().Foreground(CompletedColor).Bold(true)
	ArchivedStyle  = lipgloss.NewStyle().Foreground(ArchivedColor).Bold(true)
	DungeonStyle   = lipgloss.NewStyle().Foreground(DungeonColor).Bold(true)
)

// Entity color getters - use palette colors based on user's theme setting.

// GetFestivalColor returns the color for festival names.
func GetFestivalColor() lipgloss.TerminalColor { return Current().Festival }

// GetPhaseColor returns the color for phase names.
func GetPhaseColor() lipgloss.TerminalColor { return Current().Phase }

// GetSequenceColor returns the color for sequence names.
func GetSequenceColor() lipgloss.TerminalColor { return Current().Sequence }

// GetTaskColor returns the color for task names.
func GetTaskColor() lipgloss.TerminalColor { return Current().Task }

// GetGateColor returns the color for gate names.
func GetGateColor() lipgloss.TerminalColor { return Current().Gate }

// Entity style getters - bold styles for emphasis.

// GetFestivalStyle returns a bold style for festival names.
func GetFestivalStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Current().Festival).Bold(true)
}

// GetPhaseStyle returns a bold style for phase names.
func GetPhaseStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Current().Phase).Bold(true)
}

// GetSequenceStyle returns a bold style for sequence names.
func GetSequenceStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Current().Sequence).Bold(true)
}

// GetTaskStyle returns a bold style for task names.
func GetTaskStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Current().Task).Bold(true)
}

// GetGateStyle returns a bold style for gate names.
func GetGateStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Current().Gate).Bold(true)
}
