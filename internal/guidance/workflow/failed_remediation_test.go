package workflow

import "testing"

func TestWorkflowState_RejectWithRemediation(t *testing.T) {
	state := NewWorkflowState(3)
	state.StartCurrentStep()

	state.RejectWithRemediation("PR not ready", "005_FIX_PR_302")

	current := state.GetCurrentStepState()
	if current.Status != StepStatusFailedRemediation {
		t.Errorf("Status = %v, want StepStatusFailedRemediation", current.Status)
	}
	if current.Feedback != "PR not ready" {
		t.Errorf("Feedback = %q, want 'PR not ready'", current.Feedback)
	}
	if current.RemediationPhase != "005_FIX_PR_302" {
		t.Errorf("RemediationPhase = %q, want '005_FIX_PR_302'", current.RemediationPhase)
	}
	if state.IsComplete() {
		t.Error("IsComplete() = true, want false (failed_with_remediation is non-terminal)")
	}
}

func TestWorkflowState_ClearFailedRemediation(t *testing.T) {
	state := NewWorkflowState(2)
	state.StartCurrentStep()
	state.RejectWithRemediation("needs fix", "002_FIX")

	state.ClearFailedRemediation()

	current := state.GetCurrentStepState()
	if current.Status != StepStatusInProgress {
		t.Errorf("Status = %v, want StepStatusInProgress", current.Status)
	}
	if current.RemediationPhase != "" {
		t.Errorf("RemediationPhase = %q, want empty", current.RemediationPhase)
	}
}

func TestWorkflowState_ClearFailedRemediation_NoOpWhenNotFailed(t *testing.T) {
	state := NewWorkflowState(2)
	state.StartCurrentStep()

	state.ClearFailedRemediation()

	current := state.GetCurrentStepState()
	if current.Status != StepStatusInProgress {
		t.Errorf("Status = %v, want StepStatusInProgress (unchanged)", current.Status)
	}
}

func TestStepStatus_FailedRemediation_NotTerminal(t *testing.T) {
	if StepStatusFailedRemediation.IsTerminal() {
		t.Error("StepStatusFailedRemediation.IsTerminal() = true, want false")
	}
	if !StepStatusFailedRemediation.IsValid() {
		t.Error("StepStatusFailedRemediation.IsValid() = false, want true")
	}
}

func TestEmitStepFailRemediationEvents(t *testing.T) {
	events := EmitStepFailRemediationEvents("gate:001_REVIEW", 2, "blockers found", "005_FIX")
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	ev := events[0]
	if ev.EventType != "wf_step_fail_remediation" {
		t.Errorf("EventType = %q, want wf_step_fail_remediation", ev.EventType)
	}
	if ev.Phase != "gate:001_REVIEW" {
		t.Errorf("Phase = %q, want gate:001_REVIEW", ev.Phase)
	}
	if ev.Step != 2 {
		t.Errorf("Step = %d, want 2", ev.Step)
	}
	if ev.Feedback != "blockers found" {
		t.Errorf("Feedback = %q", ev.Feedback)
	}
	if ev.RemediationPhase != "005_FIX" {
		t.Errorf("RemediationPhase = %q, want 005_FIX", ev.RemediationPhase)
	}
}

func TestEmitStepRecheckEvents(t *testing.T) {
	events := EmitStepRecheckEvents("gate:001_REVIEW", 2)
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].EventType != "wf_step_recheck" {
		t.Errorf("EventType = %q, want wf_step_recheck", events[0].EventType)
	}
}
