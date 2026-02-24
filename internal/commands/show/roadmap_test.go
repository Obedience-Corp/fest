package show

import (
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance/orchestration"
)

func TestRenderRoadmapText_BasicStructure(t *testing.T) {
	plan := &orchestration.ExecutionPlan{
		FestivalPath: "/tmp/test-fest",
		Phases: []*orchestration.PhaseExecution{
			{
				Name:       "001_PLAN",
				Number:     1,
				Status:     "completed",
				TotalTasks: 2,
				Sequences: []*orchestration.SequenceExecution{
					{
						Name:       "01_requirements",
						Number:     1,
						TotalTasks: 2,
						Status:     "completed",
						Steps: []*orchestration.StepGroup{
							{
								Number:   1,
								Type:     "sequential",
								Parallel: false,
								Tasks: []*orchestration.PlanTask{
									{Name: "01_gather.md", Number: 1, Status: "completed"},
								},
							},
							{
								Number:   2,
								Type:     "sequential",
								Parallel: false,
								Tasks: []*orchestration.PlanTask{
									{Name: "02_review.md", Number: 2, Status: "completed"},
								},
							},
						},
					},
				},
			},
		},
		Summary: &orchestration.ExecutionSummary{
			TotalPhases:    1,
			TotalSequences: 1,
			TotalTasks:     2,
			TotalSteps:     2,
		},
	}

	output := renderRoadmapText("test-fest", plan)

	checks := []string{
		"Festival Roadmap: test-fest",
		"Phase 001_PLAN (completed)",
		"01_requirements (2 tasks)",
		"01_gather.md",
		"completed",
		"02_review.md",
		"Summary: 1 phases, 1 sequences, 2 tasks",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing %q\n\nGot:\n%s", check, output)
		}
	}
}

func TestRenderRoadmapText_ParallelTasks(t *testing.T) {
	plan := &orchestration.ExecutionPlan{
		FestivalPath: "/tmp/test-fest",
		Phases: []*orchestration.PhaseExecution{
			{
				Name:       "001_IMPLEMENT",
				Number:     1,
				Status:     "in_progress",
				TotalTasks: 3,
				Sequences: []*orchestration.SequenceExecution{
					{
						Name:       "01_api",
						Number:     1,
						TotalTasks: 3,
						Status:     "in_progress",
						Steps: []*orchestration.StepGroup{
							{
								Number:   1,
								Type:     "parallel",
								Parallel: true,
								Tasks: []*orchestration.PlanTask{
									{Name: "01_frontend.md", Number: 1, Status: "in_progress"},
									{Name: "01_backend.md", Number: 1, Status: "pending"},
								},
							},
							{
								Number:   2,
								Type:     "sequential",
								Parallel: false,
								Tasks: []*orchestration.PlanTask{
									{Name: "02_integrate.md", Number: 2, Status: "pending"},
								},
							},
						},
					},
				},
			},
		},
		Summary: &orchestration.ExecutionSummary{
			TotalPhases:    1,
			TotalSequences: 1,
			TotalTasks:     3,
			TotalSteps:     2,
			ParallelGroups: 1,
		},
	}

	output := renderRoadmapText("test-fest", plan)

	checks := []string{
		"[parallel]",
		"01_frontend.md",
		"in_progress",
		"01_backend.md",
		"pending",
		"02_integrate.md",
		"1 parallel groups",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing %q\n\nGot:\n%s", check, output)
		}
	}
}

func TestRenderRoadmapText_GateTasks(t *testing.T) {
	plan := &orchestration.ExecutionPlan{
		FestivalPath: "/tmp/test-fest",
		Phases: []*orchestration.PhaseExecution{
			{
				Name:       "001_IMPLEMENT",
				Number:     1,
				Status:     "pending",
				TotalTasks: 2,
				Sequences: []*orchestration.SequenceExecution{
					{
						Name:       "01_build",
						Number:     1,
						TotalTasks: 2,
						Status:     "pending",
						Steps: []*orchestration.StepGroup{
							{
								Number:   1,
								Type:     "sequential",
								Parallel: false,
								Tasks: []*orchestration.PlanTask{
									{Name: "01_code.md", Number: 1, Status: "pending", IsGate: false},
								},
							},
							{
								Number:   2,
								Type:     "sequential",
								Parallel: false,
								Tasks: []*orchestration.PlanTask{
									{Name: "02_testing.md", Number: 2, Status: "pending", IsGate: true},
								},
							},
						},
					},
				},
			},
		},
		Summary: &orchestration.ExecutionSummary{
			TotalPhases:    1,
			TotalSequences: 1,
			TotalTasks:     2,
			TotalSteps:     2,
			QualityGates:   1,
		},
	}

	output := renderRoadmapText("test-fest", plan)

	if !strings.Contains(output, "[gate]") {
		t.Errorf("output missing gate marker\n\nGot:\n%s", output)
	}
	if !strings.Contains(output, "1 quality gates") {
		t.Errorf("output missing quality gates in summary\n\nGot:\n%s", output)
	}
}

func TestRenderRoadmapText_EmptyPlan(t *testing.T) {
	plan := &orchestration.ExecutionPlan{
		FestivalPath: "/tmp/empty-fest",
		Phases:       nil,
		Summary:      nil,
	}

	output := renderRoadmapText("empty-fest", plan)

	if !strings.Contains(output, "Festival Roadmap: empty-fest") {
		t.Errorf("output missing title\n\nGot:\n%s", output)
	}
	if !strings.Contains(output, "(no phases found)") {
		t.Errorf("output missing empty indicator\n\nGot:\n%s", output)
	}
}

func TestRenderRoadmapText_MultiplePhases(t *testing.T) {
	plan := &orchestration.ExecutionPlan{
		FestivalPath: "/tmp/test-fest",
		Phases: []*orchestration.PhaseExecution{
			{
				Name:       "001_PLAN",
				Number:     1,
				Status:     "completed",
				TotalTasks: 1,
				Sequences: []*orchestration.SequenceExecution{
					{
						Name:       "01_requirements",
						Number:     1,
						TotalTasks: 1,
						Status:     "completed",
						Steps: []*orchestration.StepGroup{
							{
								Number:   1,
								Type:     "sequential",
								Parallel: false,
								Tasks: []*orchestration.PlanTask{
									{Name: "01_gather.md", Number: 1, Status: "completed"},
								},
							},
						},
					},
				},
			},
			{
				Name:       "002_IMPLEMENT",
				Number:     2,
				Status:     "in_progress",
				TotalTasks: 1,
				Sequences: []*orchestration.SequenceExecution{
					{
						Name:       "01_api",
						Number:     1,
						TotalTasks: 1,
						Status:     "in_progress",
						Steps: []*orchestration.StepGroup{
							{
								Number:   1,
								Type:     "sequential",
								Parallel: false,
								Tasks: []*orchestration.PlanTask{
									{Name: "01_build.md", Number: 1, Status: "in_progress"},
								},
							},
						},
					},
				},
			},
		},
		Summary: &orchestration.ExecutionSummary{
			TotalPhases:    2,
			TotalSequences: 2,
			TotalTasks:     2,
			TotalSteps:     2,
		},
	}

	output := renderRoadmapText("test-fest", plan)

	if !strings.Contains(output, "Phase 001_PLAN (completed)") {
		t.Errorf("output missing first phase\n\nGot:\n%s", output)
	}
	if !strings.Contains(output, "Phase 002_IMPLEMENT (in_progress)") {
		t.Errorf("output missing second phase\n\nGot:\n%s", output)
	}
	if !strings.Contains(output, "2 phases, 2 sequences, 2 tasks") {
		t.Errorf("output missing correct summary\n\nGot:\n%s", output)
	}
}

func TestDotPad(t *testing.T) {
	tests := []struct {
		name   string
		status string
		marker string
		want   string
	}{
		{"short.md", "pending", "", "..............................  pending"},
		{"very_long_task_name_here.md", "completed", "", ".............. completed"},
		{"task.md", "pending", "[gate] ", "........................... pending"},
	}

	for _, tc := range tests {
		got := dotPad(tc.name, tc.status, tc.marker)
		// Check it ends with the status
		if !strings.HasSuffix(got, tc.status) {
			t.Errorf("dotPad(%q, %q, %q) = %q, should end with %q", tc.name, tc.status, tc.marker, got, tc.status)
		}
		// Check it has dots
		if !strings.Contains(got, "..") {
			t.Errorf("dotPad(%q, %q, %q) = %q, should contain dots", tc.name, tc.status, tc.marker, got)
		}
	}
}

func TestStatusMarker(t *testing.T) {
	if got := statusMarker(true); got != "[gate] " {
		t.Errorf("statusMarker(true) = %q, want %q", got, "[gate] ")
	}
	if got := statusMarker(false); got != "" {
		t.Errorf("statusMarker(false) = %q, want empty", got)
	}
}
