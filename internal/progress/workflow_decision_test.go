package progress

import (
	"testing"
	"time"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func TestMaterializeWorkflowStateRecordsDecisionMetadata(t *testing.T) {
	now := time.Now().UTC()
	events := []ProgressEvent{
		{Timestamp: now, Event: EventWorkflowInit, Phase: "001_PLAN", TotalSteps: 1},
		{Timestamp: now.Add(time.Second), Event: EventWorkflowStepStart, Phase: "001_PLAN", Step: 1},
		{
			Timestamp:       now.Add(2 * time.Second),
			Event:           EventWorkflowStepDone,
			Phase:           "001_PLAN",
			Step:            1,
			DecisionActor:   "agent",
			DecisionSummary: "verified the plan",
		},
	}

	state := materializeWorkflowState(events)
	step := state.Phases["001_PLAN"].Steps[1]
	if step.DecisionActor != "agent" {
		t.Fatalf("DecisionActor = %q, want agent", step.DecisionActor)
	}
	if step.DecisionSummary != "verified the plan" {
		t.Fatalf("DecisionSummary = %q", step.DecisionSummary)
	}
	if step.DecisionAt == nil || !step.DecisionAt.Equal(now.Add(2*time.Second)) {
		t.Fatalf("DecisionAt = %v, want event timestamp", step.DecisionAt)
	}
}

func TestQueueWorkflowEventsForwardsDecisionMetadata(t *testing.T) {
	store := NewStore(t.TempDir())

	store.QueueWorkflowEvents([]wf.WorkflowEvent{{
		EventType:       string(EventWorkflowStepDone),
		Phase:           "001_PLAN",
		Step:            1,
		DecisionActor:   "agent",
		DecisionSummary: "approved generated outputs",
	}})

	if len(store.pendingEvents) != 1 {
		t.Fatalf("pending events = %d, want 1", len(store.pendingEvents))
	}
	event := store.pendingEvents[0]
	if event.DecisionActor != "agent" {
		t.Fatalf("DecisionActor = %q, want agent", event.DecisionActor)
	}
	if event.DecisionSummary != "approved generated outputs" {
		t.Fatalf("DecisionSummary = %q", event.DecisionSummary)
	}
}
