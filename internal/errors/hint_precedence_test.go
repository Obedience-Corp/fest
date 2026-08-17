package errors

import (
	"fmt"
	"strings"
	"testing"
)

func hintOf(t *testing.T, err error) string {
	t.Helper()
	rendered := err.Error()
	_, hint, found := strings.Cut(rendered, "\nHint: ")
	if !found {
		return ""
	}
	if strings.Contains(hint, "\nHint: ") {
		t.Errorf("rendered more than one hint:\n%s", rendered)
	}
	return hint
}

// The defect. Validation attaches "Run 'fest help'" from the error class alone,
// with no knowledge of what failed. Under plain innermost-wins that catch-all
// displaced the hint the call site wrote for the actual operation, which is the
// same dead end as being told to check file permissions on a network failure.
//
// Observed for real: 'fest init' against an unreachable remote printed
// "Run 'fest help' for more information" instead of its connection advice.
func TestSpecificHintBeatsAnInnerClassHint(t *testing.T) {
	inner := Validation("git command not found")
	middle := fmt.Errorf("resolving latest dev tag: %w", inner)
	outer := Wrap(middle, "auto-sync failed").
		WithHint("check your internet connection or run 'fest sync' manually")

	if got, want := hintOf(t, outer), "check your internet connection or run 'fest sync' manually"; got != want {
		t.Errorf("hint = %q, want %q", got, want)
	}
	if strings.Contains(outer.Error(), HintSeeHelp) {
		t.Errorf("the class-level hint still surfaced:\n%s", outer.Error())
	}
}

// The rule this must not break. A hint a constructor derived from the actual
// failure is specific, so innermost still wins over an outer call-site hint:
// the TLS case from the visible-hints work, where the inner layer knows it is a
// certificate problem and the outer one can only guess at the network.
func TestDerivedInnerHintStillBeatsAnOuterHint(t *testing.T) {
	inner := Network("listing remote tags",
		fmt.Errorf("exit status 128"),
		"server certificate verification failed. CAfile: none")
	outer := Wrap(inner, "auto-sync failed").
		WithHint("check your internet connection or run 'fest sync' manually")

	if got := hintOf(t, outer); got != HintCheckTLS {
		t.Errorf("hint = %q, want the TLS hint %q", got, HintCheckTLS)
	}
}

// With nothing better anywhere, a class-level hint is still better than none,
// and the innermost one is the most relevant of them.
func TestClassHintsAreUsedWhenNothingIsMoreSpecific(t *testing.T) {
	inner := IO("writing file", fmt.Errorf("permission denied"))
	outer := Wrap(inner, "saving progress")

	if got := hintOf(t, outer); got != HintCheckPermissions {
		t.Errorf("hint = %q, want %q", got, HintCheckPermissions)
	}
}

// WithHint on the same error that carried a class hint replaces it outright.
func TestWithHintOverridesTheClassHintOnItsOwnError(t *testing.T) {
	err := Validation("bad --status value").
		WithHint("valid values are: active, ready, planning")

	if got, want := hintOf(t, err), "valid values are: active, ready, planning"; got != want {
		t.Errorf("hint = %q, want %q", got, want)
	}
}

// An outer class hint must not beat an inner specific one either; specificity
// is what decides, and depth only breaks ties within the same kind.
func TestInnerSpecificHintBeatsAnOuterClassHint(t *testing.T) {
	inner := New("evidence path escapes the phase directory").
		WithHint("list only paths inside the phase directory")
	outer := Config("loading gates").WithField("k", "v")
	outer.Err = inner

	if got, want := hintOf(t, outer), "list only paths inside the phase directory"; got != want {
		t.Errorf("hint = %q, want %q", got, want)
	}
}

// A chain with no hints anywhere renders none, rather than an empty "Hint:".
func TestNoHintAnywhereRendersNoHintLine(t *testing.T) {
	err := Wrap(fmt.Errorf("underlying"), "outer")
	if strings.Contains(err.Error(), "Hint:") {
		t.Errorf("rendered a hint line for a chain with no hints:\n%s", err.Error())
	}
}

// The message chain must survive whichever hint wins.
func TestPrecedenceDoesNotDisturbTheMessageChain(t *testing.T) {
	inner := Validation("git command not found")
	middle := fmt.Errorf("resolving latest dev tag: %w", inner)
	outer := Wrap(middle, "auto-sync failed").WithHint("check your internet connection")

	for _, want := range []string{"auto-sync failed", "resolving latest dev tag", "git command not found"} {
		if !strings.Contains(outer.Error(), want) {
			t.Errorf("message chain lost %q:\n%s", want, outer.Error())
		}
	}
}
