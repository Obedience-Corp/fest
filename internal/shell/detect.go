package shell

import (
	"os"
	"path/filepath"
	"strings"
)

// FallbackShell is what fest suggests when it cannot identify the user's shell.
// POSIX sh is the safe default: its script parses under bash and zsh too, so a
// wrong guess still produces working integration rather than a syntax error.
const FallbackShell = "sh"

// DetectFromEnv names the shell fest should generate integration for, based on
// $SHELL. Bourne-family shells that are not bash or zsh (dash, ash, busybox,
// and the plain /bin/sh on minimal and embedded systems) map to "sh" rather
// than being reported as unsupported: they run the POSIX script correctly.
// An unset or unrecognized $SHELL yields FallbackShell.
func DetectFromEnv() string {
	return detect(os.Getenv("SHELL"))
}

// detect maps a shell path to a supported shell name. Split out from
// DetectFromEnv so the mapping is testable without touching the environment.
func detect(shellPath string) string {
	base := strings.TrimSpace(filepath.Base(strings.TrimSpace(shellPath)))
	// Login shells appear as "-sh" / "-bash" in some environments.
	base = strings.TrimPrefix(base, "-")
	switch base {
	case "zsh":
		return "zsh"
	case "bash":
		return "bash"
	case "fish":
		return "fish"
	case "sh", "ash", "dash", "busybox", "mksh", "yash":
		// Verified to run the POSIX script correctly, including its use of
		// 'local'. ksh is deliberately absent: it parses the script but does
		// not implement 'local', so it falls through to the same script via
		// FallbackShell and is stopped by the script's own guard with an
		// explanation, rather than being told here that it is supported.
		return "sh"
	default:
		return FallbackShell
	}
}

// InitCommand returns the exact line the user should run (or add to their
// profile) to load fest's integration for the named shell. fish reads the
// script from a pipe; everything else evals it.
//
// This exists so no surface hardcodes a shell in a hint. Telling a dash user to
// run the bash script produces a syntax error rather than integration, which is
// exactly the dead end this replaces.
func InitCommand(shellType string) string {
	if shellType == "fish" {
		return "fest shell-init fish | source"
	}
	if !IsSupported(shellType) {
		shellType = FallbackShell
	}
	return `eval "$(fest shell-init ` + shellType + `)"`
}

// InitHint returns the init command for the shell fest detected from $SHELL.
// Use it for any operator-facing "shell integration is not loaded" message, so
// the line offered is one that actually parses in the shell reading it.
func InitHint() string {
	return InitCommand(DetectFromEnv())
}
