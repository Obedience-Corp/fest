package workflow

import "testing"

func TestWorkflowStateApproveWithDecision(t *testing.T) {
	state := NewWorkflowState(2)
	state.StartCurrentStep()

	err := state.ApproveWithDecision(DecisionMetadata{
		Actor:   "agent",
		Summary: "verified outputs against the gate",
	})
	if err != nil {
		t.Fatalf("ApproveWithDecision() error: %v", err)
	}

	step := state.GetStepState(1)
	if step.DecisionActor != "agent" {
		t.Fatalf("DecisionActor = %q, want agent", step.DecisionActor)
	}
	if step.DecisionSummary != "verified outputs against the gate" {
		t.Fatalf("DecisionSummary = %q", step.DecisionSummary)
	}
	if step.DecisionAt == nil {
		t.Fatal("DecisionAt = nil, want timestamp")
	}
	if state.CurrentStep != 2 {
		t.Fatalf("CurrentStep = %d, want 2", state.CurrentStep)
	}
}

func TestWorkflowStateRejectWithDecision(t *testing.T) {
	state := NewWorkflowState(1)
	state.StartCurrentStep()

	state.RejectWithDecision("needs revision", DecisionMetadata{
		Actor:   "user",
		Summary: "scope is too broad",
	})

	step := state.GetStepState(1)
	if step.Status != StepStatusBlocked {
		t.Fatalf("Status = %s, want blocked", step.Status)
	}
	if step.DecisionActor != "user" {
		t.Fatalf("DecisionActor = %q, want user", step.DecisionActor)
	}
	if step.DecisionSummary != "scope is too broad" {
		t.Fatalf("DecisionSummary = %q", step.DecisionSummary)
	}
	if step.DecisionAt == nil {
		t.Fatal("DecisionAt = nil, want timestamp")
	}
}

func TestWorkflowStateRejectStoresAndEmitsFollowups(t *testing.T) {
	state := NewWorkflowState(1)
	state.StartCurrentStep()

	followups := []string{"attach console output", "capture the ledger transition"}
	state.RejectWithDecision("thin evidence", DecisionMetadata{
		Actor:     "agent",
		Summary:   "thin evidence",
		Followups: followups,
	})

	step := state.GetStepState(1)
	if len(step.Followups) != len(followups) {
		t.Fatalf("stored followups = %v, want %v", step.Followups, followups)
	}

	// The block event that persists to the log must carry the same fix list so
	// the followups survive an event-sourced reload, not only the in-memory state.
	events := EmitStepBlockWithDecisionEvents("gate:001_IMPLEMENT", 1, "thin evidence", DecisionMetadata{
		Actor:     "agent",
		Summary:   "thin evidence",
		Followups: followups,
	})
	if len(events) != 1 || len(events[0].Followups) != len(followups) {
		t.Fatalf("emitted event followups = %+v, want %v", events, followups)
	}

	// A judge re-run (reopen) clears the fix list along with the rejection.
	if !state.ReopenJudgeRejection(1) {
		t.Fatal("ReopenJudgeRejection = false, want reopened")
	}
	if step := state.GetStepState(1); len(step.Followups) != 0 {
		t.Fatalf("followups after reopen = %v, want empty", step.Followups)
	}
}

func TestWorkflowStateRecheckClearsDecision(t *testing.T) {
	state := NewWorkflowState(1)
	state.StartCurrentStep()
	state.RejectWithRemediationDecision("not ready", "002_FIX", DecisionMetadata{
		Actor:   "agent",
		Summary: "missing proof",
	})

	state.ClearFailedRemediation()

	step := state.GetStepState(1)
	if step.DecisionActor != "" || step.DecisionSummary != "" || step.DecisionAt != nil {
		t.Fatalf("decision metadata not cleared: %#v", step)
	}
}
