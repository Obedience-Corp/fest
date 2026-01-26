// Package picker provides an fzf-style interactive fuzzy picker.
package picker

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Item represents a pickable item with a display name and value.
type Item struct {
	Name  string // Display name (used for matching)
	Value string // Return value (e.g., path)
	Score int    // Match score for sorting
}

// Scorer is a function that scores a query against a target string.
// Returns score (0 = no match) and matched character indices.
type Scorer func(query, target string) (score int, indices []int)

// Model is the picker's bubbletea model.
type Model struct {
	items       []Item // Original items
	filtered    []Item // Items after filtering
	selected    int    // Currently selected index in filtered
	input       textinput.Model
	scorer      Scorer
	maxVisible  int // Max items to show
	scrollStart int // First visible item index
	width       int
	height      int
	cancelled   bool
	confirmed   bool

	// Styles
	promptStyle   lipgloss.Style
	cursorStyle   lipgloss.Style
	matchStyle    lipgloss.Style
	selectedStyle lipgloss.Style
	normalStyle   lipgloss.Style
	countStyle    lipgloss.Style
}

// New creates a new picker with the given items and scorer.
func New(items []Item, scorer Scorer) Model {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	m := Model{
		items:      items,
		filtered:   items,
		input:      ti,
		scorer:     scorer,
		maxVisible: 10,
		width:      60,
		height:     15,

		promptStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("12")), // Blue
		cursorStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("11")), // Yellow
		matchStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("10")), // Green
		selectedStyle: lipgloss.NewStyle().Background(lipgloss.Color("8")),  // Gray bg
		normalStyle:   lipgloss.NewStyle(),
		countStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")), // Gray
	}

	return m
}

// WithMaxVisible sets the maximum number of visible items.
func (m Model) WithMaxVisible(n int) Model {
	m.maxVisible = n
	return m
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			m.confirmed = true
			return m, tea.Quit

		case "up", "ctrl+p", "ctrl+k":
			if m.selected > 0 {
				m.selected--
				m.ensureVisible()
			}
			return m, nil

		case "down", "ctrl+n", "ctrl+j":
			if m.selected < len(m.filtered)-1 {
				m.selected++
				m.ensureVisible()
			}
			return m, nil

		case "pgup":
			m.selected -= m.maxVisible
			if m.selected < 0 {
				m.selected = 0
			}
			m.ensureVisible()
			return m, nil

		case "pgdown":
			m.selected += m.maxVisible
			if m.selected >= len(m.filtered) {
				m.selected = len(m.filtered) - 1
			}
			if m.selected < 0 {
				m.selected = 0
			}
			m.ensureVisible()
			return m, nil

		case "home", "ctrl+a":
			m.selected = 0
			m.ensureVisible()
			return m, nil

		case "end", "ctrl+e":
			if len(m.filtered) > 0 {
				m.selected = len(m.filtered) - 1
			}
			m.ensureVisible()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
		m.maxVisible = msg.Height - 4
		if m.maxVisible < 3 {
			m.maxVisible = 3
		}
	}

	// Update text input
	var cmd tea.Cmd
	prevValue := m.input.Value()
	m.input, cmd = m.input.Update(msg)

	// Re-filter if input changed
	if m.input.Value() != prevValue {
		m.filter()
	}

	return m, cmd
}

// filter applies the current query to filter items.
func (m *Model) filter() {
	query := strings.TrimSpace(m.input.Value())

	if query == "" {
		m.filtered = m.items
		m.selected = 0
		m.scrollStart = 0
		return
	}

	var matches []Item
	for _, item := range m.items {
		score, _ := m.scorer(query, item.Name)
		if score > 0 {
			item.Score = score
			matches = append(matches, item)
		}
	}

	// Sort by score descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	m.filtered = matches
	m.selected = 0
	m.scrollStart = 0
}

// ensureVisible adjusts scroll to keep selected item visible.
func (m *Model) ensureVisible() {
	if m.selected < m.scrollStart {
		m.scrollStart = m.selected
	}
	if m.selected >= m.scrollStart+m.maxVisible {
		m.scrollStart = m.selected - m.maxVisible + 1
	}
}

// View renders the picker.
func (m Model) View() string {
	var b strings.Builder

	// Prompt line
	prompt := m.promptStyle.Render("> ")
	b.WriteString(prompt)
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Count line
	count := fmt.Sprintf("  %d/%d", len(m.filtered), len(m.items))
	b.WriteString(m.countStyle.Render(count))
	b.WriteString("\n")

	// Items
	endIdx := m.scrollStart + m.maxVisible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	for i := m.scrollStart; i < endIdx; i++ {
		item := m.filtered[i]

		// Cursor
		if i == m.selected {
			b.WriteString(m.cursorStyle.Render("▶ "))
		} else {
			b.WriteString("  ")
		}

		// Item name with highlighting
		name := item.Name
		if i == m.selected {
			name = m.selectedStyle.Render(name)
		}
		b.WriteString(name)
		b.WriteString("\n")
	}

	// Padding for empty space
	for i := len(m.filtered); i < m.maxVisible; i++ {
		b.WriteString("\n")
	}

	// Help
	help := m.countStyle.Render("↑/↓ or j/k: navigate • enter: select • esc: cancel")
	b.WriteString(help)

	return b.String()
}

// Selected returns the selected item, or nil if cancelled.
func (m Model) Selected() *Item {
	if m.cancelled || len(m.filtered) == 0 {
		return nil
	}
	if m.selected >= 0 && m.selected < len(m.filtered) {
		return &m.filtered[m.selected]
	}
	return nil
}

// Cancelled returns true if the picker was cancelled.
func (m Model) Cancelled() bool {
	return m.cancelled
}

// Confirmed returns true if a selection was confirmed.
func (m Model) Confirmed() bool {
	return m.confirmed
}

// Run runs the picker and returns the selected item.
// The picker renders to stderr so stdout can capture the result for shell integration.
func Run(items []Item, scorer Scorer) (*Item, error) {
	debug := os.Getenv("FEST_DEBUG") != ""

	start := time.Now()
	m := New(items, scorer)
	if debug {
		log.Printf("[DEBUG] picker.New: %v", time.Since(start))
	}

	start = time.Now()
	// Use stderr for rendering so stdout is clean for `cd $(fest go)`
	// WithAltScreen ensures immediate rendering by using alternate screen buffer
	p := tea.NewProgram(m,
		tea.WithOutput(os.Stderr),
		tea.WithInput(os.Stdin),
		tea.WithAltScreen(),
	)
	if debug {
		log.Printf("[DEBUG] tea.NewProgram: %v", time.Since(start))
	}

	start = time.Now()
	finalModel, err := p.Run()
	if debug {
		log.Printf("[DEBUG] tea.Run: %v", time.Since(start))
	}
	if err != nil {
		return nil, err
	}
	return finalModel.(Model).Selected(), nil
}
