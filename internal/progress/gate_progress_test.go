package progress

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// setupGateFestival creates a festival with a non-workflow phase containing GATES.md and tasks.
func setupGateFestival(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	phaseDir := filepath.Join(dir, "001_IMPLEMENT")
	seqDir := filepath.Join(phaseDir, "01_sequence")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	taskContent := "---\nfest_type: task\nfest_status: pending\n---\n# Task\n"
	for _, name := range []string{"01_task.md", "02_task.md"} {
		if err := os.WriteFile(filepath.Join(seqDir, name), []byte(taskContent), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	gatesMD := `---
fest_type: phase_gate
fest_id: 001_IMPLEMENT-GATE
---

# Implementation Phase Gate

## Step 1: COMPLETENESS — Verify All Sequences Completed

**Question:** Were all implementation sequences completed successfully?

**Actions:**
1. Verify all sequence tasks show completed status

**Checkpoint:** APPROVAL REQUIRED

---

## Step 2: QUALITY — Verify Build and Tests Pass

**Question:** Do the build and all tests pass?

**Actions:**
1. Run the project build

**Checkpoint:** APPROVAL REQUIRED

---

## Step 3: REVIEW — Verify Review Findings Addressed

**Question:** Were all code review findings addressed?

**Actions:**
1. Check that review feedback was incorporated

**Checkpoint:** APPROVAL REQUIRED
`
	if err := os.WriteFile(filepath.Join(phaseDir, "GATES.md"), []byte(gatesMD), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return dir
}

// setupWorkflowPlusGateFestivalForProgress creates a festival with a workflow phase
// that also has GATES.md.
func setupWorkflowPlusGateFestivalForProgress(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	phaseDir := filepath.Join(dir, "001_PLAN")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	workflowMD := `---
fest_type: workflow
fest_id: 001_PLAN-WF
---

# Planning Workflow

## Step 1: DEFINE — Define scope

**Goal:** Define the project scope.

**Actions:**
1. Identify requirements

**Checkpoint:** None

---

## Step 2: PLAN — Create plan

**Goal:** Create the implementation plan.

**Actions:**
1. Write the plan

**Checkpoint:** None
`
	if err := os.WriteFile(filepath.Join(phaseDir, "WORKFLOW.md"), []byte(workflowMD), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	gatesMD := `---
fest_type: phase_gate
fest_id: 001_PLAN-GATE
---

# Planning Phase Gate

## Step 1: METHODOLOGY — Verify Compliance

**Question:** Were all workflow steps completed?

**Actions:**
1. Verify workflow completion

**Checkpoint:** APPROVAL REQUIRED

---

## Step 2: APPROVAL — Verify Sign-off

**Question:** Did the user approve?

**Actions:**
1. Confirm approval

**Checkpoint:** APPROVAL REQUIRED
`
	if err := os.WriteFile(filepath.Join(phaseDir, "GATES.md"), []byte(gatesMD), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return dir
}

func TestGetPhaseProgress_IncludesGateSteps(t *testing.T) {
	ctx := context.Background()
	festDir := setupGateFestival(t)

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	phasePath := filepath.Join(festDir, "001_IMPLEMENT")
	progress, err := mgr.GetPhaseProgress(ctx, phasePath)
	if err != nil {
		t.Fatalf("GetPhaseProgress: %v", err)
	}

	// 2 sequence tasks + 3 gate steps = 5 total
	if progress.Progress.Total != 5 {
		t.Errorf("Total = %d, want 5 (2 tasks + 3 gate steps)", progress.Progress.Total)
	}
	// Nothing completed yet
	if progress.Progress.Completed != 0 {
		t.Errorf("Completed = %d, want 0", progress.Progress.Completed)
	}
}

func TestGetPhaseProgress_GatePartiallyComplete(t *testing.T) {
	ctx := context.Background()
	festDir := setupGateFestival(t)

	// Complete both tasks
	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.MarkComplete(ctx, "001_IMPLEMENT/01_sequence/01_task.md"); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}
	if err := mgr.MarkComplete(ctx, "001_IMPLEMENT/01_sequence/02_task.md"); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}

	// Complete gate step 1 only (init + mark step 1 done + advance to step 2)
	store := mgr.Store()
	store.QueueWorkflowEvents(wf.EmitInitEvents("gate:001_IMPLEMENT", 3))
	store.QueueWorkflowEvents(wf.EmitStepDoneEvents("gate:001_IMPLEMENT", 1))
	store.QueueWorkflowEvents(wf.EmitAdvanceEvents("gate:001_IMPLEMENT", 2))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Reload to pick up the events
	mgr2, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager reload: %v", err)
	}

	phasePath := filepath.Join(festDir, "001_IMPLEMENT")
	progress, err := mgr2.GetPhaseProgress(ctx, phasePath)
	if err != nil {
		t.Fatalf("GetPhaseProgress: %v", err)
	}

	// 2 tasks completed + 1 gate step completed = 3 completed out of 5 total
	if progress.Progress.Total != 5 {
		t.Errorf("Total = %d, want 5", progress.Progress.Total)
	}
	if progress.Progress.Completed != 3 {
		t.Errorf("Completed = %d, want 3 (2 tasks + 1 gate step)", progress.Progress.Completed)
	}
	// Should NOT be 100%
	if progress.Progress.Percentage == 100 {
		t.Error("Percentage should not be 100% with incomplete gate")
	}
}

func TestGetWorkflowPhaseProgress_IncludesGateSteps(t *testing.T) {
	ctx := context.Background()
	festDir := setupWorkflowPlusGateFestivalForProgress(t)

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	phasePath := filepath.Join(festDir, "001_PLAN")
	progress, err := mgr.GetPhaseProgress(ctx, phasePath)
	if err != nil {
		t.Fatalf("GetPhaseProgress: %v", err)
	}

	// 2 workflow steps + 2 gate steps = 4 total
	if progress.Progress.Total != 4 {
		t.Errorf("Total = %d, want 4 (2 workflow + 2 gate steps)", progress.Progress.Total)
	}
}

func TestGetWorkflowPhaseProgress_WorkflowCompleteGateIncomplete(t *testing.T) {
	ctx := context.Background()
	festDir := setupWorkflowPlusGateFestivalForProgress(t)

	// Complete both workflow steps (init + mark each done + advance)
	store := NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	store.QueueWorkflowEvents(wf.EmitInitEvents("001_PLAN", 2))
	store.QueueWorkflowEvents(wf.EmitStepDoneEvents("001_PLAN", 1))
	store.QueueWorkflowEvents(wf.EmitAdvanceEvents("001_PLAN", 2))
	store.QueueWorkflowEvents(wf.EmitStepDoneEvents("001_PLAN", 2))
	store.QueueWorkflowEvents(wf.EmitAdvanceEvents("001_PLAN", 3)) // past last step = complete
	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save: %v", err)
	}

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	phasePath := filepath.Join(festDir, "001_PLAN")
	progress, err := mgr.GetPhaseProgress(ctx, phasePath)
	if err != nil {
		t.Fatalf("GetPhaseProgress: %v", err)
	}

	// 2 workflow completed, 0 gate completed, total = 4
	if progress.Progress.Total != 4 {
		t.Errorf("Total = %d, want 4", progress.Progress.Total)
	}
	if progress.Progress.Completed != 2 {
		t.Errorf("Completed = %d, want 2 (workflow only)", progress.Progress.Completed)
	}
	if progress.Progress.Percentage == 100 {
		t.Error("Percentage should not be 100% with incomplete gate")
	}
}

func TestGetPhaseProgress_NoGate(t *testing.T) {
	ctx := context.Background()
	festDir := createTestFestivalWithTasks(t)

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	phasePath := filepath.Join(festDir, "001_PHASE")
	progress, err := mgr.GetPhaseProgress(ctx, phasePath)
	if err != nil {
		t.Fatalf("GetPhaseProgress: %v", err)
	}

	// Should only count 2 sequence tasks, no gate inflation
	if progress.Progress.Total != 2 {
		t.Errorf("Total = %d, want 2 (no gate)", progress.Progress.Total)
	}
}
