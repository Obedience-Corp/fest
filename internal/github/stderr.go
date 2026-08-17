package github

import (
	"errors"
	"os/exec"
	"strings"
)

// gitStderr returns the message git printed before failing, or "" when there is
// none to report.
//
// exec.Cmd.Output populates ExitError.Stderr but callers have to ask for it.
// Without this, every remote failure collapses to "exit status 128", which is
// git's generic fatal and says nothing about whether the machine has no route
// to the host, cannot verify a TLS certificate, or was denied access. The
// operator needs git's own sentence, not our exit code.
//
// Output is capped: a pathological remote could return a great deal of text,
// and this lands in a one-line CLI error.
func gitStderr(err error) string {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return ""
	}
	msg := strings.TrimSpace(string(exitErr.Stderr))
	if msg == "" {
		return ""
	}
	// git prefixes its failures with "fatal: "; keep the sentence, drop the
	// ceremony, and collapse multi-line output onto one line.
	lines := make([]string, 0, 4)
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, strings.TrimPrefix(line, "fatal: "))
		if len(lines) == 4 {
			break
		}
	}
	joined := strings.Join(lines, "; ")
	const maxLen = 300
	if len(joined) > maxLen {
		return joined[:maxLen] + "..."
	}
	return joined
}
