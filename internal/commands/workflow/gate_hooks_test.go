package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/hooks"
	"github.com/Obedience-Corp/fest/internal/progress"
)

func writeHookedGateFestival(t *testing.T, gatesExtra string) (*wf.Navigator, string) {
	t.Helper()
	festivalPath := t.TempDir()
	phasePath := filepath.Join(festivalPath, "001_IMPLEMENT")
	if err := os.MkdirAll(phasePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(festivalPath, ".fest"), 0o755); err != nil {
		t.Fatal(err)
	}

	festYAML := `version: "1.0"
name: gate-hooks-test
id: GHK-001
metadata:
  id: GHK-001
hooks:
  definitions:
    gate-check:
      command: test-gate-check
`
	if err := os.WriteFile(filepath.Join(festivalPath, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
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
` + gatesExtra
	if err := os.WriteFile(filepath.Join(phasePath, "GATES.md"), []byte(gates), 0o644); err != nil {
		t.Fatal(err)
	}

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

	t.Setenv("HOME", t.TempDir())
	return nav, festivalPath
}

func fakeGateRunner(t *testing.T, exitCode int, called *[]string) {
	t.Helper()
	orig := newGateHookRunner
	t.Cleanup(func() { newGateHookRunner = orig })
	newGateHookRunner = func(workDir string) *hooks.Runner {
		r := hooks.NewRunner(workDir)
		r.Exec = func(ctx context.Context, command string, stdin []byte, dir string) hooks.CommandResult {
			if called != nil {
				*called = append(*called, command)
			}
			res := hooks.CommandResult{ExitCode: exitCode}
			if exitCode != 0 {
				res.Err = context.Canceled
			}
			return res
		}
		return r
	}
}

func readGateEvents(t *testing.T, festivalPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(festivalPath, ".fest", "progress_events.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatal(err)
	}
	return string(data)
}

func TestRunGateHookStage_BlockedPreRefusesVerb(t *testing.T) {
	nav, festivalPath := writeHookedGateFestival(t, "\n**Hooks:** pre: [gate-check]\n")
	var called []string
	fakeGateRunner(t, 1, &called)

	step := nav.GetSteps()[0]
	blocked, err := runGateHookStage(context.Background(), nav, 1, step, hooks.TimingPre)
	if err == nil || !blocked {
		t.Fatalf("want blocked error, got blocked=%v err=%v", blocked, err)
	}
	if !strings.Contains(err.Error(), "blocked by fail-closed hook") {
		t.Fatalf("err = %v", err)
	}
	if len(called) != 1 {
		t.Fatalf("exec calls = %v", called)
	}
	events := readGateEvents(t, festivalPath)
	if !strings.Contains(events, `"hook_verb":"gate_approve"`) || !strings.Contains(events, `"hook_blocked":true`) {
		t.Fatalf("blocked gate_approve event missing:\n%s", events)
	}
}

func TestRunGateHookStage_HumanGateSkipsAutomation(t *testing.T) {
	nav, festivalPath := writeHookedGateFestival(t,
		"\n**Hooks:** pre: [gate-check]\n\n**Approval:** human-required\n")
	var called []string
	fakeGateRunner(t, 1, &called) // would block if it ever ran

	step := nav.GetSteps()[0]
	if !isHumanRequired(step) {
		t.Fatal("fixture step must be human-required")
	}
	blocked, err := runGateHookStage(context.Background(), nav, 1, step, hooks.TimingPre)
	if err != nil || blocked {
		t.Fatalf("human gate must skip automation hooks: blocked=%v err=%v", blocked, err)
	}
	if len(called) != 0 {
		t.Fatalf("automation hooks must never exec on a human gate: %v", called)
	}
	events := readGateEvents(t, festivalPath)
	if !strings.Contains(events, `"hook_skip":"human-gate"`) {
		t.Fatalf("human-gate skip event missing:\n%s", events)
	}
}

func TestRunGateHookStage_NoBindingsIsNoop(t *testing.T) {
	nav, festivalPath := writeHookedGateFestival(t, "")
	var called []string
	fakeGateRunner(t, 1, &called)

	step := nav.GetSteps()[0]
	blocked, err := runGateHookStage(context.Background(), nav, 1, step, hooks.TimingPre)
	if err != nil || blocked || len(called) != 0 {
		t.Fatalf("no bindings must be a no-op: blocked=%v err=%v called=%v", blocked, err, called)
	}
	if events := readGateEvents(t, festivalPath); strings.Contains(events, "wf_hook_run") {
		t.Fatalf("unexpected hook events:\n%s", events)
	}
}

func TestEmitJudgeHookRuns_WritesAuditEvents(t *testing.T) {
	nav, festivalPath := writeHookedGateFestival(t, "")
	runs := []hooks.HookRun{{
		Name:    hooks.ApprovalJudgeName,
		Layer:   hooks.LayerFestivals,
		Timing:  hooks.TimingPost,
		Level:   hooks.LevelGate,
		Verb:    hooks.VerbGateApprove,
		Outcome: hooks.OutcomePass,
	}}
	emitJudgeHookRuns(context.Background(), nav, 1, runs)

	events := readGateEvents(t, festivalPath)
	if !strings.Contains(events, `"hook_name":"approval_judge"`) || !strings.Contains(events, `"wf_hook_run"`) {
		t.Fatalf("judge wf_hook_run event missing:\n%s", events)
	}
}
