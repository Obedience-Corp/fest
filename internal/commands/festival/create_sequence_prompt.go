package festival

import (
	"os"

	"golang.org/x/term"
)

// createSequenceStdinIsTTY reports whether stdin is an interactive terminal.
// Overridden in tests.
var createSequenceStdinIsTTY = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// shouldPromptCreateTaskFiles reports whether the optional
// "Create task files now?" Y/n prompt should be shown.
//
// Non-TTY stdin must not block: agents that omit --json/--no-prompt previously
// hung in UI.Confirm (issue #343).
func shouldPromptCreateTaskFiles(opts *CreateSequenceOptions) bool {
	if opts == nil {
		return false
	}
	if opts.NoPrompt || opts.JSONOutput || opts.SkipMarkers {
		return false
	}
	return createSequenceStdinIsTTY()
}
