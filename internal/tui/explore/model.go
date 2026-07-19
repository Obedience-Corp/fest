package explore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const ggTimeout = 500 * time.Millisecond

// navEntry stores the state of a navigation level for the drilldown stack.
type navEntry struct {
	roots  []*TreeNode
	title  string
	cursor int
	scroll int
}

// festivalsLoadedMsg is sent when festival data is loaded from the filesystem.
type festivalsLoadedMsg struct {
	items []FestivalItem
	err   error
}

// childrenLoadedMsg is sent when children of a hierarchy item are loaded.
type childrenLoadedMsg struct {
	items      []FestivalItem
	parentPath string // path of the parent node whose children were loaded
	breadcrumb string
	err        error
}

// drilldownLoadedMsg is sent when drilldown children (status→festivals, festival→phases) load.
type drilldownLoadedMsg struct {
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

// refreshItemsMsg delivers refreshed items without changing tree state.
type refreshItemsMsg struct {
	items []FestivalItem
}

// Model is the BubbleTea model for the festival explorer.
type Model struct {
	ctx          context.Context
	roots        []*TreeNode // Current level's tree nodes
	visible      []*TreeNode // Flattened visible nodes (expanded descendants)
	cursor       int         // Index into visible slice
	width        int
	height       int
	maxVisible   int
	scrollStart  int
	status       string
	loading      bool
	err          error
	quitting     bool
	selected     bool
	festivalPath string // Auto-navigate into this festival on load

	// Drilldown stack (status → festival list → festival hierarchy)
	navStack    []navEntry
	breadcrumbs []string
	pendingNav  *navEntry // pending push — only committed on successful load

	// Shared markdown renderer (fixes caching bug)
	mdRenderer *mdRenderer

	// Viewport for scrollable preview
	viewport     viewport.Model
	focusPreview bool

	// Vim gg detection
	lastGTime time.Time

	// Search filter
	filtering   bool
	filterInput textinput.Model
	allRoots    []*TreeNode // Unfiltered roots (for search restore)
}

// New creates a new explore model for the given status filter.
func New(ctx context.Context, status string) Model {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 100
	ti.Width = 40
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorFocus())
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText())

	vp := viewport.New(60, 20)

	return Model{
		ctx:         ctx,
		status:      status,
		maxVisible:  20,
		loading:     true,
		filterInput: ti,
		viewport:    vp,
		mdRenderer:  &mdRenderer{},
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

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case festivalsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}

		// Convert items to flat tree nodes (depth 0, no expand/collapse icons)
		m.roots = itemsToTreeNodes(msg.items, 0, nil)
		m.rebuildVisible()

		// Only quit on empty items for specific-status views, not the overview
		if len(m.visible) == 0 && m.status != "" {
			return m, tea.Quit
		}
		if len(m.visible) == 0 {
			return m, nil
		}

		// Auto-navigate into a specific festival if requested
		if m.festivalPath != "" {
			for i, node := range m.visible {
				if node.Item.Path == m.festivalPath {
					m.cursor = i
					m.festivalPath = ""
					return m.navigateDown()
				}
			}
			m.festivalPath = ""
		}

		return m, m.loadPreviewCmd()

	case statusCountsMsg:
		if msg.counts != nil {
			for _, node := range m.roots {
				if node.Item.Type == ItemStatus {
					if c, ok := msg.counts[node.Item.Status]; ok {
						node.Item.Count = c
					}
				}
			}
		}
		return m, nil

	case drilldownLoadedMsg:
		m.loading = false
		if msg.err != nil || len(msg.items) == 0 {
			// Discard pending nav — children failed or empty
			m.pendingNav = nil
			return m, nil
		}
		// Push nav stack on successful load
		if m.pendingNav != nil {
			m.navStack = append(m.navStack, *m.pendingNav)
			m.pendingNav = nil
		}

		// For festivals drilled into: phases become tree roots (expandable)
		depth := 0
		m.roots = itemsToTreeNodes(msg.items, depth, nil)
		m.rebuildVisible()
		m.cursor = 0
		m.scrollStart = 0
		m.breadcrumbs = append(m.breadcrumbs, msg.breadcrumb)
		return m, m.loadPreviewCmd()

	case childrenLoadedMsg:
		// This is for tree expand/collapse within a festival
		if msg.err != nil {
			if node := findNode(m.roots, msg.parentPath); node != nil {
				node.Loading = false
			}
			return m, nil
		}

		parent := findNode(m.roots, msg.parentPath)
		if parent == nil {
			return m, nil
		}

		parent.Loading = false
		parent.Loaded = true

		if len(msg.items) == 0 {
			parent.Expanded = false
			m.rebuildVisible()
			return m, nil
		}

		childDepth := parent.Depth + 1
		parent.Children = itemsToTreeNodes(msg.items, childDepth, parent)
		parent.Expanded = true
		m.rebuildVisible()
		return m, m.loadPreviewCmd()

	case previewLoadedMsg:
		if msg.rendered == "" {
			m.viewport.SetContent(dimStyle().Render("No preview available"))
		} else {
			m.viewport.SetContent(msg.rendered)
		}
		m.viewport.GotoTop()
		return m, nil

	case refreshMsg:
		return m, tea.Batch(m.refreshCurrentView(), watchCmd(m.ctx))

	case refreshItemsMsg:
		m.roots = itemsToTreeNodes(msg.items, 0, nil)
		m.rebuildVisible()
		if m.cursor >= len(m.visible) {
			m.cursor = max(len(m.visible)-1, 0)
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

// navigateDown drills into the selected item (status→festivals, festival→phases).
// Used for status and festival nodes. Phase/sequence use tree expand/collapse instead.
func (m Model) navigateDown() (tea.Model, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return m, nil
	}

	node := m.visible[m.cursor]
	item := node.Item

	// Tasks are leaf nodes — focus the preview pane
	if item.Type == ItemTask {
		m.focusPreview = true
		return m, nil
	}

	// Phase/Sequence inside a festival → tree expand/collapse
	if item.Type == ItemPhase || item.Type == ItemSequence {
		return m.toggleExpand()
	}

	// Status/Festival → drilldown (push nav stack, replace roots)
	m.pendingNav = &navEntry{
		roots:  m.roots,
		title:  m.currentTitle(),
		cursor: m.cursor,
		scroll: m.scrollStart,
	}
	m.loading = true
	ctx := m.ctx

	if item.Type == ItemStatus {
		status := item.Status
		return m, func() tea.Msg {
			result := loadFestivalsForStatus(ctx, status)
			// Convert childrenLoadedMsg to drilldownLoadedMsg
			if clm, ok := result.(childrenLoadedMsg); ok {
				return drilldownLoadedMsg{
					items:      clm.items,
					breadcrumb: clm.breadcrumb,
					err:        clm.err,
				}
			}
			return result
		}
	}

	// Festival → load phases as tree roots
	return m, func() tea.Msg {
		children, err := loadChildren(ctx, item)
		return drilldownLoadedMsg{
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
	m.roots = entry.roots
	m.cursor = entry.cursor
	m.scrollStart = entry.scroll
	m.rebuildVisible()

	if len(m.breadcrumbs) > 0 {
		m.breadcrumbs = m.breadcrumbs[:len(m.breadcrumbs)-1]
	}

	return m
}

func (m Model) cancel() (tea.Model, tea.Cmd) {
	m.quitting = true
	m.selected = false
	return m, tea.Quit
}

func (m Model) canSelectCurrent() bool {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return false
	}

	item := m.visible[m.cursor].Item
	return item.Type != ItemStatus && item.Path != ""
}

func (m Model) selectCurrent() (tea.Model, tea.Cmd) {
	if !m.canSelectCurrent() {
		return m, nil
	}

	m.quitting = true
	m.selected = true
	return m, tea.Quit
}

// toggleExpand expands or collapses a tree node (used for phase/sequence inside a festival).
func (m Model) toggleExpand() (tea.Model, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return m, nil
	}

	node := m.visible[m.cursor]

	// Tasks are leaf nodes — focus the preview pane
	if node.IsLeaf() {
		m.focusPreview = true
		return m, nil
	}

	// Already expanded — collapse
	if node.Expanded {
		node.Expanded = false
		m.rebuildVisible()
		return m, m.loadPreviewCmd()
	}

	// Already loaded — just expand
	if node.Loaded {
		node.Expanded = true
		m.rebuildVisible()
		return m, m.loadPreviewCmd()
	}

	// Need to load children asynchronously
	node.Loading = true
	m.rebuildVisible()
	ctx := m.ctx
	item := node.Item
	parentPath := node.NodeID()

	return m, func() tea.Msg {
		children, err := loadChildren(ctx, item)
		return childrenLoadedMsg{
			items:      children,
			parentPath: parentPath,
			breadcrumb: item.Name,
			err:        err,
		}
	}
}

// rebuildVisible flattens the tree into the visible slice, preserving cursor by NodeID.
func (m *Model) rebuildVisible() {
	var cursorNodeID string
	if m.cursor >= 0 && m.cursor < len(m.visible) {
		cursorNodeID = m.visible[m.cursor].NodeID()
	}

	m.visible = flattenTree(m.roots)

	// Restore cursor position by NodeID
	if cursorNodeID != "" {
		for i, node := range m.visible {
			if node.NodeID() == cursorNodeID {
				m.cursor = i
				m.ensureVisible()
				return
			}
		}
	}

	// Clamp cursor
	if m.cursor >= len(m.visible) {
		m.cursor = max(len(m.visible)-1, 0)
	}
	m.ensureVisible()
}

func (m *Model) ensureVisible() {
	if m.cursor < m.scrollStart {
		m.scrollStart = m.cursor
	}
	if m.cursor >= m.scrollStart+m.maxVisible {
		m.scrollStart = m.cursor - m.maxVisible + 1
	}
}

// loadPreviewCmd returns a tea.Cmd that loads and renders the preview asynchronously.
func (m Model) loadPreviewCmd() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return func() tea.Msg { return previewLoadedMsg{} }
	}
	path := goalFileForItem(m.visible[m.cursor].Item)
	width := m.previewWidth()
	renderer := m.mdRenderer
	return func() tea.Msg {
		raw := loadPreview(path)
		if raw == "" {
			return previewLoadedMsg{}
		}
		rendered := renderer.render(raw, width)
		return previewLoadedMsg{rendered: rendered}
	}
}

func (m Model) previewWidth() int {
	if m.width < 80 {
		return m.width
	}
	return m.width - m.treeWidth() - 4 // borders + padding
}

func (m Model) treeWidth() int {
	return min(max(m.width*30/100, 25), 50)
}

func (m *Model) syncViewportSize() {
	pw := m.previewWidth()
	ph := max(m.height-4, 5)
	m.viewport.Width = pw
	m.viewport.Height = ph
}

func (m Model) currentTitle() string {
	if len(m.breadcrumbs) > 0 {
		return m.breadcrumbs[len(m.breadcrumbs)-1]
	}
	title := "Festivals"
	if m.status != "" {
		title += " (" + m.status + ")"
	}
	return title
}

// inTreeMode returns true when we're inside a festival hierarchy (phases/sequences/tasks).
func (m Model) inTreeMode() bool {
	if len(m.navStack) == 0 {
		return false
	}
	// If we have navStack entries and the visible items are phases/sequences/tasks, we're in tree mode
	if len(m.visible) > 0 {
		t := m.visible[0].Item.Type
		return t == ItemPhase || t == ItemSequence || t == ItemTask
	}
	return false
}

// SelectedItem returns the currently selected festival item, or nil.
func (m Model) SelectedItem() *FestivalItem {
	if m.quitting && m.selected && m.canSelectCurrent() {
		return &m.visible[m.cursor].Item
	}
	return nil
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
func detectStatusFromPath(festivalPath string) string {
	parent := filepath.Base(filepath.Dir(festivalPath))
	grandparent := filepath.Base(filepath.Dir(filepath.Dir(festivalPath)))
	if workspace.IsDungeonDirName(grandparent) {
		return "dungeon/" + parent
	}
	return parent
}
