// Package e2e provides end-to-end tests for the fest CLI.
package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// TestPhaseGateE2E_CompleteImplementationGate tests the full lifecycle of a phase gate
// on a non-workflow phase (implementation). This is the critical integration test for
// verifying that phase gates work end-to-end on phases that don't have WORKFLOW.md.
func TestPhaseGateE2E_CompleteImplementationGate(t *testing.T) {
	festDir := setupPhaseGateFestival(t, "implementation")
	phaseDir := filepath.Join(festDir, "001_IMPLEMENT")

	gctx := &guidance.GuidanceContext{
		FestivalPath: festDir,
		PhasePath:    phaseDir,
		PhaseName:    "001_IMPLEMENT",
		PhaseType:    "implementation",
	}

	// Create navigator configured for GATES.md (mirrors runPhaseGateMode in next.go)
	nav, err := wf.NewNavigator(gctx, guidance.ModeWorkflow)
	if err != nil {
		t.Fatalf("failed to create navigator: %v", err)
	}
	nav.SetDocFilename("GATES.md")
	nav.SetStateKeyPrefix("gate:")

	// Inject JSONL-backed progress store
	store := progress.NewStore(festDir)
	ctx := context.Background()
	if err := store.Load(ctx); err != nil {
		t.Fatalf("failed to load store: %v", err)
	}
	nav.SetStateStore(store)

	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize navigator: %v", err)
	}

	// Verify initial state: 3 steps (matching GATES.md template)
	state := nav.GetWorkflowState()
	if state.CurrentStep != 1 {
		t.Errorf("initial step = %d, want 1", state.CurrentStep)
	}
	if state.TotalSteps != 3 {
		t.Errorf("total steps = %d, want 3", state.TotalSteps)
	}

	// Verify steps were parsed from **Question:** fields
	steps := nav.GetSteps()
	if len(steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(steps))
	}
	if steps[0].Goal == "" {
		t.Error("step 1 Goal is empty — **Question:** field not parsed")
	}

	// All gate steps have APPROVAL REQUIRED checkpoints.
	// At the command layer, `fest workflow approve` calls nav.Approve() which
	// completes the current step and advances to the next.

	// Step 1: COMPLETENESS
	if err := nav.Approve(ctx); err != nil {
		t.Fatalf("approve step 1: %v", err)
	}

	state = nav.GetWorkflowState()
	if state.CurrentStep != 2 {
		t.Errorf("after step 1, current step = %d, want 2", state.CurrentStep)
	}

	// Step 2: QUALITY
	if err := nav.Approve(ctx); err != nil {
		t.Fatalf("approve step 2: %v", err)
	}

	state = nav.GetWorkflowState()
	if state.CurrentStep != 3 {
		t.Errorf("after step 2, current step = %d, want 3", state.CurrentStep)
	}

	// Step 3: REVIEW — final step
	if err := nav.Approve(ctx); err != nil {
		if err != guidance.ErrAlreadyComplete {
			state = nav.GetWorkflowState()
			if !state.IsComplete() {
				t.Fatalf("approve step 3: %v", err)
			}
		}
	}

	// Verify gate is complete
	state = nav.GetWorkflowState()
	if !state.IsComplete() {
		t.Error("phase gate should be complete")
	}

	// Verify state was persisted with gate: prefix
	store2 := progress.NewStore(festDir)
	if err := store2.Load(ctx); err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}
	gateState, ok := store2.GatePhaseState("001_IMPLEMENT")
	if !ok {
		t.Fatal("GatePhaseState returned false — gate state not persisted")
	}
	if !gateState.IsComplete() {
		t.Error("persisted gate state should be complete")
	}

	// Verify workflow state is NOT populated (no WORKFLOW.md in this phase)
	_, hasWorkflow := store2.WorkflowPhaseState("001_IMPLEMENT")
	if hasWorkflow {
		t.Error("WorkflowPhaseState should not exist for implementation phase without WORKFLOW.md")
	}
}

// TestPhaseGateE2E_StatePersistenceAcrossSessions tests that gate progress
// persists across separate navigator sessions.
func TestPhaseGateE2E_StatePersistenceAcrossSessions(t *testing.T) {
	festDir := setupPhaseGateFestival(t, "implementation")
	phaseDir := filepath.Join(festDir, "001_IMPLEMENT")

	ctx := context.Background()

	// Session 1: approve through first step (checkpoint)
	nav1 := createGateNavigator(t, festDir, phaseDir, "001_IMPLEMENT")
	if err := nav1.Approve(ctx); err != nil {
		t.Fatalf("session 1 approve: %v", err)
	}
	state1 := nav1.GetWorkflowState()
	if state1.CurrentStep != 2 {
		t.Errorf("session 1 current step = %d, want 2", state1.CurrentStep)
	}

	// Session 2: create fresh navigator, verify state persisted
	nav2 := createGateNavigator(t, festDir, phaseDir, "001_IMPLEMENT")
	state2 := nav2.GetWorkflowState()
	if state2.CurrentStep != 2 {
		t.Errorf("session 2 (persisted) current step = %d, want 2", state2.CurrentStep)
	}
	if state2.TotalSteps != 3 {
		t.Errorf("session 2 total steps = %d, want 3", state2.TotalSteps)
	}
}

// TestPhaseGateE2E_FormatOutput tests that gate instructions render correctly
// with "PHASE GATE" header and "Question:" labels.
func TestPhaseGateE2E_FormatOutput(t *testing.T) {
	festDir := setupPhaseGateFestival(t, "implementation")
	phaseDir := filepath.Join(festDir, "001_IMPLEMENT")

	nav := createGateNavigator(t, festDir, phaseDir, "001_IMPLEMENT")
	ctx := context.Background()

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("FormatInstructions: %v", err)
	}

	// Verify gate-specific rendering
	if !strings.Contains(instructions, "PHASE GATE") {
		t.Error("instructions missing 'PHASE GATE' header")
	}
	if !strings.Contains(instructions, "**Question:**") {
		t.Error("instructions missing '**Question:**' label (should not show **Goal:**)")
	}
	if strings.Contains(instructions, "**Goal:**") {
		t.Error("instructions should show **Question:** not **Goal:** for gate documents")
	}
	if !strings.Contains(instructions, "fest workflow advance") {
		t.Error("instructions missing 'fest workflow advance' command hint")
	}
}

// TestPhaseGateE2E_WorkflowPhaseWithGate tests a workflow phase (planning) that has
// BOTH WORKFLOW.md and GATES.md. The gate should only be reachable after the workflow completes.
func TestPhaseGateE2E_WorkflowPhaseWithGate(t *testing.T) {
	festDir := setupWorkflowPlusGateFestival(t)
	phaseDir := filepath.Join(festDir, "001_PLAN")

	ctx := context.Background()

	// First, complete the WORKFLOW.md
	wfNav := createWorkflowNavigator(t, festDir, phaseDir, "001_PLAN")

	// Advance through all 2 workflow steps
	if err := wfNav.Advance(ctx); err != nil {
		t.Fatalf("workflow advance step 1: %v", err)
	}
	if err := wfNav.Advance(ctx); err != nil {
		if err != guidance.ErrAlreadyComplete {
			state := wfNav.GetWorkflowState()
			if !state.IsComplete() {
				t.Fatalf("workflow advance step 2: %v", err)
			}
		}
	}
	wfState := wfNav.GetWorkflowState()
	if !wfState.IsComplete() {
		t.Fatal("workflow should be complete before testing gates")
	}

	// Now create a gate navigator — this should work on the same phase
	gateNav := createGateNavigator(t, festDir, phaseDir, "001_PLAN")
	gateState := gateNav.GetWorkflowState()
	if gateState.CurrentStep != 1 {
		t.Errorf("gate initial step = %d, want 1", gateState.CurrentStep)
	}

	// Verify gate and workflow states are independent
	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("load store: %v", err)
	}

	workflowState, hasWf := store.WorkflowPhaseState("001_PLAN")
	if !hasWf || !workflowState.IsComplete() {
		t.Error("workflow state should be complete")
	}

	gateStoreState, hasGate := store.GatePhaseState("001_PLAN")
	if !hasGate {
		t.Fatal("gate state should exist")
	}
	if gateStoreState.IsComplete() {
		t.Error("gate state should NOT be complete yet")
	}
}

// TestPhaseGateE2E_CheckpointRejection tests that rejecting a gate step blocks it
// with feedback, same as workflow checkpoint rejection.
func TestPhaseGateE2E_CheckpointRejection(t *testing.T) {
	festDir := setupPhaseGateFestival(t, "implementation")
	phaseDir := filepath.Join(festDir, "001_IMPLEMENT")

	nav := createGateNavigator(t, festDir, phaseDir, "001_IMPLEMENT")
	ctx := context.Background()

	// Reject the first gate step
	feedback := "Sequences are not actually complete"
	if err := nav.Reject(ctx, feedback); err != nil {
		t.Fatalf("reject: %v", err)
	}

	state := nav.GetWorkflowState()
	stepState := state.GetStepState(1)
	if stepState.Status != wf.StepStatusBlocked {
		t.Errorf("step 1 status = %v, want blocked", stepState.Status)
	}
	if stepState.Feedback != feedback {
		t.Errorf("step 1 feedback = %q, want %q", stepState.Feedback, feedback)
	}
}

// --- Test helpers ---

// createGateNavigator creates a navigator configured for GATES.md with JSONL-backed state.
func createGateNavigator(t *testing.T, festDir, phaseDir, phaseName string) *wf.Navigator {
	t.Helper()

	gctx := &guidance.GuidanceContext{
		FestivalPath: festDir,
		PhasePath:    phaseDir,
		PhaseName:    phaseName,
		PhaseType:    guidance.DetectPhaseType(phaseDir),
	}

	nav, err := wf.NewNavigator(gctx, guidance.ModeWorkflow)
	if err != nil {
		t.Fatalf("NewNavigator: %v", err)
	}
	nav.SetDocFilename("GATES.md")
	nav.SetStateKeyPrefix("gate:")

	store := progress.NewStore(festDir)
	ctx := context.Background()
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	nav.SetStateStore(store)

	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	return nav
}

// createWorkflowNavigator creates a navigator for WORKFLOW.md with JSONL-backed state.
func createWorkflowNavigator(t *testing.T, festDir, phaseDir, phaseName string) *wf.Navigator {
	t.Helper()

	gctx := &guidance.GuidanceContext{
		FestivalPath: festDir,
		PhasePath:    phaseDir,
		PhaseName:    phaseName,
		PhaseType:    guidance.DetectPhaseType(phaseDir),
	}

	nav, err := wf.NewNavigator(gctx, guidance.ModeWorkflow)
	if err != nil {
		t.Fatalf("NewNavigator: %v", err)
	}

	store := progress.NewStore(festDir)
	ctx := context.Background()
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	nav.SetStateStore(store)

	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	return nav
}

// setupPhaseGateFestival creates a festival with a non-workflow phase (e.g. implementation)
// that has GATES.md but no WORKFLOW.md.
func setupPhaseGateFestival(t *testing.T, phaseType string) string {
	t.Helper()

	dir := t.TempDir()

	// Create fest.yaml
	festYAML := "name: gate-test-festival\nid: GATE-001\n"
	writeTestFile(t, filepath.Join(dir, "fest.yaml"), festYAML)

	// Create FESTIVAL_GOAL.md
	writeTestFile(t, filepath.Join(dir, "FESTIVAL_GOAL.md"), "# Gate Test Festival\n")

	// Create phase directory
	phaseDir := filepath.Join(dir, "001_IMPLEMENT")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("mkdir phase: %v", err)
	}

	// Create PHASE_GOAL.md with implementation phase type
	phaseGoal := "---\nfest_type: phase\nfest_id: 001_IMPLEMENT\nfest_phase_type: " + phaseType + "\n---\n\n# Phase Goal\n\nTest " + phaseType + " phase.\n"
	writeTestFile(t, filepath.Join(phaseDir, "PHASE_GOAL.md"), phaseGoal)

	// Create GATES.md (NO WORKFLOW.md — this is the key: non-workflow phase)
	gatesMD := `---
fest_type: phase_gate
fest_id: 001_IMPLEMENT-GATE
fest_parent: 001_IMPLEMENT
---

# Implementation Phase Gate

## Step 1: COMPLETENESS — Verify All Sequences Completed

**Question:** Were all implementation sequences completed successfully?

**Actions:**
1. Verify all sequence tasks show completed status
2. Confirm no tasks were skipped without justification

**Checkpoint:** APPROVAL REQUIRED — Confirm completeness

---

## Step 2: QUALITY — Verify Build and Tests Pass

**Question:** Do the build and all tests pass without regressions?

**Actions:**
1. Run the project build
2. Run the full test suite
3. Verify no regressions introduced

**Checkpoint:** APPROVAL REQUIRED — Confirm quality

---

## Step 3: REVIEW — Verify Review Findings Addressed

**Question:** Were all code review findings addressed?

**Actions:**
1. Check that review feedback was incorporated
2. Verify no unresolved review items remain

**Checkpoint:** APPROVAL REQUIRED — Confirm review complete
`
	writeTestFile(t, filepath.Join(phaseDir, "GATES.md"), gatesMD)

	return dir
}

// setupWorkflowPlusGateFestival creates a festival with a planning phase that has
// BOTH WORKFLOW.md and GATES.md, to test the two-stage flow.
func setupWorkflowPlusGateFestival(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "fest.yaml"), "name: wf-gate-test\nid: WFG-001\n")
	writeTestFile(t, filepath.Join(dir, "FESTIVAL_GOAL.md"), "# WF+Gate Test\n")

	phaseDir := filepath.Join(dir, "001_PLAN")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("mkdir phase: %v", err)
	}

	phaseGoal := "---\nfest_type: phase\nfest_id: 001_PLAN\nfest_phase_type: planning\n---\n\n# Planning Phase\n"
	writeTestFile(t, filepath.Join(phaseDir, "PHASE_GOAL.md"), phaseGoal)

	// WORKFLOW.md — simple 2-step workflow (no checkpoints for simplicity)
	workflowMD := `---
fest_type: workflow
fest_id: WFG-WF
fest_parent: 001_PLAN
---

# Planning Workflow

## Step 1: DEFINE — Define scope

**Goal:** Define the project scope.

**Actions:**
1. Identify requirements

**Output:** Scope document

**Checkpoint:** None

---

## Step 2: PLAN — Create plan

**Goal:** Create the implementation plan.

**Actions:**
1. Write the plan

**Output:** Plan document

**Checkpoint:** None
`
	writeTestFile(t, filepath.Join(phaseDir, "WORKFLOW.md"), workflowMD)

	// GATES.md — 2-step gate
	gatesMD := `---
fest_type: phase_gate
fest_id: 001_PLAN-GATE
fest_parent: 001_PLAN
---

# Planning Phase Gate

## Step 1: METHODOLOGY — Verify Compliance

**Question:** Were all workflow steps completed?

**Actions:**
1. Verify workflow completion

**Checkpoint:** APPROVAL REQUIRED

---

## Step 2: APPROVAL — Verify Sign-off

**Question:** Did the user approve the planning outputs?

**Actions:**
1. Confirm user approval

**Checkpoint:** APPROVAL REQUIRED
`
	writeTestFile(t, filepath.Join(phaseDir, "GATES.md"), gatesMD)

	return dir
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", filepath.Base(path), err)
	}
}
