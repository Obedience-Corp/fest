package guidance

import (
	"strings"
	"testing"
)

// TestExecutorBlockerPolicyText writes the sentence out a second time on
// purpose. A test that compares the constant to itself proves nothing; this one
// fails the moment someone edits the constant, which is the drift design doc 05
// scenario A12 names.
func TestExecutorBlockerPolicyText(t *testing.T) {
	want := "Blocked is a request for a human decision. Executors cannot defer blockers. " +
		"Exhaust every unblock option first; blocks without recorded attempts are sent back."
	if ExecutorBlockerPolicy != want {
		t.Errorf("policy sentence changed:\n got: %q\nwant: %q", ExecutorBlockerPolicy, want)
	}
}

func TestExecutorBlockerPolicyHasNoEmDash(t *testing.T) {
	if strings.ContainsRune(ExecutorBlockerPolicy, '—') {
		t.Errorf("policy sentence contains an em dash: %q", ExecutorBlockerPolicy)
	}
}
