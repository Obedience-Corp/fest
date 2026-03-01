package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
)

func createTestGuidanceContext(t *testing.T) *guidance.GuidanceContext {
	t.Helper()
	tmpDir := t.TempDir()

	// Create minimal festival structure
	if err := os.MkdirAll(filepath.Join(tmpDir, ".fest"), 0755); err != nil {
		t.Fatalf("failed to create .fest dir: %v", err)
	}

	return &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		PhasePath:    tmpDir,
	}
}

func createTestWorkflow(t *testing.T, dir string) {
	t.Helper()

	content := `# Test Workflow

## Step 1: READ — Understand

**Goal:** Build understanding.

**Actions:**
1. Read docs
2. Note key points

**Output:** Understanding

**Checkpoint:** None — proceed

---

## Step 2: PROCESS — Analyze

**Goal:** Analyze findings.

**Actions:**
1. Review notes
2. Synthesize

**Output:** Analysis

**Checkpoint:** APPROVAL REQUIRED — Wait for user
`

	if err := os.WriteFile(filepath.Join(dir, "WORKFLOW.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write WORKFLOW.md: %v", err)
	}
}

func TestNewNavigator(t *testing.T) {
	gctx := createTestGuidanceContext(t)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if nav == nil {
		t.Fatal("NewNavigator() returned nil")
	}

	if nav.parser == nil {
		t.Error("Navigator.parser should not be nil")
	}

	if nav.mode != guidance.ModeIngest {
		t.Errorf("Navigator.mode = %v, want %v", nav.mode, guidance.ModeIngest)
	}
}

func TestNewNavigator_NilContext(t *testing.T) {
	_, err := NewNavigator(nil, guidance.ModeIngest)
	if err == nil {
		t.Error("NewNavigator(nil) should return error")
	}
}

func TestNavigator_Initialize_NoWorkflow(t *testing.T) {
	gctx := createTestGuidanceContext(t)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Should have empty steps
	if len(nav.steps) != 0 {
		t.Errorf("Navigator.steps should be empty, got %d", len(nav.steps))
	}

	if nav.workflowState == nil {
		t.Error("Navigator.workflowState should not be nil")
	}
}

func TestNavigator_Initialize_WithWorkflow(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Should have parsed steps
	if len(nav.steps) != 2 {
		t.Errorf("Navigator.steps = %d, want 2", len(nav.steps))
	}

	if nav.workflowState.TotalSteps != 2 {
		t.Errorf("TotalSteps = %d, want 2", nav.workflowState.TotalSteps)
	}
}

func TestNavigator_Initialize_ContextCancelled(t *testing.T) {
	gctx := createTestGuidanceContext(t)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = nav.Initialize(ctx)
	if err == nil {
		t.Error("Initialize() with cancelled context should return error")
	}
}

func TestNavigator_GetNext_NoSteps(t *testing.T) {
	gctx := createTestGuidanceContext(t)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	step, err := nav.GetNext(context.Background())
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step != nil {
		t.Error("GetNext() should return nil when no steps")
	}
}

func TestNavigator_GetNext_WithSteps(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	step, err := nav.GetNext(context.Background())
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step == nil {
		t.Fatal("GetNext() returned nil")
	}

	if step.StepType != StepTypeWorkflowStep {
		t.Errorf("StepType = %q, want %q", step.StepType, StepTypeWorkflowStep)
	}

	if step.ID != "step_1" {
		t.Errorf("ID = %q, want step_1", step.ID)
	}

	if step.Mode != guidance.ModeIngest {
		t.Errorf("Mode = %v, want %v", step.Mode, guidance.ModeIngest)
	}
}

func TestNavigator_GetNext_ContextCancelled(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = nav.GetNext(ctx)
	if err == nil {
		t.Error("GetNext() with cancelled context should return error")
	}
}

func TestNavigator_MarkComplete(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Complete first step
	if err := nav.MarkComplete(context.Background(), "step_1"); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	// Should have advanced to step 2
	if nav.workflowState.CurrentStep != 2 {
		t.Errorf("CurrentStep = %d, want 2", nav.workflowState.CurrentStep)
	}
}

func TestNavigator_MarkSkipped(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if err := nav.MarkSkipped(context.Background(), "step_1"); err != nil {
		t.Fatalf("MarkSkipped() error = %v", err)
	}

	stepState := nav.workflowState.GetStepState(1)
	if stepState == nil {
		t.Fatal("Step state should not be nil")
	}
	if stepState.Status != StepStatusSkipped {
		t.Errorf("Status = %v, want %v", stepState.Status, StepStatusSkipped)
	}

	// Should have advanced
	if nav.workflowState.CurrentStep != 2 {
		t.Errorf("CurrentStep = %d, want 2", nav.workflowState.CurrentStep)
	}
}

func TestNavigator_MarkFailed(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if err := nav.MarkFailed(context.Background(), "step_1"); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	// Step should be blocked with feedback
	stepState := nav.workflowState.GetStepState(1)
	if stepState == nil {
		t.Fatal("Step state should not be nil")
	}
	if stepState.Status != StepStatusBlocked {
		t.Errorf("Status = %v, want %v", stepState.Status, StepStatusBlocked)
	}
}

func TestNavigator_Advance(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Advance through all steps
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("Advance() error = %v", err)
	}

	if nav.workflowState.CurrentStep != 2 {
		t.Errorf("CurrentStep = %d, want 2", nav.workflowState.CurrentStep)
	}

	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("Advance() error = %v", err)
	}

	// Workflow should be complete
	if !nav.workflowState.IsComplete() {
		t.Error("Workflow should be complete")
	}

	// Further advance should return error
	err = nav.Advance(context.Background())
	if err != guidance.ErrAlreadyComplete {
		t.Errorf("Advance() on complete workflow should return ErrAlreadyComplete, got %v", err)
	}
}

func TestNavigator_GetProgress(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	progress, err := nav.GetProgress(context.Background())
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}

	if progress.Total != 2 {
		t.Errorf("Total = %d, want 2", progress.Total)
	}

	if progress.Completed != 0 {
		t.Errorf("Completed = %d, want 0", progress.Completed)
	}

	if progress.CurrentTask != "READ" {
		t.Errorf("CurrentTask = %q, want READ", progress.CurrentTask)
	}

	// Advance and check again
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("Advance() error = %v", err)
	}

	progress, err = nav.GetProgress(context.Background())
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}

	if progress.Completed != 1 {
		t.Errorf("Completed = %d, want 1", progress.Completed)
	}
}

func TestNavigator_FormatInstructions(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	instructions, err := nav.FormatInstructions(context.Background())
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	// Check content - uses workflow/step template
	if !contains(instructions, "## Step") {
		t.Error("Instructions should contain '## Step'")
	}

	if !contains(instructions, "READ") {
		t.Error("Instructions should contain step name 'READ'")
	}
}

func TestNavigator_FormatInstructions_Complete(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Complete all steps
	for i := 0; i < 2; i++ {
		if err := nav.Advance(context.Background()); err != nil {
			t.Fatalf("Advance() error = %v", err)
		}
	}

	instructions, err := nav.FormatInstructions(context.Background())
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	// Uses workflow/complete template
	if !contains(instructions, "PHASE COMPLETE") {
		t.Error("Instructions should contain 'PHASE COMPLETE'")
	}
}

func TestNavigator_FormatInstructions_NoSteps(t *testing.T) {
	gctx := createTestGuidanceContext(t)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	instructions, err := nav.FormatInstructions(context.Background())
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	if !contains(instructions, "No workflow steps defined") {
		t.Error("Instructions should contain 'No workflow steps defined'")
	}
}

func TestNavigator_GetContextFiles(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	// Create FESTIVAL_GOAL.md
	if err := os.WriteFile(filepath.Join(gctx.FestivalPath, "FESTIVAL_GOAL.md"), []byte("# Goal"), 0644); err != nil {
		t.Fatalf("failed to write FESTIVAL_GOAL.md: %v", err)
	}

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	files, err := nav.GetContextFiles(context.Background())
	if err != nil {
		t.Fatalf("GetContextFiles() error = %v", err)
	}

	if len(files) < 1 {
		t.Error("GetContextFiles() should return at least one file")
	}
}

func TestNavigator_Approve(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if err := nav.Approve(context.Background()); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
}

func TestNavigator_Reject(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if err := nav.Reject(context.Background(), "needs revision"); err != nil {
		t.Fatalf("Reject() error = %v", err)
	}

	// Step should be blocked with feedback
	stepState := nav.workflowState.GetStepState(1)
	if stepState == nil {
		t.Fatal("Step state should not be nil")
	}
	if stepState.Status != StepStatusBlocked {
		t.Errorf("Status = %v, want %v", stepState.Status, StepStatusBlocked)
	}
	if stepState.Feedback != "needs revision" {
		t.Errorf("Feedback = %q, want %q", stepState.Feedback, "needs revision")
	}
}

func TestNavigator_GetWorkflowState(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	state := nav.GetWorkflowState()
	if state == nil {
		t.Fatal("GetWorkflowState() should not return nil")
	}

	if state.TotalSteps != 2 {
		t.Errorf("TotalSteps = %d, want 2", state.TotalSteps)
	}
}

func TestNavigator_GetSteps(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	steps := nav.GetSteps()
	if len(steps) != 2 {
		t.Errorf("GetSteps() returned %d steps, want 2", len(steps))
	}
}

func TestNavigator_Reset(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Advance a few steps
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("Advance() error = %v", err)
	}

	state := nav.GetWorkflowState()
	if state.CurrentStep != 2 {
		t.Fatalf("CurrentStep = %d, want 2", state.CurrentStep)
	}

	// Reset the workflow
	if err := nav.Reset(context.Background()); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	// Verify state is reset
	state = nav.GetWorkflowState()
	if state.CurrentStep != 1 {
		t.Errorf("CurrentStep after reset = %d, want 1", state.CurrentStep)
	}

	// Verify all steps are pending
	for _, stepState := range state.Steps {
		if stepState.Status != StepStatusPending {
			t.Errorf("Step %d status = %s, want pending", stepState.Number, stepState.Status)
		}
	}
}

func TestNavigator_Reset_ContextCancellation(t *testing.T) {
	gctx := createTestGuidanceContext(t)
	createTestWorkflow(t, gctx.FestivalPath)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = nav.Reset(ctx)
	if err == nil || err.Error() != "context canceled" {
		t.Errorf("Reset() with cancelled context should return context canceled, got %v", err)
	}
}

func TestNavigator_getAutonomyLevel(t *testing.T) {
	gctx := createTestGuidanceContext(t)

	nav, err := NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	// Non-blocking step
	step := WorkflowStep{Checkpoint: CheckpointNone}
	if nav.getAutonomyLevel(step) != guidance.AutonomyMedium {
		t.Error("Non-blocking step should have AutonomyMedium")
	}

	// Blocking step
	step = WorkflowStep{Checkpoint: CheckpointUserApproval}
	if nav.getAutonomyLevel(step) != guidance.AutonomyLow {
		t.Error("Blocking step should have AutonomyLow")
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
