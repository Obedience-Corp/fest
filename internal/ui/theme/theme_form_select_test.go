package theme_test

import (
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/ui/theme"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Full-width form frame around select content must paint complete corners and
// span the terminal (not content-hug).
func TestFormFrameAroundSelectIsFullWidth(t *testing.T) {
	var action string
	sel := huh.NewSelect[string]().
		Title("Create what?").
		Description("j/k navigate").
		Options(
			huh.NewOption("Festival", "festival"),
			huh.NewOption("Phase", "phase"),
			huh.NewOption("Back", "back"),
		).
		Value(&action)
	_ = sel.WithTheme(theme.GetTheme(theme.ThemeDark))
	_ = sel.Focus()
	_ = sel.WithWidth(76)
	inner := sel.View()

	termCols := 80
	out := theme.RenderFormFrame(theme.GetTheme(theme.ThemeDark), termCols, inner)
	t.Logf("view:\n%s", out)

	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		t.Fatal("empty view")
	}
	top := strip(lines[0])
	if lipgloss.Width(top) != termCols {
		t.Fatalf("top line width %d want %d: %q", lipgloss.Width(top), termCols, top)
	}
	if !strings.Contains(out, "╮") || !strings.Contains(out, "╯") {
		t.Fatalf("missing right corners")
	}
	if !strings.Contains(out, "Create what?") {
		t.Fatalf("missing select title in framed view")
	}
}

func strip(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r == 0x1b {
			in = true
			continue
		}
		if in {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				in = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
