// Package bginit seeds lipgloss's background-color cache before any
// charmbracelet package init can query the terminal.
//
// bubbletea v1 calls lipgloss.HasDarkBackground() from a package init
// (tea_init.go). Without a cached value, lipgloss asks termenv, which writes
// OSC 11 / DSR queries to the tty and blocks up to termenv.OSCTimeout (5s)
// waiting for a reply. Interactive terminals answer instantly, but recorder,
// CI, and agent ptys that never answer freeze every fest command for the full
// timeout before a single byte of output appears.
//
// Seeding an explicit value makes lipgloss skip the terminal query entirely.
// This package must initialize before bubbletea, so it may only import
// lipgloss (never bubbletea or huh, directly or transitively), and cmd/fest
// must import it alongside internal/commands: its path sorts before
// internal/commands, so gofmt keeps it first and the compiler records its
// inittask ahead of the bubbletea subtree.
//
// The seed mirrors termenv's own non-query fallback: the last field of
// COLORFGBG is the background ANSI color, and a missing or unparsable value
// means dark, matching the CLI palette's existing "adaptive falls back to
// dark" behavior. An explicit theme choice refines this later via
// ui.InitPalette.
package bginit

import (
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func init() {
	lipgloss.SetHasDarkBackground(backgroundIsDark(os.Getenv("COLORFGBG")))
}

func backgroundIsDark(colorFGBG string) bool {
	if !strings.Contains(colorFGBG, ";") {
		return true
	}
	fields := strings.Split(colorFGBG, ";")
	bg, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil {
		return true
	}
	return bg >= 0 && bg <= 8 && bg != 7
}
