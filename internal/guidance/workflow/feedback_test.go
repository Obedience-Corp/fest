package workflow

import (
	"testing"
	"time"
)

func TestDisplayFeedbackStripsLegacyJudgeAudit(t *testing.T) {
	got := DisplayFeedback(`approval auto mode: schema_version=fest.approval.judge/v1 judge_command="ob judge" decision=reject reason="missing \"proof\""`)
	if got != `missing "proof"` {
		t.Fatalf("DisplayFeedback() = %q, want concise reason", got)
	}
}

func TestDisplayFeedbackPreservesReasonTextContainingReasonField(t *testing.T) {
	got := DisplayFeedback(`approval auto mode: schema_version=fest.approval.judge/v1 judge_command="ob judge" decision=reject reason="the note says reason=needs more evidence"`)
	if got != `the note says reason=needs more evidence` {
		t.Fatalf("DisplayFeedback() = %q, want complete reason", got)
	}
}

func TestDisplayFeedbackLeavesNormalReasonUntouched(t *testing.T) {
	if got := DisplayFeedback("missing acceptance proof"); got != "missing acceptance proof" {
		t.Fatalf("DisplayFeedback() = %q", got)
	}
}

func TestWorkflowStateReopenJudgeRejection(t *testing.T) {
	state := NewWorkflowState(1)
	state.StartCurrentStep()
	now := time.Now().UTC()
	state.BeginJudge(1, "ob judge", "run-1", 123, now)
	if !state.RecordJudgeOutcome(1, "run-1", JudgeRejected, "missing proof", now) {
		t.Fatal("RecordJudgeOutcome() did not record rejection")
	}
	state.RejectWithDecision("missing proof", DecisionMetadata{Actor: "agent", Summary: "missing proof"})

	if !state.ReopenJudgeRejection(1) {
		t.Fatal("ReopenJudgeRejection() = false, want true")
	}
	step := state.GetStepState(1)
	if step.Status != StepStatusInProgress || step.Feedback != "" || step.Judge != nil {
		t.Fatalf("reopened step = %+v, want in-progress with cleared active judge state", step)
	}
}

func TestWorkflowStateReopenJudgeRejectionDoesNotBypassOperator(t *testing.T) {
	state := NewWorkflowState(1)
	state.StartCurrentStep()
	state.RejectWithDecision("operator review required", DecisionMetadata{Actor: "user"})

	if state.ReopenJudgeRejection(1) {
		t.Fatal("ReopenJudgeRejection() = true for ordinary operator rejection")
	}
	if state.GetStepState(1).Status != StepStatusBlocked {
		t.Fatal("operator rejection was reopened")
	}
}

func TestWorkflowStateReopenJudgeRejectionDoesNotBypassLaterOperatorDecision(t *testing.T) {
	state := NewWorkflowState(1)
	state.StartCurrentStep()
	now := time.Now().UTC()
	state.BeginJudge(1, "ob judge", "run-1", 123, now)
	if !state.RecordJudgeOutcome(1, "run-1", JudgeRejected, "judge feedback", now) {
		t.Fatal("RecordJudgeOutcome() did not record rejection")
	}
	state.RejectWithDecision("approval auto mode: operator feedback", DecisionMetadata{Actor: "user"})

	if state.ReopenJudgeRejection(1) {
		t.Fatal("ReopenJudgeRejection() = true after later operator rejection")
	}
	step := state.GetStepState(1)
	if step.Status != StepStatusBlocked || step.Feedback != "approval auto mode: operator feedback" {
		t.Fatalf("operator decision was not preserved: %+v", step)
	}
}
