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
	want := "A blocked task pauses its dependents. Record unblock attempts. " +
		"If independent work can proceed, defer with 'fest task defer <task> --reason <why> --yes'; " +
		"the blocker returns at the end. Ask for a decision when the path cannot be inferred."
	if ExecutorBlockerPolicy != want {
		t.Errorf("policy sentence changed:\n got: %q\nwant: %q", ExecutorBlockerPolicy, want)
	}
}

func TestExecutorBlockerPolicyHasNoEmDash(t *testing.T) {
	if strings.ContainsRune(ExecutorBlockerPolicy, '—') {
		t.Errorf("policy sentence contains an em dash: %q", ExecutorBlockerPolicy)
	}
}
