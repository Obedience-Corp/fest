package execution

import (
	"testing"
)

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"pending", StatusPending, "pending"},
		{"in_progress", StatusInProgress, "in_progress"},
		{"completed", StatusCompleted, "completed"},
		{"skipped", StatusSkipped, "skipped"},
		{"failed", StatusFailed, "failed"},
		{"blocked", StatusBlocked, "blocked"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.want {
				t.Errorf("Status constant %s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestTaskInfo_Fields(t *testing.T) {
	task := &TaskInfo{
		ID:            "test-task-1",
		Name:          "Test Task",
		Path:          "/path/to/task.md",
		Number:        1,
		AutonomyLevel: "high",
		Dependencies:  []string{"dep-1", "dep-2"},
		Status:        StatusPending,
		IsGate:        false,
	}

	if task.ID != "test-task-1" {
		t.Errorf("ID = %q, want %q", task.ID, "test-task-1")
	}
	if task.Name != "Test Task" {
		t.Errorf("Name = %q, want %q", task.Name, "Test Task")
	}
	if task.Path != "/path/to/task.md" {
		t.Errorf("Path = %q, want %q", task.Path, "/path/to/task.md")
	}
	if task.Number != 1 {
		t.Errorf("Number = %d, want %d", task.Number, 1)
	}
	if task.AutonomyLevel != "high" {
		t.Errorf("AutonomyLevel = %q, want %q", task.AutonomyLevel, "high")
	}
	if len(task.Dependencies) != 2 {
		t.Errorf("Dependencies length = %d, want %d", len(task.Dependencies), 2)
	}
	if task.Status != StatusPending {
		t.Errorf("Status = %q, want %q", task.Status, StatusPending)
	}
	if task.IsGate {
		t.Error("IsGate should be false")
	}
}

func TestStepGroup_Fields(t *testing.T) {
	step := &StepGroup{
		Number:   1,
		Type:     "parallel",
		Tasks:    []*TaskInfo{{ID: "task-1"}, {ID: "task-2"}},
		Parallel: true,
	}

	if step.Number != 1 {
		t.Errorf("Number = %d, want %d", step.Number, 1)
	}
	if step.Type != "parallel" {
		t.Errorf("Type = %q, want %q", step.Type, "parallel")
	}
	if len(step.Tasks) != 2 {
		t.Errorf("Tasks length = %d, want %d", len(step.Tasks), 2)
	}
	if !step.Parallel {
		t.Error("Parallel should be true")
	}
}

func TestSequenceExecution_Fields(t *testing.T) {
	seq := &SequenceExecution{
		Name:       "01_foundation",
		Path:       "/path/to/seq",
		Number:     1,
		Steps:      []*StepGroup{{Number: 1}},
		TotalTasks: 5,
		Status:     StatusInProgress,
	}

	if seq.Name != "01_foundation" {
		t.Errorf("Name = %q, want %q", seq.Name, "01_foundation")
	}
	if seq.Number != 1 {
		t.Errorf("Number = %d, want %d", seq.Number, 1)
	}
	if len(seq.Steps) != 1 {
		t.Errorf("Steps length = %d, want %d", len(seq.Steps), 1)
	}
	if seq.TotalTasks != 5 {
		t.Errorf("TotalTasks = %d, want %d", seq.TotalTasks, 5)
	}
	if seq.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", seq.Status, StatusInProgress)
	}
}

func TestPhaseExecution_Fields(t *testing.T) {
	phase := &PhaseExecution{
		Name:        "001_Core",
		Path:        "/path/to/phase",
		Number:      1,
		Sequences:   []*SequenceExecution{{Name: "seq-1"}},
		QualityGate: &QualityGateInfo{Type: "code_review"},
		TotalTasks:  10,
		Status:      StatusPending,
	}

	if phase.Name != "001_Core" {
		t.Errorf("Name = %q, want %q", phase.Name, "001_Core")
	}
	if phase.Number != 1 {
		t.Errorf("Number = %d, want %d", phase.Number, 1)
	}
	if len(phase.Sequences) != 1 {
		t.Errorf("Sequences length = %d, want %d", len(phase.Sequences), 1)
	}
	if phase.QualityGate == nil {
		t.Fatal("QualityGate should not be nil")
	}
	if phase.TotalTasks != 10 {
		t.Errorf("TotalTasks = %d, want %d", phase.TotalTasks, 10)
	}
}

func TestExecutionPlan_Fields(t *testing.T) {
	plan := &ExecutionPlan{
		FestivalPath: "/path/to/festival",
		Phases:       []*PhaseExecution{{Name: "phase-1"}},
		Summary: &ExecutionSummary{
			TotalPhases:    1,
			TotalSequences: 2,
			TotalTasks:     10,
			TotalSteps:     5,
			ParallelGroups: 2,
			QualityGates:   1,
		},
	}

	if plan.FestivalPath != "/path/to/festival" {
		t.Errorf("FestivalPath = %q, want %q", plan.FestivalPath, "/path/to/festival")
	}
	if len(plan.Phases) != 1 {
		t.Errorf("Phases length = %d, want %d", len(plan.Phases), 1)
	}
	if plan.Summary == nil {
		t.Fatal("Summary should not be nil")
	}
	if plan.Summary.TotalTasks != 10 {
		t.Errorf("Summary.TotalTasks = %d, want %d", plan.Summary.TotalTasks, 10)
	}
}

func TestQualityGateInfo_Fields(t *testing.T) {
	gate := &QualityGateInfo{
		PhaseName:   "001_Core",
		Type:        "code_review",
		Description: "Code review checkpoint",
		Criteria:    []string{"tests pass", "lint clean"},
		Passed:      false,
	}

	if gate.PhaseName != "001_Core" {
		t.Errorf("PhaseName = %q, want %q", gate.PhaseName, "001_Core")
	}
	if gate.Type != "code_review" {
		t.Errorf("Type = %q, want %q", gate.Type, "code_review")
	}
	if len(gate.Criteria) != 2 {
		t.Errorf("Criteria length = %d, want %d", len(gate.Criteria), 2)
	}
	if gate.Passed {
		t.Error("Passed should be false")
	}
}
