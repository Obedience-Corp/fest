package next

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
)

func TestFindFailedRemediationGate_NoneWhenAbsent(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	gate, err := findFailedRemediationGate(context.Background(), festDir)
	if err != nil {
		t.Fatalf("findFailedRemediationGate: %v", err)
	}
	if gate != nil {
		t.Fatalf("expected nil, got %+v", gate)
	}
}

func TestFindFailedRemediationGate_DiscoversAfterReject(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	ctx := context.Background()

	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	store.QueueWorkflowEvents(wf.EmitInitEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepStartEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepFailRemediationEvents("gate:001_REVIEW", 1, "PR not ready", "005_FIX_PR_302"))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("store.Save: %v", err)
	}

	gate, err := findFailedRemediationGate(ctx, festDir)
	if err != nil {
		t.Fatalf("findFailedRemediationGate: %v", err)
	}
	if gate == nil {
		t.Fatal("expected gate, got nil")
	}
	if gate.PhaseName != "001_REVIEW" {
		t.Errorf("PhaseName = %q, want 001_REVIEW", gate.PhaseName)
	}
	if gate.RemediationPhase != "005_FIX_PR_302" {
		t.Errorf("RemediationPhase = %q", gate.RemediationPhase)
	}
	if gate.Step != 1 {
		t.Errorf("Step = %d, want 1", gate.Step)
	}
	if gate.Reason != "PR not ready" {
		t.Errorf("Reason = %q", gate.Reason)
	}
	if gate.StepName != "READINESS" {
		t.Errorf("StepName = %q, want READINESS (parsed from GATES.md)", gate.StepName)
	}
}

func TestFindFailedRemediationGate_ClearedAfterRecheckDone(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	ctx := context.Background()

	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	store.QueueWorkflowEvents(wf.EmitInitEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepStartEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepFailRemediationEvents("gate:001_REVIEW", 1, "fail", "005_FIX_PR_302"))
	store.QueueWorkflowEvents(wf.EmitStepRecheckEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepDoneEvents("gate:001_REVIEW", 1))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("store.Save: %v", err)
	}

	gate, err := findFailedRemediationGate(ctx, festDir)
	if err != nil {
		t.Fatalf("findFailedRemediationGate: %v", err)
	}
	if gate != nil {
		t.Errorf("expected nil after recheck+done, got %+v", gate)
	}
}

func TestIsRemediationPhaseComplete(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	ctx := context.Background()

	complete, err := isRemediationPhaseComplete(ctx, festDir, "005_FIX_PR_302")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if complete {
		t.Error("expected incomplete remediation phase")
	}

	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	store.QueueWorkflowEvents(wf.EmitInitEvents("005_FIX_PR_302", 1))
	store.QueueWorkflowEvents(wf.EmitStepStartEvents("005_FIX_PR_302", 1))
	store.QueueWorkflowEvents(wf.EmitStepDoneEvents("005_FIX_PR_302", 1))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("store.Save: %v", err)
	}

	complete, err = isRemediationPhaseComplete(ctx, festDir, "005_FIX_PR_302")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !complete {
		t.Error("expected complete remediation phase after workflow done")
	}
}

func scaffoldRemediationFestival(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte("name: rem-next-test\nid: RNT-001\n"), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# Rem Next Test\n"), 0o644); err != nil {
		t.Fatalf("write goal: %v", err)
	}

	gatePhase := filepath.Join(dir, "001_REVIEW")
	if err := os.MkdirAll(gatePhase, 0o755); err != nil {
		t.Fatalf("mkdir gate phase: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gatePhase, "PHASE_GOAL.md"), []byte("---\nfest_type: phase\nfest_id: 001_REVIEW\nfest_phase_type: review\n---\n\n# Review\n"), 0o644); err != nil {
		t.Fatalf("write phase goal: %v", err)
	}
	gatesMD := "---\nfest_type: phase_gate\nfest_id: 001_REVIEW-GATE\nfest_parent: 001_REVIEW\n---\n\n# Gate\n\n## Step 1: READINESS — Verify ready\n\n**Question:** ready?\n\n**Actions:**\n1. check\n\n**Checkpoint:** APPROVAL REQUIRED\n"
	if err := os.WriteFile(filepath.Join(gatePhase, "GATES.md"), []byte(gatesMD), 0o644); err != nil {
		t.Fatalf("write GATES.md: %v", err)
	}

	remPhase := filepath.Join(dir, "005_FIX_PR_302")
	if err := os.MkdirAll(remPhase, 0o755); err != nil {
		t.Fatalf("mkdir rem: %v", err)
	}
	if err := os.WriteFile(filepath.Join(remPhase, "PHASE_GOAL.md"), []byte("---\nfest_type: phase\nfest_id: 005_FIX_PR_302\nfest_phase_type: planning\n---\n\n# Fix\n"), 0o644); err != nil {
		t.Fatalf("write rem phase goal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(remPhase, "WORKFLOW.md"), []byte("---\nfest_type: workflow\nfest_id: REM-WF\nfest_parent: 005_FIX_PR_302\n---\n\n# WF\n\n## Step 1: ADDRESS — Address\n\n**Goal:** Fix.\n\n**Actions:**\n1. fix\n\n**Output:** done\n\n**Checkpoint:** None\n"), 0o644); err != nil {
		t.Fatalf("write rem WORKFLOW.md: %v", err)
	}
	return dir
}
