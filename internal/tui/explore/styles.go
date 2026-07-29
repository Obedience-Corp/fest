// Package explore provides the BubbleTea TUI model for the festival explorer.
package explore

import (
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

// These accessors intentionally resolve at render time. The explorer model is
// constructed before Cobra's pre-run initializes the configured palette, so
// package-level Lip Gloss styles would otherwise capture stale colors.
func colorActive() lipgloss.TerminalColor    { return ui.Current().Active }
func colorPlanned() lipgloss.TerminalColor   { return ui.Current().Planning }
func colorReady() lipgloss.TerminalColor     { return ui.Current().Ready }
func colorRitual() lipgloss.TerminalColor    { return ui.Current().Ritual }
func colorCompleted() lipgloss.TerminalColor { return ui.Current().Completed }
func colorArchived() lipgloss.TerminalColor  { return ui.Current().Archived }
func colorSomeday() lipgloss.TerminalColor   { return ui.Current().Someday }
func colorDungeon() lipgloss.TerminalColor   { return ui.Current().Dungeon }
func colorText() lipgloss.TerminalColor      { return ui.Current().Value }
func colorDim() lipgloss.TerminalColor       { return ui.Current().Dim }
func colorFocus() lipgloss.TerminalColor     { return ui.Current().InProgress }
func colorBorder() lipgloss.TerminalColor    { return ui.Current().Border }

// StatusStyle returns the lipgloss style for a given festival status.
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "active":
		return lipgloss.NewStyle().Foreground(colorActive()).Bold(true)
	case "planning":
		return lipgloss.NewStyle().Foreground(colorPlanned())
	case "ready":
		return lipgloss.NewStyle().Foreground(colorReady()).Bold(true)
	case "ritual":
		return lipgloss.NewStyle().Foreground(colorRitual())
	case "completed", "dungeon/completed":
		return lipgloss.NewStyle().Foreground(colorCompleted())
	case "archived", "dungeon/archived":
		return lipgloss.NewStyle().Foreground(colorArchived()).Faint(true)
	case "someday", "dungeon/someday":
		return lipgloss.NewStyle().Foreground(colorSomeday()).Faint(true)
	case "dungeon":
		return lipgloss.NewStyle().Foreground(colorDungeon()).Faint(true)
	default:
		return lipgloss.NewStyle().Foreground(colorText())
	}
}

func selectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorFocus()).Bold(true)
}

func normalStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(colorText()) }

func dimStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(colorDim()) }

func cursorStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(colorFocus()) }

func headerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorBorder()).Bold(true)
}

func helpStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorDim()).Faint(true)
}

func borderStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(colorBorder()) }

func breadcrumbStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorDim())
}

func previewTitle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorBorder()).Bold(true)
}

// Tree node icons.
const (
	expandedIcon  = "▼"
	collapsedIcon = "▶"
	loadingIcon   = "◌"
)

// panelBorder returns a border style for a panel with the given focus state.
func panelBorder(focused bool) lipgloss.Style {
	border := ui.Current().Dim
	if focused {
		border = ui.Current().Border
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border)
}
