package ui

import "testing"

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
