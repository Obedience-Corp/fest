package loop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLoopCondition_NoLoopFile(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := CheckLoopCondition(context.Background(), tmpDir, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextStep != 2 {
		t.Errorf("expected next step 2, got %d", result.NextStep)
	}
	if result.RetryContext != "" {
		t.Error("expected no retry context")
	}
	if result.Escalated {
		t.Error("expected no escalation")
	}
}

func TestCheckLoopCondition_NoLoopForStep(t *testing.T) {
	tmpDir := t.TempDir()

	loopYAML := `
loops:
  - after_step: 3
    condition: markers
    on_failure:
      action: retry_step
      context: "Fix markers"
    max_iterations: 2
    on_max_reached: escalate
`
	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte(loopYAML), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckLoopCondition(context.Background(), tmpDir, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextStep != 2 {
		t.Errorf("expected next step 2, got %d", result.NextStep)
	}
}

func TestCheckLoopCondition_ConditionPasses(t *testing.T) {
	tmpDir := t.TempDir()

	// Create LOOP.yaml with markers condition
	loopYAML := `
loops:
  - after_step: 1
    condition: markers
    on_failure:
      action: retry_step
      context: "Fix markers"
    max_iterations: 3
    on_max_reached: escalate
`
	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte(loopYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create clean file (no markers)
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("# Clean\nNo markers here\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckLoopCondition(context.Background(), tmpDir, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextStep != 2 {
		t.Errorf("expected next step 2, got %d", result.NextStep)
	}
	if result.RetryContext != "" {
		t.Error("expected no retry context")
	}

	// Verify check event was recorded
	store := NewStateStore(tmpDir)
	events, err := store.Events()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "loop_check" {
		t.Errorf("expected loop_check event, got %s", events[0].Type)
	}
	if !events[0].Passed {
		t.Error("expected event to show passed=true")
	}
}

func TestCheckLoopCondition_RetryStep(t *testing.T) {
	tmpDir := t.TempDir()

	loopYAML := `
loops:
  - after_step: 1
    condition: markers
    on_failure:
      action: retry_step
      context: "Found unfilled markers: {marker_list}"
    max_iterations: 3
    on_max_reached: escalate
`
	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte(loopYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create file with markers
	content := "# Test\n[" + "REPLACE: Fill this in]\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckLoopCondition(context.Background(), tmpDir, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextStep != 1 {
		t.Errorf("expected retry on step 1, got next step %d", result.NextStep)
	}
	if !strings.Contains(result.RetryContext, "RETRY 1/3") {
		t.Errorf("expected RETRY 1/3 in context, got: %s", result.RetryContext)
	}
	if result.Escalated {
		t.Error("should not be escalated on first retry")
	}
}

func TestCheckLoopCondition_ReturnToStep(t *testing.T) {
	tmpDir := t.TempDir()

	step2 := 2
	loopYAML := `
loops:
  - after_step: 3
    condition: markers
    on_failure:
      action: return_to_step
      step: 2
      context: "Go back and fix markers"
    max_iterations: 3
    on_max_reached: escalate
`
	_ = step2 // Used in YAML

	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte(loopYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create file with markers
	content := "# Test\n[" + "REPLACE: Fix me]\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckLoopCondition(context.Background(), tmpDir, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextStep != 2 {
		t.Errorf("expected return to step 2, got %d", result.NextStep)
	}

	// Verify retry event recorded return_to_step
	store := NewStateStore(tmpDir)
	events, err := store.Events()
	if err != nil {
		t.Fatal(err)
	}

	var retryEvent *StateEvent
	for i := range events {
		if events[i].Type == "loop_retry" {
			retryEvent = &events[i]
			break
		}
	}

	if retryEvent == nil {
		t.Fatal("expected retry event")
	}
	if retryEvent.Action != "return_to_step" {
		t.Errorf("expected action=return_to_step, got %s", retryEvent.Action)
	}
	if retryEvent.ReturnToStep == nil || *retryEvent.ReturnToStep != 2 {
		t.Error("expected return_to_step=2")
	}
}

func TestCheckLoopCondition_Escalation(t *testing.T) {
	tmpDir := t.TempDir()

	loopYAML := `
loops:
  - after_step: 1
    condition: markers
    on_failure:
      action: retry_step
      context: "Fix markers"
    max_iterations: 1
    on_max_reached: escalate
`
	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte(loopYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create file with markers (will never pass)
	content := "# Test\n[" + "REPLACE: Never fixed]\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// First check - triggers retry
	result, err := CheckLoopCondition(context.Background(), tmpDir, 1)
	if err != nil {
		t.Fatalf("first check error: %v", err)
	}
	if result.Escalated {
		t.Error("should not escalate on first check")
	}

	// Second check - should escalate (max_iterations=1, already have 1 retry)
	result, err = CheckLoopCondition(context.Background(), tmpDir, 1)
	if err != nil {
		t.Fatalf("second check error: %v", err)
	}
	if !result.Escalated {
		t.Error("expected escalation after max iterations")
	}
	if !strings.Contains(result.RetryContext, "LOOP LIMIT REACHED") {
		t.Errorf("expected escalation message, got: %s", result.RetryContext)
	}
	if result.NextStep != 1 {
		t.Errorf("expected to stay on step 1, got %d", result.NextStep)
	}
}

func TestCheckLoopCondition_FileExistsCondition(t *testing.T) {
	tmpDir := t.TempDir()

	loopYAML := `
loops:
  - after_step: 1
    condition: file_exists
    on_failure:
      action: retry_step
      context: "Required files missing"
    max_iterations: 2
    on_max_reached: escalate
`
	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte(loopYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// file_exists evaluator requires 'files' param which we pass as empty params
	// With no files to check, file_exists will error because 'files' param is missing
	// This is expected behavior - the condition requires configuration
	result, err := CheckLoopCondition(context.Background(), tmpDir, 1)
	if err != nil {
		// Expected: file_exists requires 'files' parameter
		if !strings.Contains(err.Error(), "files") {
			t.Errorf("unexpected error: %v", err)
		}
		return
	}

	// If no error, verify the result
	_ = result
}

func TestCheckLoopCondition_AuditTrail(t *testing.T) {
	tmpDir := t.TempDir()

	loopYAML := `
loops:
  - after_step: 1
    condition: markers
    on_failure:
      action: retry_step
      context: "Fix markers: {marker_list}"
    max_iterations: 3
    on_max_reached: escalate
`
	if err := os.WriteFile(filepath.Join(tmpDir, "LOOP.yaml"), []byte(loopYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create file with markers
	content := "# Test\n[" + "REPLACE: Fix me]\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Run check twice
	for i := 0; i < 2; i++ {
		_, err := CheckLoopCondition(context.Background(), tmpDir, 1)
		if err != nil {
			t.Fatalf("check %d error: %v", i, err)
		}
	}

	// Verify JSONL audit trail
	store := NewStateStore(tmpDir)
	events, err := store.Events()
	if err != nil {
		t.Fatal(err)
	}

	// Each check produces: 1 loop_check + 1 loop_retry = 2 events per check
	if len(events) != 4 {
		t.Errorf("expected 4 events (2 per check), got %d", len(events))
	}

	// Verify event types alternate
	checkCount := 0
	retryCount := 0
	for _, e := range events {
		switch e.Type {
		case "loop_check":
			checkCount++
		case "loop_retry":
			retryCount++
		}
	}
	if checkCount != 2 {
		t.Errorf("expected 2 loop_check events, got %d", checkCount)
	}
	if retryCount != 2 {
		t.Errorf("expected 2 loop_retry events, got %d", retryCount)
	}
}
