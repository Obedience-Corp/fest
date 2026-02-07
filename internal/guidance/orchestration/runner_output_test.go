package orchestration

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRunner_FormatDryRun_IncludesSummaryAndTasks(t *testing.T) {
	runner := buildTestRunner()

	output := runner.FormatDryRun()

	// Only check for dynamic values from test data, not static template text
	dynamicValues := []string{
		"001_PHASE",   // phase name from test data
		"01_SEQUENCE", // sequence name from test data
		"01_task",     // task name from test data
	}

	for _, snippet := range dynamicValues {
		if !contains(output, snippet) {
			t.Errorf("expected output to contain dynamic value %q", snippet)
		}
	}
}

func TestRunner_FormatAgentInstructions_IncludesCommands(t *testing.T) {
	runner := buildTestRunner()

	output, err := runner.FormatAgentInstructions()
	if err != nil {
		t.Fatalf("FormatAgentInstructions() error = %v", err)
	}

	// Only check for dynamic values from test data and config, not static template text
	dynamicValues := []string{
		"01_task",     // task name from test data
		"001_PHASE",   // phase name from test data
		"01_SEQUENCE", // sequence name from test data
		"/tmp/fest/001_PHASE/01_SEQUENCE/01_task.md", // task path from test data
		runner.config.ActionInstruction,              // action instruction from config
		"fest task completed",                         // completion command
	}

	for _, snippet := range dynamicValues {
		if !contains(output, snippet) {
			t.Errorf("expected output to contain dynamic value %q", snippet)
		}
	}
}

func buildTestRunner() *Runner {
	plan := &ExecutionPlan{
		FestivalPath: "/tmp/fest",
		Summary: &ExecutionSummary{
			TotalPhases:    1,
			TotalSequences: 1,
			TotalTasks:     1,
			TotalSteps:     1,
		},
		Phases: []*PhaseExecution{
			{
				Name:   "001_PHASE",
				Path:   "/tmp/fest/001_PHASE",
				Number: 1,
				Sequences: []*SequenceExecution{
					{
						Name:   "01_SEQUENCE",
						Path:   "/tmp/fest/001_PHASE/01_SEQUENCE",
						Number: 1,
						Steps: []*StepGroup{
							{
								Number: 1,
								Tasks: []*PlanTask{
									{
										ID:            "task-1",
										Name:          "01_task",
										Path:          "/tmp/fest/001_PHASE/01_SEQUENCE/01_task.md",
										AutonomyLevel: "high",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	state := &ExecutionState{
		CurrentPhase: 0,
		CurrentSeq:   0,
		CurrentStep:  0,
		TotalTasks:   1,
		TaskStatuses: map[string]string{"task-1": StatusPending},
	}

	return &Runner{
		festivalPath: "/tmp/fest",
		config:       DefaultConfig(),
		plan:         plan,
		stateManager: &StateManager{state: state},
	}
}

func TestRunner_FormatAgentInstructions_InlineContext(t *testing.T) {
	runner := buildTestRunner()
	runner.config.InlineContext = true

	output, err := runner.FormatAgentInstructions()
	if err != nil {
		t.Fatalf("FormatAgentInstructions() error = %v", err)
	}

	// With InlineContext true, should show "Task Content" section
	stripped := stripANSI(output)
	if !strings.Contains(stripped, "--- Task Content ---") {
		t.Errorf("expected output to contain '--- Task Content ---' header")
	}

	// Should still show Context Files section (always paths only now)
	if !strings.Contains(stripped, "Context Files") {
		t.Errorf("expected output to contain 'Context Files' header")
	}

	// Should show goal file paths
	goalPaths := []string{
		"FESTIVAL_GOAL.md",
		"PHASE_GOAL.md",
		"SEQUENCE_GOAL.md",
	}

	for _, path := range goalPaths {
		if !strings.Contains(stripped, path) {
			t.Errorf("expected output to contain goal path %q", path)
		}
	}

	// Should indicate task file is not readable (since test paths don't exist)
	if !strings.Contains(stripped, "Error reading file") && !strings.Contains(stripped, "File not found") {
		t.Errorf("expected output to indicate missing/unreadable task file, got:\n%s", stripped)
	}
}

func TestRunner_FormatAgentInstructions_PathsOnly(t *testing.T) {
	runner := buildTestRunner()
	runner.config.InlineContext = false

	output, err := runner.FormatAgentInstructions()
	if err != nil {
		t.Fatalf("FormatAgentInstructions() error = %v", err)
	}

	// With InlineContext false (default), should show file paths
	expectedPaths := []string{
		"FESTIVAL_GOAL.md",
		"PHASE_GOAL.md",
		"SEQUENCE_GOAL.md",
	}

	for _, path := range expectedPaths {
		if !contains(output, path) {
			t.Errorf("expected output to contain path %q", path)
		}
	}

	// Should NOT show Task Content section when InlineContext is false
	stripped := stripANSI(output)
	if strings.Contains(stripped, "--- Task Content ---") {
		t.Errorf("expected output to NOT contain '--- Task Content ---' when InlineContext is false")
	}
}

func TestRunner_FormatAgentInstructions_InlineContext_WithRealFile(t *testing.T) {
	// Create a temporary directory with a real task file
	tmpDir := t.TempDir()
	phaseDir := filepath.Join(tmpDir, "001_PHASE")
	seqDir := filepath.Join(phaseDir, "01_SEQUENCE")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}

	// Create a task file with content
	taskPath := filepath.Join(seqDir, "01_task.md")
	taskContent := `# Task: Implement Feature

This is the task description.

## Acceptance Criteria
- Criterion 1
- Criterion 2`
	if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}

	// Build runner with real paths
	plan := &ExecutionPlan{
		FestivalPath: tmpDir,
		Summary: &ExecutionSummary{
			TotalPhases:    1,
			TotalSequences: 1,
			TotalTasks:     1,
			TotalSteps:     1,
		},
		Phases: []*PhaseExecution{
			{
				Name:   "001_PHASE",
				Path:   phaseDir,
				Number: 1,
				Sequences: []*SequenceExecution{
					{
						Name:   "01_SEQUENCE",
						Path:   seqDir,
						Number: 1,
						Steps: []*StepGroup{
							{
								Number: 1,
								Tasks: []*PlanTask{
									{
										ID:            "task-1",
										Name:          "01_task",
										Path:          taskPath,
										AutonomyLevel: "high",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	state := &ExecutionState{
		CurrentPhase: 0,
		CurrentSeq:   0,
		CurrentStep:  0,
		TotalTasks:   1,
		TaskStatuses: map[string]string{"task-1": StatusPending},
	}

	config := DefaultConfig()
	config.InlineContext = true

	runner := &Runner{
		festivalPath: tmpDir,
		config:       config,
		plan:         plan,
		stateManager: &StateManager{state: state},
	}

	output, err := runner.FormatAgentInstructions()
	if err != nil {
		t.Fatalf("FormatAgentInstructions() error = %v", err)
	}

	stripped := stripANSI(output)

	// Should show Task Content header
	if !strings.Contains(stripped, "--- Task Content ---") {
		t.Errorf("expected output to contain '--- Task Content ---' header")
	}

	// Should show task content
	expectedContent := []string{
		"Task: Implement Feature",
		"This is the task description",
		"Acceptance Criteria",
		"Criterion 1",
		"Criterion 2",
	}

	for _, content := range expectedContent {
		if !strings.Contains(stripped, content) {
			t.Errorf("expected output to contain task content %q", content)
		}
	}

	// Should still show Context Files section with paths
	if !strings.Contains(stripped, "Context Files") {
		t.Errorf("expected output to contain 'Context Files' header")
	}
}

func contains(s, substr string) bool {
	if substr == "" {
		return false
	}
	return strings.Contains(stripANSI(s), substr)
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}
