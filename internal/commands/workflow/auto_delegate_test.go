package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
)

func writeGateFestival(t *testing.T) (festivalPath, phasePath, festivalsRoot string) {
	t.Helper()
	root := t.TempDir()
	festivalsRoot = filepath.Join(root, "festivals")
	festivalPath = filepath.Join(festivalsRoot, "active", "gate-fest")
	phasePath = filepath.Join(festivalPath, "001_IMPLEMENT")
	if err := os.MkdirAll(phasePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(festivalPath, ".fest"), 0o755); err != nil {
		t.Fatal(err)
	}
	gates := `---
fest_type: phase_gate
---

# Gate

## Step 1: PHASE GOAL

**Question:** Is the phase goal met?

**Actions:**
1. Check deliverables

**Checkpoint:** APPROVAL REQUIRED
`
	if err := os.WriteFile(filepath.Join(phasePath, "GATES.md"), []byte(gates), 0o644); err != nil {
		t.Fatal(err)
	}
	return festivalPath, phasePath, festivalsRoot
}

func TestAutoDelegateBlockingCheckpoints_NoJudgeIsNoop(t *testing.T) {
	festivalPath, phasePath, _ := writeGateFestival(t)
	gctx := &guidance.GuidanceContext{
		FestivalPath: festivalPath,
		PhasePath:    phasePath,
		PhaseName:    "001_IMPLEMENT",
		Mode:         guidance.ModeWorkflow,
	}
	nav, err := wf.NewNavigator(gctx, guidance.ModeWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	nav.SetDocFilename("GATES.md")
	nav.SetStateKeyPrefix("gate:")
	store := progress.NewStore(festivalPath)
	if err := store.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	nav.SetStateStore(store)
	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := AutoDelegateBlockingCheckpoints(context.Background(), nav); err != nil {
		t.Fatalf("no-op without judge: %v", err)
	}
	// Still on step 1 awaiting human.
	if nav.GetWorkflowState().CurrentStep != 1 {
		t.Fatalf("current step = %d, want 1", nav.GetWorkflowState().CurrentStep)
	}
}
