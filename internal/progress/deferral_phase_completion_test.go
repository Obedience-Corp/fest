package progress

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// A deferred blocker settles its task so the festival moves on, but the task
// is not done, so the phase stays open even after every gate step is approved.
// This is the state FA0032's 005 phase reached: tasks deferred, gates approved,
// PHASE_GOAL.md marked completed.
func TestPhaseCompletionNotPropagatedWithDeferredBlocker(t *testing.T) {
	ctx := context.Background()
	festDir, task1, task2 := setupPropagationFestival(t)
	phasePath := filepath.Join(festDir, "001_PHASE")
	gates := "---\nfest_type: phase_gate\n---\n\n# Gate\n\n## Step 1: COMPLETENESS — Verify\n\n**Question:** Done?\n\n**Checkpoint:** APPROVAL REQUIRED\n"
	if err := os.WriteFile(filepath.Join(phasePath, "GATES.md"), []byte(gates), 0o644); err != nil {
		t.Fatalf("write GATES.md: %v", err)
	}

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.MarkComplete(ctx, task1); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}
	if err := mgr.ReportBlocker(ctx, task2, "vendor API removed", nil); err != nil {
		t.Fatalf("ReportBlocker: %v", err)
	}
	if err := mgr.DeferBlocker(ctx, task2, "independent of the gate", DeferralAudit{Actor: "test"}); err != nil {
		t.Fatalf("DeferBlocker: %v", err)
	}
	store := mgr.Store()
	store.QueueWorkflowEvents(wf.EmitInitEvents("gate:001_PHASE", 1))
	store.QueueWorkflowEvents(wf.EmitStepDoneWithDecisionEvents("gate:001_PHASE", 1, "ok", wf.DecisionMetadata{Actor: "agent", Summary: "ok"}))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// A fresh manager, as fest workflow advance builds one after the last gate
	// step, so the gate state just saved is what the phase is judged on.
	mgr, err = NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	phase, err := mgr.GetPhaseProgress(ctx, phasePath)
	if err != nil {
		t.Fatalf("GetPhaseProgress: %v", err)
	}
	if p := phase.Progress; p.Total != 3 || p.Settled != 3 || p.DeferredBlocked != 1 {
		t.Fatalf("fixture must be fully settled with one deferred blocker, got %+v", p)
	}
	if err := mgr.PropagatePhaseCompletion(ctx, phasePath); err != nil {
		t.Fatalf("PropagatePhaseCompletion: %v", err)
	}
	goal := filepath.Join(phasePath, "PHASE_GOAL.md")
	if got := readStatus(t, goal); got == frontmatter.StatusCompleted {
		t.Fatalf("PHASE_GOAL.md status = %q with a deferred blocker open; the phase must stay incomplete", got)
	}

	// Once the deferred task is actually done, the phase completes.
	if err := mgr.ClearBlocker(ctx, task2, "vendor shipped"); err != nil {
		t.Fatalf("ClearBlocker: %v", err)
	}
	if err := mgr.MarkComplete(ctx, task2); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}
	if err := mgr.PropagatePhaseCompletion(ctx, phasePath); err != nil {
		t.Fatalf("PropagatePhaseCompletion: %v", err)
	}
	if got := readStatus(t, goal); got != frontmatter.StatusCompleted {
		t.Fatalf("PHASE_GOAL.md status = %q after the blocker cleared, want %q", got, frontmatter.StatusCompleted)
	}
}
