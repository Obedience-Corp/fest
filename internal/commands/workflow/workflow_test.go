package workflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	_ = r.Close()

	return buf.String()
}

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

// --- Phase Gate Command Integration Tests ---

// setupGateOnlyFestival creates a festival with an implementation phase
// that has GATES.md but NO WORKFLOW.md. This is the critical case that
// previously caused a deadlock (getWorkflowNavigator rejected non-workflow phases).
func setupGateOnlyFestival(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	festYAML := "name: gate-cmd-test\nid: GATE-CMD-001\n"
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	phaseDir := filepath.Join(dir, "001_IMPLEMENT")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("mkdir phase: %v", err)
	}

	phaseGoal := "---\nfest_type: phase\nfest_id: 001_IMPLEMENT\nfest_phase_type: implementation\n---\n\n# Phase Goal\n\nTest implementation phase.\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("write PHASE_GOAL.md: %v", err)
	}

	gatesMD := `---
fest_type: phase_gate
fest_id: 001_IMPLEMENT-GATE
fest_parent: 001_IMPLEMENT
---

# Implementation Phase Gate

## Step 1: COMPLETENESS — Verify Work Done

**Question:** Were all tasks completed?

**Actions:**
1. Verify task completion

**Checkpoint:** APPROVAL REQUIRED

---

## Step 2: QUALITY — Verify Quality

**Question:** Do build and tests pass?

**Actions:**
1. Run build and tests

**Checkpoint:** APPROVAL REQUIRED
`
	if err := os.WriteFile(filepath.Join(phaseDir, "GATES.md"), []byte(gatesMD), 0o644); err != nil {
		t.Fatalf("write GATES.md: %v", err)
	}

	return dir
}

// TestGetWorkflowNavigator_GateOnlyPhase verifies that getWorkflowNavigator()
// correctly routes to GATES.md when called from a non-workflow phase (implementation)
// that has no WORKFLOW.md. This was the blocking bug reported in PR #100 review.
func TestGetWorkflowNavigator_GateOnlyPhase(t *testing.T) {
	dir := setupGateOnlyFestival(t)
	phaseDir := filepath.Join(dir, "001_IMPLEMENT")

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(phaseDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// This previously returned: "not a workflow-based phase (detected: implementation)"
	nav, err := getWorkflowNavigator(context.Background())
	if err != nil {
		t.Fatalf("getWorkflowNavigator() should succeed for gate-only phase, got: %v", err)
	}

	state := nav.GetWorkflowState()
	if state == nil {
		t.Fatal("GetWorkflowState returned nil")
	}
	if state.TotalSteps != 2 {
		t.Errorf("TotalSteps = %d, want 2 (from GATES.md)", state.TotalSteps)
	}

	// Verify **Question:** was parsed into Goal field
	steps := nav.GetSteps()
	if len(steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(steps))
	}
	if steps[0].Goal == "" {
		t.Error("step 1 Goal is empty — **Question:** not parsed")
	}
}

// TestGetWorkflowNavigator_GateOnlyPhase_AdvanceApprove tests the full
// advance/approve cycle through getWorkflowNavigator for a gate-only phase.
// This is the exact command path that agents use: fest workflow advance → approve.
func TestGetWorkflowNavigator_GateOnlyPhase_AdvanceApprove(t *testing.T) {
	dir := setupGateOnlyFestival(t)
	phaseDir := filepath.Join(dir, "001_IMPLEMENT")

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(phaseDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	ctx := context.Background()

	// Step 1: approve checkpoint (simulates `fest workflow approve`)
	nav1, err := getWorkflowNavigator(ctx)
	if err != nil {
		t.Fatalf("getWorkflowNavigator (step 1): %v", err)
	}
	if err := nav1.Approve(ctx); err != nil {
		t.Fatalf("approve step 1: %v", err)
	}

	// Re-acquire navigator (simulates agent calling fest workflow approve again)
	nav2, err := getWorkflowNavigator(ctx)
	if err != nil {
		t.Fatalf("getWorkflowNavigator (step 2): %v", err)
	}
	state := nav2.GetWorkflowState()
	if state.CurrentStep != 2 {
		t.Errorf("after step 1 approve, current step = %d, want 2", state.CurrentStep)
	}

	// Step 2: approve checkpoint → gate complete
	if err := nav2.Approve(ctx); err != nil {
		if err != guidance.ErrAlreadyComplete {
			state = nav2.GetWorkflowState()
			if !state.IsComplete() {
				t.Fatalf("approve step 2: %v", err)
			}
		}
	}

	// Verify completion via fresh navigator
	nav3, err := getWorkflowNavigator(ctx)
	if err != nil {
		t.Fatalf("getWorkflowNavigator (verify): %v", err)
	}
	finalState := nav3.GetWorkflowState()
	if !finalState.IsComplete() {
		t.Error("gate should be complete after approving all steps")
	}
}

// TestGetWorkflowNavigator_WorkflowCompleteRoutesToGate verifies that when
// a workflow phase has both WORKFLOW.md and GATES.md, getWorkflowNavigator
// routes to GATES.md after the workflow is complete.
func TestGetWorkflowNavigator_WorkflowCompleteRoutesToGate(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	// Add a GATES.md to the existing ingest phase
	gatesMD := `---
fest_type: phase_gate
fest_id: 001_INGEST-GATE
fest_parent: 001_INGEST
---

# Ingest Phase Gate

## Step 1: VERIFY — Verify ingest complete

**Question:** Was the ingest completed correctly?

**Actions:**
1. Verify outputs

**Checkpoint:** APPROVAL REQUIRED
`
	if err := os.WriteFile(filepath.Join(phaseDir, "GATES.md"), []byte(gatesMD), 0o644); err != nil {
		t.Fatalf("write GATES.md: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(phaseDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	ctx := context.Background()

	// First, complete the WORKFLOW.md through getWorkflowNavigator
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		t.Fatalf("getWorkflowNavigator (workflow): %v", err)
	}

	// Should start on WORKFLOW.md step 1
	if nav.GetWorkflowState().TotalSteps != 3 {
		t.Fatalf("workflow should have 3 steps, got %d", nav.GetWorkflowState().TotalSteps)
	}

	// Complete all workflow steps
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("wf advance 1: %v", err)
	}
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("wf advance 2: %v", err)
	}
	if err := nav.Approve(ctx); err != nil {
		t.Fatalf("wf approve 2: %v", err)
	}
	err = nav.Advance(ctx)
	if err != nil && err != guidance.ErrAlreadyComplete {
		state := nav.GetWorkflowState()
		if !state.IsComplete() {
			t.Fatalf("wf advance 3: %v", err)
		}
	}

	// Now get navigator again — should route to GATES.md since workflow is complete
	gateNav, err := getWorkflowNavigator(ctx)
	if err != nil {
		t.Fatalf("getWorkflowNavigator (gate): %v", err)
	}

	gateState := gateNav.GetWorkflowState()
	if gateState.TotalSteps != 1 {
		t.Errorf("gate should have 1 step, got %d (may still be on WORKFLOW.md)", gateState.TotalSteps)
	}
	if gateState.IsComplete() {
		t.Error("gate should not be complete yet")
	}
}

// TestGetWorkflowNavigator_GateBlockedUntilPhaseWorkComplete verifies that
// gate mode is not accessible until phase sequences/tasks are complete.
// This prevents the ordering bypass where gates could be completed before phase work.
func TestGetWorkflowNavigator_GateBlockedUntilPhaseWorkComplete(t *testing.T) {
	dir := t.TempDir()
	phaseDir := filepath.Join(dir, "001_IMPLEMENT")
	os.MkdirAll(phaseDir, 0o755)

	// Fest config
	os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte("name: test\nid: T\n"), 0o644)

	// Phase has GATES.md (no WORKFLOW.md — it's an implementation phase)
	gatesMD := "---\nfest_type: phase_gate\n---\n\n# Gate\n\n## Step 1: CHECK — Verify\n\n**Question:** Is implementation done?\n\n**Checkpoint:** APPROVAL REQUIRED\n"
	os.WriteFile(filepath.Join(phaseDir, "GATES.md"), []byte(gatesMD), 0o644)

	// Phase has a numbered sequence directory (sequences in progress)
	os.MkdirAll(filepath.Join(phaseDir, "01_build"), 0o755)

	// Phase is NOT marked complete in PHASE_GOAL.md
	phaseGoal := "---\nfest_status: active\n---\n\n# Implementation\n"
	os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(phaseGoal), 0o644)

	// Set up scope context pointing at the phase
	ctx := context.Background()
	ctx = scope.WithFestival(ctx, dir)
	phaseFlag = "001_IMPLEMENT"
	defer func() { phaseFlag = "" }()

	// Attempt to get navigator — should FAIL because sequences aren't done
	_, err := getWorkflowNavigator(ctx)
	if err == nil {
		t.Fatal("getWorkflowNavigator should fail when gate is not yet eligible")
	}
	if !strings.Contains(err.Error(), "not yet eligible") {
		t.Errorf("error should mention 'not yet eligible', got: %s", err.Error())
	}

	// Now mark the phase as complete
	phaseGoalDone := "---\nfest_status: completed\n---\n\n# Implementation\n"
	os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(phaseGoalDone), 0o644)

	// Attempt again — should succeed now
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		t.Fatalf("getWorkflowNavigator should succeed after phase marked complete: %v", err)
	}

	state := nav.GetWorkflowState()
	if state.TotalSteps != 1 {
		t.Errorf("gate should have 1 step, got %d", state.TotalSteps)
	}
}

// TestResolveNavigationMode verifies the routing logic directly.
func TestResolveNavigationMode(t *testing.T) {
	tests := []struct {
		name         string
		hasWorkflow  bool
		hasGates     bool
		wfComplete   bool
		hasSequences bool // add numbered subdirectories to simulate sequences
		phaseMarked  bool // mark phase as completed in PHASE_GOAL.md
		wantDoc      string
		wantPrefix   string
		wantNotReady bool // expect not-ready message instead of routing
	}{
		{
			name:        "workflow only, incomplete",
			hasWorkflow: true,
			wantDoc:     "WORKFLOW.md",
		},
		{
			name:       "gates only, no sequences (non-workflow phase)",
			hasGates:   true,
			wantDoc:    "GATES.md",
			wantPrefix: "gate:",
		},
		{
			name:        "workflow complete, gates exist, no sequences",
			hasWorkflow: true,
			hasGates:    true,
			wfComplete:  true,
			wantDoc:     "GATES.md",
			wantPrefix:  "gate:",
		},
		{
			name:        "workflow incomplete, gates exist",
			hasWorkflow: true,
			hasGates:    true,
			wfComplete:  false,
			wantDoc:     "WORKFLOW.md",
		},
		{
			name:    "neither exists",
			wantDoc: "",
		},
		{
			name:         "gates only, sequences incomplete — gate not eligible",
			hasGates:     true,
			hasSequences: true,
			phaseMarked:  false,
			wantNotReady: true,
		},
		{
			name:         "gates only, sequences complete — gate eligible",
			hasGates:     true,
			hasSequences: true,
			phaseMarked:  true,
			wantDoc:      "GATES.md",
			wantPrefix:   "gate:",
		},
		{
			name:         "workflow complete + gates + sequences incomplete — gate not eligible",
			hasWorkflow:  true,
			hasGates:     true,
			wfComplete:   true,
			hasSequences: true,
			phaseMarked:  false,
			wantNotReady: true,
		},
		{
			name:         "workflow complete + gates + sequences complete — gate eligible",
			hasWorkflow:  true,
			hasGates:     true,
			wfComplete:   true,
			hasSequences: true,
			phaseMarked:  true,
			wantDoc:      "GATES.md",
			wantPrefix:   "gate:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			phaseDir := filepath.Join(dir, "001_TEST")
			if err := os.MkdirAll(phaseDir, 0o755); err != nil {
				t.Fatal(err)
			}

			if tt.hasWorkflow {
				wfContent := "---\nfest_type: workflow\n---\n\n# WF\n\n## Step 1: DO — Do it\n\n**Goal:** Do the thing.\n\n**Checkpoint:** None\n"
				os.WriteFile(filepath.Join(phaseDir, "WORKFLOW.md"), []byte(wfContent), 0o644)
			}
			if tt.hasGates {
				gateContent := "---\nfest_type: phase_gate\n---\n\n# Gate\n\n## Step 1: CHECK — Check it\n\n**Question:** Is it done?\n\n**Checkpoint:** APPROVAL REQUIRED\n"
				os.WriteFile(filepath.Join(phaseDir, "GATES.md"), []byte(gateContent), 0o644)
			}
			if tt.hasSequences {
				// Create a numbered subdirectory to simulate a sequence
				os.MkdirAll(filepath.Join(phaseDir, "01_seq"), 0o755)
			}
			if tt.phaseMarked {
				phaseGoal := "---\nfest_status: completed\n---\n\n# Phase Goal\n"
				os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(phaseGoal), 0o644)
			}

			// If workflow should be complete, use a navigator to complete it
			if tt.wfComplete && tt.hasWorkflow {
				os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte("name: test\nid: T\n"), 0o644)

				gctx := &guidance.GuidanceContext{
					FestivalPath: dir,
					PhasePath:    phaseDir,
					PhaseName:    "001_TEST",
				}
				nav, navErr := wf.NewNavigator(gctx, guidance.ModeWorkflow)
				if navErr != nil {
					t.Fatalf("NewNavigator: %v", navErr)
				}
				store := progress.NewStore(dir)
				store.Load(context.Background())
				nav.SetStateStore(store)
				nav.Initialize(context.Background())
				// Complete the single step
				nav.Advance(context.Background())
			}

			doc, prefix, notReady := resolveNavigationMode(context.Background(), dir, phaseDir)

			if tt.wantNotReady {
				if notReady == "" {
					t.Error("expected not-ready message, got empty")
				}
				if doc != "" {
					t.Errorf("expected empty doc when not ready, got %q", doc)
				}
				return
			}

			if notReady != "" {
				t.Errorf("unexpected not-ready message: %s", notReady)
			}
			if doc != tt.wantDoc {
				t.Errorf("doc = %q, want %q", doc, tt.wantDoc)
			}
			if prefix != tt.wantPrefix {
				t.Errorf("prefix = %q, want %q", prefix, tt.wantPrefix)
			}
		})
	}
}

func TestShowNextStep_WarnsWhenProgressManagerInitFails(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	nav := getNavigator(t, phaseDir)
	ctx := context.Background()

	// Complete the workflow so showNextStep enters the completion branch.
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("first Advance: %v", err)
	}
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("second Advance: %v", err)
	}
	if err := nav.Approve(ctx); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := nav.Advance(ctx); err != nil && err != guidance.ErrAlreadyComplete {
		state := nav.GetWorkflowState()
		if !state.IsComplete() {
			t.Fatalf("third Advance: %v", err)
		}
	}

	// Force progress.NewManager to fail by providing invalid workflow YAML.
	progressDir := filepath.Join(dir, progress.ProgressDir)
	if err := os.MkdirAll(progressDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.fest): %v", err)
	}
	invalidYAML := filepath.Join(progressDir, wf.StateFileName)
	if err := os.WriteFile(invalidYAML, []byte(":\n- invalid yaml"), 0o644); err != nil {
		t.Fatalf("write invalid workflow state: %v", err)
	}

	output := captureStdout(t, func() {
		_ = showNextStep(ctx, nav, nav.GetSteps())
	})

	if !strings.Contains(output, "Warning: could not initialize progress manager:") {
		t.Fatalf("expected manager-init warning in output, got:\n%s", output)
	}
}
