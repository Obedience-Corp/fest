package shared

import (
	"fmt"
	"strings"
)

// ANSI 256-color codes matching the fest dark palette (ui/palette.go). These are
// emitted for shell completion display cells (zsh compadd -d), which lipgloss
// styling does not reach.
const (
	ansiReset     = "\033[0m"
	ansiDim       = "\033[2m"
	ansiActive    = "\033[38;5;42m"  // bright green
	ansiReady     = "\033[38;5;220m" // yellow
	ansiPlanned   = "\033[38;5;33m"  // blue
	ansiCompleted = "\033[38;5;205m" // magenta
	ansiDungeon   = "\033[38;5;248m" // light grey
)

// StatusColor returns the ANSI color escape for a festival status directory name.
func StatusColor(status string) string {
	switch status {
	case "active":
		return ansiActive
	case "ready":
		return ansiReady
	case "planning":
		return ansiPlanned
	case "completed":
		return ansiCompleted
	case "dungeon", "dungeon/completed", "dungeon/archived", "dungeon/someday":
		return ansiDungeon
	default:
		return ansiDim
	}
}

// colorCompletionDisplay renders the zsh compadd -d display cell for a completion:
// the value followed by its colorized status.
func colorCompletionDisplay(value, status string) string {
	return fmt.Sprintf("%s %s%s%s", value, StatusColor(status), status, ansiReset)
}

// OrderedSelectorNames returns completion names from picker candidates while
// preserving candidate order (active → ready → planning). When toComplete is set
// it fuzzy-filters but keeps the status order among the matches, rather than
// re-sorting them by fuzzy score.
func OrderedSelectorNames(candidates []FestivalPickCandidate, toComplete string) []string {
	selectors := selectorCandidatesFromPickCandidates(candidates, false)
	var matched map[string]bool
	if strings.TrimSpace(toComplete) != "" {
		matched = make(map[string]bool)
		for _, m := range fuzzyMatchSelectorCandidates(toComplete, selectors) {
			matched[m.Path] = true
		}
	}
	names := make([]string, 0, len(selectors))
	for _, c := range selectors {
		if matched != nil && !matched[c.Path] {
			continue
		}
		names = append(names, c.Name)
	}
	return names
}

// ColorSelectorCompletions returns "value<TAB>display" lines for zsh compadd -d,
// preserving candidate order and colorizing each entry by its status. When
// toComplete is set it fuzzy-filters like OrderedSelectorNames.
func ColorSelectorCompletions(candidates []FestivalPickCandidate, toComplete string) []string {
	statusByName := make(map[string]string, len(candidates))
	for _, c := range candidates {
		if c.StatusDirectory {
			continue
		}
		if _, ok := statusByName[c.Name]; !ok {
			statusByName[c.Name] = c.Status
		}
	}
	names := OrderedSelectorNames(candidates, toComplete)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, name+"\t"+colorCompletionDisplay(name, statusByName[name]))
	}
	return lines
}
