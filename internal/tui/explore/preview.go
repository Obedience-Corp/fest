package explore

import (
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
)

// loadPreview reads a file, strips frontmatter, and renders markdown for the preview pane.
func loadPreview(path string, width int) string {
	if path == "" {
		return "No preview available"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "No preview available"
	}

	content := stripFrontmatter(string(data))

	if width < 20 {
		width = 60
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)
	if err != nil {
		return content
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}

	return strings.TrimRight(rendered, "\n")
}

// stripFrontmatter removes YAML frontmatter from content.
// Frontmatter is delimited by --- at the start of the file.
func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return content
	}
	after := rest[idx+4:]
	return strings.TrimLeft(after, "\n")
}

// goalFileForItem returns the path to the goal/preview file for a hierarchy item.
func goalFileForItem(item FestivalItem) string {
	switch item.Type {
	case ItemFestival:
		for _, name := range []string{"FESTIVAL_GOAL.md", "FESTIVAL_OVERVIEW.md"} {
			goal := item.Path + "/" + name
			if _, err := os.Stat(goal); err == nil {
				return goal
			}
		}
		return ""
	case ItemPhase:
		return item.Path + "/PHASE_GOAL.md"
	case ItemSequence:
		return item.Path + "/SEQUENCE_GOAL.md"
	case ItemTask:
		return item.Path
	default:
		return ""
	}
}
