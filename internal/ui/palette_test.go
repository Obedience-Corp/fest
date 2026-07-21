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
		"active status":     {got: got.Active, want: shared.StatusSuccess},
		"ready status":      {got: got.Ready, want: shared.StatusWarning},
		"planning status":   {got: got.Planning, want: "#4DA3FF"},
		"ritual status":     {got: got.Ritual, want: "#B388FF"},
		"completed status":  {got: got.Completed, want: "#FF79C6"},
		"festival entity":   {got: got.Festival, want: shared.Accent},
		"task entity":       {got: got.Task, want: "#B388FF"},
		"in-progress state": {got: got.InProgress, want: shared.StatusWarning},
		"blocked state":     {got: got.Blocked, want: shared.StatusError},
		"border":            {got: got.Border, want: shared.Border},
		"primary value":     {got: got.Value, want: shared.TextPrimary},
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

func TestFestivalStatusColorsStayDistinctAcrossThemes(t *testing.T) {
	tests := []struct {
		name string
		mode sharedbrand.Mode
		want [3]string
	}{
		{name: "dark", mode: sharedbrand.ModeDark, want: [3]string{"#4DA3FF", "#B388FF", "#FF79C6"}},
		{name: "light", mode: sharedbrand.ModeLight, want: [3]string{"#1D4ED8", "#7C3AED", "#A21CAF"}},
		{name: "high contrast", mode: sharedbrand.ModeHighContrast, want: [3]string{"#60A5FA", "#D8B4FE", "#FF79C6"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shared := sharedbrand.Resolve(tt.mode, sharedbrand.Capabilities{
				IsTTY:           true,
				ColorDepth:      sharedbrand.ColorTrueColor,
				DarkBackground:  tt.mode != sharedbrand.ModeLight,
				BackgroundKnown: true,
			})
			got := paletteFromShared(shared)
			colors := []lipgloss.TerminalColor{got.Planning, got.Ritual, got.Completed}
			for i, color := range colors {
				actual, ok := color.(lipgloss.Color)
				if !ok || string(actual) != tt.want[i] {
					t.Fatalf("status color %d = %T(%v), want %q", i, color, color, tt.want[i])
				}
			}
			statusColors := []struct {
				name  string
				color lipgloss.TerminalColor
			}{
				{name: "active", color: got.Active},
				{name: "ready", color: got.Ready},
				{name: "planning", color: got.Planning},
				{name: "ritual", color: got.Ritual},
				{name: "completed", color: got.Completed},
			}
			seen := make(map[string]string, len(statusColors))
			for _, status := range statusColors {
				value := string(status.color.(lipgloss.Color))
				if previous, exists := seen[value]; exists {
					t.Fatalf("%s and %s both resolve to %q", status.name, previous, value)
				}
				seen[value] = status.name
			}
		})
	}
}

func TestFestivalStatusColorsDisableInPlainMode(t *testing.T) {
	shared := sharedbrand.Resolve(sharedbrand.ModePlain, sharedbrand.Capabilities{})
	planning, ritual, completed := festivalStatusColors(shared)
	task := festivalTaskColor(shared)
	if planning != "" || ritual != "" || completed != "" || task != "" {
		t.Fatalf("plain colors = %q, %q, %q, %q; want all empty", planning, ritual, completed, task)
	}
}

func TestInteractivePaletteKeepsColorsWhenOutputPaletteIsPlain(t *testing.T) {
	ResetPalette()
	t.Cleanup(func() {
		SetNoColor(false)
		ResetPalette()
	})
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("NO_COLOR", "")

	// Simulate a shell wrapper capturing stdout for a selected path. The
	// interactive picker still renders to a color-capable stderr TTY.
	configuredMode = sharedbrand.ModeDark
	setPalette(sharedbrand.Resolve(sharedbrand.ModePlain, sharedbrand.Capabilities{}))

	got := InteractivePalette()
	if got.Planning == nil || got.Ready == nil {
		t.Fatalf("interactive palette lost colors: planning=%v ready=%v", got.Planning, got.Ready)
	}
	if string(got.Planning.(lipgloss.Color)) != "#4DA3FF" {
		t.Fatalf("interactive planning color = %q, want #4DA3FF", got.Planning)
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
