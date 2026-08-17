package errors

import (
	goerrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// WithField("hint", ...) is always a mistake: Error() renders e.Hint, never a
// field, so the author's guidance is invisible in the terminal and whatever
// generic hint the constructor attached is shown instead. That is how a failure
// to reach GitHub told an operator to "Check file/directory permissions".
//
// This walks the tree rather than asserting on a list, so a new occurrence
// fails here instead of shipping as a hint nobody sees.
func TestNoHintsHiddenInFields(t *testing.T) {
	root := repoRoot(t)
	var offenders []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "hint_field_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, `WithField("hint"`) {
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, rel+":"+itoa(i+1))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("hints stored as fields are never rendered; use WithHint instead:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// repoRoot walks up from the package directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod above the test directory")
		}
		dir = parent
	}
}

// A remote failure must name the remote as the problem, and carry the message
// the remote itself gave, not a bare exit code.
func TestNetworkErrorReportsCauseAndNetworkHint(t *testing.T) {
	e := Network("listing remote tags", os.ErrDeadlineExceeded,
		"unable to access 'https://github.com/x/y': Could not resolve host: github.com")

	msg := e.Error()
	if !strings.Contains(msg, "Could not resolve host") {
		t.Errorf("error does not carry the remote's own message:\n%s", msg)
	}
	if !strings.Contains(msg, HintCheckNetwork) {
		t.Errorf("error does not carry the network hint:\n%s", msg)
	}
	if strings.Contains(msg, HintCheckPermissions) {
		t.Errorf("a network failure must not tell the operator to check permissions:\n%s", msg)
	}
	if e.Code != ErrCodeNetwork {
		t.Errorf("Code = %q, want %q", e.Code, ErrCodeNetwork)
	}
}

// Detail is optional: a remote failure with nothing to quote still reads as a
// reachability problem rather than as an I/O one.
func TestNetworkErrorWithoutDetail(t *testing.T) {
	e := Network("listing remote tags", os.ErrDeadlineExceeded, "   ")
	if strings.Contains(e.Error(), "  :") {
		t.Errorf("blank detail leaked punctuation into the message:\n%s", e.Error())
	}
	if !strings.Contains(e.Error(), "could not reach the remote") {
		t.Errorf("message does not name the failure:\n%s", e.Error())
	}
}

// A nested error must print one hint, not one per layer. Before this, each
// layer's Error() spliced its own "Hint:" line into the parent's message.
func TestNestedErrorsPrintExactlyOneHint(t *testing.T) {
	inner := Network("listing remote tags", os.ErrClosed, "Could not resolve host: github.com")
	outer := Wrap(inner, "auto-sync failed").WithHint("run 'fest sync' manually")

	msg := outer.Error()
	if got := strings.Count(msg, "Hint:"); got != 1 {
		t.Errorf("rendered %d hints, want exactly 1:\n%s", got, msg)
	}
	// The outermost hint wins: it is the layer closest to what was typed.
	if !strings.Contains(msg, "run 'fest sync' manually") {
		t.Errorf("outermost hint missing:\n%s", msg)
	}
	if strings.Contains(msg, HintCheckNetwork) {
		t.Errorf("inner hint should not also render:\n%s", msg)
	}
	// The cause chain still reaches the operator.
	if !strings.Contains(msg, "Could not resolve host") {
		t.Errorf("cause lost from the chain:\n%s", msg)
	}
}

// Wrapping with no hint of its own must not silently discard the inner
// guidance; the nearest available hint is used.
func TestBareWrapKeepsTheInnerHint(t *testing.T) {
	inner := Network("listing remote tags", os.ErrClosed, "Could not resolve host: github.com")
	outer := Wrap(inner, "auto-sync failed")

	msg := outer.Error()
	if !strings.Contains(msg, HintCheckNetwork) {
		t.Errorf("bare Wrap dropped the inner hint:\n%s", msg)
	}
	if got := strings.Count(msg, "Hint:"); got != 1 {
		t.Errorf("rendered %d hints, want exactly 1:\n%s", got, msg)
	}
}

// errors.Is/As must still see through the chain after the rendering change.
func TestChainRemainsUnwrappable(t *testing.T) {
	inner := Network("listing remote tags", os.ErrClosed, "detail")
	outer := Wrap(inner, "auto-sync failed")
	if !goerrors.Is(outer, os.ErrClosed) {
		t.Error("errors.Is no longer reaches the root cause")
	}
}

// A plain fmt.Errorf between two *Error layers must keep its text. Recursing
// into nested *Error values with errors.As skipped such a layer entirely,
// turning `approval readiness failed: invalid evidence path "x": escapes` into
// `approval readiness failed: escapes`. The middle sentence is the one naming
// what the operator got wrong.
func TestInterveningPlainErrorKeepsItsMessage(t *testing.T) {
	root := Validation("evidence path escapes the phase directory")
	middle := fmt.Errorf("invalid evidence path %q: %w", "../outside.md", root)
	outer := Wrap(middle, "approval readiness failed").WithHint("use a relative path")

	msg := outer.Error()
	for _, want := range []string{
		"approval readiness failed",
		`invalid evidence path "../outside.md"`,
		"evidence path escapes the phase directory",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lost %q:\n%s", want, msg)
		}
	}
	if got := strings.Count(msg, "Hint:"); got != 1 {
		t.Errorf("rendered %d hints, want exactly 1:\n%s", got, msg)
	}
}
