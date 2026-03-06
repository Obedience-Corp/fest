package explore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/id"
	tea "github.com/charmbracelet/bubbletea"
)

// setupTestFestivals creates a temporary festival hierarchy for testing.
func setupTestFestivals(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	festivalsDir := filepath.Join(root, "festivals")

	// Create active festival with full hierarchy
	activeFest := filepath.Join(festivalsDir, "active", "test-festival-TF0001")
	createTestFestival(t, activeFest, "Test Festival Goal", 2, 2, 3)

	// Create planning festival (minimal)
	planningFest := filepath.Join(festivalsDir, "planning", "planning-fest-PF0001")
	createTestFestival(t, planningFest, "Planning Festival Goal", 1, 1, 1)

	// Create empty festival (no phases)
	emptyFest := filepath.Join(festivalsDir, "active", "empty-fest-EF0001")
	if err := os.MkdirAll(emptyFest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(emptyFest, "FESTIVAL_GOAL.md"), []byte("# Empty Festival\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create completed festival
	completedFest := filepath.Join(festivalsDir, "dungeon", "completed", "done-fest-DF0001")
	createTestFestival(t, completedFest, "Completed Festival Goal", 1, 1, 2)

	return festivalsDir
}

// createTestFestival creates a festival with phases, sequences, and tasks.
func createTestFestival(t *testing.T, festivalDir, goal string, numPhases, numSequences, numTasks int) {
	t.Helper()
	if err := os.MkdirAll(festivalDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalDir, "FESTIVAL_GOAL.md"),
		[]byte("# Festival Goal\n\n"+goal+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	for p := 1; p <= numPhases; p++ {
		phaseName := padNum(p) + "_PHASE_" + strings.ToUpper(itoa(p))
		phaseDir := filepath.Join(festivalDir, phaseName)
		if err := os.MkdirAll(phaseDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"),
			[]byte("# Phase Goal\n\nPhase "+itoa(p)+" goal\n"), 0644); err != nil {
			t.Fatal(err)
		}

		for s := 1; s <= numSequences; s++ {
			seqName := padNum(s) + "_sequence_" + itoa(s)
			seqDir := filepath.Join(phaseDir, seqName)
			if err := os.MkdirAll(seqDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"),
				[]byte("# Sequence Goal\n\nSequence "+itoa(s)+" goal\n"), 0644); err != nil {
				t.Fatal(err)
			}

			for tk := 1; tk <= numTasks; tk++ {
				taskName := padNum(tk) + "_task_" + itoa(tk) + ".md"
				if err := os.WriteFile(filepath.Join(seqDir, taskName),
					[]byte("---\nfest_type: task\nfest_status: pending\nfest_tracking: true\n---\n\n# Task "+itoa(tk)+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func padNum(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

// --- Tree data structure tests ---

func TestFlattenTree(t *testing.T) {
	root := &TreeNode{Item: FestivalItem{Name: "root", Path: "/root"}, Expanded: true}
	child1 := &TreeNode{Item: FestivalItem{Name: "child1", Path: "/child1"}, Parent: root, Depth: 1}
	child2 := &TreeNode{Item: FestivalItem{Name: "child2", Path: "/child2"}, Parent: root, Depth: 1, Expanded: true}
	grandchild := &TreeNode{Item: FestivalItem{Name: "gc", Path: "/gc"}, Parent: child2, Depth: 2}
	root.Children = []*TreeNode{child1, child2}
	child2.Children = []*TreeNode{grandchild}

	visible := flattenTree([]*TreeNode{root})
	if len(visible) != 4 {
		t.Fatalf("expected 4 visible nodes, got %d", len(visible))
	}
	if visible[0].Item.Name != "root" {
		t.Errorf("expected root first, got %s", visible[0].Item.Name)
	}
	if visible[3].Item.Name != "gc" {
		t.Errorf("expected grandchild last, got %s", visible[3].Item.Name)
	}
}

func TestFlattenTreeCollapsed(t *testing.T) {
	root := &TreeNode{Item: FestivalItem{Name: "root", Path: "/root"}, Expanded: false}
	child := &TreeNode{Item: FestivalItem{Name: "child", Path: "/child"}, Parent: root, Depth: 1}
	root.Children = []*TreeNode{child}

	visible := flattenTree([]*TreeNode{root})
	if len(visible) != 1 {
		t.Fatalf("collapsed root should only show root, got %d nodes", len(visible))
	}
}

func TestFindNode(t *testing.T) {
	root := &TreeNode{Item: FestivalItem{Name: "root", Path: "/root"}, Expanded: true}
	child := &TreeNode{Item: FestivalItem{Name: "child", Path: "/child"}, Parent: root, Depth: 1}
	root.Children = []*TreeNode{child}

	found := findNode([]*TreeNode{root}, "/child")
	if found == nil {
		t.Fatal("expected to find child node")
	}
	if found.Item.Name != "child" {
		t.Errorf("expected child, got %s", found.Item.Name)
	}

	notFound := findNode([]*TreeNode{root}, "/missing")
	if notFound != nil {
		t.Error("expected nil for missing path")
	}
}

func TestItemsToTreeNodes(t *testing.T) {
	items := []FestivalItem{
		{Name: "a", Path: "/a", Type: ItemFestival},
		{Name: "b", Path: "/b", Type: ItemFestival},
	}
	parent := &TreeNode{Item: FestivalItem{Name: "parent"}}
	nodes := itemsToTreeNodes(items, 1, parent)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Depth != 1 {
		t.Errorf("expected depth 1, got %d", nodes[0].Depth)
	}
	if nodes[0].Parent != parent {
		t.Error("expected parent to be set")
	}
}

func TestItemsToTreeNodesNil(t *testing.T) {
	nodes := itemsToTreeNodes(nil, 0, nil)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes from nil input, got %d", len(nodes))
	}
}

func TestBreadcrumbsFromNode(t *testing.T) {
	root := &TreeNode{Item: FestivalItem{Name: "Active"}}
	child := &TreeNode{Item: FestivalItem{Name: "my-fest"}, Parent: root}
	grandchild := &TreeNode{Item: FestivalItem{Name: "001_DESIGN"}, Parent: child}

	crumbs := breadcrumbsFromNode(grandchild)
	if len(crumbs) != 3 {
		t.Fatalf("expected 3 breadcrumbs, got %d", len(crumbs))
	}
	if crumbs[0] != "Active" || crumbs[1] != "my-fest" || crumbs[2] != "001_DESIGN" {
		t.Errorf("unexpected breadcrumbs: %v", crumbs)
	}
}

func TestTreeNodeIsLeaf(t *testing.T) {
	task := &TreeNode{Item: FestivalItem{Type: ItemTask}}
	if !task.IsLeaf() {
		t.Error("task should be a leaf")
	}

	fest := &TreeNode{Item: FestivalItem{Type: ItemFestival}}
	if fest.IsLeaf() {
		t.Error("festival should not be a leaf")
	}
}

// --- Model tests ---

func TestModelNew(t *testing.T) {
	ctx := context.Background()
	m := New(ctx, "active")

	if m.status != "active" {
		t.Errorf("expected status 'active', got %q", m.status)
	}
	if !m.loading {
		t.Error("expected loading=true for new model")
	}
	if m.maxVisible != 20 {
		t.Errorf("expected maxVisible=20, got %d", m.maxVisible)
	}
	if m.mdRenderer == nil {
		t.Error("expected mdRenderer to be initialized")
	}
}

func TestModelNewAllStatuses(t *testing.T) {
	ctx := context.Background()
	m := New(ctx, "")

	if m.status != "" {
		t.Errorf("expected empty status, got %q", m.status)
	}
}

func TestFestivalsLoadedMsg(t *testing.T) {
	ctx := context.Background()
	m := New(ctx, "active")

	items := []FestivalItem{
		{Name: "fest-1", Status: "active", Progress: 50.0, Type: ItemFestival, Path: "/tmp/fest-1"},
		{Name: "fest-2", Status: "active", Progress: 75.0, Type: ItemFestival, Path: "/tmp/fest-2"},
	}

	msg := festivalsLoadedMsg{items: items}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	if m.loading {
		t.Error("expected loading=false after festivalsLoadedMsg")
	}
	if len(m.visible) != 2 {
		t.Errorf("expected 2 visible items, got %d", len(m.visible))
	}
	if len(m.roots) != 2 {
		t.Errorf("expected 2 root nodes, got %d", len(m.roots))
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for async preview loading")
	}
}

func TestFestivalsLoadedMsgError(t *testing.T) {
	ctx := context.Background()
	m := New(ctx, "active")

	msg := festivalsLoadedMsg{err: context.Canceled}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	if m.err == nil {
		t.Error("expected error to be set")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestFestivalsLoadedMsgEmpty(t *testing.T) {
	ctx := context.Background()
	m := New(ctx, "active")

	msg := festivalsLoadedMsg{items: nil}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	if len(m.visible) != 0 {
		t.Errorf("expected 0 visible items, got %d", len(m.visible))
	}
	if cmd == nil {
		t.Error("expected tea.Quit command for empty list")
	}
}

func TestKeyNavigation(t *testing.T) {
	m := modelWithItems(3)

	// Down
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after down, got %d", m.cursor)
	}

	// Down again
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 after second down, got %d", m.cursor)
	}

	// Down at bottom (should stay)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 at bottom, got %d", m.cursor)
	}

	// Up
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after up, got %d", m.cursor)
	}
}

func TestVimNavigation(t *testing.T) {
	m := modelWithItems(5)

	// j = down
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", m.cursor)
	}

	// k = up
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after k, got %d", m.cursor)
	}

	// G = bottom
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = newModel.(Model)
	if m.cursor != 4 {
		t.Errorf("expected cursor=4 after G, got %d", m.cursor)
	}

	// gg = top (two g presses within timeout)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = newModel.(Model)
	if m.cursor != 4 {
		t.Errorf("expected cursor=4 after first g, got %d", m.cursor)
	}
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after gg, got %d", m.cursor)
	}
}

func TestNavigateDownTask(t *testing.T) {
	m := modelWithItems(1)
	m.roots[0].Item.Type = ItemTask
	m.visible[0].Item.Type = ItemTask

	// Enter on a task should focus the preview pane
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)
	if cmd != nil {
		t.Error("expected no command when entering a task")
	}
	if !m.focusPreview {
		t.Error("expected focusPreview=true when entering a task")
	}
}

// TestDrilldownFestival tests that Enter on a festival item triggers drilldown (not tree expand).
func TestDrilldownFestival(t *testing.T) {
	m := modelWithItems(3)

	// Enter on a festival item should trigger drilldown
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)
	if cmd == nil {
		t.Error("expected load command from drilldown")
	}
	if !m.loading {
		t.Error("expected model loading=true during drilldown")
	}
	if m.pendingNav == nil {
		t.Error("expected pendingNav to be set")
	}

	// Simulate drilldown loaded (phases of the festival)
	phases := []FestivalItem{
		{Name: "001_DESIGN", Type: ItemPhase, Path: "/tmp/phase-1"},
		{Name: "002_BUILD", Type: ItemPhase, Path: "/tmp/phase-2"},
	}
	newModel, _ = m.Update(drilldownLoadedMsg{
		items:      phases,
		breadcrumb: "fest-1",
	})
	m = newModel.(Model)

	if len(m.navStack) != 1 {
		t.Errorf("expected navStack length 1, got %d", len(m.navStack))
	}
	if len(m.visible) != 2 {
		t.Errorf("expected 2 visible phase items, got %d", len(m.visible))
	}
	if m.visible[0].Item.Type != ItemPhase {
		t.Errorf("expected phase items after drilldown, got %s", m.visible[0].Item.Type)
	}
	if len(m.breadcrumbs) != 1 || m.breadcrumbs[0] != "fest-1" {
		t.Errorf("expected breadcrumbs ['fest-1'], got %v", m.breadcrumbs)
	}
}

// TestPhaseExpandCollapse tests tree expand/collapse for phase items inside a festival.
func TestPhaseExpandCollapse(t *testing.T) {
	m := modelWithPhaseItems(3)

	// Enter on a phase should trigger async child load
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)
	if cmd == nil {
		t.Error("expected load command from phase expand")
	}
	if !m.roots[0].Loading {
		t.Error("expected node loading=true during expand")
	}

	// Simulate children loaded
	children := []FestivalItem{
		{Name: "01_sequence", Type: ItemSequence, Path: "/tmp/seq-1"},
		{Name: "02_sequence", Type: ItemSequence, Path: "/tmp/seq-2"},
	}
	newModel, _ = m.Update(childrenLoadedMsg{
		items:      children,
		parentPath: m.roots[0].NodeID(),
	})
	m = newModel.(Model)

	if !m.roots[0].Expanded {
		t.Error("expected node to be expanded after children loaded")
	}
	if len(m.roots[0].Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(m.roots[0].Children))
	}
	// Visible: 3 roots + 2 children = 5
	if len(m.visible) != 5 {
		t.Errorf("expected 5 visible items, got %d", len(m.visible))
	}

	// Collapse with Enter again (cursor on first root)
	m.cursor = 0
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)
	if m.roots[0].Expanded {
		t.Error("expected node to be collapsed after second Enter")
	}
	if len(m.visible) != 3 {
		t.Errorf("expected 3 visible items after collapse, got %d", len(m.visible))
	}
}

func TestExpandAlreadyLoaded(t *testing.T) {
	m := modelWithPhaseItems(1)
	node := m.roots[0]
	node.Loaded = true
	node.Children = []*TreeNode{
		{Item: FestivalItem{Name: "child", Path: "/child", Type: ItemSequence}, Depth: 1, Parent: node},
	}

	// Enter should expand without loading
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)
	if !m.roots[0].Expanded {
		t.Error("expected node to expand")
	}
	if len(m.visible) != 2 {
		t.Errorf("expected 2 visible items, got %d", len(m.visible))
	}
}

func TestHCollapseAndParent(t *testing.T) {
	m := modelWithPhaseItems(1)
	// Put in tree mode by adding navStack entry
	m.navStack = []navEntry{{roots: nil, title: "test"}}

	node := m.roots[0]
	node.Expanded = true
	node.Loaded = true
	child := &TreeNode{Item: FestivalItem{Name: "child", Path: "/child", Type: ItemSequence}, Depth: 1, Parent: node}
	node.Children = []*TreeNode{child}
	m.rebuildVisible()

	// Move cursor to child
	m.cursor = 1

	// h should move cursor to parent (since child is not expanded)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 (parent), got %d", m.cursor)
	}
}

func TestEscCollapseOrNavigateUp(t *testing.T) {
	m := modelWithPhaseItems(1)
	// Simulate being drilled into a festival (navStack has entry)
	m.navStack = []navEntry{{
		roots: itemsToTreeNodes([]FestivalItem{
			{Name: "fest-1", Type: ItemFestival, Path: "/tmp/fest-1"},
		}, 0, nil),
		title: "Active",
	}}
	m.breadcrumbs = []string{"Active"}

	node := m.roots[0]
	node.Expanded = true
	node.Loaded = true
	node.Children = []*TreeNode{
		{Item: FestivalItem{Name: "child", Path: "/child", Type: ItemSequence}, Depth: 1, Parent: node},
	}
	m.rebuildVisible()

	// Esc on expanded node should collapse
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if m.roots[0].Expanded {
		t.Error("expected node to collapse on esc")
	}
	if m.quitting {
		t.Error("should not quit when collapsing")
	}

	// Esc on collapsed node with no parent should navigate up
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if len(m.navStack) != 0 {
		t.Errorf("expected navStack to be empty after navigate up, got %d", len(m.navStack))
	}
	if m.quitting {
		t.Error("should not quit when navigating up")
	}
}

func TestEscQuitsAtRoot(t *testing.T) {
	m := modelWithItems(3)

	// Esc at root (no navStack, no expanded nodes) should quit
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if !m.quitting {
		t.Error("expected quit on esc at root level")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestBackspaceNavigatesUp(t *testing.T) {
	m := modelWithPhaseItems(2)
	// Simulate being drilled into a festival
	originalRoots := itemsToTreeNodes([]FestivalItem{
		{Name: "fest-1", Type: ItemFestival, Path: "/tmp/fest-1"},
	}, 0, nil)
	m.navStack = []navEntry{{roots: originalRoots, title: "Active", cursor: 0}}
	m.breadcrumbs = []string{"Active"}

	// Backspace should navigate up (pop navStack)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = newModel.(Model)
	if len(m.navStack) != 0 {
		t.Errorf("expected navStack empty after backspace, got %d", len(m.navStack))
	}
	if m.quitting {
		t.Error("should not quit when navigating up")
	}
}

func TestBackspaceQuitAtRoot(t *testing.T) {
	m := modelWithItems(3)

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = newModel.(Model)
	if !m.quitting {
		t.Error("expected quitting after backspace at top level")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestSearchFilter(t *testing.T) {
	m := modelWithItems(5)

	// Enter search mode
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newModel.(Model)
	if !m.filtering {
		t.Error("expected filtering=true after /")
	}
	if cmd == nil {
		t.Error("expected blink command when entering filter mode")
	}
	if len(m.allRoots) != 5 {
		t.Errorf("expected allRoots to be saved, got %d", len(m.allRoots))
	}

	// Cancel with Escape
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if m.filtering {
		t.Error("expected filtering=false after Escape")
	}
	if len(m.visible) != 5 {
		t.Errorf("expected all 5 visible items restored, got %d", len(m.visible))
	}
}

func TestSearchFilterConfirm(t *testing.T) {
	m := modelWithItems(5)

	// Enter search mode
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newModel.(Model)

	// Confirm with Enter (keeps whatever filter is active)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)
	if m.filtering {
		t.Error("expected filtering=false after Enter")
	}
}

func TestQuit(t *testing.T) {
	m := modelWithItems(3)

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = newModel.(Model)
	if !m.quitting {
		t.Error("expected quitting after q")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestCtrlCQuit(t *testing.T) {
	m := modelWithItems(3)

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = newModel.(Model)
	if !m.quitting {
		t.Error("expected quitting after ctrl+c")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestSelectedItem(t *testing.T) {
	m := modelWithItems(3)
	m.cursor = 1
	m.quitting = true

	item := m.SelectedItem()
	if item == nil {
		t.Fatal("expected selected item")
	}
	if item.Name != "fest-2" {
		t.Errorf("expected fest-2, got %s", item.Name)
	}
}

func TestSelectedItemNotQuitting(t *testing.T) {
	m := modelWithItems(3)
	m.cursor = 1
	m.quitting = false

	item := m.SelectedItem()
	if item != nil {
		t.Error("expected nil when not quitting")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := modelWithItems(3)

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newModel.(Model)

	if m.width != 120 {
		t.Errorf("expected width=120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height=40, got %d", m.height)
	}
	if m.maxVisible != 35 { // 40 - 5
		t.Errorf("expected maxVisible=35, got %d", m.maxVisible)
	}
}

func TestWindowSizeMsgSmall(t *testing.T) {
	m := modelWithItems(3)

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 8})
	m = newModel.(Model)

	if m.maxVisible != 5 { // max(8-5, 5) = max(3, 5) = 5
		t.Errorf("expected maxVisible=5 (minimum), got %d", m.maxVisible)
	}
}

func TestViewLoading(t *testing.T) {
	m := New(context.Background(), "active")
	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Error("expected loading message in view")
	}
}

func TestViewError(t *testing.T) {
	m := New(context.Background(), "active")
	m.loading = false
	m.err = context.Canceled
	view := m.View()
	if !strings.Contains(view, "Error") {
		t.Error("expected error message in view")
	}
}

func TestViewEmpty(t *testing.T) {
	m := New(context.Background(), "active")
	m.loading = false
	view := m.View()
	if !strings.Contains(view, "No festivals found") {
		t.Error("expected 'No festivals found' in view")
	}
}

func TestViewNarrowTerminal(t *testing.T) {
	m := modelWithItems(3)
	m.width = 60 // Less than 80
	m.height = 20

	view := m.View()
	// Should render list only (no preview pane border split)
	if strings.Contains(view, "Festival Goal") && strings.Contains(view, "Preview") {
		t.Error("narrow terminal should not show preview pane")
	}
}

func TestViewBreadcrumbsWithNavStack(t *testing.T) {
	m := modelWithPhaseItems(2)
	m.width = 120
	m.height = 30
	m.breadcrumbs = []string{"Active", "my-fest"}
	m.navStack = []navEntry{
		{roots: nil, title: "root"},
		{roots: nil, title: "Active"},
	}

	view := m.View()
	if !strings.Contains(view, "Active") {
		t.Error("expected breadcrumb 'Active' in view")
	}
}

func TestRenderBreadcrumbSingle(t *testing.T) {
	m := New(context.Background(), "active")
	m.loading = false
	crumb := m.renderBreadcrumb()
	if !strings.Contains(crumb, "Festivals") {
		t.Error("expected 'Festivals' in breadcrumb")
	}
}

func TestEnsureVisible(t *testing.T) {
	m := modelWithItems(20)
	m.maxVisible = 5

	// Select item beyond viewport
	m.cursor = 10
	m.ensureVisible()
	if m.scrollStart > 10 {
		t.Errorf("scrollStart should be <= cursor, got scrollStart=%d cursor=%d", m.scrollStart, m.cursor)
	}
	if m.cursor >= m.scrollStart+m.maxVisible {
		t.Error("cursor should be within visible range")
	}

	// Select item above viewport
	m.cursor = 0
	m.ensureVisible()
	if m.scrollStart != 0 {
		t.Errorf("expected scrollStart=0, got %d", m.scrollStart)
	}
}

func TestGGTimeout(t *testing.T) {
	m := modelWithItems(5)
	m.cursor = 3

	// First g
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = newModel.(Model)
	if m.cursor != 3 {
		t.Errorf("first g should not move, got cursor=%d", m.cursor)
	}
	if m.lastGTime.IsZero() {
		t.Error("lastGTime should be set after first g")
	}

	// Simulate timeout by setting lastGTime in the past
	m.lastGTime = time.Now().Add(-time.Second)

	// Second g after timeout
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = newModel.(Model)
	// Should record new time, not jump to top.
	// The g after timeout sets a new time without moving to top.
	if m.cursor == 0 {
		t.Error("second g after timeout should not jump to top")
	}
	if m.lastGTime.IsZero() {
		t.Error("lastGTime should be set after second g (new pending gg)")
	}
}

func TestElementsToItems(t *testing.T) {
	items := elementsToItems(nil, ItemPhase)
	if len(items) != 0 {
		t.Errorf("expected 0 items from nil input, got %d", len(items))
	}
}

func TestGoalFileForItem(t *testing.T) {
	tmp := t.TempDir()

	// Test festival goal file detection
	festDir := filepath.Join(tmp, "festival")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatal(err)
	}
	goalFile := filepath.Join(festDir, "FESTIVAL_GOAL.md")
	if err := os.WriteFile(goalFile, []byte("# Goal\n"), 0644); err != nil {
		t.Fatal(err)
	}

	item := FestivalItem{Type: ItemFestival, Path: festDir}
	result := goalFileForItem(item)
	if result != goalFile {
		t.Errorf("expected %s, got %s", goalFile, result)
	}

	// Test phase goal file
	phaseDir := filepath.Join(tmp, "phase")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}
	phaseGoal := filepath.Join(phaseDir, "PHASE_GOAL.md")
	if err := os.WriteFile(phaseGoal, []byte("# Phase\n"), 0644); err != nil {
		t.Fatal(err)
	}

	item = FestivalItem{Type: ItemPhase, Path: phaseDir}
	result = goalFileForItem(item)
	if result != phaseGoal {
		t.Errorf("expected %s, got %s", phaseGoal, result)
	}

	// Test task (returns item path)
	taskPath := filepath.Join(tmp, "task.md")
	if err := os.WriteFile(taskPath, []byte("# Task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	item = FestivalItem{Type: ItemTask, Path: taskPath}
	result = goalFileForItem(item)
	if result != taskPath {
		t.Errorf("expected %s, got %s", taskPath, result)
	}
}

func TestLoadPreview(t *testing.T) {
	tmp := t.TempDir()

	// Test normal file
	testFile := filepath.Join(tmp, "test.md")
	if err := os.WriteFile(testFile, []byte("# Title\n\nSome content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	preview := loadPreview(testFile)
	if !strings.Contains(preview, "Title") {
		t.Error("expected title in preview")
	}

	// Test missing file
	preview = loadPreview(filepath.Join(tmp, "missing.md"))
	if preview != "" {
		t.Errorf("expected empty string for missing file, got %q", preview)
	}

	// Test empty path
	preview = loadPreview("")
	if preview != "" {
		t.Errorf("expected empty string for empty path, got %q", preview)
	}
}

func TestMdRendererCachesInstance(t *testing.T) {
	r := &mdRenderer{}

	content := "# Hello\n\nWorld\n"
	result := r.render(content, 80)
	if !strings.Contains(result, "Hello") {
		t.Error("expected rendered content to contain 'Hello'")
	}

	// Second call at same width should reuse renderer
	result2 := r.render(content, 80)
	if !strings.Contains(result2, "Hello") {
		t.Error("expected cached render to contain 'Hello'")
	}

	// Different width should recreate
	result3 := r.render(content, 40)
	if !strings.Contains(result3, "Hello") {
		t.Error("expected re-rendered content to contain 'Hello'")
	}
}

func TestTabTogglesFocus(t *testing.T) {
	m := modelWithItems(3)
	if m.focusPreview {
		t.Error("expected focusPreview=false initially")
	}

	// Tab to focus preview
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newModel.(Model)
	if !m.focusPreview {
		t.Error("expected focusPreview=true after Tab")
	}

	// Tab back to tree
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newModel.(Model)
	if m.focusPreview {
		t.Error("expected focusPreview=false after second Tab")
	}
}

func TestPreviewKeyEscBackToTree(t *testing.T) {
	m := modelWithItems(3)
	m.focusPreview = true

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if m.focusPreview {
		t.Error("expected focusPreview=false after Esc in preview mode")
	}
}

func TestLoadGenericChildren(t *testing.T) {
	tmp := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmp, "input_specs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "decisions"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".hidden"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("# README\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "PHASE_GOAL.md"), []byte("# Goal\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "notes.txt"), []byte("notes"), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := loadGenericChildren(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := make(map[string]bool)
	for _, item := range items {
		names[item.Name] = true
	}

	if !names["input_specs"] {
		t.Error("expected input_specs directory")
	}
	if !names["decisions"] {
		t.Error("expected decisions directory")
	}
	if !names["README.md"] {
		t.Error("expected README.md file")
	}
	if names[".hidden"] {
		t.Error("should not include hidden directories")
	}
	if names["PHASE_GOAL.md"] {
		t.Error("should not include goal files")
	}
	if names["notes.txt"] {
		t.Error("should not include non-markdown files")
	}
}

func TestChildrenLoadedMsgSetsChildren(t *testing.T) {
	m := modelWithPhaseItems(3)
	m.roots[0].Loading = true

	children := []FestivalItem{
		{Name: "seq-1", Type: ItemSequence, Path: "/tmp/seq-1"},
		{Name: "seq-2", Type: ItemSequence, Path: "/tmp/seq-2"},
	}

	parentPath := m.roots[0].NodeID()
	newModel, _ := m.Update(childrenLoadedMsg{items: children, parentPath: parentPath})
	m = newModel.(Model)

	if !m.roots[0].Expanded {
		t.Error("expected node expanded after children loaded")
	}
	if !m.roots[0].Loaded {
		t.Error("expected node marked as loaded")
	}
	if m.roots[0].Loading {
		t.Error("expected loading=false after load")
	}
	if len(m.roots[0].Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(m.roots[0].Children))
	}
}

func TestChildrenLoadedMsgEmpty(t *testing.T) {
	m := modelWithPhaseItems(3)
	parentPath := m.roots[0].NodeID()
	m.roots[0].Loading = true

	newModel, _ := m.Update(childrenLoadedMsg{items: nil, parentPath: parentPath})
	m = newModel.(Model)

	if m.roots[0].Expanded {
		t.Error("expected node collapsed for empty children")
	}
	if len(m.visible) != 3 {
		t.Errorf("expected 3 visible items, got %d", len(m.visible))
	}
}

func TestChildrenLoadedMsgError(t *testing.T) {
	m := modelWithPhaseItems(3)
	parentPath := m.roots[0].NodeID()
	m.roots[0].Loading = true

	newModel, _ := m.Update(childrenLoadedMsg{err: context.Canceled, parentPath: parentPath})
	m = newModel.(Model)

	if m.roots[0].Loading {
		t.Error("expected loading cleared after error")
	}
}

func TestFilterValue(t *testing.T) {
	item := FestivalItem{Name: "test-festival"}
	if item.FilterValue() != "test-festival" {
		t.Errorf("expected 'test-festival', got %q", item.FilterValue())
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m := New(ctx, "active")
	cmd := m.Init()

	if cmd == nil {
		t.Fatal("expected init command")
	}

	msg := cmd()
	batchMsg, isBatch := msg.(tea.BatchMsg)
	if !isBatch {
		loadedMsg, ok := msg.(festivalsLoadedMsg)
		if !ok {
			t.Fatalf("expected festivalsLoadedMsg, got %T", msg)
		}
		if loadedMsg.err == nil {
			t.Error("expected error from cancelled context")
		}
		return
	}

	var found bool
	for _, subCmd := range batchMsg {
		if subCmd == nil {
			continue
		}
		subMsg := subCmd()
		if loadedMsg, ok := subMsg.(festivalsLoadedMsg); ok {
			found = true
			if loadedMsg.err == nil {
				t.Error("expected error from cancelled context")
			}
			break
		}
	}
	if !found {
		t.Error("expected festivalsLoadedMsg in batch")
	}
}

func TestStatusStyle(t *testing.T) {
	tests := []struct {
		status string
	}{
		{"active"},
		{"planning"},
		{"dungeon/completed"},
		{"dungeon"},
		{"unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			style := StatusStyle(tt.status)
			_ = style.Render("test")
		})
	}
}

func TestStatusOverviewInit(t *testing.T) {
	ctx := context.Background()
	m := New(ctx, "")
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected init command")
	}

	items := buildStatusItems()
	if len(items) != len(id.StatusDirectories) {
		t.Fatalf("expected %d status items, got %d", len(id.StatusDirectories), len(items))
	}
	for _, item := range items {
		if item.Type != ItemStatus {
			t.Errorf("expected ItemStatus type, got %q", item.Type)
		}
		if item.Count != -1 {
			t.Errorf("expected Count=-1 (loading), got %d", item.Count)
		}
	}
}

func TestStatusCountsMsg(t *testing.T) {
	m := modelWithStatusItems()

	counts := map[string]int{
		"planning":          3,
		"active":            1,
		"dungeon/completed": 10,
	}

	newModel, _ := m.Update(statusCountsMsg{counts: counts})
	m = newModel.(Model)

	for _, node := range m.roots {
		if node.Item.Type != ItemStatus {
			continue
		}
		if expected, ok := counts[node.Item.Status]; ok {
			if node.Item.Count != expected {
				t.Errorf("status %q: expected count %d, got %d", node.Item.Status, expected, node.Item.Count)
			}
		}
	}
}

func TestStatusCountsMsgNil(t *testing.T) {
	m := modelWithStatusItems()

	newModel, _ := m.Update(statusCountsMsg{counts: nil})
	m = newModel.(Model)

	for _, node := range m.roots {
		if node.Item.Type == ItemStatus && node.Item.Count != -1 {
			t.Errorf("expected Count=-1, got %d for %q", node.Item.Count, node.Item.Status)
		}
	}
}

func TestNavigateDownStatus(t *testing.T) {
	m := modelWithStatusItems()

	m.cursor = 0
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if cmd == nil {
		t.Error("expected load command when drilling into status item")
	}
	if !m.loading {
		t.Error("expected model loading=true during status drilldown")
	}
	if m.pendingNav == nil {
		t.Error("expected pendingNav to be set for drilldown")
	}
}

func TestStatusOverviewDoesNotQuit(t *testing.T) {
	ctx := context.Background()
	m := New(ctx, "")

	msg := festivalsLoadedMsg{items: nil}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	if cmd != nil {
		t.Error("expected no quit command for empty status overview")
	}
	if len(m.visible) != 0 {
		t.Errorf("expected 0 visible items, got %d", len(m.visible))
	}
}

func TestStatusDisplayName(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"planning", "Planning"},
		{"active", "Active"},
		{"ready", "Ready"},
		{"ritual", "Ritual"},
		{"dungeon/completed", "Completed"},
		{"dungeon/archived", "Archived"},
		{"dungeon/someday", "Someday"},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := statusDisplayName(tt.status)
			if got != tt.expected {
				t.Errorf("statusDisplayName(%q) = %q, want %q", tt.status, got, tt.expected)
			}
		})
	}
}

func TestStatusItemRendering(t *testing.T) {
	m := modelWithStatusItems()
	m.roots[0].Item.Count = 5
	m.width = 120
	m.height = 30

	view := m.View()
	if !strings.Contains(view, "(5)") {
		t.Error("expected count '(5)' in status item render")
	}
}

func TestStatusItemRenderingLoading(t *testing.T) {
	m := modelWithStatusItems()
	m.width = 120
	m.height = 30

	view := m.View()
	if !strings.Contains(view, "...") {
		t.Error("expected '...' for loading count")
	}
}

func TestDetectStatusFromPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"active festival", "/festivals/active/my-fest", "active"},
		{"planning festival", "/festivals/planning/my-fest", "planning"},
		{"dungeon completed", "/festivals/dungeon/completed/my-fest", "dungeon/completed"},
		{"dungeon archived", "/festivals/dungeon/archived/my-fest", "dungeon/archived"},
		{"dungeon someday", "/festivals/dungeon/someday/my-fest", "dungeon/someday"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectStatusFromPath(tt.path)
			if got != tt.want {
				t.Errorf("detectStatusFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestTreeWidthCalculation(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		wantMin int
		wantMax int
	}{
		{"narrow", 60, 25, 25},
		{"medium", 120, 25, 50},
		{"wide", 200, 25, 50},
		{"very narrow", 20, 25, 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := modelWithItems(3)
			m.width = tt.width
			tw := m.treeWidth()
			if tw < tt.wantMin || tw > tt.wantMax {
				t.Errorf("treeWidth(%d) = %d, want [%d, %d]", tt.width, tw, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"no frontmatter", "# Hello\nWorld", "# Hello\nWorld"},
		{"with frontmatter", "---\nfoo: bar\n---\n# Hello", "# Hello"},
		{"unclosed frontmatter", "---\nfoo: bar\n# Hello", "---\nfoo: bar\n# Hello"},
		{"empty after frontmatter", "---\nfoo: bar\n---\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripFrontmatter(tt.content)
			if got != tt.want {
				t.Errorf("stripFrontmatter() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusStyleAllStatuses(t *testing.T) {
	statuses := []string{
		"active", "planning", "ready", "ritual",
		"completed", "dungeon/completed",
		"archived", "dungeon/archived",
		"someday", "dungeon/someday",
		"dungeon", "unknown",
	}
	for _, s := range statuses {
		t.Run(s, func(t *testing.T) {
			style := StatusStyle(s)
			_ = style.Render("test")
		})
	}
}

func TestHandlePreviewKeyScroll(t *testing.T) {
	m := modelWithItems(3)
	m.focusPreview = true
	m.width = 120
	m.height = 30
	m.syncViewportSize()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m2 := updated.(Model)
	if !m2.focusPreview {
		t.Error("preview should still be focused after j")
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m3 := updated.(Model)
	if !m3.quitting {
		t.Error("q should quit from preview mode")
	}
	_ = cmd
}

func TestRenderContentItemTypes(t *testing.T) {
	types := []struct {
		itemType  ItemType
		wantTitle string
	}{
		{ItemStatus, "Status"},
		{ItemFestival, "Festival Goal"},
		{ItemPhase, "Phase Goal"},
		{ItemSequence, "Sequence Goal"},
		{ItemTask, "Task"},
	}
	for _, tt := range types {
		t.Run(string(tt.itemType), func(t *testing.T) {
			m := New(context.Background(), "")
			m.loading = false
			m.width = 120
			m.height = 30
			m.roots = []*TreeNode{{Item: FestivalItem{Name: "test", Type: tt.itemType, Path: "/tmp/test"}}}
			m.rebuildVisible()
			m.syncViewportSize()
			content := m.renderContent(60)
			if !strings.Contains(content, tt.wantTitle) {
				t.Errorf("renderContent for %s should contain %q", tt.itemType, tt.wantTitle)
			}
		})
	}
}

func TestPreviewLoadedMsgSetsContent(t *testing.T) {
	m := modelWithItems(3)
	m.width = 120
	m.height = 30
	m.syncViewportSize()

	msg := previewLoadedMsg{rendered: "# Hello World\n\nSome content here."}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	content := m.viewport.View()
	if !strings.Contains(content, "Hello World") {
		t.Error("expected viewport content to contain rendered markdown")
	}
}

func TestPreviewLoadedMsgEmptyShowsFallback(t *testing.T) {
	m := modelWithItems(3)
	m.width = 120
	m.height = 30
	m.syncViewportSize()

	msg := previewLoadedMsg{rendered: ""}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	content := m.viewport.View()
	if !strings.Contains(content, "No preview available") {
		t.Error("expected 'No preview available' fallback in viewport")
	}
}

func TestLoadPreviewCmdOutOfBounds(t *testing.T) {
	m := modelWithItems(3)
	m.cursor = -1

	cmd := m.loadPreviewCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd even for out-of-bounds")
	}

	msg := cmd()
	loaded, ok := msg.(previewLoadedMsg)
	if !ok {
		t.Fatalf("expected previewLoadedMsg, got %T", msg)
	}
	if loaded.rendered != "" {
		t.Errorf("expected empty rendered for out-of-bounds, got %q", loaded.rendered)
	}
}

func TestCursorStabilityAfterExpand(t *testing.T) {
	m := modelWithPhaseItems(3)
	// Move cursor to item 1
	m.cursor = 1
	originalNodeID := m.visible[1].NodeID()

	// Expand item 0
	m.roots[0].Loaded = true
	m.roots[0].Children = []*TreeNode{
		{Item: FestivalItem{Name: "child", Path: "/child", Type: ItemSequence}, Depth: 1, Parent: m.roots[0]},
	}
	m.roots[0].Expanded = true
	m.rebuildVisible()

	// Cursor should have followed to its new position
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		t.Fatalf("cursor out of bounds: %d (visible: %d)", m.cursor, len(m.visible))
	}
	if m.visible[m.cursor].NodeID() != originalNodeID {
		t.Errorf("cursor should still be on %s, but is on %s", originalNodeID, m.visible[m.cursor].NodeID())
	}
}

func TestGOnEmptyList(t *testing.T) {
	m := modelWithItems(0)

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("G on empty list should be no-op, got cursor=%d", m.cursor)
	}
	if cmd != nil {
		t.Error("G on empty list should return nil cmd")
	}
}

func TestKAtTopBoundary(t *testing.T) {
	m := modelWithItems(3)
	m.cursor = 0

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("k at top should stay at 0, got %d", m.cursor)
	}
	if cmd != nil {
		t.Error("k at top boundary should return nil cmd")
	}
}

func TestJAtBottomBoundary(t *testing.T) {
	m := modelWithItems(3)
	m.cursor = 2

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newModel.(Model)
	if m.cursor != 2 {
		t.Errorf("j at bottom should stay at 2, got %d", m.cursor)
	}
	if cmd != nil {
		t.Error("j at bottom boundary should return nil cmd")
	}
}

func TestHAtTopLevelNoOp(t *testing.T) {
	m := modelWithItems(3)

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = newModel.(Model)

	if m.quitting {
		t.Error("h at top level should be no-op, not quit")
	}
	if cmd != nil {
		t.Error("h at top level should return nil cmd")
	}
}

func TestHNavigatesUpWithNavStack(t *testing.T) {
	m := modelWithItems(2)
	originalRoots := m.roots
	// Push to navStack (simulate drilldown)
	m.navStack = []navEntry{{roots: originalRoots, title: "root", cursor: 0}}
	m.breadcrumbs = []string{"Active"}
	// Replace roots with phase items
	m.roots = itemsToTreeNodes([]FestivalItem{
		{Name: "phase-1", Type: ItemPhase, Path: "/tmp/p1"},
	}, 0, nil)
	m.rebuildVisible()

	// h should navigate up since we're not in tree mode (phases at root but navStack check first)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = newModel.(Model)

	if len(m.navStack) != 0 {
		t.Errorf("expected navStack empty after h, got %d", len(m.navStack))
	}
}

func TestPreviewKeyHReturnsToTree(t *testing.T) {
	m := modelWithItems(3)
	m.focusPreview = true

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = newModel.(Model)
	if m.focusPreview {
		t.Error("h should return focus to tree from preview mode")
	}
}

func TestPreviewKeyTabReturnsToTree(t *testing.T) {
	m := modelWithItems(3)
	m.focusPreview = true

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newModel.(Model)
	if m.focusPreview {
		t.Error("Tab should return focus to tree from preview mode")
	}
}

func TestPreviewKeyCtrlCQuits(t *testing.T) {
	m := modelWithItems(3)
	m.focusPreview = true

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = newModel.(Model)
	if !m.quitting {
		t.Error("ctrl+c should quit from preview mode")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestFilterTypingFiltersItems(t *testing.T) {
	m := modelWithItems(5)

	// Enter filter mode
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newModel.(Model)
	if !m.filtering {
		t.Fatal("expected filtering mode")
	}

	// Type "3" to filter
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = newModel.(Model)

	if len(m.visible) != 1 {
		t.Errorf("expected 1 filtered item, got %d", len(m.visible))
	}
	if len(m.visible) > 0 && m.visible[0].Item.Name != "fest-3" {
		t.Errorf("expected fest-3, got %s", m.visible[0].Item.Name)
	}
}

func TestFilterTypingNoMatch(t *testing.T) {
	m := modelWithItems(3)

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newModel.(Model)

	for _, r := range "xyz" {
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newModel.(Model)
	}

	if len(m.visible) != 0 {
		t.Errorf("expected 0 items for non-matching filter, got %d", len(m.visible))
	}
}

func TestFilterKeyEsc(t *testing.T) {
	m := modelWithItems(5)
	m.filtering = true
	m.allRoots = m.roots

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(Model)
	if m2.filtering {
		t.Error("esc should exit filter mode")
	}
	if len(m2.visible) != 5 {
		t.Errorf("esc should restore all items, got %d", len(m2.visible))
	}
}

func TestFilterKeyEnter(t *testing.T) {
	m := modelWithItems(5)
	m.filtering = true
	m.allRoots = m.roots
	// Simulate having filtered to 2 roots
	m.roots = m.roots[:2]
	m.rebuildVisible()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	if m2.filtering {
		t.Error("enter should exit filter mode")
	}
	if len(m2.visible) != 2 {
		t.Errorf("enter should keep filtered items, got %d", len(m2.visible))
	}
}

func TestBuildStatusItems(t *testing.T) {
	items := buildStatusItems()
	if len(items) != len(id.StatusDirectories) {
		t.Errorf("buildStatusItems() produced %d items, want %d", len(items), len(id.StatusDirectories))
	}
	for _, item := range items {
		if item.Type != ItemStatus {
			t.Errorf("item %q has type %v, want ItemStatus", item.Name, item.Type)
		}
		if item.Count != -1 {
			t.Errorf("item %q has count %d, want -1 (loading)", item.Name, item.Count)
		}
	}
}

func TestStatusDisplayNameAllCases(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"dungeon/completed", "Completed"},
		{"dungeon/archived", "Archived"},
		{"dungeon/someday", "Someday"},
		{"planning", "Planning"},
		{"ready", "Ready"},
		{"active", "Active"},
		{"ritual", "Ritual"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := statusDisplayName(tt.status)
			if got != tt.want {
				t.Errorf("statusDisplayName(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestGoalFileForItemSequence(t *testing.T) {
	item := FestivalItem{Type: ItemSequence, Path: "/tmp/seq"}
	result := goalFileForItem(item)
	if result != "/tmp/seq/SEQUENCE_GOAL.md" {
		t.Errorf("expected /tmp/seq/SEQUENCE_GOAL.md, got %s", result)
	}
}

func TestGoalFileForItemStatus(t *testing.T) {
	item := FestivalItem{Type: ItemStatus, Path: "/tmp/status"}
	result := goalFileForItem(item)
	if result != "" {
		t.Errorf("expected empty string for status item, got %s", result)
	}
}

func TestGoalFileForItemFestivalFallback(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "FESTIVAL_OVERVIEW.md"), []byte("# Overview\n"), 0644); err != nil {
		t.Fatal(err)
	}

	item := FestivalItem{Type: ItemFestival, Path: tmp}
	result := goalFileForItem(item)
	expected := filepath.Join(tmp, "FESTIVAL_OVERVIEW.md")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestGoalFileForItemFestivalNoGoal(t *testing.T) {
	tmp := t.TempDir()
	item := FestivalItem{Type: ItemFestival, Path: tmp}
	result := goalFileForItem(item)
	if result != "" {
		t.Errorf("expected empty string for festival without goal, got %s", result)
	}
}

func TestRefreshMsgTriggersReload(t *testing.T) {
	m := modelWithItems(3)

	newModel, cmd := m.Update(refreshMsg{})
	_ = newModel.(Model)

	if cmd == nil {
		t.Error("expected reload command from refreshMsg")
	}
}

func TestRefreshMsgPreservesSelection(t *testing.T) {
	m := modelWithItems(5)
	m.cursor = 3
	m.scrollStart = 1

	newModel, _ := m.Update(refreshMsg{})
	m = newModel.(Model)

	if m.cursor != 3 {
		t.Errorf("expected cursor=3 preserved, got %d", m.cursor)
	}
	if m.scrollStart != 1 {
		t.Errorf("expected scrollStart=1 preserved, got %d", m.scrollStart)
	}
}

func TestRefreshItemsMsgClampsSelection(t *testing.T) {
	m := modelWithItems(5)
	m.cursor = 4

	newModel, _ := m.Update(refreshItemsMsg{items: []FestivalItem{
		{Name: "a", Type: ItemFestival, Path: "/a"},
		{Name: "b", Type: ItemFestival, Path: "/b"},
	}})
	m = newModel.(Model)

	if m.cursor != 1 {
		t.Errorf("expected cursor clamped to 1, got %d", m.cursor)
	}
	if len(m.visible) != 2 {
		t.Errorf("expected 2 items, got %d", len(m.visible))
	}
}

func TestRefreshItemsMsgEmptyList(t *testing.T) {
	m := modelWithItems(3)
	m.cursor = 2

	newModel, _ := m.Update(refreshItemsMsg{items: nil})
	m = newModel.(Model)

	if m.cursor != 0 {
		t.Errorf("expected cursor=0 for empty list, got %d", m.cursor)
	}
}

func TestSortFestivalsByCreated(t *testing.T) {
	now := time.Now()
	items := []FestivalItem{
		{Name: "old-fest", Type: ItemFestival, CreatedAt: now.Add(-48 * time.Hour)},
		{Name: "new-fest", Type: ItemFestival, CreatedAt: now},
		{Name: "mid-fest", Type: ItemFestival, CreatedAt: now.Add(-24 * time.Hour)},
	}

	sortFestivalsByCreated(items)

	if items[0].Name != "new-fest" {
		t.Errorf("expected newest first, got %q", items[0].Name)
	}
	if items[1].Name != "mid-fest" {
		t.Errorf("expected mid second, got %q", items[1].Name)
	}
	if items[2].Name != "old-fest" {
		t.Errorf("expected oldest last, got %q", items[2].Name)
	}
}

func TestSortFestivalsByCreatedMissingDates(t *testing.T) {
	now := time.Now()
	items := []FestivalItem{
		{Name: "no-date", Type: ItemFestival},
		{Name: "has-date", Type: ItemFestival, CreatedAt: now},
	}

	sortFestivalsByCreated(items)

	if items[0].Name != "has-date" {
		t.Errorf("expected dated item first, got %q", items[0].Name)
	}
	if items[1].Name != "no-date" {
		t.Errorf("expected undated item last, got %q", items[1].Name)
	}
}

func TestSortFestivalsByCreatedStable(t *testing.T) {
	now := time.Now()
	items := []FestivalItem{
		{Name: "alpha", Type: ItemFestival, CreatedAt: now},
		{Name: "beta", Type: ItemFestival, CreatedAt: now},
	}

	sortFestivalsByCreated(items)

	if items[0].Name != "alpha" {
		t.Errorf("expected stable sort to preserve order, got %q first", items[0].Name)
	}
}

func TestSortFestivalsByCreatedAllZero(t *testing.T) {
	items := []FestivalItem{
		{Name: "beta", Type: ItemFestival},
		{Name: "alpha", Type: ItemFestival},
	}

	sortFestivalsByCreated(items)

	if items[0].Name != "alpha" {
		t.Errorf("expected alphabetical fallback, got %q first", items[0].Name)
	}
}

func TestMdRendererMinWidth(t *testing.T) {
	r := &mdRenderer{}
	content := "# Hello\n\nWorld\n"

	result := r.render(content, 10)
	if !strings.Contains(result, "Hello") {
		t.Error("expected rendered content with clamped width")
	}
	if r.width != 60 {
		t.Errorf("expected width clamped to 60, got %d", r.width)
	}
}

func TestCurrentTitle(t *testing.T) {
	m := modelWithItems(1)
	m.status = ""
	if got := m.currentTitle(); got != "Festivals" {
		t.Errorf("currentTitle() = %q, want %q", got, "Festivals")
	}

	m.status = "active"
	if got := m.currentTitle(); !strings.Contains(got, "active") {
		t.Errorf("currentTitle() = %q, want to contain 'active'", got)
	}
}

func TestRefreshCurrentViewAtStatusRoot(t *testing.T) {
	m := New(context.Background(), "")
	m.loading = false
	m.roots = itemsToTreeNodes(buildStatusItems(), 0, nil)
	m.rebuildVisible()

	cmd := m.refreshCurrentView()
	if cmd == nil {
		t.Error("expected reload cmd for status root")
	}
}

func TestRefreshCurrentViewAtFestivalList(t *testing.T) {
	m := modelWithItems(3)

	cmd := m.refreshCurrentView()
	if cmd == nil {
		t.Error("expected reload cmd for festival list root")
	}
}

func TestRenderTreeLine(t *testing.T) {
	m := modelWithItems(1)

	// Test status node renders WITHOUT tree icons (flat mode)
	node := &TreeNode{
		Item: FestivalItem{Name: "Active", Type: ItemStatus, Count: 5},
	}
	line := m.renderTreeLine(node, false, 40)
	if strings.Contains(line, expandedIcon) || strings.Contains(line, collapsedIcon) {
		t.Error("status items should not have tree icons")
	}
	if !strings.Contains(line, "(5)") {
		t.Error("expected count in tree line")
	}

	// Test festival node renders WITHOUT tree icons (flat mode)
	festNode := &TreeNode{
		Item: FestivalItem{Name: "my-fest", Type: ItemFestival, Status: "active"},
	}
	line = m.renderTreeLine(festNode, false, 40)
	if strings.Contains(line, expandedIcon) || strings.Contains(line, collapsedIcon) {
		t.Error("festival items should not have tree icons")
	}

	// Test phase node renders WITH tree icons
	phaseNode := &TreeNode{
		Item: FestivalItem{Name: "001_DESIGN", Type: ItemPhase, Path: "/tmp/phase"},
	}
	line = m.renderTreeLine(phaseNode, false, 40)
	if !strings.Contains(line, collapsedIcon) {
		t.Error("expected collapsed icon for phase node")
	}

	// Test expanded phase node
	phaseNode.Expanded = true
	phaseNode.Loaded = true
	line = m.renderTreeLine(phaseNode, false, 40)
	if !strings.Contains(line, expandedIcon) {
		t.Error("expected expanded icon for expanded phase node")
	}

	// Test loading phase node
	phaseNode.Loading = true
	line = m.renderTreeLine(phaseNode, false, 40)
	if !strings.Contains(line, loadingIcon) {
		t.Error("expected loading icon in tree line")
	}

	// Test leaf node (task)
	taskNode := &TreeNode{
		Item:  FestivalItem{Name: "task.md", Type: ItemTask},
		Depth: 3,
	}
	line = m.renderTreeLine(taskNode, false, 40)
	if strings.Contains(line, expandedIcon) || strings.Contains(line, collapsedIcon) {
		t.Error("leaf nodes should not have expand/collapse icons")
	}
}

func TestNavigateUp(t *testing.T) {
	// Create a model that has been drilled into a festival
	m := modelWithPhaseItems(2)
	originalRoots := itemsToTreeNodes([]FestivalItem{
		{Name: "fest-1", Type: ItemFestival, Path: "/tmp/fest-1"},
		{Name: "fest-2", Type: ItemFestival, Path: "/tmp/fest-2"},
	}, 0, nil)
	m.navStack = []navEntry{{roots: originalRoots, title: "Active", cursor: 0, scroll: 0}}
	m.breadcrumbs = []string{"Active"}

	// Navigate up
	m = m.navigateUp()

	if len(m.navStack) != 0 {
		t.Errorf("expected empty navStack, got %d", len(m.navStack))
	}
	if len(m.breadcrumbs) != 0 {
		t.Errorf("expected empty breadcrumbs, got %v", m.breadcrumbs)
	}
	if len(m.roots) != 2 {
		t.Errorf("expected 2 restored roots, got %d", len(m.roots))
	}
	if m.roots[0].Item.Name != "fest-1" {
		t.Errorf("expected fest-1 as first root, got %s", m.roots[0].Item.Name)
	}
}

func TestNavigateUpAtRoot(t *testing.T) {
	m := modelWithItems(3)

	// Navigate up with empty navStack should be no-op
	m2 := m.navigateUp()
	if len(m2.visible) != len(m.visible) {
		t.Error("navigateUp at root should be no-op")
	}
}

func TestInTreeMode(t *testing.T) {
	// Not in tree mode: no navStack
	m := modelWithItems(3)
	if m.inTreeMode() {
		t.Error("expected not in tree mode with empty navStack")
	}

	// Not in tree mode: navStack has entries but visible items are festivals
	m.navStack = []navEntry{{roots: nil, title: "test"}}
	if m.inTreeMode() {
		t.Error("expected not in tree mode with festival items visible")
	}

	// In tree mode: navStack has entries and visible items are phases
	m2 := modelWithPhaseItems(2)
	m2.navStack = []navEntry{{roots: nil, title: "test"}}
	if !m2.inTreeMode() {
		t.Error("expected in tree mode with phase items and navStack")
	}
}

func TestDrilldownLoadedMsg(t *testing.T) {
	m := modelWithStatusItems()
	// Simulate pending drilldown
	m.pendingNav = &navEntry{roots: m.roots, title: "Festivals", cursor: 0}
	m.loading = true

	festivals := []FestivalItem{
		{Name: "my-fest", Type: ItemFestival, Path: "/tmp/fest-1", Status: "active"},
	}

	newModel, _ := m.Update(drilldownLoadedMsg{items: festivals, breadcrumb: "Active"})
	m = newModel.(Model)

	if m.loading {
		t.Error("expected loading=false after drilldown loaded")
	}
	if m.pendingNav != nil {
		t.Error("expected pendingNav=nil after commit")
	}
	if len(m.navStack) != 1 {
		t.Errorf("expected navStack length 1, got %d", len(m.navStack))
	}
	if len(m.visible) != 1 {
		t.Errorf("expected 1 visible item, got %d", len(m.visible))
	}
	if m.visible[0].Item.Type != ItemFestival {
		t.Errorf("expected festival item, got %s", m.visible[0].Item.Type)
	}
}

func TestDrilldownLoadedMsgEmpty(t *testing.T) {
	m := modelWithStatusItems()
	m.pendingNav = &navEntry{roots: m.roots, title: "Festivals", cursor: 0}
	m.loading = true

	// Empty drilldown — should discard pendingNav
	newModel, _ := m.Update(drilldownLoadedMsg{items: nil, breadcrumb: "Active"})
	m = newModel.(Model)

	if m.pendingNav != nil {
		t.Error("expected pendingNav discarded on empty drilldown")
	}
	if len(m.navStack) != 0 {
		t.Errorf("expected navStack unchanged (empty), got %d", len(m.navStack))
	}
}

func TestDrilldownLoadedMsgError(t *testing.T) {
	m := modelWithStatusItems()
	m.pendingNav = &navEntry{roots: m.roots, title: "Festivals", cursor: 0}
	m.loading = true

	newModel, _ := m.Update(drilldownLoadedMsg{err: context.Canceled})
	m = newModel.(Model)

	if m.pendingNav != nil {
		t.Error("expected pendingNav discarded on error")
	}
	if len(m.navStack) != 0 {
		t.Error("expected navStack unchanged on error")
	}
}

// --- Test helpers ---

// modelWithStatusItems creates a test model with status overview items.
func modelWithStatusItems() Model {
	items := buildStatusItems()
	m := New(context.Background(), "")
	m.loading = false
	m.roots = itemsToTreeNodes(items, 0, nil)
	m.rebuildVisible()
	m.width = 120
	m.height = 30
	return m
}

// modelWithItems creates a test model with N festival items as tree nodes.
func modelWithItems(n int) Model {
	tmp := os.TempDir()
	items := make([]FestivalItem, n)
	for i := range n {
		items[i] = FestivalItem{
			Name:      "fest-" + itoa(i+1),
			Status:    "active",
			Progress:  float64(i) * 25,
			CreatedAt: time.Now(),
			Path:      filepath.Join(tmp, "fest-"+itoa(i+1)),
			Type:      ItemFestival,
		}
	}

	m := New(context.Background(), "active")
	m.loading = false
	m.roots = itemsToTreeNodes(items, 0, nil)
	m.rebuildVisible()
	m.width = 120
	m.height = 30
	return m
}

// modelWithPhaseItems creates a test model with N phase items as tree nodes.
// Simulates being inside a festival hierarchy (for tree expand/collapse testing).
func modelWithPhaseItems(n int) Model {
	tmp := os.TempDir()
	items := make([]FestivalItem, n)
	for i := range n {
		items[i] = FestivalItem{
			Name: padNum(i+1) + "_PHASE_" + itoa(i+1),
			Path: filepath.Join(tmp, padNum(i+1)+"_PHASE_"+itoa(i+1)),
			Type: ItemPhase,
		}
	}

	m := New(context.Background(), "active")
	m.loading = false
	m.roots = itemsToTreeNodes(items, 0, nil)
	m.rebuildVisible()
	m.width = 120
	m.height = 30
	return m
}
