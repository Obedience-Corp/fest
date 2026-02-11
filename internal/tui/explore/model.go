package explore

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const ggTimeout = 500 * time.Millisecond

// navEntry stores the state of a navigation level for the stack.
type navEntry struct {
	items      []FestivalItem
	title      string
	selected   int
	scroll     int
	breadcrumb string
}

// festivalsLoadedMsg is sent when festival data is loaded from the filesystem.
type festivalsLoadedMsg struct {
	items []FestivalItem
	err   error
}

// childrenLoadedMsg is sent when children of a hierarchy item are loaded.
type childrenLoadedMsg struct {
	items      []FestivalItem
	breadcrumb string
	err        error
}

// Model is the BubbleTea model for the festival explorer.
type Model struct {
	ctx         context.Context
	items       []FestivalItem
	allItems    []FestivalItem // Unfiltered items (for search restore)
	selected    int
	width       int
	height      int
	maxVisible  int
	scrollStart int
	status      string
	loading     bool
	err         error
	quitting    bool
	navStack    []navEntry
	breadcrumbs []string
	preview     string

	// Vim gg detection
	lastGTime time.Time

	// Search filter
	filtering   bool
	filterInput textinput.Model
}

// New creates a new explore model for the given status filter.
func New(ctx context.Context, status string) Model {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 100
	ti.Width = 40
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorFocus)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)

	return Model{
		ctx:         ctx,
		status:      status,
		maxVisible:  20,
		loading:     true,
		filterInput: ti,
	}
}

// Init returns the command to load festivals.
func (m Model) Init() tea.Cmd {
	ctx := m.ctx
	status := m.status
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return festivalsLoadedMsg{err: err}
		}

		cwd, err := os.Getwd()
		if err != nil {
			return festivalsLoadedMsg{err: fmt.Errorf("getting working directory: %w", err)}
		}

		festivalsDir, err := workspace.FindFestivals(cwd)
		if err != nil || festivalsDir == "" {
			return festivalsLoadedMsg{err: fmt.Errorf("not in a festivals workspace")}
		}

		statuses := []string{status}
		if status == "" {
			statuses = []string{"active", "ready", "planned", "completed", "dungeon/completed", "dungeon/archived", "dungeon/someday"}
		}

		var items []FestivalItem
		for _, s := range statuses {
			festivals, loadErr := show.ListFestivalsByStatus(ctx, festivalsDir, s)
			if loadErr != nil {
				continue
			}
			for _, f := range festivals {
				var progress float64
				if f.Stats != nil {
					progress = f.Stats.Progress
				}
				items = append(items, FestivalItem{
					Name:      f.Name,
					Status:    f.Status,
					Progress:  progress,
					CreatedAt: f.ModTime,
					Path:      f.Path,
					Type:      ItemFestival,
				})
			}
		}

		return festivalsLoadedMsg{items: items}
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case festivalsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.items = msg.items
		if len(m.items) == 0 {
			return m, tea.Quit
		}
		m.updatePreview()
		return m, nil

	case childrenLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if len(msg.items) == 0 {
			return m, nil
		}
		m.items = msg.items
		m.selected = 0
		m.scrollStart = 0
		m.breadcrumbs = append(m.breadcrumbs, msg.breadcrumb)
		m.updatePreview()
		return m, nil

	case tea.KeyMsg:
		if m.filtering {
			return m.handleFilterKey(msg)
		}
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.maxVisible = max(msg.Height-5, 5)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc", "backspace", "h":
		if len(m.navStack) > 0 {
			return m.navigateUp(), nil
		}
		if msg.String() != "h" {
			m.quitting = true
			return m, tea.Quit
		}

	case "up", "k":
		if m.selected > 0 {
			m.selected--
			m.ensureVisible()
			m.updatePreview()
		}

	case "down", "j":
		if m.selected < len(m.items)-1 {
			m.selected++
			m.ensureVisible()
			m.updatePreview()
		}

	case "enter", "l":
		return m.navigateDown()

	case "G":
		if len(m.items) > 0 {
			m.selected = len(m.items) - 1
			m.ensureVisible()
			m.updatePreview()
		}

	case "g":
		now := time.Now()
		if now.Sub(m.lastGTime) < ggTimeout {
			m.selected = 0
			m.ensureVisible()
			m.updatePreview()
			m.lastGTime = time.Time{}
		} else {
			m.lastGTime = now
		}

	case "/":
		m.filtering = true
		m.filterInput.SetValue("")
		m.filterInput.Focus()
		m.allItems = m.items
		return m, textinput.Blink
	}

	return m, nil
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
		m.items = m.allItems
		m.allItems = nil
		m.selected = 0
		m.scrollStart = 0
		m.updatePreview()
		return m, nil
	}

	// Update text input
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)

	// Apply filter
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if query == "" {
		m.items = m.allItems
	} else {
		var filtered []FestivalItem
		for _, item := range m.allItems {
			if strings.Contains(strings.ToLower(item.Name), query) {
				filtered = append(filtered, item)
			}
		}
		m.items = filtered
	}
	m.selected = 0
	m.scrollStart = 0
	m.updatePreview()

	return m, cmd
}

// navigateDown drills into the selected item's children.
func (m Model) navigateDown() (tea.Model, tea.Cmd) {
	if m.selected < 0 || m.selected >= len(m.items) {
		return m, nil
	}

	item := m.items[m.selected]

	// Tasks are the deepest level - no drilldown
	if item.Type == ItemTask {
		return m, nil
	}

	// Push current state onto navigation stack
	m.navStack = append(m.navStack, navEntry{
		items:    m.items,
		title:    m.currentTitle(),
		selected: m.selected,
		scroll:   m.scrollStart,
	})

	m.loading = true
	ctx := m.ctx

	return m, func() tea.Msg {
		children, err := loadChildren(ctx, item)
		return childrenLoadedMsg{
			items:      children,
			breadcrumb: item.Name,
			err:        err,
		}
	}
}

// navigateUp pops the navigation stack and restores the previous level.
func (m Model) navigateUp() Model {
	if len(m.navStack) == 0 {
		return m
	}

	entry := m.navStack[len(m.navStack)-1]
	m.navStack = m.navStack[:len(m.navStack)-1]
	m.items = entry.items
	m.selected = entry.selected
	m.scrollStart = entry.scroll

	if len(m.breadcrumbs) > 0 {
		m.breadcrumbs = m.breadcrumbs[:len(m.breadcrumbs)-1]
	}

	m.updatePreview()
	return m
}

// loadChildren loads the child items for a given hierarchy item.
func loadChildren(ctx context.Context, item FestivalItem) ([]FestivalItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	parser := festival.NewParser()

	switch item.Type {
	case ItemFestival:
		phases, err := parser.ParsePhases(ctx, item.Path)
		if err != nil {
			return nil, err
		}
		return elementsToItems(phases, ItemPhase), nil

	case ItemPhase:
		sequences, err := parser.ParseSequences(ctx, item.Path)
		if err != nil {
			return nil, err
		}
		return elementsToItems(sequences, ItemSequence), nil

	case ItemSequence:
		tasks, err := parser.ParseTasks(ctx, item.Path)
		if err != nil {
			return nil, err
		}
		return elementsToItems(tasks, ItemTask), nil

	default:
		return nil, nil
	}
}

// elementsToItems converts festival parser elements to FestivalItems.
func elementsToItems(elements []festival.FestivalElement, itemType ItemType) []FestivalItem {
	items := make([]FestivalItem, 0, len(elements))
	for _, el := range elements {
		items = append(items, FestivalItem{
			Name: el.FullName,
			Path: el.Path,
			Type: itemType,
		})
	}
	return items
}

func (m *Model) ensureVisible() {
	if m.selected < m.scrollStart {
		m.scrollStart = m.selected
	}
	if m.selected >= m.scrollStart+m.maxVisible {
		m.scrollStart = m.selected - m.maxVisible + 1
	}
}

func (m *Model) updatePreview() {
	if m.selected < 0 || m.selected >= len(m.items) {
		m.preview = ""
		return
	}
	goalFile := goalFileForItem(m.items[m.selected])
	m.preview = loadPreview(goalFile)
}

func (m Model) currentTitle() string {
	if len(m.breadcrumbs) == 0 {
		title := "Festivals"
		if m.status != "" {
			title += " (" + m.status + ")"
		}
		return title
	}
	return m.breadcrumbs[len(m.breadcrumbs)-1]
}

// SelectedItem returns the currently selected festival item, or nil.
func (m Model) SelectedItem() *FestivalItem {
	if m.quitting && m.selected >= 0 && m.selected < len(m.items) {
		return &m.items[m.selected]
	}
	return nil
}

// View renders the model with a split layout: list on the left, preview on the right.
func (m Model) View() string {
	if m.loading {
		return dimStyle.Render("  Loading...")
	}
	if m.err != nil {
		return fmt.Sprintf("  Error: %v\n", m.err)
	}
	if len(m.items) == 0 {
		label := "all statuses"
		if m.status != "" {
			label = m.status
		}
		return dimStyle.Render(fmt.Sprintf("  No festivals found (%s)\n", label))
	}

	leftContent := m.renderList()
	rightContent := m.renderPreview()

	if m.width < 80 {
		// Narrow terminal: list only
		return leftContent
	}

	leftWidth := m.width * 3 / 5
	rightWidth := m.width - leftWidth - 1

	left := lipgloss.NewStyle().Width(leftWidth).Render(leftContent)
	right := previewBorder.Width(rightWidth - 2).Height(m.listHeight()).Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) listHeight() int {
	// header + border + items + blank + help
	return max(min(m.maxVisible+4, m.height-1), 10)
}

func (m Model) renderList() string {
	var b strings.Builder

	// Breadcrumb header
	crumb := m.renderBreadcrumb()
	b.WriteString(crumb)
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d items", len(m.items))))
	b.WriteString("\n")

	lineWidth := min(m.width*3/5, 70)
	if m.width < 80 {
		lineWidth = min(m.width, 70)
	}
	b.WriteString(borderStyle.Render(strings.Repeat("─", max(lineWidth, 20))))
	b.WriteString("\n")

	// Items
	endIdx := min(m.scrollStart+m.maxVisible, len(m.items))

	for i := m.scrollStart; i < endIdx; i++ {
		item := m.items[i]
		isSelected := i == m.selected

		cursor := "  "
		if isSelected {
			cursor = cursorStyle.Render("▶ ")
		}

		b.WriteString(cursor)
		b.WriteString(m.renderItem(item, isSelected))
		b.WriteString("\n")
	}

	// Filter input or help
	b.WriteString("\n")
	if m.filtering {
		b.WriteString(cursorStyle.Render("/ "))
		b.WriteString(m.filterInput.View())
	} else {
		help := "j/k: navigate • h/l: back/enter • /: search • gg/G: top/bottom • q: quit"
		b.WriteString(helpStyle.Render(help))
	}

	return b.String()
}

func (m Model) renderBreadcrumb() string {
	parts := []string{"Festivals"}
	if m.status != "" {
		parts[0] = fmt.Sprintf("Festivals (%s)", m.status)
	}
	parts = append(parts, m.breadcrumbs...)

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

func (m Model) renderItem(item FestivalItem, isSelected bool) string {
	nameText := item.Name
	if isSelected {
		nameText = selectedStyle.Render(nameText)
	} else {
		nameText = normalStyle.Render(nameText)
	}

	switch item.Type {
	case ItemFestival:
		status := StatusStyle(item.Status).Render(fmt.Sprintf("%-10s", item.Status))
		progress := dimStyle.Render(fmt.Sprintf("%5.1f%%", item.Progress))
		date := dimStyle.Render(item.CreatedAt.Format("Jan 02"))
		return fmt.Sprintf("%s  %s %s  %s", nameText, status, progress, date)

	case ItemPhase, ItemSequence:
		return nameText

	case ItemTask:
		return nameText

	default:
		return nameText
	}
}

func (m Model) renderPreview() string {
	if m.preview == "" {
		return dimStyle.Render("No preview available")
	}

	var b strings.Builder

	// Title
	item := m.items[m.selected]
	title := "Preview"
	switch item.Type {
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
	b.WriteString("\n")

	// Preview content (truncated to fit pane)
	lines := strings.Split(m.preview, "\n")
	maxLines := max(m.listHeight()-3, 5)
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, dimStyle.Render("..."))
	}

	for _, line := range lines {
		b.WriteString(dimStyle.Render(line))
		b.WriteString("\n")
	}

	return b.String()
}

// Run starts the explore TUI and returns the selected festival item.
func Run(ctx context.Context, status string) (*FestivalItem, error) {
	m := New(ctx, status)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	return finalModel.(Model).SelectedItem(), nil
}
