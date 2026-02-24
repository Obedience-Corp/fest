package explore

import (
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
)

// mdRenderer caches a glamour renderer and its configured width.
// Thread-safe for use from async tea.Cmd goroutines.
type mdRenderer struct {
	mu       sync.Mutex
	renderer *glamour.TermRenderer
	width    int
}

// render renders markdown content, reusing the cached renderer when possible.
func (r *mdRenderer) render(content string, width int) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if width < 20 {
		width = 60
	}

	if r.renderer == nil || r.width != width {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width-4),
		)
		if err != nil {
			return content
		}
		r.renderer = renderer
		r.width = width
	}

	rendered, err := r.renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimRight(rendered, "\n")
}

// loadPreview reads a file, strips frontmatter, and returns raw content for rendering.
func loadPreview(path string) string {
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return stripFrontmatter(string(data))
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
	case ItemStatus:
		return ""
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
