package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// createGuidanceContext creates a guidance context for testing.
func createGuidanceContext(phaseDir string) *guidance.GuidanceContext {
	return &guidance.GuidanceContext{
		FestivalPath: filepath.Dir(phaseDir),
		PhasePath:    phaseDir,
		PhaseName:    filepath.Base(phaseDir),
	}
}

// setupWorkflowFestival creates a minimal festival with a workflow phase.
func setupWorkflowFestival(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Create fest.yaml
	festYAML := `name: test-festival
id: TEST-001
`
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
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
fest_phase_type: ingest
---

# Phase Goal

Test ingest phase.
`
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("write PHASE_GOAL.md: %v", err)
	}

	// Create WORKFLOW.md
	workflowMD := `---
fest_type: workflow
fest_id: TEST-WF
fest_parent: 001_INGEST
---

# Test Workflow

## Step 1: READ — First Step

**Goal:** Read the input data.

**Actions:**
1. Read all input files
2. Note key information

**Output:** Reading notes

**Checkpoint:** None

---

## Step 2: ANALYZE — Second Step

**Goal:** Analyze the data.

**Actions:**
1. Process the data
2. Create analysis

**Output:** Analysis document

**Checkpoint:** USER APPROVAL REQUIRED

---

## Step 3: COMPLETE — Final Step

**Goal:** Complete the work.

**Actions:**
1. Wrap up
2. Verify results

**Output:** Final deliverable

**Checkpoint:** None
`
	if err := os.WriteFile(filepath.Join(phaseDir, "WORKFLOW.md"), []byte(workflowMD), 0o644); err != nil {
		t.Fatalf("write WORKFLOW.md: %v", err)
	}

	return dir
}

// getNavigator creates a navigator for testing.
func getNavigator(t *testing.T, phaseDir string) *wf.Navigator {
	t.Helper()

	gctx := createGuidanceContext(phaseDir)
	nav, err := wf.NewNavigator(gctx, "ingest")
	if err != nil {
		t.Fatalf("NewNavigator: %v", err)
	}
	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return nav
}

func TestGetWorkflowNavigator(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	// Save original directory
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	// Change to phase directory
	if err := os.Chdir(phaseDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create a guidance context manually since we're testing helpers
	nav := getNavigator(t, phaseDir)

	// Verify navigator is working
	state := nav.GetWorkflowState()
	if state == nil {
		t.Fatal("GetWorkflowState returned nil")
	}
	if state.TotalSteps != 3 {
		t.Errorf("TotalSteps = %d, want 3", state.TotalSteps)
	}
}

func TestWorkflowAdvance(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	nav := getNavigator(t, phaseDir)

	// Start at step 1
	state := nav.GetWorkflowState()
	if state.CurrentStep != 1 {
		t.Fatalf("CurrentStep = %d, want 1", state.CurrentStep)
	}

	// Advance to step 2
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	state = nav.GetWorkflowState()
	if state.CurrentStep != 2 {
		t.Errorf("CurrentStep after advance = %d, want 2", state.CurrentStep)
	}
}

func TestWorkflowAdvanceToCheckpoint(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	nav := getNavigator(t, phaseDir)

	// Advance to step 2 (which has checkpoint)
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("first Advance: %v", err)
	}

	state := nav.GetWorkflowState()
	if state.CurrentStep != 2 {
		t.Errorf("CurrentStep = %d, want 2", state.CurrentStep)
	}

	// Step 2 has a checkpoint - it should be in_progress waiting for approval
	steps := nav.GetSteps()
	if len(steps) < 2 {
		t.Fatal("not enough steps")
	}
	if !steps[1].Checkpoint.IsBlocking() {
		t.Error("Step 2 should have blocking checkpoint")
	}
}

func TestWorkflowApprove(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	nav := getNavigator(t, phaseDir)

	// Advance to step 2 (checkpoint step)
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("first Advance: %v", err)
	}

	// Advance at checkpoint (completes step 2 but needs approval)
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("second Advance: %v", err)
	}

	// Approve the checkpoint
	if err := nav.Approve(context.Background()); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	state := nav.GetWorkflowState()
	// After approval, should advance to step 3
	if state.CurrentStep != 3 {
		t.Errorf("CurrentStep after approve = %d, want 3", state.CurrentStep)
	}
}

func TestWorkflowReject(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	nav := getNavigator(t, phaseDir)

	// Advance to step 2
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("first Advance: %v", err)
	}

	// Reject with feedback
	feedback := "needs more detail"
	if err := nav.Reject(context.Background(), feedback); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	state := nav.GetWorkflowState()
	stepState := state.GetStepState(2)
	if stepState.Status != wf.StepStatusBlocked {
		t.Errorf("Status = %v, want blocked", stepState.Status)
	}
	if stepState.Feedback != feedback {
		t.Errorf("Feedback = %q, want %q", stepState.Feedback, feedback)
	}
}

func TestWorkflowReset(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	nav := getNavigator(t, phaseDir)

	// Advance a couple steps
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("first Advance: %v", err)
	}
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("second Advance: %v", err)
	}

	// Reset
	if err := nav.Reset(context.Background()); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	state := nav.GetWorkflowState()
	if state.CurrentStep != 1 {
		t.Errorf("CurrentStep after reset = %d, want 1", state.CurrentStep)
	}
}

func TestWorkflowComplete(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	nav := getNavigator(t, phaseDir)

	// Advance through all steps
	// Step 1 -> 2
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("first Advance: %v", err)
	}
	// Step 2 (complete and approve)
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("second Advance: %v", err)
	}
	if err := nav.Approve(context.Background()); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	// Step 3 (complete) - at step 3 now, advance completes it
	err := nav.Advance(context.Background())
	// Might return error if already complete, which is ok
	if err != nil && err != guidance.ErrAlreadyComplete {
		// Check the state
		state := nav.GetWorkflowState()
		if !state.IsComplete() {
			t.Fatalf("third Advance error and not complete: %v", err)
		}
	}

	state := nav.GetWorkflowState()
	if !state.IsComplete() {
		t.Error("workflow should be complete")
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status       wf.StepStatus
		wantContains string
	}{
		{wf.StepStatusPending, "○"},
		{wf.StepStatusInProgress, "●"},
		{wf.StepStatusCompleted, "✓"},
		{wf.StepStatusSkipped, "⤼"},
		{wf.StepStatusBlocked, "✗"},
		{wf.StepStatus("unknown"), "○"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := shared.WorkflowStepIcon(tt.status)
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("WorkflowStepIcon(%v) = %q, want to contain %q", tt.status, got, tt.wantContains)
			}
		})
	}
}

func TestIsWorkflowPhase(t *testing.T) {
	tests := []struct {
		phaseType string
		want      bool
	}{
		{"ingest", true},
		{"research", true},
		{"planning", true},
		{"plan", false}, // "plan" is not a workflow phase, "planning" is
		{"implementation", false},
		{"review", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.phaseType, func(t *testing.T) {
			got := isWorkflowPhase(tt.phaseType)
			if got != tt.want {
				t.Errorf("isWorkflowPhase(%q) = %v, want %v", tt.phaseType, got, tt.want)
			}
		})
	}
}

func TestFormatProgress(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	nav := getNavigator(t, phaseDir)

	output, err := nav.FormatProgress(context.Background())
	if err != nil {
		t.Fatalf("FormatProgress: %v", err)
	}

	// Verify output contains expected content
	if !strings.Contains(output, "Workflow Progress") {
		t.Error("output missing 'Workflow Progress'")
	}
	if !strings.Contains(output, "Progress:") {
		t.Error("output missing 'Progress:'")
	}
}

func TestFormatInstructions(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	nav := getNavigator(t, phaseDir)

	output, err := nav.FormatInstructions(context.Background())
	if err != nil {
		t.Fatalf("FormatInstructions: %v", err)
	}

	// Verify output contains expected content
	if !strings.Contains(output, "## Step") {
		t.Error("output missing '## Step'")
	}
	if !strings.Contains(output, "READ") {
		t.Error("output missing step name 'READ'")
	}
}
