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
	Name        string                 // Display name (used for matching)
	Value       string                 // Return value (e.g., path)
	Detail      string                 // Optional trailing text shown after the name (not matched)
	Prefix      string                 // Optional leading label shown before the name (not matched)
	PrefixColor lipgloss.TerminalColor // Optional foreground color for Prefix
	Score       int                    // Match score for sorting
}

// Scorer is a function that scores a query against a target string.
// Returns score (0 = no match) and matched character indices.
type Scorer func(query, target string) (score int, indices []int)

// Fest theme colors using adaptive colors (auto-detect light/dark terminal)
var (
	colorText        = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
	colorPlaceholder = lipgloss.AdaptiveColor{Light: "#808080", Dark: "#949494"}
	colorFocus       = lipgloss.AdaptiveColor{Light: "#FF8700", Dark: "#FFD700"}
	colorSelected    = lipgloss.AdaptiveColor{Light: "#00AF00", Dark: "#00FF5F"}
	colorBorder      = lipgloss.AdaptiveColor{Light: "#005FAF", Dark: "#00D7FF"}
)

// Model is the picker's bubbletea model.
type Model struct {
	items       []Item // Original items
	filtered    []Item // Items after filtering
	selected    int    // Currently selected index in filtered
	input       textinput.Model
	scorer      Scorer
	maxVisible  int // Max items to show (fixed, not dynamic)
	scrollStart int // First visible item index
	width       int
	height      int
	cancelled   bool
	confirmed   bool
	renderer    *lipgloss.Renderer

	// Styles
	promptStyle   lipgloss.Style
	cursorStyle   lipgloss.Style
	matchStyle    lipgloss.Style
	selectedStyle lipgloss.Style
	normalStyle   lipgloss.Style
	countStyle    lipgloss.Style
	helpStyle     lipgloss.Style
	borderStyle   lipgloss.Style
}

// New creates a new picker with the given items, scorer, and lipgloss renderer.
// The renderer controls which output the color profile is detected from.
// When rendering to stderr (e.g. fgo piping stdout), pass a renderer created
// from os.Stderr so colors are detected against the actual TTY.
func New(items []Item, scorer Scorer, renderer *lipgloss.Renderer) Model {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50
	ti.PromptStyle = renderer.NewStyle().Foreground(colorFocus)
	ti.TextStyle = renderer.NewStyle().Foreground(colorText)
	ti.PlaceholderStyle = renderer.NewStyle().Foreground(colorPlaceholder)
	ti.Cursor.Style = renderer.NewStyle().Foreground(colorFocus)

	m := Model{
		items:      items,
		filtered:   items,
		input:      ti,
		scorer:     scorer,
		renderer:   renderer,
		maxVisible: 10, // Fixed height like fzf --height

		promptStyle:   renderer.NewStyle().Foreground(colorFocus).Bold(true),
		cursorStyle:   renderer.NewStyle().Foreground(colorFocus),
		matchStyle:    renderer.NewStyle().Foreground(colorSelected).Bold(true),
		selectedStyle: renderer.NewStyle().Foreground(colorSelected).Bold(true),
		normalStyle:   renderer.NewStyle().Foreground(colorText),
		countStyle:    renderer.NewStyle().Foreground(colorPlaceholder),
		helpStyle:     renderer.NewStyle().Foreground(colorPlaceholder).Faint(true),
		borderStyle:   renderer.NewStyle().Foreground(colorBorder),
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

		case "tab":
			if len(m.filtered) > 0 {
				m.selected = (m.selected + 1) % len(m.filtered)
				m.ensureVisible()
			}
			return m, nil

		case "shift+tab":
			if len(m.filtered) > 0 {
				m.selected = (m.selected - 1 + len(m.filtered)) % len(m.filtered)
				m.ensureVisible()
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		// Only update width for input, keep maxVisible fixed for inline display
		m.width = msg.Width
		m.height = msg.Height
		if msg.Width > 4 {
			m.input.Width = msg.Width - 4
		}
		// Don't override maxVisible - keep it fixed for fzf-like inline display
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

	// Top border with count
	count := fmt.Sprintf(" %d/%d ", len(m.filtered), len(m.items))
	topBorder := m.borderStyle.Render("─") + m.countStyle.Render(count) + m.borderStyle.Render(strings.Repeat("─", 50))
	b.WriteString(topBorder + "\n")

	// Prompt line
	prompt := m.promptStyle.Render("> ")
	b.WriteString(prompt)
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Items
	endIdx := m.scrollStart + m.maxVisible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	// Align the optional prefix label and trailing detail (e.g. progress bars)
	// into columns when present.
	nameWidth, prefixWidth, showDetail := 0, 0, false
	for _, it := range m.filtered {
		if it.Detail != "" {
			showDetail = true
		}
		if w := lipgloss.Width(it.Name); w > nameWidth {
			nameWidth = w
		}
		if w := lipgloss.Width(it.Prefix); w > prefixWidth {
			prefixWidth = w
		}
	}

	for i := m.scrollStart; i < endIdx; i++ {
		item := m.filtered[i]
		name := item.Name
		if showDetail {
			name = padRight(name, nameWidth)
		}

		prefix := ""
		if prefixWidth > 0 {
			prefix = padRight(m.renderPrefix(item), prefixWidth) + " "
		}

		// Cursor, prefix label, and item name
		if i == m.selected {
			b.WriteString(m.cursorStyle.Render("▶ ") + prefix + m.selectedStyle.Render(name))
		} else {
			b.WriteString("  " + prefix + m.normalStyle.Render(name))
		}
		if showDetail && item.Detail != "" {
			b.WriteString("  " + item.Detail)
		}
		b.WriteString("\n")
	}

	// Padding for empty space (only if we have fewer items than maxVisible)
	displayed := endIdx - m.scrollStart
	for i := displayed; i < m.maxVisible && i < len(m.items); i++ {
		b.WriteString("\n")
	}

	// Help line
	help := m.helpStyle.Render("↑/↓/tab: navigate • enter: select • esc: cancel")
	b.WriteString(help)

	return b.String()
}

// renderPrefix colorizes an item's prefix label with its PrefixColor using the
// picker's renderer, or returns the plain prefix when no color is set.
func (m Model) renderPrefix(item Item) string {
	if item.Prefix == "" || item.PrefixColor == nil || m.renderer == nil {
		return item.Prefix
	}
	return m.renderer.NewStyle().Foreground(item.PrefixColor).Render(item.Prefix)
}

func padRight(s string, width int) string {
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
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
// Renders inline (no alt screen) for fzf-like experience.
func Run(items []Item, scorer Scorer) (*Item, error) {
	debug := os.Getenv("FEST_DEBUG") != ""

	// Create a renderer bound to stderr so the color profile is detected against
	// the actual TTY, not stdout which may be a pipe (e.g. fgo captures stdout).
	renderer := lipgloss.NewRenderer(os.Stderr)

	start := time.Now()
	m := New(items, scorer, renderer)
	if debug {
		log.Printf("[DEBUG] picker.New: %v", time.Since(start))
	}

	start = time.Now()
	// Use stderr for rendering so stdout is clean for `cd $(fest go)`
	// No AltScreen - render inline like fzf for a lightweight experience
	p := tea.NewProgram(m,
		tea.WithOutput(os.Stderr),
		tea.WithInput(os.Stdin),
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
