package picker

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestPadRight(t *testing.T) {
	if got := padRight("abc", 6); got != "abc   " {
		t.Fatalf("padRight(\"abc\", 6) = %q, want %q", got, "abc   ")
	}
	if got := padRight("abcdef", 3); got != "abcdef" {
		t.Fatalf("padRight must not truncate, got %q", got)
	}
}

func TestViewRendersDetailColumn(t *testing.T) {
	items := []Item{
		{Name: "alpha", Value: "/a", Detail: "BAR-50%"},
		{Name: "bravo-longer", Value: "/b"},
	}
	out := New(items, dummyScorer, testRenderer).View()
	if !strings.Contains(out, "BAR-50%") {
		t.Fatalf("View should render item Detail, got:\n%s", out)
	}
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "bravo-longer") {
		t.Fatalf("View should render item names, got:\n%s", out)
	}
}

func dummyScorer(q, target string) (int, []int) {
	return 1, nil
}

var testRenderer = lipgloss.NewRenderer(os.Stderr)

func TestTabCycling(t *testing.T) {
	items := []Item{
		{Name: "alpha", Value: "/a"},
		{Name: "bravo", Value: "/b"},
		{Name: "charlie", Value: "/c"},
	}
	m := New(items, dummyScorer, testRenderer)

	if m.selected != 0 {
		t.Fatalf("initial selected = %d, want 0", m.selected)
	}

	tabMsg := tea.KeyMsg{Type: tea.KeyTab}

	updated, _ := m.Update(tabMsg)
	m = updated.(Model)
	if m.selected != 1 {
		t.Errorf("after tab: selected = %d, want 1", m.selected)
	}

	updated, _ = m.Update(tabMsg)
	m = updated.(Model)
	if m.selected != 2 {
		t.Errorf("after 2nd tab: selected = %d, want 2", m.selected)
	}

	// Wrap around
	updated, _ = m.Update(tabMsg)
	m = updated.(Model)
	if m.selected != 0 {
		t.Errorf("after 3rd tab (wrap): selected = %d, want 0", m.selected)
	}
}

func TestShiftTabCycling(t *testing.T) {
	items := []Item{
		{Name: "alpha", Value: "/a"},
		{Name: "bravo", Value: "/b"},
		{Name: "charlie", Value: "/c"},
	}
	m := New(items, dummyScorer, testRenderer)

	shiftTabMsg := tea.KeyMsg{Type: tea.KeyShiftTab}

	// Shift+Tab from 0 wraps to last
	updated, _ := m.Update(shiftTabMsg)
	m = updated.(Model)
	if m.selected != 2 {
		t.Errorf("after shift+tab from 0: selected = %d, want 2", m.selected)
	}

	updated, _ = m.Update(shiftTabMsg)
	m = updated.(Model)
	if m.selected != 1 {
		t.Errorf("after 2nd shift+tab: selected = %d, want 1", m.selected)
	}
}

func TestTabCycling_EmptyFiltered(t *testing.T) {
	m := New(nil, dummyScorer, testRenderer)
	m.filtered = nil

	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := m.Update(tabMsg)
	m = updated.(Model)
	if m.selected != 0 {
		t.Errorf("tab on empty: selected = %d, want 0", m.selected)
	}
}

func TestSelected_Confirmed(t *testing.T) {
	items := []Item{
		{Name: "alpha", Value: "/a"},
		{Name: "bravo", Value: "/b"},
	}
	m := New(items, dummyScorer, testRenderer)

	// Move to second item
	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := m.Update(tabMsg)
	m = updated.(Model)

	// Confirm
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ = m.Update(enterMsg)
	m = updated.(Model)

	sel := m.Selected()
	if sel == nil {
		t.Fatal("expected selected item, got nil")
	}
	if sel.Name != "bravo" {
		t.Errorf("selected = %q, want bravo", sel.Name)
	}
	if !m.Confirmed() {
		t.Error("expected Confirmed() to be true")
	}
}

func TestSelected_Cancelled(t *testing.T) {
	items := []Item{{Name: "alpha", Value: "/a"}}
	m := New(items, dummyScorer, testRenderer)

	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ := m.Update(escMsg)
	m = updated.(Model)

	if m.Selected() != nil {
		t.Error("expected nil Selected after cancel")
	}
	if !m.Cancelled() {
		t.Error("expected Cancelled() to be true")
	}
}

func TestView_RendersItems(t *testing.T) {
	items := []Item{
		{Name: "alpha", Value: "/a"},
		{Name: "bravo", Value: "/b"},
	}
	m := New(items, dummyScorer, testRenderer)

	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	// Should contain item names
	if !containsString(view, "alpha") {
		t.Error("view missing 'alpha'")
	}
	if !containsString(view, "bravo") {
		t.Error("view missing 'bravo'")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
