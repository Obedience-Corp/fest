// Package ui provides UI components and styling for the fest CLI.
// This file defines the status color scheme used throughout the fest CLI
// to provide visual distinction between different festival statuses.
//
// Colors are now loaded from the user's theme configuration via the palette system.
// Call InitPalette() early in main() to load the user's theme preference.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Status color getters - use palette colors based on user's theme setting.
// These replace the old hardcoded color variables.

// GetActiveColor returns the color for active/executing items.
func GetActiveColor() lipgloss.TerminalColor { return Current().Active }

// GetPlanningColor returns the color for planning/future items.
func GetPlanningColor() lipgloss.TerminalColor { return Current().Planning }

// GetCompletedColor returns the color for completed items.
func GetCompletedColor() lipgloss.TerminalColor { return Current().Completed }

// GetArchivedColor returns the color for archived/paused items.
func GetArchivedColor() lipgloss.TerminalColor { return Current().Archived }

// GetRitualColor returns the color for ritual/review items.
func GetRitualColor() lipgloss.TerminalColor { return Current().Ritual }

// GetSomedayColor returns the color for someday/deferred items.
func GetSomedayColor() lipgloss.TerminalColor { return Current().Someday }

// GetDungeonColor returns the color for dungeon (deep archive) items.
func GetDungeonColor() lipgloss.TerminalColor { return Current().Dungeon }

// Legacy color variables for backward compatibility.
// These are used by code that hasn't been updated to use the palette yet.
// NOTE: These use the dark theme defaults and won't change with theme setting.
// New code should use the Get*Color() functions instead.
var (
	ActiveColor    = lipgloss.Color("42")  // Green - currently executing
	ReadyColor     = lipgloss.Color("220") // Yellow - ready for execution (distinct from planning)
	PlanningColor  = lipgloss.Color("33")  // Blue - future work
	RitualColor    = lipgloss.Color("141") // Purple-blue - review/wrap-up
	CompletedColor = lipgloss.Color("205") // Purple/Magenta - finished successfully
	ArchivedColor  = lipgloss.Color("250") // Light grey - deprioritized/paused
	SomedayColor   = lipgloss.Color("139") // Muted purple - deferred
	DungeonColor   = lipgloss.Color("248") // Light grey - deep archived
)

// Entity type colors for visual hierarchy in fest CLI output.
// Each entity type gets a distinct color to make nested structures easier to scan.
var (
	FestivalColor = ActiveColor           // Green (42) - reuse active color for top-level entities
	PhaseColor    = PlanningColor         // Blue (33) - reuse planning color for major divisions
	SequenceColor = lipgloss.Color("51")  // Cyan - distinct color for mid-level groupings
	TaskColor     = lipgloss.Color("141") // Purple - for individual work items
	GateColor     = lipgloss.Color("214") // Orange - for quality gates and checkpoints
)

// State colors for progress and status indication across all entity types.
var (
	PendingColor    = lipgloss.Color("250") // Light grey - not yet started (updated from 245)
	InProgressColor = lipgloss.Color("220") // Yellow/amber - actively being worked
	JudgeColor      = lipgloss.Color("135") // Purple - checkpoint waiting on a delegated judge
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
	case "ready":
		return lipgloss.NewStyle().Foreground(p.Ready).Bold(true)
	case "planning":
		return lipgloss.NewStyle().Foreground(p.Planning).Bold(true)
	case "ritual":
		return lipgloss.NewStyle().Foreground(p.Ritual).Bold(true)
	case "completed", "dungeon/completed":
		return lipgloss.NewStyle().Foreground(p.Completed).Bold(true)
	case "archived", "dungeon/archived":
		return lipgloss.NewStyle().Foreground(p.Archived).Bold(true)
	case "someday", "dungeon/someday":
		return lipgloss.NewStyle().Foreground(p.Someday).Bold(true)
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
	case "ready":
		return p.Ready
	case "planning":
		return p.Planning
	case "ritual":
		return p.Ritual
	case "completed", "dungeon/completed":
		return p.Completed
	case "archived", "dungeon/archived":
		return p.Archived
	case "someday", "dungeon/someday":
		return p.Someday
	case "dungeon":
		return p.Dungeon
	default:
		return lipgloss.Color("")
	}
}

// StatusColorSequence returns the raw ANSI 256-color foreground escape for a
// festival status, sourced from the active theme palette via GetStatusColor.
// Unlike lipgloss styling it is emitted unconditionally, so colorized shell
// completions render even when stdout is not a TTY. Palette entries are ANSI-256
// indices; an unknown status falls back to dim.
func StatusColorSequence(status string) string {
	color, ok := GetStatusColor(status).(lipgloss.Color)
	code := string(color)
	if !ok || code == "" {
		return "\x1b[2m"
	}
	return fmt.Sprintf("\x1b[38;5;%sm", code)
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
	ReadyStyle     = lipgloss.NewStyle().Foreground(ReadyColor).Bold(true)
	PlanningStyle  = lipgloss.NewStyle().Foreground(PlanningColor).Bold(true)
	RitualStyle    = lipgloss.NewStyle().Foreground(RitualColor).Bold(true)
	CompletedStyle = lipgloss.NewStyle().Foreground(CompletedColor).Bold(true)
	ArchivedStyle  = lipgloss.NewStyle().Foreground(ArchivedColor).Bold(true)
	SomedayStyle   = lipgloss.NewStyle().Foreground(SomedayColor).Bold(true)
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

// Work type colors for visual indicators
var (
	WorkTypeImplColor     = lipgloss.Color("42")  // Green - implementation
	WorkTypeAnalysisColor = lipgloss.Color("33")  // Blue - analysis/research
	WorkTypeReviewColor   = lipgloss.Color("214") // Orange - review
	WorkTypeVerifyColor   = lipgloss.Color("220") // Yellow - verification
	WorkTypeConfigColor   = lipgloss.Color("250") // Grey - configuration
	WorkTypeDocsColor     = lipgloss.Color("51")  // Cyan - documentation
	WorkTypePlanColor     = lipgloss.Color("141") // Purple - planning
	WorkTypeActionColor   = lipgloss.Color("205") // Magenta - action
)

// FormatWorkType returns a styled work type indicator string.
// Work type indicates the kind of work a task involves (impl, analysis, review, verify, etc.)
func FormatWorkType(workType string) string {
	if workType == "" {
		return ""
	}

	label := "[" + workType + "]"
	var color lipgloss.Color

	switch workType {
	case "impl":
		color = WorkTypeImplColor
	case "analysis":
		color = WorkTypeAnalysisColor
	case "review":
		color = WorkTypeReviewColor
	case "verify":
		color = WorkTypeVerifyColor
	case "config":
		color = WorkTypeConfigColor
	case "docs":
		color = WorkTypeDocsColor
	case "plan":
		color = WorkTypePlanColor
	case "action":
		color = WorkTypeActionColor
	default:
		color = MetadataColor
	}

	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(label)
}
