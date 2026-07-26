package workflow

import (
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
