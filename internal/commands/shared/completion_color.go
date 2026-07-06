package shared

import (
	"strings"

	"github.com/Obedience-Corp/fest/internal/ui"
)

// ansiReset clears SGR attributes after a colorized completion cell.
const ansiReset = "\x1b[0m"

// colorCompletionDisplay renders the zsh compadd -d display cell for a completion:
// the value followed by its status, colorized from the active theme palette.
func colorCompletionDisplay(value, status string) string {
	return value + " " + ui.StatusColorSequence(status) + status + ansiReset
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
