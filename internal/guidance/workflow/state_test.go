package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWorkflowState(t *testing.T) {
	state := NewWorkflowState(5)

	if state.CurrentStep != 1 {
		t.Errorf("CurrentStep = %d, want 1", state.CurrentStep)
	}

	if state.TotalSteps != 5 {
		t.Errorf("TotalSteps = %d, want 5", state.TotalSteps)
	}

	if state.Steps == nil {
		t.Error("Steps map should not be nil")
	}

	if state.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	if state.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestNewFestivalWorkflowState(t *testing.T) {
	festState := NewFestivalWorkflowState()

	if festState.Version != stateVersion {
		t.Errorf("Version = %d, want %d", festState.Version, stateVersion)
	}

	if festState.Phases == nil {
		t.Error("Phases map should not be nil")
	}

	if festState.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	if festState.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestWorkflowState_SaveLoad(t *testing.T) {
	festivalDir := t.TempDir()
	phaseName := "001_INGEST"

	state := NewWorkflowState(5)
	state.CurrentStep = 3
	state.Steps[1] = &StepState{
		Number: 1,
		Status: StepStatusCompleted,
	}
	state.Steps[2] = &StepState{
		Number: 2,
		Status: StepStatusCompleted,
	}

	ctx := context.Background()
	if err := state.Save(ctx, festivalDir, phaseName); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists at festival level
	statePath := filepath.Join(festivalDir, ".fest", "workflow_state.yaml")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("State file was not created")
	}

	// Load state
	loaded, err := LoadState(ctx, festivalDir, phaseName)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if loaded.CurrentStep != state.CurrentStep {
		t.Errorf("Loaded CurrentStep = %d, want %d", loaded.CurrentStep, state.CurrentStep)
	}

	if loaded.TotalSteps != state.TotalSteps {
		t.Errorf("Loaded TotalSteps = %d, want %d", loaded.TotalSteps, state.TotalSteps)
	}

	if len(loaded.Steps) != 2 {
		t.Errorf("Loaded Steps has %d entries, want 2", len(loaded.Steps))
	}

	if loaded.Steps[1].Status != StepStatusCompleted {
		t.Errorf("Loaded Steps[1].Status = %v, want StepStatusCompleted", loaded.Steps[1].Status)
	}
}

func TestWorkflowState_SaveLoad_MultiplePhases(t *testing.T) {
	festivalDir := t.TempDir()

	ctx := context.Background()

	// Save state for phase 1
	state1 := NewWorkflowState(3)
	state1.CurrentStep = 2
	if err := state1.Save(ctx, festivalDir, "001_INGEST"); err != nil {
		t.Fatalf("Save phase 1 error = %v", err)
	}

	// Save state for phase 2
	state2 := NewWorkflowState(5)
	state2.CurrentStep = 4
	if err := state2.Save(ctx, festivalDir, "002_RESEARCH"); err != nil {
		t.Fatalf("Save phase 2 error = %v", err)
	}

	// Load and verify phase 1
	loaded1, err := LoadState(ctx, festivalDir, "001_INGEST")
	if err != nil {
		t.Fatalf("LoadState phase 1 error = %v", err)
	}
	if loaded1.CurrentStep != 2 {
		t.Errorf("Phase 1 CurrentStep = %d, want 2", loaded1.CurrentStep)
	}
	if loaded1.TotalSteps != 3 {
		t.Errorf("Phase 1 TotalSteps = %d, want 3", loaded1.TotalSteps)
	}

	// Load and verify phase 2
	loaded2, err := LoadState(ctx, festivalDir, "002_RESEARCH")
	if err != nil {
		t.Fatalf("LoadState phase 2 error = %v", err)
	}
	if loaded2.CurrentStep != 4 {
		t.Errorf("Phase 2 CurrentStep = %d, want 4", loaded2.CurrentStep)
	}
	if loaded2.TotalSteps != 5 {
		t.Errorf("Phase 2 TotalSteps = %d, want 5", loaded2.TotalSteps)
	}
}

func TestLoadState_NotExists(t *testing.T) {
	festivalDir := t.TempDir()

	ctx := context.Background()
	state, err := LoadState(ctx, festivalDir, "001_INGEST")
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if state == nil {
		t.Fatal("LoadState() returned nil for non-existent file")
	}

	if state.CurrentStep != 1 {
		t.Errorf("Default CurrentStep = %d, want 1", state.CurrentStep)
	}

	if state.TotalSteps != 0 {
		t.Errorf("Default TotalSteps = %d, want 0", state.TotalSteps)
	}
}

func TestWorkflowState_Initialize(t *testing.T) {
	state := NewWorkflowState(0)

	steps := []WorkflowStep{
		{Number: 1, Name: "READ"},
		{Number: 2, Name: "PROCESS"},
		{Number: 3, Name: "COMPLETE"},
	}

	state.Initialize(steps)

	if state.TotalSteps != 3 {
		t.Errorf("TotalSteps = %d, want 3", state.TotalSteps)
	}

	if len(state.Steps) != 3 {
		t.Errorf("Steps has %d entries, want 3", len(state.Steps))
	}

	for i := 1; i <= 3; i++ {
		step := state.Steps[i]
		if step == nil {
			t.Errorf("Steps[%d] is nil", i)
			continue
		}
		if step.Number != i {
			t.Errorf("Steps[%d].Number = %d, want %d", i, step.Number, i)
		}
		if step.Status != StepStatusPending {
			t.Errorf("Steps[%d].Status = %v, want StepStatusPending", i, step.Status)
		}
	}
}

func TestWorkflowState_GetCurrentStepState(t *testing.T) {
	state := NewWorkflowState(3)
	state.Steps[1] = &StepState{Number: 1, Status: StepStatusInProgress}

	current := state.GetCurrentStepState()
	if current == nil {
		t.Fatal("GetCurrentStepState() returned nil")
	}

	if current.Number != 1 {
		t.Errorf("Current step number = %d, want 1", current.Number)
	}
}

func TestWorkflowState_GetCurrentStepState_NoState(t *testing.T) {
	state := NewWorkflowState(3)

	current := state.GetCurrentStepState()
	if current != nil {
		t.Error("GetCurrentStepState() should return nil when no state exists")
	}
}

func TestWorkflowState_StartCurrentStep(t *testing.T) {
	state := NewWorkflowState(3)

	state.StartCurrentStep()

	current := state.GetCurrentStepState()
	if current == nil {
		t.Fatal("GetCurrentStepState() returned nil")
	}

	if current.Status != StepStatusInProgress {
		t.Errorf("Status = %v, want StepStatusInProgress", current.Status)
	}

	if current.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}
}

func TestWorkflowState_CompleteCurrentStep(t *testing.T) {
	state := NewWorkflowState(3)
	state.StartCurrentStep()

	state.CompleteCurrentStep()

	current := state.GetCurrentStepState()
	if current.Status != StepStatusCompleted {
		t.Errorf("Status = %v, want StepStatusCompleted", current.Status)
	}

	if current.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
}

func TestWorkflowState_Advance(t *testing.T) {
	state := NewWorkflowState(3)
	state.StartCurrentStep()
	state.CompleteCurrentStep()

	if err := state.Advance(); err != nil {
		t.Fatalf("Advance() error = %v", err)
	}

	if state.CurrentStep != 2 {
		t.Errorf("CurrentStep = %d, want 2", state.CurrentStep)
	}

	current := state.GetCurrentStepState()
	if current.Status != StepStatusInProgress {
		t.Errorf("New step Status = %v, want StepStatusInProgress", current.Status)
	}
}

func TestWorkflowState_Advance_NotCompleted(t *testing.T) {
	state := NewWorkflowState(3)
	state.StartCurrentStep()

	err := state.Advance()
	if err == nil {
		t.Error("Advance() should fail when current step is not completed")
	}
}

func TestWorkflowState_Advance_AtLastStep(t *testing.T) {
	state := NewWorkflowState(3)
	state.CurrentStep = 3
	state.Steps[3] = &StepState{Number: 3, Status: StepStatusCompleted}

	err := state.Advance()
	if err == nil {
		t.Error("Advance() should fail when at last step")
	}
}

func TestWorkflowState_Approve(t *testing.T) {
	state := NewWorkflowState(3)
	state.StartCurrentStep()

	if err := state.Approve(); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if state.Steps[1].Status != StepStatusCompleted {
		t.Errorf("Step 1 Status = %v, want StepStatusCompleted", state.Steps[1].Status)
	}

	if state.CurrentStep != 2 {
		t.Errorf("CurrentStep = %d, want 2", state.CurrentStep)
	}
}

func TestWorkflowState_Approve_AtLastStep(t *testing.T) {
	state := NewWorkflowState(2)
	state.CurrentStep = 2
	state.StartCurrentStep()

	if err := state.Approve(); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if state.Steps[2].Status != StepStatusCompleted {
		t.Errorf("Step 2 Status = %v, want StepStatusCompleted", state.Steps[2].Status)
	}

	if state.CurrentStep != 2 {
		t.Errorf("CurrentStep = %d, want 2 (should stay at last)", state.CurrentStep)
	}
}

func TestWorkflowState_Reject(t *testing.T) {
	state := NewWorkflowState(3)
	state.StartCurrentStep()

	state.Reject("needs more detail")

	current := state.GetCurrentStepState()
	if current.Status != StepStatusBlocked {
		t.Errorf("Status = %v, want StepStatusBlocked", current.Status)
	}

	if current.Feedback != "needs more detail" {
		t.Errorf("Feedback = %q, want 'needs more detail'", current.Feedback)
	}
}

func TestWorkflowState_RejectCancelsRunningJudge(t *testing.T) {
	state := NewWorkflowState(1)
	state.StartCurrentStep()
	state.BeginJudge(1, "ob judge", "run-1", 42, time.Now().UTC())

	state.Reject("operator override")

	judge := state.GetStepState(1).Judge
	if judge == nil || judge.Status != JudgeCanceled || judge.RunID != "run-1" {
		t.Fatalf("manual rejection did not cancel judge lease: %+v", judge)
	}
	if state.JudgeOwned(1, "run-1") {
		t.Fatal("canceled judge must not retain decision ownership")
	}
}

func TestWorkflowState_Reset(t *testing.T) {
	state := NewWorkflowState(3)

	// Complete some steps
	state.StartCurrentStep()
	state.CompleteCurrentStep()
	if err := state.Advance(); err != nil {
		t.Fatalf("Advance() error = %v", err)
	}
	state.StartCurrentStep()
	state.CompleteCurrentStep()
	state.CurrentStep = 3

	// A delegated judge run recorded on a step must not survive a reset,
	// otherwise ghost waiting/outcome UI persists after the workflow is reset.
	state.Steps[2].Judge = &JudgeState{Status: JudgeRunning, Command: "ob judge"}

	state.Reset()

	if state.CurrentStep != 1 {
		t.Errorf("CurrentStep = %d, want 1", state.CurrentStep)
	}

	for i, step := range state.Steps {
		if step.Status != StepStatusPending {
			t.Errorf("Steps[%d].Status = %v, want StepStatusPending", i, step.Status)
		}
		if step.StartedAt != nil {
			t.Errorf("Steps[%d].StartedAt should be nil", i)
		}
		if step.CompletedAt != nil {
			t.Errorf("Steps[%d].CompletedAt should be nil", i)
		}
		if step.Feedback != "" {
			t.Errorf("Steps[%d].Feedback should be empty", i)
		}
		if step.Judge != nil {
			t.Errorf("Steps[%d].Judge should be nil after reset", i)
		}
	}
}

func TestWorkflowState_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*WorkflowState)
		expected bool
	}{
		{
			name: "all completed",
			setup: func(s *WorkflowState) {
				s.TotalSteps = 3
				s.Steps[1] = &StepState{Number: 1, Status: StepStatusCompleted}
				s.Steps[2] = &StepState{Number: 2, Status: StepStatusCompleted}
				s.Steps[3] = &StepState{Number: 3, Status: StepStatusCompleted}
			},
			expected: true,
		},
		{
			name: "all skipped",
			setup: func(s *WorkflowState) {
				s.TotalSteps = 3
				s.Steps[1] = &StepState{Number: 1, Status: StepStatusSkipped}
				s.Steps[2] = &StepState{Number: 2, Status: StepStatusSkipped}
				s.Steps[3] = &StepState{Number: 3, Status: StepStatusSkipped}
			},
			expected: true,
		},
		{
			name: "mixed completed and skipped",
			setup: func(s *WorkflowState) {
				s.TotalSteps = 3
				s.Steps[1] = &StepState{Number: 1, Status: StepStatusCompleted}
				s.Steps[2] = &StepState{Number: 2, Status: StepStatusSkipped}
				s.Steps[3] = &StepState{Number: 3, Status: StepStatusCompleted}
			},
			expected: true,
		},
		{
			name: "some pending",
			setup: func(s *WorkflowState) {
				s.TotalSteps = 3
				s.Steps[1] = &StepState{Number: 1, Status: StepStatusCompleted}
				s.Steps[2] = &StepState{Number: 2, Status: StepStatusPending}
				s.Steps[3] = &StepState{Number: 3, Status: StepStatusPending}
			},
			expected: false,
		},
		{
			name: "some in progress",
			setup: func(s *WorkflowState) {
				s.TotalSteps = 3
				s.Steps[1] = &StepState{Number: 1, Status: StepStatusCompleted}
				s.Steps[2] = &StepState{Number: 2, Status: StepStatusInProgress}
				s.Steps[3] = &StepState{Number: 3, Status: StepStatusPending}
			},
			expected: false,
		},
		{
			name: "zero steps",
			setup: func(s *WorkflowState) {
				s.TotalSteps = 0
			},
			expected: false,
		},
		{
			name: "missing step state",
			setup: func(s *WorkflowState) {
				s.TotalSteps = 3
				s.Steps[1] = &StepState{Number: 1, Status: StepStatusCompleted}
				// Steps 2 and 3 don't have state
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewWorkflowState(0)
			tt.setup(state)

			if got := state.IsComplete(); got != tt.expected {
				t.Errorf("IsComplete() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWorkflowState_CompletedCount(t *testing.T) {
	state := NewWorkflowState(5)
	state.Steps[1] = &StepState{Number: 1, Status: StepStatusCompleted}
	state.Steps[2] = &StepState{Number: 2, Status: StepStatusCompleted}
	state.Steps[3] = &StepState{Number: 3, Status: StepStatusSkipped}
	state.Steps[4] = &StepState{Number: 4, Status: StepStatusPending}

	count := state.CompletedCount()
	if count != 3 {
		t.Errorf("CompletedCount() = %d, want 3", count)
	}
}

func TestWorkflowState_ProgressPercent(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		completed int
		expected  float64
	}{
		{"zero total", 0, 0, 0},
		{"none completed", 5, 0, 0},
		{"half completed", 4, 2, 50},
		{"all completed", 5, 5, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewWorkflowState(tt.total)
			for i := 1; i <= tt.completed; i++ {
				state.Steps[i] = &StepState{Number: i, Status: StepStatusCompleted}
			}

			percent := state.ProgressPercent()
			if percent != tt.expected {
				t.Errorf("ProgressPercent() = %f, want %f", percent, tt.expected)
			}
		})
	}
}

func TestWorkflowState_ContextCancellation(t *testing.T) {
	festivalDir := t.TempDir()
	phaseName := "001_INGEST"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// LoadState should fail with cancelled context
	_, err := LoadState(ctx, festivalDir, phaseName)
	if err == nil {
		t.Error("LoadState() should fail with cancelled context")
	}

	// Save should fail with cancelled context
	state := NewWorkflowState(3)
	err = state.Save(ctx, festivalDir, phaseName)
	if err == nil {
		t.Error("Save() should fail with cancelled context")
	}
}

func TestWorkflowState_GetStepState(t *testing.T) {
	state := NewWorkflowState(3)
	state.Steps[2] = &StepState{Number: 2, Status: StepStatusInProgress}

	step := state.GetStepState(2)
	if step == nil {
		t.Fatal("GetStepState(2) returned nil")
	}

	if step.Number != 2 {
		t.Errorf("Step.Number = %d, want 2", step.Number)
	}

	// Non-existent step
	step = state.GetStepState(99)
	if step != nil {
		t.Error("GetStepState(99) should return nil for non-existent step")
	}
}

func TestStepStatus_Methods(t *testing.T) {
	tests := []struct {
		status     StepStatus
		wantString string
		wantValid  bool
		wantTerm   bool
	}{
		{StepStatusPending, "pending", true, false},
		{StepStatusInProgress, "in_progress", true, false},
		{StepStatusCompleted, "completed", true, true},
		{StepStatusSkipped, "skipped", true, true},
		{StepStatusBlocked, "blocked", true, false},
		{StepStatus("invalid"), "invalid", false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.String(); got != tt.wantString {
				t.Errorf("String() = %q, want %q", got, tt.wantString)
			}
			if got := tt.status.IsValid(); got != tt.wantValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.wantValid)
			}
			if got := tt.status.IsTerminal(); got != tt.wantTerm {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.wantTerm)
			}
		})
	}
}

func TestCheckpointType_Methods(t *testing.T) {
	tests := []struct {
		ctype        CheckpointType
		wantString   string
		wantValid    bool
		wantBlocking bool
	}{
		{CheckpointNone, "", true, false},
		{CheckpointUserApproval, "user_approval", true, true},
		{CheckpointDocumentation, "documentation", true, false},
		{CheckpointVerification, "verification", true, false},
		{CheckpointType("invalid"), "invalid", false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.ctype), func(t *testing.T) {
			if got := tt.ctype.String(); got != tt.wantString {
				t.Errorf("String() = %q, want %q", got, tt.wantString)
			}
			if got := tt.ctype.IsValid(); got != tt.wantValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.wantValid)
			}
			if got := tt.ctype.IsBlocking(); got != tt.wantBlocking {
				t.Errorf("IsBlocking() = %v, want %v", got, tt.wantBlocking)
			}
		})
	}
}

func TestWorkflowStep_HasCheckpoint(t *testing.T) {
	tests := []struct {
		name       string
		checkpoint CheckpointType
		expected   bool
	}{
		{"none", CheckpointNone, false},
		{"user approval", CheckpointUserApproval, true},
		{"documentation", CheckpointDocumentation, true},
		{"verification", CheckpointVerification, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := WorkflowStep{Checkpoint: tt.checkpoint}
			if got := step.HasCheckpoint(); got != tt.expected {
				t.Errorf("HasCheckpoint() = %v, want %v", got, tt.expected)
			}
		})
	}
}
