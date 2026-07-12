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
// lipgloss (never bubbletea, huh, or glamour, directly or transitively).
// internal/commands blank-imports it at the top of its import block: the
// bginit path sorts ahead of every bubbletea-linked import there, so gofmt
// keeps it first and the compiler records its inittask ahead of the bubbletea
// subtree. Every binary built on internal/commands inherits the seed, and
// depgraph_test.go enforces the no-bubbletea import rule structurally.
//
// The seed approximates termenv's own non-query fallback: the last field of
// COLORFGBG is the background ANSI color, and a missing, unparsable, or
// out-of-range value means dark, matching the CLI palette's existing
// "adaptive falls back to dark" behavior. It intentionally diverges from
// termenv for ANSI 8 (#808080), which termenv's luminance math reads as
// light; bginit keeps 8 dark as the safer readability default. An explicit
// theme choice refines this later via ui.InitPalette.
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
	// Anything outside the 0..15 ANSI range is unknown; default to dark.
	if bg < 0 || bg > 15 {
		return true
	}
	// Dark backgrounds are ANSI 0..6 and 8; 7 and bright 9..15 read light.
	// Treating 8 as dark diverges from termenv on purpose (see package doc).
	return bg <= 8 && bg != 7
}
