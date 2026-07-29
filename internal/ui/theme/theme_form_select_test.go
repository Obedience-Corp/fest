package theme_test

import (
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/ui/theme"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Content-sized select (width 0) must paint a complete rounded box.
func TestThemedSelectViewHasRightBorder(t *testing.T) {
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
	// width 0: content-sized (do not call WithWidth)
	view := sel.View()
	t.Logf("view:\n%s", view)
	t.Logf("first line W=%d", lipgloss.Width(strings.Split(view, "\n")[0]))
	if !strings.Contains(view, "╮") || !strings.Contains(view, "╯") {
		t.Fatalf("missing right corners")
	}
}
