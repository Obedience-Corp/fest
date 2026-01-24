package execute

import (
	"regexp"
	"strings"
	"testing"
)

func TestRunner_FormatDryRun_IncludesSummaryAndTasks(t *testing.T) {
	runner := buildTestRunner()

	output := runner.FormatDryRun()

	// Only check for dynamic values from test data, not static template text
	dynamicValues := []string{
		"001_PHASE",    // phase name from test data
		"01_SEQUENCE",  // sequence name from test data
		"01_task",      // task name from test data
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
		"01_task",                              // task name from test data
		"001_PHASE",                            // phase name from test data
		"01_SEQUENCE",                          // sequence name from test data
		"/tmp/fest/001_PHASE/01_SEQUENCE/01_task.md", // task path from test data
		runner.config.ActionInstruction,        // action instruction from config
		"fest progress --task",                 // progress command
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
