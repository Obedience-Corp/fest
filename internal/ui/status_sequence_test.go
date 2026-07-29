package ui

import (
	"testing"

	sharedbrand "github.com/Obedience-Corp/obey-shared/brand"
)

func TestStatusColorSequence(t *testing.T) {
	ResetPalette()
	cases := map[string]string{
		"active":            "\x1b[38;5;84m",
		"ready":             "\x1b[38;5;214m",
		"planning":          "\x1b[38;5;75m",
		"ritual":            "\x1b[38;5;141m",
		"completed":         "\x1b[38;5;212m",
		"dungeon/completed": "\x1b[38;5;212m",
		"dungeon/someday":   "\x1b[38;5;130m",
		"dungeon/archived":  "\x1b[38;5;145m",
		"dungeon":           "\x1b[38;5;145m",
		"unknown-status":    "\x1b[2m",
	}
	for status, want := range cases {
		if got := StatusColorSequence(status); got != want {
			t.Errorf("StatusColorSequence(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestCompletionStatusColorSequenceSurvivesPipedOutput(t *testing.T) {
	ResetPalette()
	t.Cleanup(ResetPalette)

	// A shell completion command writes through a pipe, which resolves ordinary
	// command output to plain even though zsh can render the display cells.
	setPalette(sharedbrand.Resolve(sharedbrand.ModePlain, sharedbrand.Capabilities{}))

	if got := StatusColorSequence("planning"); got != "" {
		t.Fatalf("ordinary piped status sequence = %q, want no color", got)
	}
	if got := CompletionStatusColorSequence("planning"); got != "\x1b[38;5;75m" {
		t.Fatalf("completion planning sequence = %q, want blue ANSI sequence", got)
	}
	if got := CompletionAccentSequence(); got != "\x1b[38;5;214m" {
		t.Fatalf("completion accent sequence = %q, want amber ANSI sequence", got)
	}
}
