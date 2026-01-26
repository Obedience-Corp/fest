package guidance

import (
	"testing"
)

func TestNewNextStep(t *testing.T) {
	step := NewNextStep(ModePlan, StepTypePlanningStep, "step1", "Plan Step", "/path/to/step")

	if step.Mode != ModePlan {
		t.Errorf("Mode = %v, want ModePlan", step.Mode)
	}
	if step.StepType != StepTypePlanningStep {
		t.Errorf("StepType = %v, want %v", step.StepType, StepTypePlanningStep)
	}
	if step.ID != "step1" {
		t.Errorf("ID = %v, want step1", step.ID)
	}
	if step.Title != "Plan Step" {
		t.Errorf("Title = %v, want 'Plan Step'", step.Title)
	}
	if step.Path != "/path/to/step" {
		t.Errorf("Path = %v, want /path/to/step", step.Path)
	}
}

func TestNewTaskStep(t *testing.T) {
	step := NewTaskStep("task1", "Test Task", "/path/to/task.md", AutonomyHigh)

	if step.Mode != ModeExecute {
		t.Errorf("Mode = %v, want ModeExecute", step.Mode)
	}
	if step.StepType != StepTypeTask {
		t.Errorf("StepType = %v, want %v", step.StepType, StepTypeTask)
	}
	if step.ID != "task1" {
		t.Errorf("ID = %v, want task1", step.ID)
	}
	if step.Title != "Test Task" {
		t.Errorf("Title = %v, want 'Test Task'", step.Title)
	}
	if step.Path != "/path/to/task.md" {
		t.Errorf("Path = %v, want /path/to/task.md", step.Path)
	}
	if step.AutonomyLevel != AutonomyHigh {
		t.Errorf("AutonomyLevel = %v, want AutonomyHigh", step.AutonomyLevel)
	}
}

func TestNewGateStep(t *testing.T) {
	step := NewGateStep("gate1", "Test Gate", "/path/to/gate.md")

	if step.Mode != ModeExecute {
		t.Errorf("Mode = %v, want ModeExecute", step.Mode)
	}
	if step.StepType != StepTypeGate {
		t.Errorf("StepType = %v, want %v", step.StepType, StepTypeGate)
	}
	if step.AutonomyLevel != AutonomyLow {
		t.Errorf("AutonomyLevel = %v, want AutonomyLow (gates require review)", step.AutonomyLevel)
	}
}

func TestNextStep_IsGate(t *testing.T) {
	tests := []struct {
		name string
		step *NextStep
		want bool
	}{
		{
			name: "task is not a gate",
			step: NewTaskStep("t1", "Task", "/path", AutonomyHigh),
			want: false,
		},
		{
			name: "gate is a gate",
			step: NewGateStep("g1", "Gate", "/path"),
			want: true,
		},
		{
			name: "planning step is not a gate",
			step: NewNextStep(ModePlan, StepTypePlanningStep, "p1", "Plan", "/path"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.step.IsGate(); got != tt.want {
				t.Errorf("IsGate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNextStep_IsParallel(t *testing.T) {
	tests := []struct {
		name string
		step *NextStep
		want bool
	}{
		{
			name: "single task is not parallel",
			step: NewTaskStep("t1", "Task", "/path", AutonomyHigh),
			want: false,
		},
		{
			name: "parallel flag without group is not parallel",
			step: func() *NextStep {
				s := NewNextStep(ModeExecute, StepTypeTask, "p1", "Parallel", "")
				s.Parallel = true // Flag set but no group
				return s
			}(),
			want: false,
		},
		{
			name: "step with group is parallel",
			step: func() *NextStep {
				s := NewNextStep(ModeExecute, StepTypeTask, "p1", "Parallel", "")
				s.AddToGroup(NewTaskStep("t2", "T2", "/p2", AutonomyHigh))
				s.AddToGroup(NewTaskStep("t3", "T3", "/p3", AutonomyHigh))
				return s
			}(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.step.IsParallel(); got != tt.want {
				t.Errorf("IsParallel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNextStep_RequiresHumanReview(t *testing.T) {
	tests := []struct {
		name string
		step *NextStep
		want bool
	}{
		{
			name: "gate requires review",
			step: NewGateStep("g1", "Gate", "/path"),
			want: true,
		},
		{
			name: "low autonomy task requires review",
			step: NewTaskStep("t1", "Task", "/path", AutonomyLow),
			want: true,
		},
		{
			name: "medium autonomy task does not require review",
			step: NewTaskStep("t1", "Task", "/path", AutonomyMedium),
			want: false,
		},
		{
			name: "high autonomy task does not require review",
			step: NewTaskStep("t1", "Task", "/path", AutonomyHigh),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.step.RequiresHumanReview(); got != tt.want {
				t.Errorf("RequiresHumanReview() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNextStep_TaskCount(t *testing.T) {
	tests := []struct {
		name string
		step *NextStep
		want int
	}{
		{
			name: "single task returns 1",
			step: NewTaskStep("t1", "Task", "/path", AutonomyHigh),
			want: 1,
		},
		{
			name: "parallel step returns group count",
			step: func() *NextStep {
				s := NewNextStep(ModeExecute, StepTypeTask, "p1", "Parallel", "")
				s.AddToGroup(NewTaskStep("t2", "T2", "/p2", AutonomyHigh))
				s.AddToGroup(NewTaskStep("t3", "T3", "/p3", AutonomyHigh))
				s.AddToGroup(NewTaskStep("t4", "T4", "/p4", AutonomyHigh))
				return s
			}(),
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.step.TaskCount(); got != tt.want {
				t.Errorf("TaskCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNextStep_GetCompletionCommand(t *testing.T) {
	tests := []struct {
		name string
		step *NextStep
		want string
	}{
		{
			name: "default completion command uses path",
			step: NewTaskStep("t1", "Task", "/path/to/task.md", AutonomyHigh),
			want: "fest progress --task /path/to/task.md --complete",
		},
		{
			name: "custom completion command is used",
			step: func() *NextStep {
				s := NewTaskStep("t1", "Task", "/path/to/task.md", AutonomyHigh)
				s.CompletionCommand = "custom-command --do-thing"
				return s
			}(),
			want: "custom-command --do-thing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.step.GetCompletionCommand(); got != tt.want {
				t.Errorf("GetCompletionCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNextStep_WithInstructions(t *testing.T) {
	step := NewTaskStep("t1", "Task", "/path", AutonomyHigh).
		WithInstructions("Step 1", "Step 2").
		WithInstructions("Step 3")

	if len(step.Instructions) != 3 {
		t.Errorf("Instructions count = %d, want 3", len(step.Instructions))
	}
	if step.Instructions[0] != "Step 1" {
		t.Errorf("Instructions[0] = %v, want 'Step 1'", step.Instructions[0])
	}
	if step.Instructions[2] != "Step 3" {
		t.Errorf("Instructions[2] = %v, want 'Step 3'", step.Instructions[2])
	}
}

func TestNextStep_WithObjective(t *testing.T) {
	step := NewTaskStep("t1", "Task", "/path", AutonomyHigh).
		WithObjective("Complete the feature")

	if step.Objective != "Complete the feature" {
		t.Errorf("Objective = %v, want 'Complete the feature'", step.Objective)
	}
}

func TestNextStep_WithContextFiles(t *testing.T) {
	step := NewTaskStep("t1", "Task", "/path", AutonomyHigh).
		WithContextFiles("/path/to/goal.md", "/path/to/seq.md").
		WithContextFiles("/path/to/phase.md")

	if len(step.ContextFiles) != 3 {
		t.Errorf("ContextFiles count = %d, want 3", len(step.ContextFiles))
	}
}

func TestNextStep_WithDependencies(t *testing.T) {
	step := NewTaskStep("t1", "Task", "/path", AutonomyHigh).
		WithDependencies("task1", "task2", "task3")

	if len(step.Dependencies) != 3 {
		t.Errorf("Dependencies count = %d, want 3", len(step.Dependencies))
	}
	if step.Dependencies[0] != "task1" {
		t.Errorf("Dependencies[0] = %v, want 'task1'", step.Dependencies[0])
	}
}

func TestNextStep_WithMetadata(t *testing.T) {
	step := NewTaskStep("t1", "Task", "/path", AutonomyHigh).
		WithMetadata("iteration", 1).
		WithMetadata("approved", true).
		WithMetadata("name", "test")

	if step.Metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	if step.Metadata["iteration"] != 1 {
		t.Errorf("Metadata[iteration] = %v, want 1", step.Metadata["iteration"])
	}
	if step.Metadata["approved"] != true {
		t.Errorf("Metadata[approved] = %v, want true", step.Metadata["approved"])
	}
	if step.Metadata["name"] != "test" {
		t.Errorf("Metadata[name] = %v, want 'test'", step.Metadata["name"])
	}
}

func TestNextStep_AddToGroup(t *testing.T) {
	parent := NewNextStep(ModeExecute, StepTypeTask, "parent", "Parent", "")
	child1 := NewTaskStep("c1", "Child 1", "/c1", AutonomyHigh)
	child2 := NewTaskStep("c2", "Child 2", "/c2", AutonomyHigh)

	parent.AddToGroup(child1).AddToGroup(child2)

	if !parent.Parallel {
		t.Error("Parallel should be true after AddToGroup")
	}
	if len(parent.Group) != 2 {
		t.Errorf("Group count = %d, want 2", len(parent.Group))
	}
	if parent.Group[0] != child1 {
		t.Error("Group[0] should be child1")
	}
	if parent.Group[1] != child2 {
		t.Error("Group[1] should be child2")
	}
}

func TestNextStep_FluentChaining(t *testing.T) {
	step := NewTaskStep("t1", "Task", "/path", AutonomyHigh).
		WithObjective("Test objective").
		WithInstructions("Step 1", "Step 2").
		WithContextFiles("/context1", "/context2").
		WithDependencies("dep1", "dep2").
		WithMetadata("key", "value")

	// Verify all fields were set
	if step.Objective != "Test objective" {
		t.Error("Fluent chaining failed for Objective")
	}
	if len(step.Instructions) != 2 {
		t.Error("Fluent chaining failed for Instructions")
	}
	if len(step.ContextFiles) != 2 {
		t.Error("Fluent chaining failed for ContextFiles")
	}
	if len(step.Dependencies) != 2 {
		t.Error("Fluent chaining failed for Dependencies")
	}
	if step.Metadata["key"] != "value" {
		t.Error("Fluent chaining failed for Metadata")
	}
}

func TestStepTypeConstants(t *testing.T) {
	// Ensure constants have expected values
	tests := []struct {
		constant string
		want     string
	}{
		{StepTypeTask, "task"},
		{StepTypeGate, "gate"},
		{StepTypePlanningStep, "planning_step"},
		{StepTypeIngestStep, "ingest_step"},
		{StepTypeResearchStep, "research_step"},
		{StepTypeReviewStep, "review_step"},
		{StepTypeActionStep, "action_step"},
		{StepTypeApproval, "approval"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if tt.constant != tt.want {
				t.Errorf("StepType constant = %v, want %v", tt.constant, tt.want)
			}
		})
	}
}
