package workflow

import (
	"context"
	"strings"
	"testing"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// TestReadinessRefusesUnfilledEvidence pins that an unfilled gate never reaches
// the judge. Before this, the scaffold placeholder was dropped by the parser, the
// remaining entry looked like a deliberate list, and the judge was launched with
// a goal template and no deliverables. It rejected, correctly, for missing
// evidence it was never given, which costs a full round-trip on the FIRST run of
// every phase and reads as the judge being wrong.
func TestReadinessRefusesUnfilledEvidence(t *testing.T) {
	step := wf.WorkflowStep{
		Number:           1,
		Name:             "PHASE GOAL",
		CheckpointClass:  wf.CheckpointClassArtifactReview,
		EvidencePaths:    []string{"PHASE_GOAL.md"},
		EvidenceUnparsed: []string{"(attach each sequence's SEQUENCE_GOAL.md and task outputs relevant to this gate step)"},
	}

	err := checkApprovalReadinessWithInspector(t.TempDir(), step, func(string, string) (bool, error) {
		return true, nil
	})
	if err == nil {
		t.Fatal("an unfilled evidence list must not reach the judge")
	}
	msg := err.Error()
	for _, want := range []string{"not paths", "attach each sequence", "GATES.md"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error must name the problem and the file to fix; missing %q in: %s", want, msg)
		}
	}
}

func TestReadinessAllowsAFilledEvidenceList(t *testing.T) {
	step := wf.WorkflowStep{
		Number:          1,
		Name:            "PHASE GOAL",
		CheckpointClass: wf.CheckpointClassArtifactReview,
		EvidencePaths:   []string{"PHASE_GOAL.md", "01_seq/outputs/spec.md"},
	}
	if err := checkApprovalReadinessWithInspector(t.TempDir(), step, func(string, string) (bool, error) {
		return true, nil
	}); err != nil {
		t.Errorf("a filled list must pass readiness: %v", err)
	}
}

// The unit tests above build a WorkflowStep by hand, so the parser and the
// readiness gate are only ever exercised apart. This runs a scaffold-shaped
// GATES.md section through the real parse and then into readiness, so the field
// wiring between them cannot regress independently of either.
func TestScaffoldGateDoesNotReachTheJudge(t *testing.T) {
	const content = `## Step 1: PHASE GOAL — Verify Goal Achievement

**Question:** Did the phase achieve its stated objective?

**Evidence:**
- PHASE_GOAL.md
- (attach each sequence's SEQUENCE_GOAL.md and task outputs relevant to this gate step)

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Confirm goal is met
`

	steps, err := (&wf.Parser{}).ParseContent(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("parsed %d steps, want 1", len(steps))
	}
	step := steps[0]

	// The placeholder must survive parsing rather than vanishing, which is what
	// let the gate look deliberate.
	if len(step.EvidenceUnparsed) != 1 {
		t.Fatalf("unparsed = %v, want the placeholder line", step.EvidenceUnparsed)
	}
	if len(step.EvidencePaths) != 1 || step.EvidencePaths[0] != "PHASE_GOAL.md" {
		t.Fatalf("paths = %v, want just PHASE_GOAL.md", step.EvidencePaths)
	}

	err = checkApprovalReadinessWithInspector(t.TempDir(), step, func(string, string) (bool, error) {
		return true, nil
	})
	if err == nil {
		t.Fatal("a scaffold gate must not reach the judge")
	}
	if !strings.Contains(err.Error(), "attach each sequence") {
		t.Errorf("error must quote the unfilled line: %s", err)
	}
}

// An evidence list of only unparsed bullets is the worst case: no paths at all,
// so nothing looks deliberate and the judge would see an empty list.
func TestGateWithOnlyUnparsedEvidenceIsRefused(t *testing.T) {
	const content = `## Step 1: PHASE GOAL — Verify

**Evidence:**
- (attach the outputs for this gate step)

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED
`

	steps, err := (&wf.Parser{}).ParseContent(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("parsed %d steps, want 1", len(steps))
	}
	if len(steps[0].EvidencePaths) != 0 {
		t.Errorf("paths = %v, want none", steps[0].EvidencePaths)
	}
	if len(steps[0].EvidenceUnparsed) != 1 {
		t.Fatalf("unparsed = %v, want the placeholder", steps[0].EvidenceUnparsed)
	}

	if err := checkApprovalReadinessWithInspector(t.TempDir(), steps[0],
		func(string, string) (bool, error) { return true, nil }); err == nil {
		t.Fatal("a gate with no real evidence must not reach the judge")
	}
}
