// Package e2e provides end-to-end tests for the fest CLI.
package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// TestWorkflowE2E_CompleteIngestWorkflow tests a complete ingest workflow cycle.
func TestWorkflowE2E_CompleteIngestWorkflow(t *testing.T) {
	// Setup: Create a complete festival with ingest phase
	festDir := setupE2EFestival(t)
	phaseDir := filepath.Join(festDir, "001_INGEST")

	// Create guidance context
	gctx := &guidance.GuidanceContext{
		FestivalPath: festDir,
		PhasePath:    phaseDir,
		PhaseName:    "001_INGEST",
	}

	// Initialize navigator
	nav, err := wf.NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("failed to create navigator: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize navigator: %v", err)
	}

	// Verify initial state
	state := nav.GetWorkflowState()
	if state.CurrentStep != 1 {
		t.Errorf("initial step = %d, want 1", state.CurrentStep)
	}
	if state.TotalSteps != 3 {
		t.Errorf("total steps = %d, want 3", state.TotalSteps)
	}

	// Step 1: Advance through first step (no checkpoint)
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("advance step 1: %v", err)
	}

	state = nav.GetWorkflowState()
	if state.CurrentStep != 2 {
		t.Errorf("after step 1, current step = %d, want 2", state.CurrentStep)
	}

	// Verify state was persisted
	nav2, _ := wf.NewNavigator(gctx, guidance.ModeIngest)
	if err := nav2.Initialize(ctx); err != nil {
		t.Fatalf("failed to reinitialize navigator: %v", err)
	}
	state2 := nav2.GetWorkflowState()
	if state2.CurrentStep != 2 {
		t.Errorf("persisted state current step = %d, want 2", state2.CurrentStep)
	}

	// Step 2: This step has a checkpoint - complete it
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("advance step 2: %v", err)
	}

	// Step 2 has checkpoint - needs approval
	if err := nav.Approve(ctx); err != nil {
		t.Fatalf("approve step 2: %v", err)
	}

	state = nav.GetWorkflowState()
	if state.CurrentStep != 3 {
		t.Errorf("after approve, current step = %d, want 3", state.CurrentStep)
	}

	// Step 3: Final step
	if err := nav.Advance(ctx); err != nil {
		// Might return already complete, that's ok
		if err != guidance.ErrAlreadyComplete {
			state = nav.GetWorkflowState()
			if !state.IsComplete() {
				t.Fatalf("advance step 3: %v", err)
			}
		}
	}

	// Verify workflow is complete
	state = nav.GetWorkflowState()
	if !state.IsComplete() {
		t.Error("workflow should be complete")
	}
}

// TestWorkflowE2E_CheckpointRejection tests the checkpoint rejection flow.
func TestWorkflowE2E_CheckpointRejection(t *testing.T) {
	festDir := setupE2EFestival(t)
	phaseDir := filepath.Join(festDir, "001_INGEST")

	gctx := &guidance.GuidanceContext{
		FestivalPath: festDir,
		PhasePath:    phaseDir,
		PhaseName:    "001_INGEST",
	}

	nav, err := wf.NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("failed to create navigator: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize navigator: %v", err)
	}

	// Advance to step 2 (checkpoint step)
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("advance to step 2: %v", err)
	}

	// Reject with feedback
	feedback := "Need more analysis"
	if err := nav.Reject(ctx, feedback); err != nil {
		t.Fatalf("reject: %v", err)
	}

	// Verify rejection state
	state := nav.GetWorkflowState()
	stepState := state.GetStepState(2)
	if stepState.Status != wf.StepStatusBlocked {
		t.Errorf("step 2 status = %v, want blocked", stepState.Status)
	}
	if stepState.Feedback != feedback {
		t.Errorf("step 2 feedback = %q, want %q", stepState.Feedback, feedback)
	}
}

// TestWorkflowE2E_Reset tests the workflow reset flow.
func TestWorkflowE2E_Reset(t *testing.T) {
	festDir := setupE2EFestival(t)
	phaseDir := filepath.Join(festDir, "001_INGEST")

	gctx := &guidance.GuidanceContext{
		FestivalPath: festDir,
		PhasePath:    phaseDir,
		PhaseName:    "001_INGEST",
	}

	nav, err := wf.NewNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		t.Fatalf("failed to create navigator: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize navigator: %v", err)
	}

	// Advance several steps
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("advance 1: %v", err)
	}
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("advance 2: %v", err)
	}
	if err := nav.Approve(ctx); err != nil {
		t.Fatalf("approve: %v", err)
	}

	state := nav.GetWorkflowState()
	if state.CurrentStep != 3 {
		t.Errorf("before reset, current step = %d, want 3", state.CurrentStep)
	}

	// Reset
	if err := nav.Reset(ctx); err != nil {
		t.Fatalf("reset: %v", err)
	}

	state = nav.GetWorkflowState()
	if state.CurrentStep != 1 {
		t.Errorf("after reset, current step = %d, want 1", state.CurrentStep)
	}

	// Verify persistence after reset
	nav2, _ := wf.NewNavigator(gctx, guidance.ModeIngest)
	if err := nav2.Initialize(ctx); err != nil {
		t.Fatalf("failed to reinitialize: %v", err)
	}
	state2 := nav2.GetWorkflowState()
	if state2.CurrentStep != 1 {
		t.Errorf("persisted reset state current step = %d, want 1", state2.CurrentStep)
	}
}

// TestWorkflowE2E_StatePersistence tests that workflow state persists across sessions.
func TestWorkflowE2E_StatePersistence(t *testing.T) {
	festDir := setupE2EFestival(t)
	phaseDir := filepath.Join(festDir, "001_INGEST")

	gctx := &guidance.GuidanceContext{
		FestivalPath: festDir,
		PhasePath:    phaseDir,
		PhaseName:    "001_INGEST",
	}

	// Session 1: Make some progress
	nav1, _ := wf.NewNavigator(gctx, guidance.ModeIngest)
	ctx := context.Background()
	if err := nav1.Initialize(ctx); err != nil {
		t.Fatalf("session 1 init: %v", err)
	}

	if err := nav1.Advance(ctx); err != nil {
		t.Fatalf("session 1 advance: %v", err)
	}

	state1 := nav1.GetWorkflowState()
	if state1.CurrentStep != 2 {
		t.Errorf("session 1 current step = %d, want 2", state1.CurrentStep)
	}

	// Session 2: Verify state persisted
	nav2, _ := wf.NewNavigator(gctx, guidance.ModeIngest)
	if err := nav2.Initialize(ctx); err != nil {
		t.Fatalf("session 2 init: %v", err)
	}

	state2 := nav2.GetWorkflowState()
	if state2.CurrentStep != 2 {
		t.Errorf("session 2 (persisted) current step = %d, want 2", state2.CurrentStep)
	}
	if state2.TotalSteps != 3 {
		t.Errorf("session 2 total steps = %d, want 3", state2.TotalSteps)
	}
}

// TestWorkflowE2E_FormatOutput tests that workflow output formatting works.
func TestWorkflowE2E_FormatOutput(t *testing.T) {
	festDir := setupE2EFestival(t)
	phaseDir := filepath.Join(festDir, "001_INGEST")

	gctx := &guidance.GuidanceContext{
		FestivalPath: festDir,
		PhasePath:    phaseDir,
		PhaseName:    "001_INGEST",
	}

	nav, _ := wf.NewNavigator(gctx, guidance.ModeIngest)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Test FormatInstructions
	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("FormatInstructions: %v", err)
	}
	if instructions == "" {
		t.Error("FormatInstructions returned empty string")
	}

	// Test FormatProgress
	progress, err := nav.FormatProgress(ctx)
	if err != nil {
		t.Fatalf("FormatProgress: %v", err)
	}
	if progress == "" {
		t.Error("FormatProgress returned empty string")
	}
}

// setupE2EFestival creates a complete festival with ingest phase for testing.
func setupE2EFestival(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Create fest.yaml
	festYAML := `name: e2e-test-festival
id: E2E-001
`
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	// Create FESTIVAL_GOAL.md
	festGoal := `# E2E Test Festival

Test festival for end-to-end workflow testing.
`
	if err := os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte(festGoal), 0o644); err != nil {
		t.Fatalf("write FESTIVAL_GOAL.md: %v", err)
	}

	// Create phase directory
	phaseDir := filepath.Join(dir, "001_INGEST")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("mkdir phase: %v", err)
	}

	// Create PHASE_GOAL.md
	phaseGoal := `---
fest_type: phase_goal
fest_id: 001_INGEST
fest_mode: ingest
---

# Phase Goal

E2E test ingest phase.
`
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("write PHASE_GOAL.md: %v", err)
	}

	// Create WORKFLOW.md
	workflowMD := `---
fest_type: workflow
fest_id: E2E-WF
fest_parent: 001_INGEST
---

# E2E Test Workflow

## Step 1: READ — First Step

**Goal:** Read and understand the input data.

**Actions:**
1. Read all input files
2. Note key information

**Output:** Reading notes

**Checkpoint:** None

---

## Step 2: ANALYZE — Second Step

**Goal:** Analyze the collected data.

**Actions:**
1. Process the data
2. Create analysis document

**Output:** Analysis document

**Checkpoint:** USER APPROVAL REQUIRED

---

## Step 3: COMPLETE — Final Step

**Goal:** Complete the ingest work.

**Actions:**
1. Wrap up findings
2. Verify all outputs

**Output:** Final deliverable

**Checkpoint:** None
`
	if err := os.WriteFile(filepath.Join(phaseDir, "WORKFLOW.md"), []byte(workflowMD), 0o644); err != nil {
		t.Fatalf("write WORKFLOW.md: %v", err)
	}

	return dir
}
