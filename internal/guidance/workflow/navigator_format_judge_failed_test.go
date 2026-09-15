package workflow

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFormatCheckpoint_JudgeFailed_SurfacesFailureNotWaiting(t *testing.T) {
	state := NewWorkflowState(2)
	state.CurrentStep = 2
	state.BeginJudge(2, "ob judge", "run-1", 0, time.Now())
	if !state.RecordJudgeOutcome(2, "run-1", JudgeFailed, "rpc error: model does not fit at long", time.Now()) {
		t.Fatal("RecordJudgeOutcome refused")
	}
	nav := &Navigator{workflowState: state}
	step := WorkflowStep{Number: 2, Name: "PHASE GOAL", Goal: "Is the phase goal met?", CheckpointClass: CheckpointClassArtifactReview}

	out, err := nav.formatCheckpoint(context.Background(), step)
	if err != nil {
		t.Fatalf("formatCheckpoint() error = %v", err)
	}
	for _, want := range []string{
		"judge **failed**",
		"Judge command: ob judge",
		"rpc error: model does not fit at long",
		"does not relaunch a failed judge",
		"fest workflow judge",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("checkpoint output missing %q:\n%s", want, out)
		}
	}
	for _, reject := range []string{"already running", "Leave the checkpoint blocked"} {
		if strings.Contains(out, reject) {
			t.Errorf("failed judge must not render as waiting (%q):\n%s", reject, out)
		}
	}
}
