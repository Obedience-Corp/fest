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

// Legacy color variables for backward compatibility. InitPalette refreshes
// these values from the shared semantic palette so older callers still follow
// the configured theme. New code should use the Get*Color() functions instead.
var (
	ActiveColor    = paletteLipglossColor(defaultDarkPalette().Active)
	ReadyColor     = paletteLipglossColor(defaultDarkPalette().Ready)
	PlanningColor  = paletteLipglossColor(defaultDarkPalette().Planning)
	RitualColor    = paletteLipglossColor(defaultDarkPalette().Ritual)
	CompletedColor = paletteLipglossColor(defaultDarkPalette().Completed)
	ArchivedColor  = paletteLipglossColor(defaultDarkPalette().Archived)
	SomedayColor   = paletteLipglossColor(defaultDarkPalette().Someday)
	DungeonColor   = paletteLipglossColor(defaultDarkPalette().Dungeon)
)

// Entity type colors for visual hierarchy in fest CLI output.
// Each entity type gets a distinct color to make nested structures easier to scan.
var (
	FestivalColor = ActiveColor   // Reuse the active color for top-level entities.
	PhaseColor    = PlanningColor // Reuse the planning color for major divisions.
	SequenceColor = paletteLipglossColor(defaultDarkPalette().Sequence)
	TaskColor     = paletteLipglossColor(defaultDarkPalette().Task)
	GateColor     = paletteLipglossColor(defaultDarkPalette().Gate)
)

// State colors for progress and status indication across all entity types.
var (
	PendingColor    = paletteLipglossColor(defaultDarkPalette().Pending)
	InProgressColor = paletteLipglossColor(defaultDarkPalette().InProgress)
	JudgeColor      = paletteLipglossColor(defaultDarkPalette().Task)
	BlockedColor    = paletteLipglossColor(defaultDarkPalette().Blocked)
)

// Structural element colors for UI components and formatting.
var (
	BorderColor   = paletteLipglossColor(defaultDarkPalette().Border)
	ValueColor    = paletteLipglossColor(defaultDarkPalette().Value)
	MetadataColor = paletteLipglossColor(defaultDarkPalette().Metadata)
	SuccessColor  = ActiveColor
	WarningColor  = paletteLipglossColor(defaultDarkPalette().Warning)
	ErrorColor    = paletteLipglossColor(defaultDarkPalette().Error)
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

func statusColor(p Palette, status string) lipgloss.TerminalColor {
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

// GetStatusColor returns the appropriate color for a given status string.
// Now uses the palette for theme-aware colors.
func GetStatusColor(status string) lipgloss.TerminalColor {
	return statusColor(*Current(), status)
}

// GetInteractiveStatusColor returns a status color for a TUI rendered to a
// terminal while the command's stdout may be captured by a shell wrapper.
func GetInteractiveStatusColor(status string) lipgloss.TerminalColor {
	return statusColor(InteractivePalette(), status)
}

// StatusColorSequence returns an ANSI-256 foreground escape for a festival
// status, sourced from the shared semantic palette via GetStatusColor.
// Unknown statuses fall back to dim; a plain resolved palette emits no escape.
func StatusColorSequence(status string) string {
	if !CurrentBrandPalette().ColorEnabled {
		return ""
	}
	return statusColorSequence(*Current(), status)
}

// CompletionStatusColorSequence returns the ANSI-256 foreground escape for
// an explicitly colorized shell completion. Unlike ordinary command output,
// completion display cells are intentionally written through a pipe, so they
// use the configured theme even when the normal palette is plain.
func CompletionStatusColorSequence(status string) string {
	p := completionPalette()
	if p.Active == nil {
		return ""
	}
	return statusColorSequence(p, status)
}

// CompletionAccentSequence returns the configured accent for non-status
// completion entries such as shortcuts.
func CompletionAccentSequence() string {
	return colorSequence(completionPalette().Gate)
}

func statusColorSequence(p Palette, status string) string {
	color := statusColor(p, status)
	if sequence := colorSequence(color); sequence != "" {
		return sequence
	}
	return "\x1b[2m"
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
	WorkTypeImplColor     = paletteLipglossColor(defaultDarkPalette().Success)
	WorkTypeAnalysisColor = paletteLipglossColor(defaultDarkPalette().Festival)
	WorkTypeReviewColor   = paletteLipglossColor(defaultDarkPalette().Planning)
	WorkTypeVerifyColor   = paletteLipglossColor(defaultDarkPalette().Warning)
	WorkTypeConfigColor   = paletteLipglossColor(defaultDarkPalette().Metadata)
	WorkTypeDocsColor     = paletteLipglossColor(defaultDarkPalette().InProgress)
	WorkTypePlanColor     = paletteLipglossColor(defaultDarkPalette().Task)
	WorkTypeActionColor   = paletteLipglossColor(defaultDarkPalette().Sequence)
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
