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
	if err := os.Chdir(festDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

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
	if err := os.Chdir(festDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

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
	if err := os.Chdir(festDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	err := runRejectWithRemediation(ctx, "x", "BAD_NAME_NO_NUMBER")
	if err == nil {
		t.Fatal("expected validation error for non-numbered phase name")
	}
}

func TestRunReject_DefaultBehaviorUnchanged(t *testing.T) {
	festDir := setupGateRemediationFestival(t)
	ctx := scope.WithFestival(context.Background(), festDir)
	if err := os.Chdir(festDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

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
