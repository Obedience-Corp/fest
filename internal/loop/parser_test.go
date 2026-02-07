package loop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLoopConfig_Valid(t *testing.T) {
	yaml := `
loops:
  - after_step: 1
    condition: validate
    on_failure:
      action: retry_step
      context: "Validation failed: {validation_output}"
    max_iterations: 3
    on_max_reached: escalate
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "LOOP.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := ParseLoopConfig(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(config.Loops) != 1 {
		t.Fatalf("expected 1 loop, got %d", len(config.Loops))
	}

	loop := config.Loops[0]
	if loop.AfterStep != 1 {
		t.Errorf("expected after_step=1, got %d", loop.AfterStep)
	}
	if loop.Condition != "validate" {
		t.Errorf("expected condition=validate, got %s", loop.Condition)
	}
	if loop.MaxIterations != 3 {
		t.Errorf("expected max_iterations=3, got %d", loop.MaxIterations)
	}
	if loop.OnFailure.Action != "retry_step" {
		t.Errorf("expected action=retry_step, got %s", loop.OnFailure.Action)
	}
	if loop.OnMaxReached != "escalate" {
		t.Errorf("expected on_max_reached=escalate, got %s", loop.OnMaxReached)
	}
}

func TestParseLoopConfig_MultipleLoops(t *testing.T) {
	yaml := `
loops:
  - after_step: 1
    condition: validate
    on_failure:
      action: retry_step
      context: "Fix validation"
    max_iterations: 3
    on_max_reached: escalate
  - after_step: 3
    condition: markers
    on_failure:
      action: return_to_step
      step: 2
      context: "Fill markers: {marker_list}"
    max_iterations: 5
    on_max_reached: escalate
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "LOOP.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := ParseLoopConfig(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(config.Loops) != 2 {
		t.Fatalf("expected 2 loops, got %d", len(config.Loops))
	}
}

func TestParseLoopConfig_MissingMaxIterations(t *testing.T) {
	yaml := `
loops:
  - after_step: 1
    condition: validate
    on_failure:
      action: retry_step
    on_max_reached: escalate
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "LOOP.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseLoopConfig(path)
	if err == nil {
		t.Fatal("expected error for missing max_iterations")
	}
}

func TestParseLoopConfig_InvalidCondition(t *testing.T) {
	yaml := `
loops:
  - after_step: 1
    condition: invalid_type
    on_failure:
      action: retry_step
    max_iterations: 3
    on_max_reached: escalate
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "LOOP.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseLoopConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid condition type")
	}
}

func TestParseLoopConfig_InvalidAction(t *testing.T) {
	yaml := `
loops:
  - after_step: 1
    condition: validate
    on_failure:
      action: invalid_action
    max_iterations: 3
    on_max_reached: escalate
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "LOOP.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseLoopConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestParseLoopConfig_ReturnToStepMissingStep(t *testing.T) {
	yaml := `
loops:
  - after_step: 1
    condition: validate
    on_failure:
      action: return_to_step
    max_iterations: 3
    on_max_reached: escalate
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "LOOP.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseLoopConfig(path)
	if err == nil {
		t.Fatal("expected error for missing step with return_to_step action")
	}
}

func TestParseLoopConfig_InvalidOnMaxReached(t *testing.T) {
	yaml := `
loops:
  - after_step: 1
    condition: validate
    on_failure:
      action: retry_step
    max_iterations: 3
    on_max_reached: ignore
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "LOOP.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseLoopConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid on_max_reached")
	}
}

func TestParseLoopConfig_FileNotFound(t *testing.T) {
	_, err := ParseLoopConfig("/nonexistent/LOOP.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestFindLoopForStep(t *testing.T) {
	config := &LoopConfig{
		Loops: []Loop{
			{AfterStep: 1, Condition: "validate"},
			{AfterStep: 3, Condition: "markers"},
		},
	}

	loop := config.FindLoopForStep(1)
	if loop == nil {
		t.Fatal("expected to find loop for step 1")
	}
	if loop.Condition != "validate" {
		t.Errorf("expected validate condition, got %s", loop.Condition)
	}

	loop = config.FindLoopForStep(3)
	if loop == nil {
		t.Fatal("expected to find loop for step 3")
	}
	if loop.Condition != "markers" {
		t.Errorf("expected markers condition, got %s", loop.Condition)
	}

	loop = config.FindLoopForStep(5)
	if loop != nil {
		t.Error("expected nil for step with no loop")
	}
}

func TestIsValidConditionType(t *testing.T) {
	validTypes := []string{"validate", "markers", "file_exists", "schema_valid", "command"}
	for _, ct := range validTypes {
		if !isValidConditionType(ct) {
			t.Errorf("expected %s to be valid", ct)
		}
	}

	invalidTypes := []string{"", "unknown", "test", "loop"}
	for _, ct := range invalidTypes {
		if isValidConditionType(ct) {
			t.Errorf("expected %s to be invalid", ct)
		}
	}
}
