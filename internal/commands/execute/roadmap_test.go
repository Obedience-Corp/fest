package execute

import (
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/guidance/orchestration"
)

func TestFormatRoadmap(t *testing.T) {
	tests := []struct {
		name           string
		festivalPath   string
		plan           *orchestration.ExecutionPlan
		expectContains []string
	}{
		{
			name:         "simple plan with one phase",
			festivalPath: "/test/festival",
			plan: &orchestration.ExecutionPlan{
				FestivalPath: "/test/festival",
				Summary: &orchestration.ExecutionSummary{
					TotalTasks: 5,
				},
				Phases: []*orchestration.PhaseExecution{
					{
						Name:       "001_TEST_PHASE",
						Path:       "/test/festival/001_TEST_PHASE",
						Number:     1,
						TotalTasks: 5,
						Sequences: []*orchestration.SequenceExecution{
							{
								Name:   "01_test_sequence",
								Path:   "/test/festival/001_TEST_PHASE/01_test_sequence",
								Number: 1,
								Steps: []*orchestration.StepGroup{
									{
										Number: 1,
										Tasks: []*orchestration.PlanTask{
											{Name: "01_task_one", Path: "/test/festival/001_TEST_PHASE/01_test_sequence/01_task_one.md"},
										},
									},
									{
										Number: 2,
										Tasks: []*orchestration.PlanTask{
											{Name: "02_task_two", Path: "/test/festival/001_TEST_PHASE/01_test_sequence/02_task_two.md"},
										},
									},
								},
							},
						},
					},
				},
			},
			expectContains: []string{
				"FESTIVAL ROADMAP",
				"001_TEST_PHASE",
				"01_test_sequence",
				"01_task_one",
				"02_task_two",
				"Total Phases:",
				"Total Tasks:",
			},
		},
		{
			name:         "plan with quality gate",
			festivalPath: "/test/festival-with-gate",
			plan: &orchestration.ExecutionPlan{
				FestivalPath: "/test/festival-with-gate",
				Summary: &orchestration.ExecutionSummary{
					TotalTasks: 2,
				},
				Phases: []*orchestration.PhaseExecution{
					{
						Name:       "001_GATED_PHASE",
						Path:       "/test/festival-with-gate/001_GATED_PHASE",
						Number:     1,
						TotalTasks: 2,
						Sequences:  []*orchestration.SequenceExecution{},
						QualityGate: &orchestration.QualityGateInfo{
							Type: "approval",
						},
					},
				},
			},
			expectContains: []string{
				"Quality Gate:",
				"approval",
			},
		},
		{
			name:         "plan with parallel tasks",
			festivalPath: "/test/parallel-festival",
			plan: &orchestration.ExecutionPlan{
				FestivalPath: "/test/parallel-festival",
				Summary: &orchestration.ExecutionSummary{
					TotalTasks: 3,
				},
				Phases: []*orchestration.PhaseExecution{
					{
						Name:       "001_PARALLEL_PHASE",
						Path:       "/test/parallel-festival/001_PARALLEL_PHASE",
						Number:     1,
						TotalTasks: 3,
						Sequences: []*orchestration.SequenceExecution{
							{
								Name:   "01_parallel_sequence",
								Path:   "/test/parallel-festival/001_PARALLEL_PHASE/01_parallel_sequence",
								Number: 1,
								Steps: []*orchestration.StepGroup{
									{
										Number:   1,
										Parallel: true,
										Tasks: []*orchestration.PlanTask{
											{Name: "01_parallel_a", Path: "/test/parallel-festival/001_PARALLEL_PHASE/01_parallel_sequence/01_parallel_a.md"},
											{Name: "02_parallel_b", Path: "/test/parallel-festival/001_PARALLEL_PHASE/01_parallel_sequence/02_parallel_b.md"},
										},
									},
								},
							},
						},
					},
				},
			},
			expectContains: []string{
				"[Parallel]",
				"01_parallel_a",
				"02_parallel_b",
			},
		},
		{
			name:         "empty plan",
			festivalPath: "/test/empty-festival",
			plan: &orchestration.ExecutionPlan{
				FestivalPath: "/test/empty-festival",
				Summary: &orchestration.ExecutionSummary{
					TotalTasks: 0,
				},
				Phases: []*orchestration.PhaseExecution{},
			},
			expectContains: []string{
				"FESTIVAL ROADMAP",
				"empty-festival",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatRoadmap(tt.festivalPath, tt.plan)

			for _, expected := range tt.expectContains {
				if !strings.Contains(output, expected) {
					t.Errorf("output missing expected string: %q\nOutput:\n%s", expected, output)
				}
			}
		})
	}
}

func TestDetectPhaseMode(t *testing.T) {
	// Test with non-existent path - should default to implementation
	phaseType, mode := detectPhaseMode("/nonexistent/path")
	if phaseType != "" {
		t.Errorf("expected empty phaseType for nonexistent path, got %q", phaseType)
	}
	if mode != guidance.ModeImplementation {
		t.Errorf("expected ModeImplementation for nonexistent path, got %v", mode)
	}
}

func TestFormatPhase(t *testing.T) {
	tests := []struct {
		name           string
		phase          *orchestration.PhaseExecution
		expectContains []string
	}{
		{
			name: "phase with sequences",
			phase: &orchestration.PhaseExecution{
				Name:       "001_TEST_PHASE",
				Path:       "/nonexistent/001_TEST_PHASE", // Will default to implementation mode
				Number:     1,
				TotalTasks: 2,
				Sequences: []*orchestration.SequenceExecution{
					{
						Name:   "01_seq",
						Number: 1,
						Steps:  []*orchestration.StepGroup{},
					},
				},
			},
			expectContains: []string{
				"001_TEST_PHASE",
				"2 tasks",
				"Mode:",
				"implementation",
				"01_seq",
			},
		},
		{
			name: "phase with quality gate",
			phase: &orchestration.PhaseExecution{
				Name:       "002_GATED_PHASE",
				Path:       "/nonexistent/002_GATED_PHASE",
				Number:     2,
				TotalTasks: 0,
				Sequences:  []*orchestration.SequenceExecution{},
				QualityGate: &orchestration.QualityGateInfo{
					Type: "review",
				},
			},
			expectContains: []string{
				"002_GATED_PHASE",
				"Quality Gate:",
				"review",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			formatPhase(&buf, tt.phase)
			output := buf.String()

			for _, expected := range tt.expectContains {
				if !strings.Contains(output, expected) {
					t.Errorf("output missing expected string: %q\nOutput:\n%s", expected, output)
				}
			}
		})
	}
}

func TestFormatSequence(t *testing.T) {
	tests := []struct {
		name           string
		seq            *orchestration.SequenceExecution
		expectContains []string
	}{
		{
			name: "sequence with tasks",
			seq: &orchestration.SequenceExecution{
				Name:   "01_test_sequence",
				Number: 1,
				Steps: []*orchestration.StepGroup{
					{
						Number: 1,
						Tasks: []*orchestration.PlanTask{
							{Name: "01_task_a", Path: "/path/01_task_a.md"},
							{Name: "02_task_b", Path: "/path/02_task_b.md"},
						},
					},
				},
			},
			expectContains: []string{
				"01_test_sequence",
				"01_task_a",
				"02_task_b",
			},
		},
		{
			name: "empty sequence",
			seq: &orchestration.SequenceExecution{
				Name:   "02_empty_sequence",
				Number: 2,
				Steps:  []*orchestration.StepGroup{},
			},
			expectContains: []string{
				"02_empty_sequence",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			formatSequence(&buf, tt.seq)
			output := buf.String()

			for _, expected := range tt.expectContains {
				if !strings.Contains(output, expected) {
					t.Errorf("output missing expected string: %q\nOutput:\n%s", expected, output)
				}
			}
		})
	}
}

func TestFormatStep(t *testing.T) {
	tests := []struct {
		name           string
		step           *orchestration.StepGroup
		isLast         bool
		expectContains []string
	}{
		{
			name: "single task step",
			step: &orchestration.StepGroup{
				Number: 1,
				Tasks: []*orchestration.PlanTask{
					{Name: "01_task", Path: "/path/01_task.md"},
				},
			},
			isLast: false,
			expectContains: []string{
				"├──",
				"01_task",
			},
		},
		{
			name: "last single task step",
			step: &orchestration.StepGroup{
				Number: 2,
				Tasks: []*orchestration.PlanTask{
					{Name: "02_last_task", Path: "/path/02_last_task.md"},
				},
			},
			isLast: true,
			expectContains: []string{
				"└──",
				"02_last_task",
			},
		},
		{
			name: "parallel task step",
			step: &orchestration.StepGroup{
				Number:   1,
				Parallel: true,
				Tasks: []*orchestration.PlanTask{
					{Name: "01_parallel_a", Path: "/path/01_parallel_a.md"},
					{Name: "02_parallel_b", Path: "/path/02_parallel_b.md"},
				},
			},
			isLast: false,
			expectContains: []string{
				"[Parallel]",
				"01_parallel_a",
				"02_parallel_b",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			formatStep(&buf, tt.step, tt.isLast)
			output := buf.String()

			for _, expected := range tt.expectContains {
				if !strings.Contains(output, expected) {
					t.Errorf("output missing expected string: %q\nOutput:\n%s", expected, output)
				}
			}
		})
	}
}
