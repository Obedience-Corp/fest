package explore

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the model with an IDE-style split layout: tree on left, content on right.
func (m Model) View() string {
	if m.loading {
		return dimStyle.Render("  Loading...")
	}
	if m.err != nil {
		return fmt.Sprintf("  Error: %v\n", m.err)
	}
	if len(m.visible) == 0 {
		label := "all statuses"
		if m.status != "" {
			label = m.status
		}
		return dimStyle.Render(fmt.Sprintf("  No festivals found (%s)\n", label))
	}

	if m.width < 80 {
		return m.renderNarrowView()
	}

	treeW := m.treeWidth()
	contentW := m.width - treeW - 2 // Gap between panels

	leftContent := m.renderTree(treeW)
	rightContent := m.renderContent(contentW)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftContent, rightContent)
}

func (m Model) renderNarrowView() string {
	var b strings.Builder
	crumb := m.renderBreadcrumb()
	b.WriteString(crumb)
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d items", len(m.visible))))
	b.WriteString("\n")
	b.WriteString(borderStyle.Render(strings.Repeat("─", max(m.width-2, 20))))
	b.WriteString("\n")

	endIdx := min(m.scrollStart+m.maxVisible, len(m.visible))
	for i := m.scrollStart; i < endIdx; i++ {
		node := m.visible[i]
		isSelected := i == m.cursor
		cursor := "  "
		if isSelected {
			cursor = cursorStyle.Render("▶ ")
		}
		b.WriteString(cursor)
		b.WriteString(m.renderTreeLine(node, isSelected, max(m.width-4, 20)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k nav • Enter open • s select • h back • q quit"))
	return b.String()
}

func (m Model) renderTree(width int) string {
	var b strings.Builder

	// Header
	crumb := m.renderBreadcrumb()
	b.WriteString(crumb)
	b.WriteString("\n")

	// Items
	endIdx := min(m.scrollStart+m.maxVisible, len(m.visible))
	for i := m.scrollStart; i < endIdx; i++ {
		node := m.visible[i]
		isSelected := i == m.cursor

		cursor := "  "
		if isSelected {
			cursor = cursorStyle.Render("▶ ")
		}

		b.WriteString(cursor)
		b.WriteString(m.renderTreeLine(node, isSelected, width-4))
		b.WriteString("\n")
	}

	// Pad remaining lines to fill panel height
	rendered := min(m.scrollStart+m.maxVisible, len(m.visible))
	usedLines := rendered - m.scrollStart
	panelH := max(m.height-4, 5)
	for i := usedLines; i < panelH-2; i++ { // -2 for header + help
		b.WriteString("\n")
	}

	// Help / Filter
	if m.filtering {
		b.WriteString(cursorStyle.Render("/ "))
		b.WriteString(m.filterInput.View())
	} else {
		help := "j/k nav • Enter open • s select • h back • /search"
		b.WriteString(helpStyle.Render(help))
	}

	innerW := max(width-2, 20) // Subtract border
	innerH := max(m.height-2, 10)

	return panelBorder(!m.focusPreview).
		Width(innerW).
		Height(innerH).
		Render(b.String())
}

func (m Model) renderContent(width int) string {
	var b strings.Builder

	// Title
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		b.WriteString(previewTitle.Render("Preview"))
	} else {
		item := m.visible[m.cursor].Item
		title := "Preview"
		switch item.Type {
		case ItemStatus:
			title = "Status"
		case ItemFestival:
			title = "Festival Goal"
		case ItemPhase:
			title = "Phase Goal"
		case ItemSequence:
			title = "Sequence Goal"
		case ItemTask:
			title = "Task"
		}
		b.WriteString(previewTitle.Render(title))
	}
	b.WriteString("\n")

	// Viewport content
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Help
	if m.focusPreview {
		pct := m.viewport.ScrollPercent() * 100
		help := fmt.Sprintf("j/k scroll • Enter/s select • Tab/h: tree  %.0f%%", pct)
		b.WriteString(helpStyle.Render(help))
	} else {
		b.WriteString(helpStyle.Render("Tab: focus preview • s: select"))
	}

	innerW := max(width-4, 20)
	innerH := max(m.height-2, 10)

	return panelBorder(m.focusPreview).
		Width(innerW).
		Height(innerH).
		Render(b.String())
}

func (m Model) renderBreadcrumb() string {
	parts := []string{"Festivals"}
	if m.status != "" {
		parts[0] = fmt.Sprintf("Festivals (%s)", m.status)
	}

	// Add breadcrumbs from navStack drilldowns
	parts = append(parts, m.breadcrumbs...)

	// In tree mode, add parent chain from current cursor node
	if m.inTreeMode() && m.cursor >= 0 && m.cursor < len(m.visible) {
		crumbs := breadcrumbsFromNode(m.visible[m.cursor])
		if len(crumbs) > 1 {
			// Skip the last element (it's the current node, not an ancestor)
			parts = append(parts, crumbs[:len(crumbs)-1]...)
		}
	}

	if len(parts) == 1 {
		return headerStyle.Render(parts[0])
	}

	var b strings.Builder
	for i, part := range parts {
		if i == len(parts)-1 {
			b.WriteString(headerStyle.Render(part))
		} else {
			b.WriteString(breadcrumbStyle.Render(part))
			b.WriteString(dimStyle.Render(" > "))
		}
	}
	return b.String()
}

// renderTreeLine renders a single tree node with indentation and expand/collapse icons.
func (m Model) renderTreeLine(node *TreeNode, isSelected bool, maxW int) string {
	// Indentation: 2 spaces per depth level
	indent := strings.Repeat("  ", node.Depth)

	// Expand/collapse icon — only for phase/sequence nodes (tree expand mode)
	// Status and Festival items use drilldown navigation, not tree expand
	var icon string
	if node.Item.Type == ItemStatus || node.Item.Type == ItemFestival || node.IsLeaf() {
		icon = "  " // No tree icon for drilldown items or leaves
	} else if node.Loading {
		icon = loadingIcon + " "
	} else if node.Expanded {
		icon = expandedIcon + " "
	} else {
		icon = collapsedIcon + " "
	}

	// Name
	name := node.Item.Name
	// Calculate available width for name
	indentW := node.Depth * 2
	iconW := 2
	suffixW := 0

	// Compute suffix first
	var suffix string
	switch node.Item.Type {
	case ItemStatus:
		countText := "..."
		if node.Item.Count >= 0 {
			countText = fmt.Sprintf("%d", node.Item.Count)
		}
		suffix = " " + dimStyle.Render(fmt.Sprintf("(%s)", countText))
		suffixW = len(countText) + 3 // parens + space
	case ItemFestival:
		suffix = " " + StatusStyle(node.Item.Status).Render(node.Item.Status)
		suffixW = len(node.Item.Status) + 1
	}

	nameW := max(maxW-indentW-iconW-suffixW, 5)
	if len(name) > nameW && nameW > 3 {
		name = name[:nameW-3] + "..."
	}

	nameText := normalStyle.Render(name)
	if isSelected {
		nameText = selectedStyle.Render(name)
	}

	return indent + icon + nameText + suffix
}
