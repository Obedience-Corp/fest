package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
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

func TestAutoDelegateBlockingCheckpoints_LaunchesJudgeWhenConfigured(t *testing.T) {
	festivalPath, phasePath, festivalsRoot := writeGateFestival(t)
	cfg := config.DefaultWorkspaceConfig()
	cfg.Hooks.ApprovalJudge.Command = "fake-judge"
	if err := config.SaveWorkspaceConfig(festivalsRoot, cfg); err != nil {
		t.Fatal(err)
	}

	// fest next auto-delegate is fire-and-forget: mock launch, never spawn.
	var launchedPayload string
	withMockedJudgeLaunch(t, func(payloadPath, phaseDir, logPath string) (int, error) {
		launchedPayload = payloadPath
		if phaseDir != phasePath {
			t.Fatalf("phaseDir = %q, want %q", phaseDir, phasePath)
		}
		if logPath == "" {
			t.Fatal("expected judge log path")
		}
		return 4242, nil
	})

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

	// No workspace in context — same as fest next scope.Global.
	if err := AutoDelegateBlockingCheckpoints(context.Background(), nav); err != nil {
		t.Fatalf("AutoDelegateBlockingCheckpoints: %v", err)
	}
	// Async: checkpoint stays open; durable "running" record for show/watch.
	state := nav.GetWorkflowState()
	if state.IsComplete() {
		t.Fatal("async launch must not complete the gate before the verdict")
	}
	if state.CurrentStep != 1 {
		t.Fatalf("current step = %d, want 1 (still waiting on judge)", state.CurrentStep)
	}
	judge := state.GetStepState(1).Judge
	if judge == nil || judge.Status != wf.JudgeRunning {
		t.Fatalf("judge state = %+v, want running", judge)
	}
	if judge.Pid != 4242 {
		t.Fatalf("judge pid = %d, want 4242", judge.Pid)
	}
	if judge.Command != "fake-judge" {
		t.Fatalf("judge command = %q, want fake-judge", judge.Command)
	}
	if launchedPayload == "" {
		t.Fatal("expected launchJudgeProcess to be called with a payload path")
	}
	if _, err := os.Stat(launchedPayload); err != nil {
		t.Fatalf("payload file missing: %v", err)
	}
}
