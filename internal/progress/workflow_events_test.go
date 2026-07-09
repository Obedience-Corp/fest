package progress

import (
	"testing"
	"time"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func TestMaterializeWorkflowState_SkippedAndCompleted(t *testing.T) {
	now := time.Now().UTC()
	events := []ProgressEvent{
		{Timestamp: now, Event: EventWorkflowInit, Phase: "001_INGEST", TotalSteps: 2},
		{Timestamp: now.Add(1 * time.Second), Event: EventWorkflowStepSkip, Phase: "001_INGEST", Step: 1, Feedback: "external work already complete"},
		{Timestamp: now.Add(2 * time.Second), Event: EventWorkflowAdvance, Phase: "001_INGEST", Step: 2},
		{Timestamp: now.Add(3 * time.Second), Event: EventWorkflowStepDone, Phase: "001_INGEST", Step: 2, Feedback: "operator override"},
	}

	state := materializeWorkflowState(events)
	phaseState, ok := state.Phases["001_INGEST"]
	if !ok {
		t.Fatal("expected phase state for 001_INGEST")
	}

	step1 := phaseState.GetStepState(1)
	if step1 == nil {
		t.Fatal("expected step 1 state")
	}
	if step1.Status != wf.StepStatusSkipped {
		t.Fatalf("step 1 status = %s, want %s", step1.Status, wf.StepStatusSkipped)
	}
	if step1.Feedback != "external work already complete" {
		t.Fatalf("step 1 feedback = %q, want %q", step1.Feedback, "external work already complete")
	}

	step2 := phaseState.GetStepState(2)
	if step2 == nil {
		t.Fatal("expected step 2 state")
	}
	if step2.Status != wf.StepStatusCompleted {
		t.Fatalf("step 2 status = %s, want %s", step2.Status, wf.StepStatusCompleted)
	}
	if step2.Feedback != "operator override" {
		t.Fatalf("step 2 feedback = %q, want %q", step2.Feedback, "operator override")
	}

	if !phaseState.IsComplete() {
		t.Fatal("phase should be complete with skipped + completed terminal states")
	}
}

func TestGenerateWorkflowEventsFromYAML_EmitsStepSkip(t *testing.T) {
	now := time.Now().UTC()
	phaseState := wf.NewWorkflowState(2)
	phaseState.CreatedAt = now
	phaseState.UpdatedAt = now.Add(3 * time.Second)
	phaseState.CurrentStep = 2
	phaseState.Steps[1] = &wf.StepState{
		Number:      1,
		Status:      wf.StepStatusSkipped,
		CompletedAt: ptrTime(now.Add(1 * time.Second)),
		Feedback:    "manual override",
	}
	phaseState.Steps[2] = &wf.StepState{
		Number:      2,
		Status:      wf.StepStatusCompleted,
		CompletedAt: ptrTime(now.Add(2 * time.Second)),
	}

	state := wf.NewFestivalWorkflowState()
	state.CreatedAt = now
	state.UpdatedAt = now.Add(3 * time.Second)
	state.Phases["001_INGEST"] = phaseState

	events := generateWorkflowEventsFromYAML(state)
	foundSkip := false
	for _, event := range events {
		if event.Event == EventWorkflowStepSkip && event.Phase == "001_INGEST" && event.Step == 1 {
			foundSkip = true
			if event.Feedback != "manual override" {
				t.Fatalf("skip feedback = %q, want %q", event.Feedback, "manual override")
			}
		}
	}
	if !foundSkip {
		t.Fatal("expected wf_step_skip event in generated workflow events")
	}
}

func TestGenerateWorkflowEventsFromYAML_EmitsFailRemediation(t *testing.T) {
	now := time.Now().UTC()
	phaseState := wf.NewWorkflowState(1)
	phaseState.CreatedAt = now
	phaseState.UpdatedAt = now.Add(2 * time.Second)
	phaseState.CurrentStep = 1
	phaseState.Steps[1] = &wf.StepState{
		Number:           1,
		Status:           wf.StepStatusFailedRemediation,
		StartedAt:        ptrTime(now.Add(1 * time.Second)),
		Feedback:         "PR not ready",
		RemediationPhase: "005_FIX_PR_302",
		DecisionActor:    "agent",
		DecisionSummary:  "missing acceptance proof",
	}

	state := wf.NewFestivalWorkflowState()
	state.CreatedAt = now
	state.UpdatedAt = now.Add(2 * time.Second)
	state.Phases["gate:001_REVIEW"] = phaseState

	events := generateWorkflowEventsFromYAML(state)

	var fail *ProgressEvent
	for i := range events {
		if events[i].Event == EventWorkflowStepFailRemediation && events[i].Phase == "gate:001_REVIEW" && events[i].Step == 1 {
			fail = &events[i]
			break
		}
	}
	if fail == nil {
		t.Fatal("expected wf_step_fail_remediation event in generated workflow events")
	}
	if fail.RemediationPhase != "005_FIX_PR_302" {
		t.Errorf("RemediationPhase = %q, want 005_FIX_PR_302", fail.RemediationPhase)
	}
	if fail.DecisionActor != "agent" || fail.DecisionSummary != "missing acceptance proof" {
		t.Errorf("decision metadata not preserved: actor=%q summary=%q", fail.DecisionActor, fail.DecisionSummary)
	}

	round := materializeWorkflowState(events)
	rehydrated := round.Phases["gate:001_REVIEW"].GetStepState(1)
	if rehydrated == nil {
		t.Fatal("step 1 missing after round-trip")
	}
	if rehydrated.Status != wf.StepStatusFailedRemediation {
		t.Errorf("round-trip Status = %v, want %v", rehydrated.Status, wf.StepStatusFailedRemediation)
	}
	if rehydrated.RemediationPhase != "005_FIX_PR_302" {
		t.Errorf("round-trip RemediationPhase = %q, want 005_FIX_PR_302", rehydrated.RemediationPhase)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
