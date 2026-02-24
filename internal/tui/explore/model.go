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

const (
	ggTimeout       = 500 * time.Millisecond
	previewDebounce = 80 * time.Millisecond
)

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

// debouncePreviewMsg fires after debounce delay to load preview for a specific node.
type debouncePreviewMsg struct {
	nodePath string
}

// Model is the BubbleTea model for the festival explorer.
type Model struct {
	ctx    context.Context
	roots  []*TreeNode // Top-level tree nodes
	visible []*TreeNode // Flattened visible nodes (expanded descendants)
	cursor  int         // Index into visible slice
	width   int
	height  int
	maxVisible  int
	scrollStart int
	status      string
	loading     bool
	err         error
	quitting    bool
	festivalPath string // Auto-expand to this festival on load

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

		// Convert items to tree nodes
		depth := 0
		if m.status != "" {
			depth = 1 // festival nodes under a specific status
		}
		m.roots = itemsToTreeNodes(msg.items, depth, nil)
		m.rebuildVisible()

		// Only quit on empty items for specific-status views, not the overview
		if len(m.visible) == 0 && m.status != "" {
			return m, tea.Quit
		}
		if len(m.visible) == 0 {
			return m, nil
		}

		// Auto-expand into a specific festival if requested
		if m.festivalPath != "" {
			m.autoExpandToFestival()
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

	case childrenLoadedMsg:
		if msg.err != nil {
			// Find the loading node and clear its state
			if node := findNode(m.roots, msg.parentPath); node != nil {
				node.Loading = false
			}
			return m, nil
		}

		// Find parent node and set children
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
		return m, m.debouncedPreview()

	case previewLoadedMsg:
		if msg.rendered == "" {
			m.viewport.SetContent(dimStyle.Render("No preview available"))
		} else {
			m.viewport.SetContent(msg.rendered)
		}
		m.viewport.GotoTop()
		return m, nil

	case debouncePreviewMsg:
		// Only load preview if cursor is still on the same node
		if m.cursor >= 0 && m.cursor < len(m.visible) && m.visible[m.cursor].NodeID() == msg.nodePath {
			return m, m.loadPreviewCmd()
		}
		return m, nil

	case refreshMsg:
		return m, tea.Batch(m.refreshCurrentView(), watchCmd(m.ctx))

	case refreshItemsMsg:
		// Rebuild root nodes from refreshed items
		depth := 0
		if m.status != "" {
			depth = 1
		}
		m.roots = itemsToTreeNodes(msg.items, depth, nil)
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

// toggleExpand expands or collapses the currently selected tree node.
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
		return m, m.debouncedPreview()
	}

	// Already loaded — just expand
	if node.Loaded {
		node.Expanded = true
		m.rebuildVisible()
		return m, m.debouncedPreview()
	}

	// Need to load children asynchronously
	node.Loading = true
	m.rebuildVisible()
	ctx := m.ctx

	// Status items: lazy-load festivals for that status
	if node.Item.Type == ItemStatus {
		parentPath := node.Item.Status
		return m, func() tea.Msg {
			return loadFestivalsForStatus(ctx, parentPath)
		}
	}

	// Other hierarchy items: load children
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

// autoExpandToFestival expands the tree to show the specified festival's children.
func (m *Model) autoExpandToFestival() {
	festPath := m.festivalPath
	m.festivalPath = "" // Clear to prevent re-navigation

	// If we loaded at a specific status, the roots are festival nodes
	if m.status != "" {
		for i, node := range m.roots {
			if node.Item.Path == festPath {
				m.cursor = i
				m.rebuildVisible()
				return
			}
		}
		return
	}

	// At status overview, find and expand the status node, then the festival
	statusDir := detectStatusFromPath(festPath)
	for _, statusNode := range m.roots {
		if statusNode.Item.Status == statusDir {
			// Expand status node — children will load async
			// We can't auto-expand further until children arrive
			statusNode.Loading = true
			m.rebuildVisible()
			// Store festivalPath for second pass after children load
			m.festivalPath = festPath
			return
		}
	}
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

// debouncedPreview returns a tea.Cmd that fires after a short delay for debounced preview loading.
func (m Model) debouncedPreview() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return nil
	}
	nodePath := m.visible[m.cursor].NodeID()
	return tea.Tick(previewDebounce, func(time.Time) tea.Msg {
		return debouncePreviewMsg{nodePath: nodePath}
	})
}

func (m Model) previewWidth() int {
	if m.width < 80 {
		return m.width
	}
	return m.width - m.treeWidth() - 4 // borders + padding
}

func (m Model) treeWidth() int {
	w := min(max(m.width*30/100, 25), 50)
	return w
}

func (m *Model) syncViewportSize() {
	pw := m.previewWidth()
	ph := max(m.height-4, 5)
	m.viewport.Width = pw
	m.viewport.Height = ph
}

func (m Model) currentTitle() string {
	title := "Festivals"
	if m.status != "" {
		title += " (" + m.status + ")"
	}
	return title
}

// SelectedItem returns the currently selected festival item, or nil.
func (m Model) SelectedItem() *FestivalItem {
	if m.quitting && m.cursor >= 0 && m.cursor < len(m.visible) {
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
