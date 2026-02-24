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
	os.MkdirAll(emptyFest, 0755)
	os.WriteFile(filepath.Join(emptyFest, "FESTIVAL_GOAL.md"), []byte("# Empty Festival\n"), 0644)

	// Create completed festival
	completedFest := filepath.Join(festivalsDir, "dungeon", "completed", "done-fest-DF0001")
	createTestFestival(t, completedFest, "Completed Festival Goal", 1, 1, 2)

	return festivalsDir
}

// createTestFestival creates a festival with phases, sequences, and tasks.
func createTestFestival(t *testing.T, festivalDir, goal string, numPhases, numSequences, numTasks int) {
	t.Helper()
	os.MkdirAll(festivalDir, 0755)
	os.WriteFile(filepath.Join(festivalDir, "FESTIVAL_GOAL.md"),
		[]byte("# Festival Goal\n\n"+goal+"\n"), 0644)

	for p := 1; p <= numPhases; p++ {
		phaseName := padNum(p) + "_PHASE_" + strings.ToUpper(itoa(p))
		phaseDir := filepath.Join(festivalDir, phaseName)
		os.MkdirAll(phaseDir, 0755)
		os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"),
			[]byte("# Phase Goal\n\nPhase "+itoa(p)+" goal\n"), 0644)

		for s := 1; s <= numSequences; s++ {
			seqName := padNum(s) + "_sequence_" + itoa(s)
			seqDir := filepath.Join(phaseDir, seqName)
			os.MkdirAll(seqDir, 0755)
			os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"),
				[]byte("# Sequence Goal\n\nSequence "+itoa(s)+" goal\n"), 0644)

			for tk := 1; tk <= numTasks; tk++ {
				taskName := padNum(tk) + "_task_" + itoa(tk) + ".md"
				os.WriteFile(filepath.Join(seqDir, taskName),
					[]byte("---\nfest_type: task\nfest_status: pending\nfest_tracking: true\n---\n\n# Task "+itoa(tk)+"\n"), 0644)
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
	if len(m.items) != 2 {
		t.Errorf("expected 2 items, got %d", len(m.items))
	}
	// Preview is now loaded async — cmd should be non-nil (loadPreviewCmd)
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

	if len(m.items) != 0 {
		t.Errorf("expected 0 items, got %d", len(m.items))
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
	if m.selected != 1 {
		t.Errorf("expected selected=1 after down, got %d", m.selected)
	}

	// Down again
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)
	if m.selected != 2 {
		t.Errorf("expected selected=2 after second down, got %d", m.selected)
	}

	// Down at bottom (should stay)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)
	if m.selected != 2 {
		t.Errorf("expected selected=2 at bottom, got %d", m.selected)
	}

	// Up
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(Model)
	if m.selected != 1 {
		t.Errorf("expected selected=1 after up, got %d", m.selected)
	}
}

func TestVimNavigation(t *testing.T) {
	m := modelWithItems(5)

	// j = down
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newModel.(Model)
	if m.selected != 1 {
		t.Errorf("expected selected=1 after j, got %d", m.selected)
	}

	// k = up
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = newModel.(Model)
	if m.selected != 0 {
		t.Errorf("expected selected=0 after k, got %d", m.selected)
	}

	// G = bottom
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = newModel.(Model)
	if m.selected != 4 {
		t.Errorf("expected selected=4 after G, got %d", m.selected)
	}

	// gg = top (two g presses within timeout)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = newModel.(Model)
	// First g just records time
	if m.selected != 4 {
		t.Errorf("expected selected=4 after first g, got %d", m.selected)
	}
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = newModel.(Model)
	if m.selected != 0 {
		t.Errorf("expected selected=0 after gg, got %d", m.selected)
	}
}

func TestVimHLNavigation(t *testing.T) {
	m := modelWithItems(3)
	m.items[0].Type = ItemFestival

	// l = enter/drilldown (won't work without real filesystem, but shouldn't crash)
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = newModel.(Model)
	// Should trigger a load command
	if cmd == nil {
		t.Error("expected command from l key (drilldown)")
	}

	// h at top level should be no-op (not quit)
	m2 := modelWithItems(3)
	newModel, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m2 = newModel.(Model)
	if m2.quitting {
		t.Error("h should not quit at top level")
	}
}

func TestNavigateDownTask(t *testing.T) {
	m := modelWithItems(1)
	m.items[0].Type = ItemTask

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

func TestNavigateUpEmptyStack(t *testing.T) {
	m := modelWithItems(3)

	// Backspace at top level should quit
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = newModel.(Model)
	if !m.quitting {
		t.Error("expected quitting after backspace at top level")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestNavigateUpWithStack(t *testing.T) {
	m := modelWithItems(3)

	// Simulate having navigated down
	m.navStack = append(m.navStack, navEntry{
		items:    m.items,
		selected: 0,
		scroll:   0,
	})
	m.breadcrumbs = append(m.breadcrumbs, "fest-1")
	m.items = []FestivalItem{
		{Name: "phase-1", Type: ItemPhase, Path: "/tmp/phase-1"},
	}
	m.selected = 0

	// Backspace should pop the stack
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = newModel.(Model)

	if m.quitting {
		t.Error("should not quit when popping navStack")
	}
	if len(m.navStack) != 0 {
		t.Errorf("expected empty navStack, got %d entries", len(m.navStack))
	}
	if len(m.items) != 3 {
		t.Errorf("expected 3 items restored, got %d", len(m.items))
	}
	if len(m.breadcrumbs) != 0 {
		t.Errorf("expected empty breadcrumbs, got %d", len(m.breadcrumbs))
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
	if len(m.allItems) != 5 {
		t.Errorf("expected allItems to be saved, got %d", len(m.allItems))
	}

	// Cancel with Escape
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if m.filtering {
		t.Error("expected filtering=false after Escape")
	}
	if len(m.items) != 5 {
		t.Errorf("expected all 5 items restored, got %d", len(m.items))
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
	m.selected = 1
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
	m.selected = 1
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

	if m.maxVisible != 5 { // min(8-5, 5) = max(3, 5) = 5
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
	m.items = nil
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
	// Should render list only (no preview pane)
	if strings.Contains(view, "Festival Goal") && strings.Contains(view, "Preview") {
		t.Error("narrow terminal should not show preview pane")
	}
}

func TestViewBreadcrumbs(t *testing.T) {
	m := modelWithItems(3)
	m.width = 120
	m.height = 30
	m.breadcrumbs = []string{"fest-1", "phase-1"}

	view := m.View()
	if !strings.Contains(view, "fest-1") {
		t.Error("expected breadcrumb 'fest-1' in view")
	}
}

func TestRenderBreadcrumbSingle(t *testing.T) {
	m := New(context.Background(), "active")
	crumb := m.renderBreadcrumb()
	if !strings.Contains(crumb, "Festivals") {
		t.Error("expected 'Festivals' in breadcrumb")
	}
}

func TestRenderBreadcrumbMultiple(t *testing.T) {
	m := New(context.Background(), "active")
	m.breadcrumbs = []string{"my-festival", "001_PHASE"}
	crumb := m.renderBreadcrumb()
	if !strings.Contains(crumb, ">") {
		t.Error("expected '>' separator in breadcrumb")
	}
}

func TestEnsureVisible(t *testing.T) {
	m := modelWithItems(20)
	m.maxVisible = 5

	// Select item beyond viewport
	m.selected = 10
	m.ensureVisible()
	if m.scrollStart > 10 {
		t.Errorf("scrollStart should be <= selected, got scrollStart=%d selected=%d", m.scrollStart, m.selected)
	}
	if m.selected >= m.scrollStart+m.maxVisible {
		t.Error("selected should be within visible range")
	}

	// Select item above viewport
	m.selected = 0
	m.ensureVisible()
	if m.scrollStart != 0 {
		t.Errorf("expected scrollStart=0, got %d", m.scrollStart)
	}
}

func TestGGTimeout(t *testing.T) {
	m := modelWithItems(5)
	m.selected = 3

	// First g
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = newModel.(Model)
	if m.selected != 3 {
		t.Errorf("first g should not move, got selected=%d", m.selected)
	}
	if m.lastGTime.IsZero() {
		t.Error("lastGTime should be set after first g")
	}

	// Simulate timeout by setting lastGTime in the past
	m.lastGTime = time.Now().Add(-time.Second)

	// Second g after timeout
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = newModel.(Model)
	// Should record new time, not jump to top
	if m.selected == 0 && !m.lastGTime.IsZero() {
		// This is expected: the g after timeout sets a new time
	}
}

func TestElementsToItems(t *testing.T) {
	// Just testing the conversion function
	items := elementsToItems(nil, ItemPhase)
	if len(items) != 0 {
		t.Errorf("expected 0 items from nil input, got %d", len(items))
	}
}

func TestGoalFileForItem(t *testing.T) {
	tmp := t.TempDir()

	// Test festival goal file detection
	festDir := filepath.Join(tmp, "festival")
	os.MkdirAll(festDir, 0755)
	goalFile := filepath.Join(festDir, "FESTIVAL_GOAL.md")
	os.WriteFile(goalFile, []byte("# Goal\n"), 0644)

	item := FestivalItem{Type: ItemFestival, Path: festDir}
	result := goalFileForItem(item)
	if result != goalFile {
		t.Errorf("expected %s, got %s", goalFile, result)
	}

	// Test phase goal file
	phaseDir := filepath.Join(tmp, "phase")
	os.MkdirAll(phaseDir, 0755)
	phaseGoal := filepath.Join(phaseDir, "PHASE_GOAL.md")
	os.WriteFile(phaseGoal, []byte("# Phase\n"), 0644)

	item = FestivalItem{Type: ItemPhase, Path: phaseDir}
	result = goalFileForItem(item)
	if result != phaseGoal {
		t.Errorf("expected %s, got %s", phaseGoal, result)
	}

	// Test task (returns item path)
	taskPath := filepath.Join(tmp, "task.md")
	os.WriteFile(taskPath, []byte("# Task\n"), 0644)
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
	os.WriteFile(testFile, []byte("# Title\n\nSome content\n"), 0644)

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

	// Create a directory structure with non-numbered items
	os.MkdirAll(filepath.Join(tmp, "input_specs"), 0755)
	os.MkdirAll(filepath.Join(tmp, "decisions"), 0755)
	os.MkdirAll(filepath.Join(tmp, ".hidden"), 0755)
	os.WriteFile(filepath.Join(tmp, "README.md"), []byte("# README\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "PHASE_GOAL.md"), []byte("# Goal\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "notes.txt"), []byte("notes"), 0644)

	items, err := loadGenericChildren(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should include: decisions/, input_specs/, README.md
	// Should exclude: .hidden/, PHASE_GOAL.md, notes.txt
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

func TestNavStackNotCorruptedOnEmptyChildren(t *testing.T) {
	m := modelWithItems(3)
	m.items[0].Type = ItemFestival

	// Simulate navigateDown which sets pendingNav
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = newModel.(Model)

	if cmd == nil {
		t.Fatal("expected load command from navigateDown")
	}
	if m.pendingNav == nil {
		t.Fatal("expected pendingNav to be set")
	}

	// Simulate empty children response
	newModel, _ = m.Update(childrenLoadedMsg{items: nil})
	m = newModel.(Model)

	if len(m.navStack) != 0 {
		t.Errorf("expected empty navStack after empty children, got %d entries", len(m.navStack))
	}
	if m.pendingNav != nil {
		t.Error("expected pendingNav to be cleared after empty children")
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

	// Init returns a tea.Batch; execute the result to get inner msgs
	msg := cmd()
	// The batch returns a tea.BatchMsg containing sub-commands
	batchMsg, isBatch := msg.(tea.BatchMsg)
	if !isBatch {
		// Direct cmd (non-batch) — check it directly
		loadedMsg, ok := msg.(festivalsLoadedMsg)
		if !ok {
			t.Fatalf("expected festivalsLoadedMsg, got %T", msg)
		}
		if loadedMsg.err == nil {
			t.Error("expected error from cancelled context")
		}
		return
	}

	// Find the festivalsLoadedMsg in the batch
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

func TestChildrenLoadedMsgEmpty(t *testing.T) {
	m := modelWithItems(3)

	msg := childrenLoadedMsg{items: nil}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Should stay at current level
	if len(m.items) != 3 {
		t.Errorf("expected 3 items (unchanged), got %d", len(m.items))
	}
}

func TestChildrenLoadedMsgError(t *testing.T) {
	m := modelWithItems(3)

	msg := childrenLoadedMsg{err: context.Canceled}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Should stay at current level (error discards pending nav)
	if len(m.items) != 3 {
		t.Errorf("expected 3 items (unchanged), got %d", len(m.items))
	}
	if m.pendingNav != nil {
		t.Error("expected pendingNav to be cleared")
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
			// Just verify it doesn't panic
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

	// buildStatusItems creates items synchronously — verify via festivalsLoadedMsg
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

	for _, item := range m.items {
		if item.Type != ItemStatus {
			continue
		}
		if expected, ok := counts[item.Status]; ok {
			if item.Count != expected {
				t.Errorf("status %q: expected count %d, got %d", item.Status, expected, item.Count)
			}
		}
	}
}

func TestStatusCountsMsgNil(t *testing.T) {
	m := modelWithStatusItems()

	// nil counts should not crash
	newModel, _ := m.Update(statusCountsMsg{counts: nil})
	m = newModel.(Model)

	// Counts should still be -1 (unchanged)
	for _, item := range m.items {
		if item.Type == ItemStatus && item.Count != -1 {
			t.Errorf("expected Count=-1, got %d for %q", item.Count, item.Status)
		}
	}
}

func TestNavigateDownStatus(t *testing.T) {
	m := modelWithStatusItems()

	// Select first status item and navigate down
	m.selected = 0
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if cmd == nil {
		t.Error("expected load command when drilling into status item")
	}
	if m.pendingNav == nil {
		t.Error("expected pendingNav to be set")
	}
	if !m.loading {
		t.Error("expected loading=true during status drill-down")
	}
}

func TestStatusOverviewDoesNotQuit(t *testing.T) {
	ctx := context.Background()
	m := New(ctx, "") // Empty status = overview mode

	// Receive empty festivals loaded message — should NOT quit
	msg := festivalsLoadedMsg{items: nil}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	if cmd != nil {
		t.Error("expected no quit command for empty status overview")
	}
	if len(m.items) != 0 {
		t.Errorf("expected 0 items, got %d", len(m.items))
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
	m.items[0].Count = 5
	m.width = 120
	m.height = 30

	// Should render without panic
	view := m.View()
	if !strings.Contains(view, "(5)") {
		t.Error("expected count '(5)' in status item render")
	}
}

func TestStatusItemRenderingLoading(t *testing.T) {
	m := modelWithStatusItems()
	m.width = 120
	m.height = 30

	// Count = -1 should show "..."
	view := m.View()
	if !strings.Contains(view, "...") {
		t.Error("expected '...' for loading count")
	}
}

// modelWithStatusItems creates a test model with status overview items.
func modelWithStatusItems() Model {
	items := buildStatusItems()
	m := New(context.Background(), "")
	m.loading = false
	m.items = items
	m.width = 120
	m.height = 30
	return m
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
			// Should not panic
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

	// j key should delegate to viewport
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m2 := updated.(Model)
	if !m2.focusPreview {
		t.Error("preview should still be focused after j")
	}

	// q quits
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
			m.items = []FestivalItem{{Name: "test", Type: tt.itemType, Path: "/tmp/test"}}
			m.syncViewportSize()
			content := m.renderContent(60)
			if !strings.Contains(content, tt.wantTitle) {
				t.Errorf("renderContent for %s should contain %q", tt.itemType, tt.wantTitle)
			}
		})
	}
}

func TestRenderItemFestivalShowsDate(t *testing.T) {
	m := modelWithItems(1)
	item := m.items[0]
	rendered := m.renderItem(item, false)
	// Should contain date like "Feb 24"
	if !strings.Contains(rendered, time.Now().Format("Jan")) {
		t.Errorf("renderItem for festival should show date, got: %s", rendered)
	}
}

func TestCurrentTitle(t *testing.T) {
	m := modelWithItems(1)
	m.status = ""
	m.breadcrumbs = nil
	if got := m.currentTitle(); got != "Festivals" {
		t.Errorf("currentTitle() = %q, want %q", got, "Festivals")
	}

	m.status = "active"
	if got := m.currentTitle(); !strings.Contains(got, "active") {
		t.Errorf("currentTitle() = %q, want to contain 'active'", got)
	}

	m.breadcrumbs = []string{"phase1", "seq1"}
	if got := m.currentTitle(); got != "seq1" {
		t.Errorf("currentTitle() = %q, want %q", got, "seq1")
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

func TestFilterKeyEsc(t *testing.T) {
	m := modelWithItems(5)
	m.filtering = true
	m.allItems = m.items

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(Model)
	if m2.filtering {
		t.Error("esc should exit filter mode")
	}
	if len(m2.items) != 5 {
		t.Errorf("esc should restore all items, got %d", len(m2.items))
	}
}

func TestFilterKeyEnter(t *testing.T) {
	m := modelWithItems(5)
	m.filtering = true
	m.allItems = m.items
	// Simulate having filtered to 2 items
	m.items = m.items[:2]

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	if m2.filtering {
		t.Error("enter should exit filter mode")
	}
	// Items should stay filtered (not restored)
	if len(m2.items) != 2 {
		t.Errorf("enter should keep filtered items, got %d", len(m2.items))
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

func TestEscWithNavStack(t *testing.T) {
	m := modelWithItems(3)
	m.navStack = append(m.navStack, navEntry{
		items:    m.items,
		selected: 0,
		scroll:   0,
	})
	m.breadcrumbs = append(m.breadcrumbs, "test")
	m.items = []FestivalItem{{Name: "child", Type: ItemPhase, Path: "/tmp/child"}}

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)

	if m.quitting {
		t.Error("esc with navStack should navigate up, not quit")
	}
	if len(m.navStack) != 0 {
		t.Error("expected navStack to be popped")
	}
	if len(m.items) != 3 {
		t.Errorf("expected 3 items restored, got %d", len(m.items))
	}
}

func TestFilterTypingFiltersItems(t *testing.T) {
	m := modelWithItems(5)
	// Items: fest-1 through fest-5

	// Enter filter mode
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newModel.(Model)
	if !m.filtering {
		t.Fatal("expected filtering mode")
	}

	// Type "3" to filter
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = newModel.(Model)

	if len(m.items) != 1 {
		t.Errorf("expected 1 filtered item, got %d", len(m.items))
	}
	if len(m.items) > 0 && m.items[0].Name != "fest-3" {
		t.Errorf("expected fest-3, got %s", m.items[0].Name)
	}
}

func TestFilterTypingNoMatch(t *testing.T) {
	m := modelWithItems(3)

	// Enter filter mode
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newModel.(Model)

	// Type "xyz" — no match
	for _, r := range "xyz" {
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newModel.(Model)
	}

	if len(m.items) != 0 {
		t.Errorf("expected 0 items for non-matching filter, got %d", len(m.items))
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

func TestGOnEmptyList(t *testing.T) {
	m := modelWithItems(0)

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = newModel.(Model)
	if m.selected != 0 {
		t.Errorf("G on empty list should be no-op, got selected=%d", m.selected)
	}
	if cmd != nil {
		t.Error("G on empty list should return nil cmd")
	}
}

func TestKAtTopBoundary(t *testing.T) {
	m := modelWithItems(3)
	m.selected = 0

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = newModel.(Model)
	if m.selected != 0 {
		t.Errorf("k at top should stay at 0, got %d", m.selected)
	}
	if cmd != nil {
		t.Error("k at top boundary should return nil cmd")
	}
}

func TestJAtBottomBoundary(t *testing.T) {
	m := modelWithItems(3)
	m.selected = 2

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newModel.(Model)
	if m.selected != 2 {
		t.Errorf("j at bottom should stay at 2, got %d", m.selected)
	}
	if cmd != nil {
		t.Error("j at bottom boundary should return nil cmd")
	}
}

func TestLoadPreviewCmdOutOfBounds(t *testing.T) {
	m := modelWithItems(3)
	m.selected = -1

	cmd := m.loadPreviewCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd even for out-of-bounds")
	}

	// Execute cmd — should return empty previewLoadedMsg
	msg := cmd()
	loaded, ok := msg.(previewLoadedMsg)
	if !ok {
		t.Fatalf("expected previewLoadedMsg, got %T", msg)
	}
	if loaded.rendered != "" {
		t.Errorf("expected empty rendered for out-of-bounds, got %q", loaded.rendered)
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

func TestChildrenLoadedSuccessPushesNavStack(t *testing.T) {
	m := modelWithItems(3)
	m.pendingNav = &navEntry{
		items:    m.items,
		selected: 1,
		scroll:   0,
	}

	children := []FestivalItem{
		{Name: "phase-1", Type: ItemPhase, Path: "/tmp/phase-1"},
		{Name: "phase-2", Type: ItemPhase, Path: "/tmp/phase-2"},
	}

	newModel, _ := m.Update(childrenLoadedMsg{items: children, breadcrumb: "fest-1"})
	m = newModel.(Model)

	if len(m.navStack) != 1 {
		t.Errorf("expected 1 navStack entry, got %d", len(m.navStack))
	}
	if m.pendingNav != nil {
		t.Error("expected pendingNav to be cleared")
	}
	if len(m.items) != 2 {
		t.Errorf("expected 2 child items, got %d", len(m.items))
	}
	if m.selected != 0 {
		t.Errorf("expected selected=0 after drill-down, got %d", m.selected)
	}
	if len(m.breadcrumbs) != 1 || m.breadcrumbs[0] != "fest-1" {
		t.Errorf("expected breadcrumbs=[fest-1], got %v", m.breadcrumbs)
	}
}

func TestNavigateDownOutOfBounds(t *testing.T) {
	m := modelWithItems(3)
	m.selected = 10 // out of bounds

	result, cmd := m.navigateDown()
	m = result.(Model)

	if cmd != nil {
		t.Error("navigateDown with out-of-bounds should return nil cmd")
	}
}

func TestMdRendererMinWidth(t *testing.T) {
	r := &mdRenderer{}
	content := "# Hello\n\nWorld\n"

	// Width < 20 should be clamped to 60
	result := r.render(content, 10)
	if !strings.Contains(result, "Hello") {
		t.Error("expected rendered content with clamped width")
	}
	if r.width != 60 {
		t.Errorf("expected width clamped to 60, got %d", r.width)
	}
}

func TestHAtTopLevelNoOp(t *testing.T) {
	m := modelWithItems(3)
	// No navStack entries

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = newModel.(Model)

	if m.quitting {
		t.Error("h at top level should be no-op, not quit")
	}
	if cmd != nil {
		t.Error("h at top level should return nil cmd")
	}
}

func TestNavigateDownEnterKey(t *testing.T) {
	m := modelWithItems(3)
	m.items[0].Type = ItemFestival

	// Enter key should behave same as l (drilldown)
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if cmd == nil {
		t.Error("expected command from enter key (drilldown)")
	}
	if m.pendingNav == nil {
		t.Error("expected pendingNav to be set")
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

	// Festival with FESTIVAL_OVERVIEW.md but no FESTIVAL_GOAL.md
	os.WriteFile(filepath.Join(tmp, "FESTIVAL_OVERVIEW.md"), []byte("# Overview\n"), 0644)

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
	m = newModel.(Model)

	if cmd == nil {
		t.Error("expected reload command from refreshMsg")
	}
}

func TestRefreshMsgPreservesSelection(t *testing.T) {
	m := modelWithItems(5)
	m.selected = 3
	m.scrollStart = 1

	newModel, _ := m.Update(refreshMsg{})
	m = newModel.(Model)

	if m.selected != 3 {
		t.Errorf("expected selected=3 preserved, got %d", m.selected)
	}
	if m.scrollStart != 1 {
		t.Errorf("expected scrollStart=1 preserved, got %d", m.scrollStart)
	}
}

func TestRefreshItemsMsgClampsSelection(t *testing.T) {
	m := modelWithItems(5)
	m.selected = 4

	// Refresh with fewer items
	newModel, _ := m.Update(refreshItemsMsg{items: []FestivalItem{
		{Name: "a", Type: ItemFestival},
		{Name: "b", Type: ItemFestival},
	}})
	m = newModel.(Model)

	if m.selected != 1 {
		t.Errorf("expected selected clamped to 1, got %d", m.selected)
	}
	if len(m.items) != 2 {
		t.Errorf("expected 2 items, got %d", len(m.items))
	}
}

func TestRefreshItemsMsgEmptyList(t *testing.T) {
	m := modelWithItems(3)
	m.selected = 2

	newModel, _ := m.Update(refreshItemsMsg{items: nil})
	m = newModel.(Model)

	if m.selected != 0 {
		t.Errorf("expected selected=0 for empty list, got %d", m.selected)
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

func TestRefreshCurrentViewAtStatusRoot(t *testing.T) {
	m := New(context.Background(), "")
	m.loading = false
	m.items = buildStatusItems()

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

// modelWithItems creates a test model with N festival items.
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
	m.items = items
	m.width = 120
	m.height = 30
	return m
}
