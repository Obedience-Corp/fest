package workflow

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

// The readiness gate is the reason a checkpoint with nothing produced costs
// nothing: it blocks on real filesystem state before a judge process starts.
// These tests exercise the production path against real files rather than the
// stubbed inspector, so the non-empty filter in hooks.WithinRoot is what is
// actually under test, and install a judge runner that fails the test on any
// invocation, so a reordering that launches the judge first is a hard failure
// rather than a slow, paid rejection.

// writeEvidenceGateFestival builds a gate whose step declares one deliverable.
func writeEvidenceGateFestival(t *testing.T, evidencePath string) (*wf.Navigator, string) {
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
name: readiness-zero-invocation-test
id: RZI-001
metadata:
  id: RZI-001
`
	if err := os.WriteFile(filepath.Join(festivalPath, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	gates := `---
fest_type: phase_gate
---

# Gate

## Step 1: VERIFY

**Question:** Are the phase deliverables real?

**Actions:**
1. Read the deliverable

**Evidence:**
- ` + evidencePath + `

**Checkpoint:** APPROVAL REQUIRED
`
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
	return nav, phasePath
}

// refuseJudgeInvocation fails the test the moment a judge process would start.
// Failing inside the fake pinpoints the regression; counting and asserting
// afterward only reports that one happened.
func refuseJudgeInvocation(t *testing.T) {
	t.Helper()
	withApprovalJudgeRunner(t, func(_ context.Context, command string, _ []byte, _ string) ([]byte, error) {
		t.Fatalf("judge %q invoked for a checkpoint that should have blocked at readiness", command)
		return nil, nil
	})
}

func evidenceGateStep(t *testing.T, nav *wf.Navigator) wf.WorkflowStep {
	t.Helper()
	steps := nav.GetSteps()
	if len(steps) == 0 {
		t.Fatal("gate parsed no steps")
	}
	if len(steps[0].EvidencePaths) != 1 {
		t.Fatalf("EvidencePaths = %v, want exactly the declared deliverable", steps[0].EvidencePaths)
	}
	return steps[0]
}

func TestPrepareAutoJudgeReadiness_MissingDeliverableBlocksWithoutJudge(t *testing.T) {
	refuseJudgeInvocation(t)
	nav, _ := writeEvidenceGateFestival(t, "output_specs/PRESENTATION.md")
	step := evidenceGateStep(t, nav)

	err := prepareAutoJudgeReadiness(context.Background(), nav, 1, step)
	if err == nil {
		t.Fatal("a declared deliverable that was never created must block")
	}
	if !strings.Contains(err.Error(), "output_specs/PRESENTATION.md") {
		t.Fatalf("error = %v, want the missing deliverable named so an operator knows what to produce", err)
	}
}

func TestPrepareAutoJudgeReadiness_EmptyDeliverableBlocksWithoutJudge(t *testing.T) {
	refuseJudgeInvocation(t)
	nav, phasePath := writeEvidenceGateFestival(t, "output_specs/PRESENTATION.md")
	step := evidenceGateStep(t, nav)

	// Present on disk, zero bytes: the case the non-empty filter exists for.
	// An existence-only check would let this through and pay for a judge run
	// that has nothing to read.
	outputs := filepath.Join(phasePath, "output_specs")
	if err := os.MkdirAll(outputs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputs, "PRESENTATION.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	err := prepareAutoJudgeReadiness(context.Background(), nav, 1, step)
	if err == nil {
		t.Fatal("a zero-byte deliverable must block: it exists but has nothing to judge")
	}
	if !strings.Contains(err.Error(), "output_specs/PRESENTATION.md") {
		t.Fatalf("error = %v, want the empty deliverable named", err)
	}
}

// Control: the same fixture must pass readiness once the deliverable has
// content. Without this the two blocking tests could be green because the
// fixture never reaches the evidence check at all.
func TestPrepareAutoJudgeReadiness_NonEmptyDeliverablePasses(t *testing.T) {
	nav, phasePath := writeEvidenceGateFestival(t, "output_specs/PRESENTATION.md")
	step := evidenceGateStep(t, nav)

	outputs := filepath.Join(phasePath, "output_specs")
	if err := os.MkdirAll(outputs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputs, "PRESENTATION.md"), []byte("real work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := prepareAutoJudgeReadiness(context.Background(), nav, 1, step); err != nil {
		t.Fatalf("readiness must pass for a non-empty deliverable: %v", err)
	}
}

// The ledger records what the judge was pointed at, never what it read. Once
// the judge opens files itself, nothing fest can see distinguishes two runs
// that consulted different things, so the record must not imply it knows.
func TestJudgeInputsOffered_RecordsDeclaredEvidence(t *testing.T) {
	nav, phasePath := writeEvidenceGateFestival(t, "output_specs/PRESENTATION.md")
	step := evidenceGateStep(t, nav)

	outputs := filepath.Join(phasePath, "output_specs")
	if err := os.MkdirAll(outputs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputs, "PRESENTATION.md"), []byte("real work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	offered := judgeInputsOffered(nav, step)
	if len(offered.Evidence) != 1 || offered.Evidence[0] != "output_specs/PRESENTATION.md" {
		t.Fatalf("Evidence = %v, want the declared deliverable", offered.Evidence)
	}
}

// A deliverable that does not exist is not offered, because the judge is never
// pointed at it: resolveExistingEvidencePaths drops it and the readiness gate
// blocks the run before it starts.
func TestJudgeInputsOffered_OmitsMissingDeliverables(t *testing.T) {
	nav, _ := writeEvidenceGateFestival(t, "output_specs/PRESENTATION.md")
	step := evidenceGateStep(t, nav)

	if offered := judgeInputsOffered(nav, step); len(offered.Evidence) != 0 {
		t.Fatalf("Evidence = %v, want nothing offered when the deliverable is absent", offered.Evidence)
	}
}

func TestJudgeInputsOffered_NilNavigatorIsEmpty(t *testing.T) {
	if offered := judgeInputsOffered(nil, wf.WorkflowStep{}); len(offered.Evidence) != 0 || len(offered.WorkingDirs) != 0 {
		t.Fatalf("offered = %+v, want empty", offered)
	}
}
