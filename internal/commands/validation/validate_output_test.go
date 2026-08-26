package validation

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/ui"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	_ = r.Close()
	return buf.String()
}

func TestPrintValidationResult_FailingRunOmitsAllChecksPassed(t *testing.T) {
	display := ui.New(true, false)
	result := &ValidationResult{
		Festival: "missing-gates",
		Score:    0,
		Valid:    false,
		Issues: []ValidationIssue{{
			Level:   LevelError,
			Code:    CodeMissingQualityGate,
			Message: "Festival is missing quality gates",
			Path:    "gates/",
		}},
	}

	out := captureStdout(t, func() {
		printValidationResult(display, t.TempDir(), result)
	})
	if !strings.Contains(out, "VALIDATION FAILED") {
		t.Fatalf("expected VALIDATION FAILED banner, got:\n%s", out)
	}
	if strings.Contains(out, "All checks passed") {
		t.Fatalf("failing run must not print All checks passed for empty categories, got:\n%s", out)
	}
}

func TestPrintValidationResult_CleanRunStillSaysAllChecksPassed(t *testing.T) {
	display := ui.New(true, false)
	result := &ValidationResult{
		Festival: "clean",
		Score:    100,
		Valid:    true,
	}

	out := captureStdout(t, func() {
		printValidationResult(display, t.TempDir(), result)
	})
	if !strings.Contains(out, "VALIDATION PASSED") {
		t.Fatalf("expected VALIDATION PASSED banner, got:\n%s", out)
	}
	if !strings.Contains(out, "All checks passed") {
		t.Fatalf("clean run should still report empty categories as passed, got:\n%s", out)
	}
}
