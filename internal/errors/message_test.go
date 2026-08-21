package errors

import (
	"fmt"
	"strings"
	"testing"
)

// A hint reads as advice about what to do next, so it belongs on the last line
// of a failure. Splicing it into the middle of a warning the command then
// recovers from offers a next step for something already handled.
func TestMessageStripsTheHint(t *testing.T) {
	err := Validation("could not reach the remote").
		WithHint("check your internet connection or run 'fest sync' manually")

	if !strings.Contains(err.Error(), "Hint: ") {
		t.Fatal("precondition failed: Error() should render a hint")
	}

	got := Message(err)
	if strings.Contains(got, "Hint: ") {
		t.Errorf("Message() kept the hint: %q", got)
	}
	if !strings.Contains(got, "could not reach the remote") {
		t.Errorf("Message() dropped the message: %q", got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Errorf("Message() left a trailing newline: %q", got)
	}
}

// The nested case is the one that matters: an outer wrap renders one hint at
// the end, and stripping must not take any of the message chain with it.
func TestMessageKeepsTheWholeChain(t *testing.T) {
	inner := Validation("certificate could not be verified").WithHint("install CA certificates")
	middle := fmt.Errorf("resolving latest dev tag: %w", inner)
	outer := Wrap(middle, "auto-sync failed")

	got := Message(outer)
	for _, want := range []string{"auto-sync failed", "resolving latest dev tag", "certificate could not be verified"} {
		if !strings.Contains(got, want) {
			t.Errorf("Message() lost %q from the chain: %q", want, got)
		}
	}
	if strings.Contains(got, "Hint: ") {
		t.Errorf("Message() kept a hint: %q", got)
	}
}

// Fields are JSON metadata. The terminal path is Error(), which never splices
// them in — so a readiness block that puts the missing path only in WithField
// hides it from the operator. Put identifying facts in the message or Hint.
func TestErrorStringOmitsFields(t *testing.T) {
	err := Validation("missing required deliverable").
		WithField("path", "SEQUENCE_GOAL.md").
		WithHint("add the file and retry")

	got := err.Error()
	if strings.Contains(got, "SEQUENCE_GOAL.md") {
		t.Errorf("Error() rendered a WithField value; fields are JSON-only: %q", got)
	}
	if !strings.Contains(got, "missing required deliverable") {
		t.Errorf("Error() dropped the message: %q", got)
	}
	if !strings.Contains(got, "Hint: add the file and retry") {
		t.Errorf("Error() dropped the hint: %q", got)
	}
}

// Leaf helpers may return fmt.Errorf. The command boundary lifts them with Wrap
// so the operator still sees the leaf text.
func TestWrapLiftsFmtErrorfCause(t *testing.T) {
	leaf := fmt.Errorf("fest_working_dir must be relative, got %q", "/abs")
	got := Wrap(leaf, "normalizing working dir").WithOp("validate").Error()
	for _, want := range []string{
		"validate",
		"normalizing working dir",
		"fest_working_dir must be relative",
		"/abs",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Wrap of fmt.Errorf lost %q: %q", want, got)
		}
	}
}

func TestMessageOnPlainAndNilErrors(t *testing.T) {
	if got := Message(nil); got != "" {
		t.Errorf("Message(nil) = %q, want empty", got)
	}
	plain := fmt.Errorf("something broke")
	if got := Message(plain); got != "something broke" {
		t.Errorf("Message(plain) = %q, want %q", got, "something broke")
	}
}
