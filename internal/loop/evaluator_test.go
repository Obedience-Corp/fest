package loop

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewEvaluator(t *testing.T) {
	validTypes := []string{"validate", "markers", "file_exists", "schema_valid", "command"}
	for _, ct := range validTypes {
		eval, err := NewEvaluator(ct)
		if err != nil {
			t.Errorf("expected no error for %s, got: %v", ct, err)
		}
		if eval == nil {
			t.Errorf("expected evaluator for %s, got nil", ct)
		}
	}

	_, err := NewEvaluator("invalid")
	if err == nil {
		t.Error("expected error for invalid condition type")
	}
}

func TestMarkersEvaluator_WithMarkers(t *testing.T) {
	tmpDir := t.TempDir()

	mdFile := filepath.Join(tmpDir, "test.md")
	content := "# Test\n[REPLACE: Fill this in]\nSome text\n[FILL: Another marker]\n"
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	eval := &MarkersEvaluator{}
	result, err := eval.Evaluate(context.Background(), tmpDir, map[string]any{
		"scope": "phase",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("expected condition to fail when markers present")
	}
	if result.FailureContext == "" {
		t.Error("expected failure context to contain marker info")
	}
}

func TestMarkersEvaluator_NoMarkers(t *testing.T) {
	tmpDir := t.TempDir()

	mdFile := filepath.Join(tmpDir, "test.md")
	content := "# Test\nAll filled in\nSome text\n"
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	eval := &MarkersEvaluator{}
	result, err := eval.Evaluate(context.Background(), tmpDir, map[string]any{
		"scope": "phase",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected condition to pass when no markers present")
	}
}

func TestMarkersEvaluator_SkipsHiddenDirs(t *testing.T) {
	tmpDir := t.TempDir()

	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}

	hiddenFile := filepath.Join(hiddenDir, "test.md")
	if err := os.WriteFile(hiddenFile, []byte("[REPLACE: hidden]"), 0644); err != nil {
		t.Fatal(err)
	}

	eval := &MarkersEvaluator{}
	result, err := eval.Evaluate(context.Background(), tmpDir, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected condition to pass when markers only in hidden dirs")
	}
}

func TestFileExistsEvaluator_AllExist(t *testing.T) {
	tmpDir := t.TempDir()

	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	eval := &FileExistsEvaluator{}
	result, err := eval.Evaluate(context.Background(), tmpDir, map[string]any{
		"files": []string{"a.txt", "b.txt"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected condition to pass when all files exist")
	}
}

func TestFileExistsEvaluator_SomeMissing(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "exists.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	eval := &FileExistsEvaluator{}
	result, err := eval.Evaluate(context.Background(), tmpDir, map[string]any{
		"files": []string{"exists.txt", "missing.txt"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("expected condition to fail when file missing")
	}
}

func TestFileExistsEvaluator_InterfaceSlice(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	eval := &FileExistsEvaluator{}
	result, err := eval.Evaluate(context.Background(), tmpDir, map[string]any{
		"files": []any{"test.txt"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected condition to pass with interface{} slice")
	}
}

func TestFileExistsEvaluator_MissingParam(t *testing.T) {
	eval := &FileExistsEvaluator{}
	_, err := eval.Evaluate(context.Background(), t.TempDir(), map[string]any{})
	if err == nil {
		t.Error("expected error when files parameter missing")
	}
}

func TestCommandEvaluator_ContainsMatch(t *testing.T) {
	eval := &CommandEvaluator{}
	result, err := eval.Evaluate(context.Background(), t.TempDir(), map[string]any{
		"command":    "echo 'hello world'",
		"expression": "contains:hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected condition to pass for contains match")
	}
}

func TestCommandEvaluator_ContainsNoMatch(t *testing.T) {
	eval := &CommandEvaluator{}
	result, err := eval.Evaluate(context.Background(), t.TempDir(), map[string]any{
		"command":    "echo 'hello world'",
		"expression": "contains:goodbye",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("expected condition to fail when text not found")
	}
}

func TestCommandEvaluator_ExitCode(t *testing.T) {
	eval := &CommandEvaluator{}
	result, err := eval.Evaluate(context.Background(), t.TempDir(), map[string]any{
		"command":    "true",
		"expression": "exitcode:0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected condition to pass for exitcode:0")
	}
}

func TestCommandEvaluator_NonZeroExitCode(t *testing.T) {
	eval := &CommandEvaluator{}
	result, err := eval.Evaluate(context.Background(), t.TempDir(), map[string]any{
		"command":    "exit 1",
		"expression": "exitcode:1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected condition to pass for exitcode:1")
	}
}

func TestCommandEvaluator_NotContains(t *testing.T) {
	eval := &CommandEvaluator{}
	result, err := eval.Evaluate(context.Background(), t.TempDir(), map[string]any{
		"command":    "echo 'hello'",
		"expression": "not_contains:error",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected condition to pass for not_contains")
	}
}

func TestCommandEvaluator_MissingParams(t *testing.T) {
	eval := &CommandEvaluator{}

	_, err := eval.Evaluate(context.Background(), t.TempDir(), map[string]any{
		"expression": "contains:test",
	})
	if err == nil {
		t.Error("expected error when command parameter missing")
	}

	_, err = eval.Evaluate(context.Background(), t.TempDir(), map[string]any{
		"command": "echo test",
	})
	if err == nil {
		t.Error("expected error when expression parameter missing")
	}
}

func TestSchemaValidEvaluator_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	yamlFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(yamlFile, []byte("key: value\nlist:\n  - item1\n  - item2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	eval := &SchemaValidEvaluator{}
	result, err := eval.Evaluate(context.Background(), tmpDir, map[string]any{
		"file":   "test.yaml",
		"schema": "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected condition to pass for valid YAML")
	}
}

func TestSchemaValidEvaluator_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	yamlFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(yamlFile, []byte("invalid: yaml: [\n"), 0644); err != nil {
		t.Fatal(err)
	}

	eval := &SchemaValidEvaluator{}
	result, err := eval.Evaluate(context.Background(), tmpDir, map[string]any{
		"file":   "test.yaml",
		"schema": "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("expected condition to fail for invalid YAML")
	}
}

func TestSchemaValidEvaluator_FileNotFound(t *testing.T) {
	eval := &SchemaValidEvaluator{}
	result, err := eval.Evaluate(context.Background(), t.TempDir(), map[string]any{
		"file":   "nonexistent.yaml",
		"schema": "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("expected condition to fail for nonexistent file")
	}
}

func TestSchemaValidEvaluator_MissingParam(t *testing.T) {
	eval := &SchemaValidEvaluator{}
	_, err := eval.Evaluate(context.Background(), t.TempDir(), map[string]any{})
	if err == nil {
		t.Error("expected error when file parameter missing")
	}
}

func TestEvaluateExpression(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		output   string
		exitCode int
		want     bool
	}{
		{"contains match", "contains:hello", "hello world", 0, true},
		{"contains no match", "contains:goodbye", "hello world", 0, false},
		{"not_contains match", "not_contains:error", "success", 0, true},
		{"not_contains no match", "not_contains:error", "error occurred", 0, false},
		{"exitcode match", "exitcode:0", "", 0, true},
		{"exitcode no match", "exitcode:0", "", 1, false},
		{"invalid format", "invalid", "", 0, false},
		{"unknown operator", "unknown:value", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateExpression(tt.expr, tt.output, tt.exitCode)
			if got != tt.want {
				t.Errorf("evaluateExpression(%q, %q, %d) = %v, want %v", tt.expr, tt.output, tt.exitCode, got, tt.want)
			}
		})
	}
}
