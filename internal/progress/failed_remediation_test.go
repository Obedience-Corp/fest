package progress

import (
	"testing"
	"time"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func TestMaterializeWorkflowState_FailRemediationAndRecheck(t *testing.T) {
	t0 := time.Now().UTC()
	t1 := t0.Add(1 * time.Minute)
	t2 := t0.Add(2 * time.Minute)
	t3 := t0.Add(3 * time.Minute)

	cases := []struct {
		name             string
		events           []ProgressEvent
		wantStatus       wf.StepStatus
		wantFeedback     string
		wantRemediation  string
		wantStepComplete bool
	}{
		{
			name: "fail records remediation phase and non-terminal status",
			events: []ProgressEvent{
				{Timestamp: t0, Event: EventWorkflowInit, Phase: "gate:001_REVIEW", TotalSteps: 1},
				{Timestamp: t1, Event: EventWorkflowStepStart, Phase: "gate:001_REVIEW", Step: 1},
				{Timestamp: t2, Event: EventWorkflowStepFailRemediation, Phase: "gate:001_REVIEW", Step: 1, Feedback: "PR not ready", RemediationPhase: "005_FIX_PR_302"},
			},
			wantStatus:       wf.StepStatusFailedRemediation,
			wantFeedback:     "PR not ready",
			wantRemediation:  "005_FIX_PR_302",
			wantStepComplete: false,
		},
		{
			name: "recheck clears remediation and returns to in_progress",
			events: []ProgressEvent{
				{Timestamp: t0, Event: EventWorkflowInit, Phase: "gate:001_REVIEW", TotalSteps: 1},
				{Timestamp: t1, Event: EventWorkflowStepStart, Phase: "gate:001_REVIEW", Step: 1},
				{Timestamp: t2, Event: EventWorkflowStepFailRemediation, Phase: "gate:001_REVIEW", Step: 1, Feedback: "PR not ready", RemediationPhase: "005_FIX_PR_302"},
				{Timestamp: t3, Event: EventWorkflowStepRecheck, Phase: "gate:001_REVIEW", Step: 1},
			},
			wantStatus:       wf.StepStatusInProgress,
			wantFeedback:     "PR not ready",
			wantRemediation:  "",
			wantStepComplete: false,
		},
		{
			name: "fail then recheck then approve closes the gate",
			events: []ProgressEvent{
				{Timestamp: t0, Event: EventWorkflowInit, Phase: "gate:001_REVIEW", TotalSteps: 1},
				{Timestamp: t1, Event: EventWorkflowStepStart, Phase: "gate:001_REVIEW", Step: 1},
				{Timestamp: t2, Event: EventWorkflowStepFailRemediation, Phase: "gate:001_REVIEW", Step: 1, Feedback: "broken", RemediationPhase: "005_FIX"},
				{Timestamp: t3, Event: EventWorkflowStepRecheck, Phase: "gate:001_REVIEW", Step: 1},
				{Timestamp: t3.Add(time.Second), Event: EventWorkflowStepDone, Phase: "gate:001_REVIEW", Step: 1},
			},
			wantStatus:       wf.StepStatusCompleted,
			wantFeedback:     "",
			wantRemediation:  "",
			wantStepComplete: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := materializeWorkflowState(tc.events)
			phaseState, ok := state.Phases["gate:001_REVIEW"]
			if !ok {
				t.Fatalf("phase state missing")
			}
			step := phaseState.GetStepState(1)
			if step == nil {
				t.Fatalf("step 1 state missing")
			}
			if step.Status != tc.wantStatus {
				t.Errorf("Status = %v, want %v", step.Status, tc.wantStatus)
			}
			if step.Feedback != tc.wantFeedback {
				t.Errorf("Feedback = %q, want %q", step.Feedback, tc.wantFeedback)
			}
			if step.RemediationPhase != tc.wantRemediation {
				t.Errorf("RemediationPhase = %q, want %q", step.RemediationPhase, tc.wantRemediation)
			}
			if phaseState.IsComplete() != tc.wantStepComplete {
				t.Errorf("IsComplete() = %v, want %v", phaseState.IsComplete(), tc.wantStepComplete)
			}
		})
	}
}

func TestQueueWorkflowEvents_ForwardsRemediationPhase(t *testing.T) {
	store := NewStore(t.TempDir())
	store.QueueWorkflowEvents([]wf.WorkflowEvent{
		{
			EventType:        string(EventWorkflowStepFailRemediation),
			Phase:            "gate:001_REVIEW",
			Step:             1,
			Feedback:         "blockers",
			RemediationPhase: "005_FIX",
		},
	})
	if len(store.pendingEvents) != 1 {
		t.Fatalf("got %d pending events, want 1", len(store.pendingEvents))
	}
	pe := store.pendingEvents[0]
	if pe.RemediationPhase != "005_FIX" {
		t.Errorf("RemediationPhase = %q, want 005_FIX", pe.RemediationPhase)
	}
	if pe.Event != EventWorkflowStepFailRemediation {
		t.Errorf("Event = %q, want %q", pe.Event, EventWorkflowStepFailRemediation)
	}
}
