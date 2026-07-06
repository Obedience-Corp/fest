package ui

import "testing"

func TestStatusColorSequence(t *testing.T) {
	ResetPalette()
	cases := map[string]string{
		"active":            "\x1b[38;5;42m",
		"ready":             "\x1b[38;5;220m",
		"planning":          "\x1b[38;5;33m",
		"ritual":            "\x1b[38;5;141m",
		"completed":         "\x1b[38;5;205m",
		"dungeon/completed": "\x1b[38;5;205m",
		"dungeon/someday":   "\x1b[38;5;139m",
		"dungeon/archived":  "\x1b[38;5;250m",
		"dungeon":           "\x1b[38;5;248m",
		"unknown-status":    "\x1b[2m",
	}
	for status, want := range cases {
		if got := StatusColorSequence(status); got != want {
			t.Errorf("StatusColorSequence(%q) = %q, want %q", status, got, want)
		}
	}
}
