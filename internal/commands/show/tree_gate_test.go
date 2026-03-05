package show

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// setupShowGateFestival creates a festival with a phase that has tasks and GATES.md.
func setupShowGateFestival(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	phaseDir := filepath.Join(dir, "001_IMPLEMENT")
	seqDir := filepath.Join(phaseDir, "01_sequence")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	taskContent := "---\nfest_type: task\nfest_tracking: true\n---\n# Task\n"
	if err := os.WriteFile(filepath.Join(seqDir, "01_task.md"), []byte(taskContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	gatesMD := `---
fest_type: phase_gate
fest_id: 001_IMPLEMENT-GATE
---

# Implementation Phase Gate

## Step 1: COMPLETENESS — Verify Completeness

**Question:** Were all sequences completed?

**Actions:**
1. Verify all tasks completed

**Checkpoint:** APPROVAL REQUIRED

---

## Step 2: QUALITY — Verify Quality

**Question:** Do tests pass?

**Actions:**
1. Run tests

**Checkpoint:** APPROVAL REQUIRED
`
	if err := os.WriteFile(filepath.Join(phaseDir, "GATES.md"), []byte(gatesMD), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return dir
}

func TestBuildFestivalTree_IncludesGateNodes(t *testing.T) {
	festDir := setupShowGateFestival(t)

	tree, err := BuildFestivalTree(context.Background(), festDir)
	if err != nil {
		t.Fatalf("BuildFestivalTree: %v", err)
	}

	if len(tree.Children) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(tree.Children))
	}
	phase := tree.Children[0]

	// Phase should have 1 sequence + 2 gate nodes = 3 children
	if len(phase.Children) != 3 {
		t.Fatalf("expected 3 children (1 sequence + 2 gate steps), got %d", len(phase.Children))
	}

	// First child should be the sequence (gates come after)
	if phase.Children[0].NodeType != "sequence" {
		t.Errorf("first child should be sequence, got %s", phase.Children[0].NodeType)
	}

	// Last two children should be gate steps
	for i := 1; i <= 2; i++ {
		child := phase.Children[i]
		if child.NodeType != "step" {
			t.Errorf("child[%d] NodeType = %s, want step", i, child.NodeType)
		}
		if len(child.Name) < 4 || child.Name[:4] != "Gate" {
			t.Errorf("child[%d] Name = %q, want it to start with 'Gate'", i, child.Name)
		}
	}

	// Phase stats: 1 task + 2 gate steps = 3 total
	if phase.Stats.Total != 3 {
		t.Errorf("phase Stats.Total = %d, want 3 (1 task + 2 gate steps)", phase.Stats.Total)
	}

	// Festival stats should match
	if tree.Stats.Total != 3 {
		t.Errorf("festival Stats.Total = %d, want 3", tree.Stats.Total)
	}
}

func TestBuildFestivalTree_GateCompletionAffectsStatus(t *testing.T) {
	ctx := context.Background()
	festDir := setupShowGateFestival(t)

	// Complete the task via the progress manager (ensures proper persistence)
	mgr, err := progress.NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.MarkComplete(ctx, "001_IMPLEMENT/01_sequence/01_task.md"); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}

	// Complete gate step 1 only (not step 2)
	store := mgr.Store()
	store.QueueWorkflowEvents(wf.EmitInitEvents("gate:001_IMPLEMENT", 2))
	store.QueueWorkflowEvents(wf.EmitStepDoneEvents("gate:001_IMPLEMENT", 1))
	store.QueueWorkflowEvents(wf.EmitAdvanceEvents("gate:001_IMPLEMENT", 2))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tree, err := BuildFestivalTree(ctx, festDir)
	if err != nil {
		t.Fatalf("BuildFestivalTree: %v", err)
	}

	phase := tree.Children[0]

	// 1 task completed + 1 gate completed = 2 completed out of 3
	if phase.Stats.Completed != 2 {
		t.Errorf("phase Completed = %d, want 2 (1 task + 1 gate)", phase.Stats.Completed)
	}
	if phase.Stats.Total != 3 {
		t.Errorf("phase Total = %d, want 3", phase.Stats.Total)
	}

	// Phase should NOT be completed (gate step 2 still pending)
	if phase.Status == "completed" {
		t.Error("phase status should not be 'completed' with incomplete gate")
	}
}

func TestBuildFestivalTree_NoGateNoInflation(t *testing.T) {
	// Festival without GATES.md should not have inflated counts
	tmpDir := t.TempDir()

	phaseDir := filepath.Join(tmpDir, "001_PHASE")
	seqDir := filepath.Join(phaseDir, "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatal(err)
	}

	taskContent := "---\nfest_type: task\nfest_tracking: true\n---\n# Task\n"
	if err := os.WriteFile(filepath.Join(seqDir, "01_task.md"), []byte(taskContent), 0o644); err != nil {
		t.Fatal(err)
	}

	tree, err := BuildFestivalTree(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("BuildFestivalTree: %v", err)
	}

	phase := tree.Children[0]
	if phase.Stats.Total != 1 {
		t.Errorf("phase Total = %d, want 1 (no gate inflation)", phase.Stats.Total)
	}
}
