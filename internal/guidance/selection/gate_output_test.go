package selection

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildGateSection_NonGateTask(t *testing.T) {
	tmp := t.TempDir()

	// Create a regular task file (no gate_type)
	taskPath := filepath.Join(tmp, "01_task.md")
	content := "---\nfest_type: task\nfest_id: 01_task.md\nfest_name: test\nfest_status: pending\n---\n# Regular Task\n"
	os.WriteFile(taskPath, []byte(content), 0644)

	task := &TaskInfo{
		Name:         "01_task",
		Path:         taskPath,
		PhaseName:    "001_PHASE",
		SequenceName: "01_seq",
	}

	result := buildGateSection(task)
	if result != "" {
		t.Errorf("expected empty string for non-gate task, got %q", result)
	}
}

func TestBuildGateSection_GateTask(t *testing.T) {
	tmp := t.TempDir()

	taskPath := filepath.Join(tmp, "06_testing.md")
	content := "---\nfest_type: gate\nfest_id: 06_testing.md\nfest_name: Testing\nfest_gate_type: testing\nfest_status: pending\n---\n# Testing Gate\n\n- [ ] All tests pass\n- [ ] Coverage meets 90%\n"
	os.WriteFile(taskPath, []byte(content), 0644)

	task := &TaskInfo{
		Name:         "06_testing",
		Path:         taskPath,
		PhaseName:    "001_PHASE",
		SequenceName: "01_seq",
	}

	result := buildGateSection(task)
	if result == "" {
		t.Fatal("expected gate section, got empty string")
	}

	// Should contain gate criteria
	if !strings.Contains(result, "All tests pass") {
		t.Error("expected gate content to include criteria")
	}
	if !strings.Contains(result, "Coverage meets 90%") {
		t.Error("expected gate content to include coverage requirement")
	}
	if !strings.Contains(result, "Testing") {
		t.Error("expected gate title")
	}
	if !strings.Contains(result, "fest task completed") {
		t.Error("expected completion hint")
	}
}

func TestBuildGateSection_EmptyBody(t *testing.T) {
	tmp := t.TempDir()

	taskPath := filepath.Join(tmp, "07_review.md")
	content := "---\nfest_type: gate\nfest_id: 07_review.md\nfest_name: Review\nfest_gate_type: review\nfest_status: pending\n---\n"
	os.WriteFile(taskPath, []byte(content), 0644)

	task := &TaskInfo{
		Name:         "07_review",
		Path:         taskPath,
		PhaseName:    "001_PHASE",
		SequenceName: "01_seq",
	}

	result := buildGateSection(task)
	if result == "" {
		t.Fatal("expected gate section for empty body")
	}
	if !strings.Contains(result, "fest gates") {
		t.Error("expected fallback message with fest gates reference")
	}
}

func TestBuildGateSection_NilTask(t *testing.T) {
	result := buildGateSection(nil)
	if result != "" {
		t.Errorf("expected empty string for nil task, got %q", result)
	}
}

func TestBuildGateSection_MissingFile(t *testing.T) {
	task := &TaskInfo{
		Name: "nonexistent",
		Path: "/nonexistent/path/task.md",
	}

	result := buildGateSection(task)
	if result != "" {
		t.Errorf("expected empty string for missing file, got %q", result)
	}
}
