package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
)

func TestRunRejectWithRemediation_RecordsFailedGate(t *testing.T) {
	festDir := setupGateRemediationFestival(t)
	ctx := scope.WithFestival(context.Background(), festDir)
	t.Chdir(festDir)

	if err := runRejectWithRemediation(ctx, "blockers found", "005_FIX_PR_302"); err != nil {
		t.Fatalf("runRejectWithRemediation: %v", err)
	}

	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	gateState, ok := store.GatePhaseState("001_REVIEW")
	if !ok {
		t.Fatal("gate state missing after reject")
	}
	step := gateState.GetStepState(1)
	if step == nil {
		t.Fatal("step 1 state missing")
	}
	if string(step.Status) != "failed_with_remediation" {
		t.Errorf("Status = %v, want failed_with_remediation", step.Status)
	}
	if step.RemediationPhase != "005_FIX_PR_302" {
		t.Errorf("RemediationPhase = %q, want 005_FIX_PR_302", step.RemediationPhase)
	}
	if step.Feedback != "blockers found" {
		t.Errorf("Feedback = %q, want blockers found", step.Feedback)
	}
}

func TestRunRejectWithRemediation_ValidatesPhaseExists(t *testing.T) {
	festDir := setupGateRemediationFestival(t)
	ctx := scope.WithFestival(context.Background(), festDir)
	t.Chdir(festDir)

	err := runRejectWithRemediation(ctx, "x", "999_DOES_NOT_EXIST")
	if err == nil {
		t.Fatal("expected error for missing remediation phase")
	}
	if !strings.Contains(err.Error(), "remediation phase") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunRejectWithRemediation_ValidatesPhaseNameFormat(t *testing.T) {
	festDir := setupGateRemediationFestival(t)
	badPhase := filepath.Join(festDir, "BAD_NAME_NO_NUMBER")
	if err := os.MkdirAll(badPhase, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ctx := scope.WithFestival(context.Background(), festDir)
	t.Chdir(festDir)

	err := runRejectWithRemediation(ctx, "x", "BAD_NAME_NO_NUMBER")
	if err == nil {
		t.Fatal("expected validation error for non-numbered phase name")
	}
}

func TestRunRejectWithRemediation_RejectsNonGateNavigator(t *testing.T) {
	festDir := setupWorkflowCheckpointFestival(t)
	ctx := scope.WithFestival(context.Background(), festDir)
	t.Chdir(festDir)

	err := runRejectWithRemediation(ctx, "blockers", "005_FIX_PR_302")
	if err == nil {
		t.Fatal("expected error rejecting --remediation-phase on a non-gate navigator")
	}
	if !strings.Contains(err.Error(), "phase gate") {
		t.Errorf("unexpected error: %v", err)
	}

	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if _, ok := store.GatePhaseState("001_PLAN"); ok {
		t.Error("non-gate rejection must not persist gate-prefixed state")
	}
}

func TestValidateRemediationPhase_RequiresActionableWork(t *testing.T) {
	festDir := t.TempDir()
	emptyPhase := filepath.Join(festDir, "005_FIX_PR_302")
	if err := os.MkdirAll(emptyPhase, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(emptyPhase, "GATES.md"), []byte("---\nfest_type: phase_gate\n---\n\n# Gate\n"), 0o644); err != nil {
		t.Fatalf("write GATES.md: %v", err)
	}

	err := validateRemediationPhase(festDir, "005_FIX_PR_302")
	if err == nil {
		t.Fatal("expected error for remediation phase with no actionable work")
	}
	if !strings.Contains(err.Error(), "actionable work") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateRemediationPhase_AcceptsSequenceDirs(t *testing.T) {
	festDir := t.TempDir()
	phase := filepath.Join(festDir, "005_FIX_PR_302")
	if err := os.MkdirAll(filepath.Join(phase, "001_FIX"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := validateRemediationPhase(festDir, "005_FIX_PR_302"); err != nil {
		t.Errorf("expected sequence-dir remediation phase to validate, got %v", err)
	}
}

func TestRunReject_DefaultBehaviorUnchanged(t *testing.T) {
	festDir := setupGateRemediationFestival(t)
	ctx := scope.WithFestival(context.Background(), festDir)
	t.Chdir(festDir)

	if err := runReject(ctx, "needs revision"); err != nil {
		t.Fatalf("runReject: %v", err)
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
	if string(step.Status) != "blocked" {
		t.Errorf("Status = %v, want blocked", step.Status)
	}
	if step.RemediationPhase != "" {
		t.Errorf("RemediationPhase = %q, want empty", step.RemediationPhase)
	}
}

func setupWorkflowCheckpointFestival(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	festYAML := "name: wf-test\nid: WFT-001\nversion: \"1.0\"\nmetadata:\n  id: WFT-001\n  status_history:\n    - status: active\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	planPhase := filepath.Join(dir, "001_PLAN")
	if err := os.MkdirAll(planPhase, 0o755); err != nil {
		t.Fatalf("mkdir plan phase: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planPhase, "PHASE_GOAL.md"), []byte("---\nfest_type: phase\nfest_id: 001_PLAN\nfest_phase_type: planning\n---\n\n# Plan\n"), 0o644); err != nil {
		t.Fatalf("write PHASE_GOAL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planPhase, "WORKFLOW.md"), []byte("---\nfest_type: workflow\nfest_id: PLAN-WF\nfest_parent: 001_PLAN\n---\n\n# Plan Workflow\n\n## Step 1: REVIEW — Review the plan\n\n**Goal:** Review.\n\n**Actions:**\n1. Review\n\n**Output:** Reviewed\n\n**Checkpoint:** APPROVAL REQUIRED\n"), 0o644); err != nil {
		t.Fatalf("write WORKFLOW.md: %v", err)
	}

	remPhase := filepath.Join(dir, "005_FIX_PR_302")
	if err := os.MkdirAll(remPhase, 0o755); err != nil {
		t.Fatalf("mkdir rem phase: %v", err)
	}
	if err := os.WriteFile(filepath.Join(remPhase, "PHASE_GOAL.md"), []byte("---\nfest_type: phase\nfest_id: 005_FIX_PR_302\nfest_phase_type: planning\n---\n\n# Fix PR 302\n"), 0o644); err != nil {
		t.Fatalf("write rem PHASE_GOAL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(remPhase, "WORKFLOW.md"), []byte("---\nfest_type: workflow\nfest_id: REM-WF\nfest_parent: 005_FIX_PR_302\n---\n\n# Fix Workflow\n\n## Step 1: ADDRESS — Address blockers\n\n**Goal:** Address it.\n\n**Actions:**\n1. Fix\n\n**Output:** Fixed\n\n**Checkpoint:** None\n"), 0o644); err != nil {
		t.Fatalf("write rem WORKFLOW.md: %v", err)
	}

	return dir
}

func setupGateRemediationFestival(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	festYAML := "name: rem-test\nid: REM-001\nversion: \"1.0\"\nmetadata:\n  id: REM-001\n  status_history:\n    - status: active\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	gatePhase := filepath.Join(dir, "001_REVIEW")
	if err := os.MkdirAll(gatePhase, 0o755); err != nil {
		t.Fatalf("mkdir gate phase: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gatePhase, "PHASE_GOAL.md"), []byte("---\nfest_type: phase\nfest_id: 001_REVIEW\nfest_phase_type: review\n---\n\n# Review\n"), 0o644); err != nil {
		t.Fatalf("write PHASE_GOAL.md: %v", err)
	}
	gatesMD := `---
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
`
	if err := os.WriteFile(filepath.Join(gatePhase, "GATES.md"), []byte(gatesMD), 0o644); err != nil {
		t.Fatalf("write GATES.md: %v", err)
	}

	remPhase := filepath.Join(dir, "005_FIX_PR_302")
	if err := os.MkdirAll(remPhase, 0o755); err != nil {
		t.Fatalf("mkdir rem phase: %v", err)
	}
	if err := os.WriteFile(filepath.Join(remPhase, "PHASE_GOAL.md"), []byte("---\nfest_type: phase\nfest_id: 005_FIX_PR_302\nfest_phase_type: planning\n---\n\n# Fix PR 302\n"), 0o644); err != nil {
		t.Fatalf("write rem PHASE_GOAL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(remPhase, "WORKFLOW.md"), []byte("---\nfest_type: workflow\nfest_id: REM-WF\nfest_parent: 005_FIX_PR_302\n---\n\n# Fix Workflow\n\n## Step 1: ADDRESS — Address blockers\n\n**Goal:** Address it.\n\n**Actions:**\n1. Fix\n\n**Output:** Fixed\n\n**Checkpoint:** None\n"), 0o644); err != nil {
		t.Fatalf("write rem WORKFLOW.md: %v", err)
	}

	return dir
}
