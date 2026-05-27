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

func TestFailedRemediation_FullLoop(t *testing.T) {
	festDir := setupRemediationFestival(t)
	gatePhaseDir := filepath.Join(festDir, "001_REVIEW")
	remediationPhaseDir := filepath.Join(festDir, "005_FIX_PR_302")
	ctx := context.Background()

	gateNav := createGateNavigator(t, festDir, gatePhaseDir, "001_REVIEW")
	if err := gateNav.RejectWithRemediation(ctx, "PR 302 not ready", "005_FIX_PR_302"); err != nil {
		t.Fatalf("RejectWithRemediation: %v", err)
	}

	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	gateState, ok := store.GatePhaseState("001_REVIEW")
	if !ok {
		t.Fatal("gate state missing")
	}
	step := gateState.GetStepState(1)
	if step == nil || step.Status != wf.StepStatusFailedRemediation {
		t.Fatalf("step status = %v, want failed_with_remediation", step)
	}
	if step.RemediationPhase != "005_FIX_PR_302" {
		t.Errorf("RemediationPhase = %q, want 005_FIX_PR_302", step.RemediationPhase)
	}
	if gateState.IsComplete() {
		t.Error("gate state IsComplete() = true, want false")
	}

	completeRemediationWorkflow(t, festDir, remediationPhaseDir)

	store2 := progress.NewStore(festDir)
	if err := store2.Load(ctx); err != nil {
		t.Fatalf("store2.Load: %v", err)
	}
	remState, ok := store2.WorkflowPhaseState("005_FIX_PR_302")
	if !ok || !remState.IsComplete() {
		t.Fatal("remediation workflow should be complete")
	}

	gateNav2 := createGateNavigator(t, festDir, gatePhaseDir, "001_REVIEW")
	if err := gateNav2.Recheck(ctx); err != nil {
		t.Fatalf("Recheck: %v", err)
	}

	if err := gateNav2.Approve(ctx); err != nil {
		state := gateNav2.GetWorkflowState()
		if !state.IsComplete() {
			t.Fatalf("Approve after recheck: %v", err)
		}
	}

	store3 := progress.NewStore(festDir)
	if err := store3.Load(ctx); err != nil {
		t.Fatalf("store3.Load: %v", err)
	}
	finalState, ok := store3.GatePhaseState("001_REVIEW")
	if !ok {
		t.Fatal("final gate state missing")
	}
	if !finalState.IsComplete() {
		t.Errorf("gate IsComplete() = false after recheck+approve")
	}
	finalStep := finalState.GetStepState(1)
	if finalStep.Status != wf.StepStatusCompleted {
		t.Errorf("final step status = %v, want completed", finalStep.Status)
	}
	if finalStep.RemediationPhase != "" {
		t.Errorf("RemediationPhase = %q, want empty after re-approval", finalStep.RemediationPhase)
	}

	auditEvents := readGateAuditEvents(t, festDir)
	wantSequence := []string{
		"wf_step_fail_remediation",
		"wf_step_recheck",
		"wf_step_done",
	}
	if !containsSequence(auditEvents, wantSequence) {
		t.Errorf("audit events %v missing expected sequence %v", auditEvents, wantSequence)
	}
}

func TestFailedRemediation_BlocksGateCompletionWithoutRecheck(t *testing.T) {
	festDir := setupRemediationFestival(t)
	gatePhaseDir := filepath.Join(festDir, "001_REVIEW")
	ctx := context.Background()

	gateNav := createGateNavigator(t, festDir, gatePhaseDir, "001_REVIEW")
	if err := gateNav.RejectWithRemediation(ctx, "broken", "005_FIX_PR_302"); err != nil {
		t.Fatalf("RejectWithRemediation: %v", err)
	}

	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	gateState, _ := store.GatePhaseState("001_REVIEW")
	if gateState == nil || gateState.IsComplete() {
		t.Fatal("gate state must not be complete with outstanding failed remediation")
	}
}

func completeRemediationWorkflow(t *testing.T, festDir, phaseDir string) {
	t.Helper()
	ctx := context.Background()

	gctx := &guidance.GuidanceContext{
		FestivalPath: festDir,
		PhasePath:    phaseDir,
		PhaseName:    filepath.Base(phaseDir),
		PhaseType:    guidance.DetectPhaseType(phaseDir),
	}
	nav, err := wf.NewNavigator(gctx, guidance.ModeWorkflow)
	if err != nil {
		t.Fatalf("NewNavigator: %v", err)
	}
	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	nav.SetStateStore(store)
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	steps := nav.GetSteps()
	for i := 0; i < len(steps); i++ {
		if err := nav.Advance(ctx); err != nil && err != guidance.ErrAlreadyComplete {
			state := nav.GetWorkflowState()
			if !state.IsComplete() {
				t.Fatalf("advance step %d: %v", i+1, err)
			}
		}
	}
}

func setupRemediationFestival(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "fest.yaml"), "name: remediation-test\nid: REM-001\n")
	writeTestFile(t, filepath.Join(dir, "FESTIVAL_GOAL.md"), "# Remediation Test\n")

	gatePhase := filepath.Join(dir, "001_REVIEW")
	if err := os.MkdirAll(gatePhase, 0o755); err != nil {
		t.Fatalf("mkdir gate phase: %v", err)
	}
	writeTestFile(t, filepath.Join(gatePhase, "PHASE_GOAL.md"), "---\nfest_type: phase\nfest_id: 001_REVIEW\nfest_phase_type: review\n---\n\n# Review\n")
	writeTestFile(t, filepath.Join(gatePhase, "GATES.md"), `---
fest_type: phase_gate
fest_id: 001_REVIEW-GATE
fest_parent: 001_REVIEW
---

# Review Phase Gate

## Step 1: READINESS — Verify PR ready

**Question:** Is the PR ready to merge?

**Actions:**
1. Read review notes

**Checkpoint:** APPROVAL REQUIRED
`)

	remPhase := filepath.Join(dir, "005_FIX_PR_302")
	if err := os.MkdirAll(remPhase, 0o755); err != nil {
		t.Fatalf("mkdir remediation phase: %v", err)
	}
	writeTestFile(t, filepath.Join(remPhase, "PHASE_GOAL.md"), "---\nfest_type: phase\nfest_id: 005_FIX_PR_302\nfest_phase_type: planning\n---\n\n# Fix PR 302\n")
	writeTestFile(t, filepath.Join(remPhase, "WORKFLOW.md"), `---
fest_type: workflow
fest_id: REM-WF
fest_parent: 005_FIX_PR_302
---

# Remediation Workflow

## Step 1: ADDRESS — Address blockers

**Goal:** Address the blockers.

**Actions:**
1. Fix things

**Output:** Fixed PR

**Checkpoint:** None
`)
	return dir
}

func readGateAuditEvents(t *testing.T, festDir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(festDir, ".fest", "progress_events.jsonl"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var types []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		if !strings.Contains(line, `"gate:001_REVIEW"`) {
			continue
		}
		for _, marker := range []string{
			"wf_step_fail_remediation",
			"wf_step_recheck",
			"wf_step_done",
			"wf_step_start",
			"wf_init",
			"wf_step_block",
		} {
			if strings.Contains(line, `"event":"`+marker+`"`) {
				types = append(types, marker)
				break
			}
		}
	}
	return types
}

func containsSequence(seq, want []string) bool {
	idx := 0
	for _, s := range seq {
		if idx < len(want) && s == want[idx] {
			idx++
		}
	}
	return idx == len(want)
}
