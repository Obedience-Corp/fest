package github

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// The whole point: an operator saw "exit status 128" and no cause. git had
// already said what was wrong on stderr, and Cmd.Output had captured it.
func TestGitStderrSurfacesGitsOwnMessage(t *testing.T) {
	cmd := exec.Command("git", "ls-remote", "--tags", "https://invalid.invalid/nope.git")
	_, err := cmd.Output()
	if err == nil {
		t.Skip("the unreachable remote resolved; no failure to inspect")
	}

	got := gitStderr(err)
	if got == "" {
		t.Fatalf("gitStderr returned nothing for a failed ls-remote (err = %v)", err)
	}
	if strings.Contains(got, "exit status") {
		t.Errorf("gitStderr returned the exit code rather than the reason: %q", got)
	}
	t.Logf("git said: %s", got)
}

func TestGitStderrFormatting(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   string
	}{
		{"strips the fatal prefix", "fatal: could not read Username\n", "could not read Username"},
		{"joins multiple lines", "fatal: one\nfatal: two\n", "one; two"},
		{"drops blank lines", "fatal: one\n\n\n", "one"},
		{"empty stays empty", "   \n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := error(&exec.ExitError{Stderr: []byte(tt.stderr)})
			if got := gitStderr(err); got != tt.want {
				t.Errorf("gitStderr(%q) = %q, want %q", tt.stderr, got, tt.want)
			}
		})
	}
}

// A pathological remote must not blow up a one-line CLI error.
func TestGitStderrIsBounded(t *testing.T) {
	err := error(&exec.ExitError{Stderr: []byte("fatal: " + strings.Repeat("x", 5000))})
	if got := gitStderr(err); len(got) > 320 {
		t.Errorf("gitStderr returned %d chars; want it capped", len(got))
	}
}

// Not every failure is an ExitError (git missing entirely, context cancelled).
// Those have no stderr to quote and must not panic.
func TestGitStderrIgnoresNonExitErrors(t *testing.T) {
	if got := gitStderr(errors.New("boom")); got != "" {
		t.Errorf("gitStderr(non-ExitError) = %q, want empty", got)
	}
	if got := gitStderr(nil); got != "" {
		t.Errorf("gitStderr(nil) = %q, want empty", got)
	}
}
