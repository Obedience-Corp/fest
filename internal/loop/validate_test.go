package loop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLoopConfig_NoFile(t *testing.T) {
	issues := ValidateLoopConfig(t.TempDir())
	if len(issues) != 0 {
		t.Errorf("expected no issues when LOOP.yaml absent, got %d", len(issues))
	}
}

func TestValidateLoopConfig_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	loopYAML := `
loops:
  - after_step: 1
    condition: validate
    on_failure:
      action: retry_step
      context: "Validation failed"
    max_iterations: 3
    on_max_reached: escalate
`
	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte(loopYAML), 0644); err != nil {
		t.Fatal(err)
	}

	issues := ValidateLoopConfig(tmpDir)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid LOOP.yaml, got: %v", issues)
	}
}

func TestValidateLoopConfig_MissingMaxIterations(t *testing.T) {
	tmpDir := t.TempDir()

	loopYAML := `
loops:
  - after_step: 1
    condition: validate
    on_failure:
      action: retry_step
    on_max_reached: escalate
`
	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte(loopYAML), 0644); err != nil {
		t.Fatal(err)
	}

	issues := ValidateLoopConfig(tmpDir)
	if len(issues) == 0 {
		t.Fatal("expected validation error for missing max_iterations")
	}

	found := false
	for _, issue := range issues {
		if strings.Contains(issue.Message, "max_iterations") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error about max_iterations")
	}
}

func TestValidateLoopConfig_InvalidCondition(t *testing.T) {
	tmpDir := t.TempDir()

	loopYAML := `
loops:
  - after_step: 1
    condition: invalid_type
    on_failure:
      action: retry_step
    max_iterations: 3
    on_max_reached: escalate
`
	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte(loopYAML), 0644); err != nil {
		t.Fatal(err)
	}

	issues := ValidateLoopConfig(tmpDir)
	if len(issues) == 0 {
		t.Fatal("expected validation error for invalid condition type")
	}
}

func TestValidateLoopConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte("not: valid: yaml: [\n"), 0644); err != nil {
		t.Fatal(err)
	}

	issues := ValidateLoopConfig(tmpDir)
	if len(issues) == 0 {
		t.Fatal("expected validation error for invalid YAML")
	}
}

func TestValidateLoopConfig_ErrorLevel(t *testing.T) {
	tmpDir := t.TempDir()

	loopYAML := `
loops:
  - after_step: 1
    condition: validate
    on_failure:
      action: retry_step
    on_max_reached: escalate
`
	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte(loopYAML), 0644); err != nil {
		t.Fatal(err)
	}

	issues := ValidateLoopConfig(tmpDir)
	for _, issue := range issues {
		if issue.Level != "error" {
			t.Errorf("expected error level, got %s", issue.Level)
		}
	}
}
