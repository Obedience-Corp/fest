package ui

import (
	"testing"

	sharedbrand "github.com/Obedience-Corp/obey-shared/brand"
	"github.com/charmbracelet/lipgloss"
)

func TestPaletteFromSharedUsesSemanticRoles(t *testing.T) {
	shared := sharedbrand.Resolve(sharedbrand.ModeDark, sharedbrand.Capabilities{
		IsTTY:           true,
		ColorDepth:      sharedbrand.ColorTrueColor,
		DarkBackground:  true,
		BackgroundKnown: true,
	})
	got := paletteFromShared(shared)

	checks := map[string]struct {
		got  lipgloss.TerminalColor
		want string
	}{
		"active status":   {got: got.Active, want: shared.StatusSuccess},
		"planning status": {got: got.Planning, want: shared.AccentHighlight},
		"festival entity": {got: got.Festival, want: shared.Accent},
		"task entity":     {got: got.Task, want: shared.AccentSubtle},
		"blocked state":   {got: got.Blocked, want: shared.StatusError},
		"border":          {got: got.Border, want: shared.Border},
		"primary value":   {got: got.Value, want: shared.TextPrimary},
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			color, ok := check.got.(lipgloss.Color)
			if !ok || string(color) != check.want {
				t.Fatalf("got %T(%v), want lipgloss color %q", check.got, check.got, check.want)
			}
		})
	}
}

func TestResolveBrandPaletteHonorsExplicitNoColor(t *testing.T) {
	ResetPalette()
	SetNoColor(true)
	t.Cleanup(func() { SetNoColor(false); ResetPalette() })

	got := ResolveBrandPalette(sharedbrand.ModeDark)
	if got.Mode != sharedbrand.ModePlain {
		t.Fatalf("resolved mode = %q, want plain", got.Mode)
	}
	if got.ColorEnabled {
		t.Fatal("plain palette must disable color")
	}
}
