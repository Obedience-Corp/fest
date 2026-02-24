package explore

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		// If cursor is on an expanded node, collapse it
		if m.cursor >= 0 && m.cursor < len(m.visible) {
			node := m.visible[m.cursor]
			if node.Expanded {
				node.Expanded = false
				m.rebuildVisible()
				return m, m.debouncedPreview()
			}
			// If collapsed but has parent, move cursor to parent
			if node.Parent != nil {
				m.moveCursorToNode(node.Parent)
				return m, m.debouncedPreview()
			}
		}
		m.quitting = true
		return m, tea.Quit

	case "backspace", "h":
		if m.cursor >= 0 && m.cursor < len(m.visible) {
			node := m.visible[m.cursor]
			// If expanded, collapse
			if node.Expanded {
				node.Expanded = false
				m.rebuildVisible()
				return m, m.debouncedPreview()
			}
			// If collapsed with parent, move to parent
			if node.Parent != nil {
				m.moveCursorToNode(node.Parent)
				return m, m.debouncedPreview()
			}
		}
		// h at root level is no-op; backspace quits
		if msg.String() != "h" {
			m.quitting = true
			return m, tea.Quit
		}

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
			return m, m.debouncedPreview()
		}

	case "down", "j":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
			m.ensureVisible()
			return m, m.debouncedPreview()
		}

	case "enter", "l":
		return m.toggleExpand()

	case "tab":
		m.focusPreview = true
		return m, nil

	case "G":
		if len(m.visible) > 0 {
			m.cursor = len(m.visible) - 1
			m.ensureVisible()
			return m, m.debouncedPreview()
		}

	case "g":
		now := time.Now()
		if now.Sub(m.lastGTime) < ggTimeout {
			m.cursor = 0
			m.ensureVisible()
			m.lastGTime = time.Time{}
			return m, m.debouncedPreview()
		}
		m.lastGTime = now

	case "/":
		m.filtering = true
		m.filterInput.SetValue("")
		m.filterInput.Focus()
		m.allRoots = m.roots
		return m, textinput.Blink
	}

	return m, nil
}

func (m Model) handlePreviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "tab", "esc", "h":
		m.focusPreview = false
		return m, nil
	}

	// Delegate scrolling to viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.filtering = false
		m.filterInput.Blur()
		return m, nil

	case "esc":
		m.filtering = false
		m.filterInput.Blur()
		m.roots = m.allRoots
		m.allRoots = nil
		m.rebuildVisible()
		m.cursor = 0
		m.scrollStart = 0
		return m, m.loadPreviewCmd()
	}

	// Update text input
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)

	// Apply filter to visible (top-level) nodes
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if query == "" {
		m.roots = m.allRoots
	} else {
		var filtered []*TreeNode
		for _, node := range m.allRoots {
			if strings.Contains(strings.ToLower(node.Item.Name), query) {
				filtered = append(filtered, node)
			}
		}
		m.roots = filtered
	}
	m.rebuildVisible()
	m.cursor = 0
	m.scrollStart = 0

	// Batch preview cmd with filter input cmd
	previewCmd := m.loadPreviewCmd()
	return m, tea.Batch(cmd, previewCmd)
}

// moveCursorToNode sets the cursor on the given node in the visible list.
func (m *Model) moveCursorToNode(target *TreeNode) {
	for i, node := range m.visible {
		if node == target {
			m.cursor = i
			m.ensureVisible()
			return
		}
	}
}
