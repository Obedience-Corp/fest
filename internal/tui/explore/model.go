package explore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/watch"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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

// previewLoadedMsg is sent when async preview rendering completes.
type previewLoadedMsg struct {
	rendered string
}

// statusCountsMsg is sent when festival counts per status are loaded asynchronously.
type statusCountsMsg struct {
	counts map[string]int
}

// refreshMsg triggers a reload of the current view's data.
type refreshMsg struct{}

// refreshItemsMsg delivers refreshed items without pushing the nav stack.
type refreshItemsMsg struct {
	items []FestivalItem
}

// Model is the BubbleTea model for the festival explorer.
type Model struct {
	ctx          context.Context
	items        []FestivalItem
	allItems     []FestivalItem // Unfiltered items (for search restore)
	selected     int
	width        int
	height       int
	maxVisible   int
	scrollStart  int
	status       string
	loading      bool
	err          error
	quitting     bool
	navStack     []navEntry
	breadcrumbs  []string
	festivalPath string // Pre-navigate into this festival on load

	// Viewport for scrollable preview
	viewport     viewport.Model
	focusPreview bool

	// Pending navigation (push to stack only on success)
	pendingNav *navEntry

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

	vp := viewport.New(60, 20)

	return Model{
		ctx:         ctx,
		status:      status,
		maxVisible:  20,
		loading:     true,
		filterInput: ti,
		viewport:    vp,
	}
}

// Init returns the command to load festivals.
func (m Model) Init() tea.Cmd {
	ctx := m.ctx
	status := m.status

	// Empty status: show status overview instantly, load counts async
	if status == "" {
		items := buildStatusItems()
		return tea.Batch(
			func() tea.Msg {
				return festivalsLoadedMsg{items: items}
			},
			func() tea.Msg {
				return loadStatusCounts(ctx)
			},
			watchCmd(ctx),
		)
	}

	// Specific status: load festivals for that status
	loadCmd := func() tea.Msg {
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

		var items []FestivalItem
		festivals, loadErr := show.ListFestivalsByStatusLight(ctx, festivalsDir, status)
		if loadErr != nil {
			return festivalsLoadedMsg{err: loadErr}
		}
		for _, f := range festivals {
			items = append(items, FestivalItem{
				Name:      f.Name,
				Status:    f.Status,
				CreatedAt: f.ModTime,
				Path:      f.Path,
				Type:      ItemFestival,
			})
		}

		sortFestivalsByCreated(items)
		return festivalsLoadedMsg{items: items}
	}

	return tea.Batch(loadCmd, watchCmd(ctx))
}

// buildStatusItems creates status overview items from constants (zero I/O).
func buildStatusItems() []FestivalItem {
	items := make([]FestivalItem, 0, len(id.StatusDirectories))
	for _, s := range id.StatusDirectories {
		items = append(items, FestivalItem{
			Name:   statusDisplayName(s),
			Status: s,
			Path:   s,
			Type:   ItemStatus,
			Count:  -1, // loading
		})
	}
	return items
}

// sortFestivalsByCreated sorts festival items by CreatedAt descending (newest first).
// Items without dates sort last; ties fall back to alphabetical name.
func sortFestivalsByCreated(items []FestivalItem) {
	sort.SliceStable(items, func(i, j int) bool {
		ti := items[i].CreatedAt
		tj := items[j].CreatedAt
		if ti.IsZero() && tj.IsZero() {
			return items[i].Name < items[j].Name
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		return ti.After(tj)
	})
}

// loadStatusCounts does lightweight directory counting (ReadDir only, no stat validation).
func loadStatusCounts(ctx context.Context) tea.Msg {
	cwd, err := os.Getwd()
	if err != nil {
		return statusCountsMsg{}
	}
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return statusCountsMsg{}
	}

	counts := make(map[string]int, len(id.StatusDirectories))
	for _, s := range id.StatusDirectories {
		if ctx.Err() != nil {
			break
		}
		statusDir := filepath.Join(festivalsDir, s)
		entries, readErr := os.ReadDir(statusDir)
		if readErr != nil {
			counts[s] = 0
			continue
		}
		count := 0
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				count++
			}
		}
		counts[s] = count
	}
	return statusCountsMsg{counts: counts}
}

// loadFestivalsForStatus loads festivals for a specific status directory.
func loadFestivalsForStatus(ctx context.Context, status string) tea.Msg {
	cwd, err := os.Getwd()
	if err != nil {
		return childrenLoadedMsg{err: err}
	}
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return childrenLoadedMsg{err: fmt.Errorf("not in a festivals workspace")}
	}

	festivals, loadErr := show.ListFestivalsByStatusLight(ctx, festivalsDir, status)
	if loadErr != nil {
		return childrenLoadedMsg{err: loadErr}
	}

	var items []FestivalItem
	for _, f := range festivals {
		items = append(items, FestivalItem{
			Name:      f.Name,
			Status:    f.Status,
			CreatedAt: f.ModTime,
			Path:      f.Path,
			Type:      ItemFestival,
		})
	}

	sortFestivalsByCreated(items)
	return childrenLoadedMsg{
		items:      items,
		breadcrumb: statusDisplayName(status),
	}
}

// statusDisplayName maps a status directory path to a user-friendly name.
func statusDisplayName(status string) string {
	switch status {
	case "dungeon/completed":
		return "Completed"
	case "dungeon/archived":
		return "Archived"
	case "dungeon/someday":
		return "Someday"
	case "planning":
		return "Planning"
	case "ready":
		return "Ready"
	case "active":
		return "Active"
	case "ritual":
		return "Ritual"
	default:
		return status
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
		// Only quit on empty items for specific-status views, not the overview
		if len(m.items) == 0 && m.status != "" {
			return m, tea.Quit
		}
		if len(m.items) == 0 {
			return m, nil
		}

		// Auto-navigate into a specific festival if requested
		if m.festivalPath != "" {
			for i, item := range m.items {
				if item.Path == m.festivalPath {
					m.selected = i
					m.festivalPath = "" // Clear to prevent re-navigation
					return m.navigateDown()
				}
			}
			m.festivalPath = "" // Not found, proceed normally
		}
		return m, m.loadPreviewCmd()

	case statusCountsMsg:
		if msg.counts != nil {
			for i, item := range m.items {
				if item.Type == ItemStatus {
					if c, ok := msg.counts[item.Status]; ok {
						m.items[i].Count = c
					}
				}
			}
		}
		return m, nil

	case childrenLoadedMsg:
		m.loading = false
		if msg.err != nil || len(msg.items) == 0 {
			// Discard pending nav — children failed or empty
			m.pendingNav = nil
			return m, nil
		}
		// Push nav stack only on successful load
		if m.pendingNav != nil {
			m.navStack = append(m.navStack, *m.pendingNav)
			m.pendingNav = nil
		}
		m.items = msg.items
		m.selected = 0
		m.scrollStart = 0
		m.breadcrumbs = append(m.breadcrumbs, msg.breadcrumb)
		return m, m.loadPreviewCmd()

	case previewLoadedMsg:
		if msg.rendered == "" {
			m.viewport.SetContent(dimStyle.Render("No preview available"))
		} else {
			m.viewport.SetContent(msg.rendered)
		}
		m.viewport.GotoTop()
		return m, nil

	case refreshMsg:
		// Refresh current view and restart watcher for next change
		return m, tea.Batch(m.refreshCurrentView(), watchCmd(m.ctx))

	case refreshItemsMsg:
		// Replace items without changing navigation state
		m.items = msg.items
		if m.selected >= len(m.items) {
			m.selected = max(len(m.items)-1, 0)
		}
		m.ensureVisible()
		return m, m.loadPreviewCmd()

	case tea.KeyMsg:
		if m.filtering {
			return m.handleFilterKey(msg)
		}
		if m.focusPreview {
			return m.handlePreviewKey(msg)
		}
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.maxVisible = max(msg.Height-5, 5)
		m.syncViewportSize()
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
			m2 := m.navigateUp()
			return m2, m2.loadPreviewCmd()
		}
		if msg.String() != "h" {
			m.quitting = true
			return m, tea.Quit
		}

	case "up", "k":
		if m.selected > 0 {
			m.selected--
			m.ensureVisible()
			return m, m.loadPreviewCmd()
		}

	case "down", "j":
		if m.selected < len(m.items)-1 {
			m.selected++
			m.ensureVisible()
			return m, m.loadPreviewCmd()
		}

	case "enter", "l":
		return m.navigateDown()

	case "tab":
		m.focusPreview = true
		return m, nil

	case "G":
		if len(m.items) > 0 {
			m.selected = len(m.items) - 1
			m.ensureVisible()
			return m, m.loadPreviewCmd()
		}

	case "g":
		now := time.Now()
		if now.Sub(m.lastGTime) < ggTimeout {
			m.selected = 0
			m.ensureVisible()
			m.lastGTime = time.Time{}
			return m, m.loadPreviewCmd()
		}
		m.lastGTime = now

	case "/":
		m.filtering = true
		m.filterInput.SetValue("")
		m.filterInput.Focus()
		m.allItems = m.items
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
		m.items = m.allItems
		m.allItems = nil
		m.selected = 0
		m.scrollStart = 0
		return m, m.loadPreviewCmd()
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

	// Batch preview cmd with filter input cmd
	previewCmd := m.loadPreviewCmd()
	return m, tea.Batch(cmd, previewCmd)
}

// navigateDown drills into the selected item's children.
func (m Model) navigateDown() (tea.Model, tea.Cmd) {
	if m.selected < 0 || m.selected >= len(m.items) {
		return m, nil
	}

	item := m.items[m.selected]

	// Tasks are leaf nodes — focus the preview pane
	if item.Type == ItemTask {
		m.focusPreview = true
		return m, nil
	}

	// Store pending nav — only push to stack when children load successfully
	m.pendingNav = &navEntry{
		items:    m.items,
		title:    m.currentTitle(),
		selected: m.selected,
		scroll:   m.scrollStart,
	}

	m.loading = true
	ctx := m.ctx

	// Status items: lazy-load festivals for that status
	if item.Type == ItemStatus {
		status := item.Status
		return m, func() tea.Msg {
			return loadFestivalsForStatus(ctx, status)
		}
	}

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
		if len(phases) > 0 {
			return elementsToItems(phases, ItemPhase), nil
		}
		return loadGenericChildren(item.Path)

	case ItemPhase:
		sequences, err := parser.ParseSequences(ctx, item.Path)
		if err != nil {
			return nil, err
		}
		if len(sequences) > 0 {
			return elementsToItems(sequences, ItemSequence), nil
		}
		return loadGenericChildren(item.Path)

	case ItemSequence:
		tasks, err := parser.ParseTasks(ctx, item.Path)
		if err != nil {
			return nil, err
		}
		if len(tasks) > 0 {
			return elementsToItems(tasks, ItemTask), nil
		}
		return loadGenericChildren(item.Path)

	default:
		return nil, nil
	}
}

// loadGenericChildren lists non-numbered subdirectories and .md files as browsable items.
func loadGenericChildren(dir string) ([]FestivalItem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var items []FestivalItem
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		// Skip known goal files
		switch name {
		case "FESTIVAL_GOAL.md", "FESTIVAL_OVERVIEW.md", "PHASE_GOAL.md", "SEQUENCE_GOAL.md":
			continue
		}

		if e.IsDir() {
			items = append(items, FestivalItem{
				Name: name,
				Path: filepath.Join(dir, name),
				Type: ItemSequence, // Navigable directory
			})
		} else if strings.HasSuffix(name, ".md") {
			items = append(items, FestivalItem{
				Name: name,
				Path: filepath.Join(dir, name),
				Type: ItemTask, // Leaf node, preview-able
			})
		}
	}
	return items, nil
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

// loadPreviewCmd returns a tea.Cmd that loads and renders the preview asynchronously.
func (m Model) loadPreviewCmd() tea.Cmd {
	if m.selected < 0 || m.selected >= len(m.items) {
		return func() tea.Msg { return previewLoadedMsg{} }
	}
	path := goalFileForItem(m.items[m.selected])
	width := m.previewWidth()
	return func() tea.Msg {
		raw := loadPreview(path)
		if raw == "" {
			return previewLoadedMsg{}
		}
		r := &mdRenderer{}
		rendered := r.render(raw, width)
		return previewLoadedMsg{rendered: rendered}
	}
}

// refreshCurrentView reloads the current view's data while preserving navigation state.
func (m Model) refreshCurrentView() tea.Cmd {
	ctx := m.ctx

	// At status overview root: reload status counts
	if len(m.navStack) == 0 && m.status == "" {
		return func() tea.Msg { return loadStatusCounts(ctx) }
	}

	// At festival list root: reload festivals
	if len(m.navStack) == 0 {
		status := m.status
		return func() tea.Msg {
			cwd, err := os.Getwd()
			if err != nil {
				return refreshItemsMsg{}
			}
			festivalsDir, err := workspace.FindFestivals(cwd)
			if err != nil || festivalsDir == "" {
				return refreshItemsMsg{}
			}
			festivals, loadErr := show.ListFestivalsByStatusLight(ctx, festivalsDir, status)
			if loadErr != nil {
				return refreshItemsMsg{}
			}
			var items []FestivalItem
			for _, f := range festivals {
				items = append(items, FestivalItem{
					Name:      f.Name,
					Status:    f.Status,
					CreatedAt: f.ModTime,
					Path:      f.Path,
					Type:      ItemFestival,
				})
			}
			sortFestivalsByCreated(items)
			return refreshItemsMsg{items: items}
		}
	}

	// Inside a hierarchy: reload children for the current parent
	entry := m.navStack[len(m.navStack)-1]
	if entry.selected < len(entry.items) {
		parent := entry.items[entry.selected]
		return func() tea.Msg {
			children, err := loadChildren(ctx, parent)
			if err != nil {
				return refreshItemsMsg{}
			}
			return refreshItemsMsg{items: children}
		}
	}
	return nil
}

// watchCmd returns a tea.Cmd that monitors the festivals directory for changes
// and sends refreshMsg to the TUI when files are modified.
func watchCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		cwd, err := os.Getwd()
		if err != nil {
			return nil
		}
		festivalsDir, err := workspace.FindFestivals(cwd)
		if err != nil || festivalsDir == "" {
			return nil
		}

		changed := make(chan struct{}, 1)

		w, err := watch.New(watch.Config{
			Paths:    []string{festivalsDir},
			Debounce: 300 * time.Millisecond,
		}, func() {
			select {
			case changed <- struct{}{}:
			default:
			}
		})
		if err != nil {
			return nil
		}

		go func() {
			w.Watch(ctx)
		}()

		select {
		case <-ctx.Done():
			w.Close()
			return nil
		case <-changed:
			w.Close()
			return refreshMsg{}
		}
	}
}

func (m Model) previewWidth() int {
	if m.width < 80 {
		return m.width
	}
	return m.width - m.treeWidth() - 4 // borders + padding
}

func (m Model) treeWidth() int {
	w := m.width * 30 / 100
	if w < 25 {
		w = 25
	}
	if w > 50 {
		w = 50
	}
	return w
}

func (m *Model) syncViewportSize() {
	pw := m.previewWidth()
	// Height: full terminal minus top/bottom border (2) minus title line (1)
	ph := max(m.height-4, 5)
	m.viewport.Width = pw
	m.viewport.Height = ph
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

// View renders the model with an IDE-style split layout: tree on left, content on right.
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

	if m.width < 80 {
		// Narrow terminal: list only
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
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d items", len(m.items))))
	b.WriteString("\n")
	b.WriteString(borderStyle.Render(strings.Repeat("─", max(m.width-2, 20))))
	b.WriteString("\n")

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

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k: nav • l: enter • h: back • q: quit"))
	return b.String()
}

func (m Model) renderTree(width int) string {
	var b strings.Builder

	// Header
	crumb := m.renderBreadcrumb()
	b.WriteString(crumb)
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
		b.WriteString(m.renderTreeItem(item, isSelected, width-4))
		b.WriteString("\n")
	}

	// Pad remaining lines to fill panel height
	rendered := min(m.scrollStart+m.maxVisible, len(m.items))
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
		help := "j/k nav • l enter • /search"
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
	item := m.items[m.selected]
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
	b.WriteString("\n")

	// Viewport content
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Help
	if m.focusPreview {
		pct := m.viewport.ScrollPercent() * 100
		help := fmt.Sprintf("j/k scroll • Tab/h: tree  %.0f%%", pct)
		b.WriteString(helpStyle.Render(help))
	} else {
		b.WriteString(helpStyle.Render("Tab: focus preview"))
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

func (m Model) renderTreeItem(item FestivalItem, isSelected bool, maxW int) string {
	name := item.Name
	// Truncate long names to fit tree panel
	if len(name) > maxW && maxW > 3 {
		name = name[:maxW-3] + "..."
	}

	nameText := normalStyle.Render(name)
	if isSelected {
		nameText = selectedStyle.Render(name)
	}

	switch item.Type {
	case ItemStatus:
		countText := "..."
		if item.Count >= 0 {
			countText = fmt.Sprintf("%d", item.Count)
		}
		countLabel := dimStyle.Render(fmt.Sprintf("(%s)", countText))
		return fmt.Sprintf("%s %s", nameText, countLabel)
	case ItemFestival:
		status := StatusStyle(item.Status).Render(fmt.Sprintf("%-8s", item.Status))
		return fmt.Sprintf("%s %s", nameText, status)
	case ItemTask:
		return nameText
	default:
		return nameText
	}
}

func (m Model) renderItem(item FestivalItem, isSelected bool) string {
	nameText := item.Name
	if isSelected {
		nameText = selectedStyle.Render(nameText)
	} else {
		nameText = normalStyle.Render(nameText)
	}

	switch item.Type {
	case ItemStatus:
		countText := "..."
		if item.Count >= 0 {
			countText = fmt.Sprintf("%d", item.Count)
		}
		countLabel := dimStyle.Render(fmt.Sprintf("(%s festivals)", countText))
		return fmt.Sprintf("%s  %s", nameText, countLabel)
	case ItemFestival:
		status := StatusStyle(item.Status).Render(fmt.Sprintf("%-10s", item.Status))
		date := dimStyle.Render(item.CreatedAt.Format("Jan 02"))
		return fmt.Sprintf("%s  %s  %s", nameText, status, date)
	default:
		return nameText
	}
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

// RunWithFestival starts the explore TUI pre-navigated into a specific festival.
// The user sees the festival's phases immediately instead of the festival list.
func RunWithFestival(ctx context.Context, festivalPath string) (*FestivalItem, error) {
	status := detectStatusFromPath(festivalPath)
	m := New(ctx, status)
	m.festivalPath = festivalPath

	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	return finalModel.(Model).SelectedItem(), nil
}

// detectStatusFromPath determines the status directory from a festival's path.
// For /festivals/active/my-fest -> "active"
// For /festivals/dungeon/completed/my-fest -> "dungeon/completed"
func detectStatusFromPath(festivalPath string) string {
	parent := filepath.Base(filepath.Dir(festivalPath))
	grandparent := filepath.Base(filepath.Dir(filepath.Dir(festivalPath)))
	if grandparent == "dungeon" {
		return "dungeon/" + parent
	}
	return parent
}
