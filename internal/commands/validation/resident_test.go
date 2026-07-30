package validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeResidentMarker(t *testing.T, dir, wfType string) {
	t.Helper()
	body := "version: v1alpha8\nkind: workitem\nid: " + wfType + "-thing-2026-07-29\n" +
		"type: " + wfType + "\ntitle: Thing\n"
	if err := os.WriteFile(filepath.Join(dir, ".workitem"), []byte(body), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
}

func TestResidentTarget(t *testing.T) {
	t.Run("path arg pointing at a resident", func(t *testing.T) {
		dir := t.TempDir()
		writeResidentMarker(t, dir, "design")

		target, m := residentTarget(dir)
		if m == nil {
			t.Fatal("expected a marker")
		}
		if m.Type != "design" {
			t.Errorf("Type = %q, want design", m.Type)
		}
		if !filepath.IsAbs(target) {
			t.Errorf("target %q should be absolute", target)
		}
	})

	t.Run("path arg pointing at a plain directory", func(t *testing.T) {
		dir := t.TempDir()
		if _, m := residentTarget(dir); m != nil {
			t.Errorf("expected no marker, got %+v", m)
		}
	})

	t.Run("malformed marker reports no resident", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".workitem"), []byte("kind: workitem\n\tbad: [\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// An unparseable marker must fall through to the normal "not a festival"
		// error rather than being reported as a healthy resident.
		if _, m := residentTarget(dir); m != nil {
			t.Errorf("expected no marker for malformed file, got %+v", m)
		}
	})

	t.Run("empty path arg falls back to cwd", func(t *testing.T) {
		dir := t.TempDir()
		writeResidentMarker(t, dir, "explore")
		restore := chdirForTest(t, dir)
		defer restore()

		_, m := residentTarget("")
		if m == nil {
			t.Fatal("expected a marker from cwd")
		}
		if m.Type != "explore" {
			t.Errorf("Type = %q, want explore", m.Type)
		}
	})
}

func chdirForTest(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { _ = os.Chdir(prev) }
}

func TestEmitResidentInfo_JSONShape(t *testing.T) {
	dir := t.TempDir()
	writeResidentMarker(t, dir, "design")
	_, m := residentTarget(dir)
	if m == nil {
		t.Fatal("fixture did not produce a marker")
	}

	// Build the same result emitResidentInfo builds, and assert the envelope a
	// --json consumer sees: ok/valid true with a single info issue.
	result := residentResult(dir, m)
	if !result.OK || !result.Valid {
		t.Errorf("ok=%v valid=%v, want both true: a resident is not a validation failure", result.OK, result.Valid)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("want exactly 1 issue, got %d", len(result.Issues))
	}
	issue := result.Issues[0]
	if issue.Level != LevelInfo {
		t.Errorf("level = %q, want %q", issue.Level, LevelInfo)
	}
	if issue.Code != "lifecycle-resident" {
		t.Errorf("code = %q, want lifecycle-resident", issue.Code)
	}
	if issue.AutoFixable {
		t.Error("a resident is not auto-fixable; there is nothing to fix")
	}
	for _, want := range []string{"design", "owned by camp", "nothing to validate"} {
		if !strings.Contains(issue.Message, want) {
			t.Errorf("message %q missing %q", issue.Message, want)
		}
	}
}
