package localstore

import "time"

// Event type names mirror LOCAL_RUN_STATE.md.
const (
	EventWorkflowRunCreated   = "workflow_run_created"
	EventWorkflowRunStarted   = "workflow_run_started"
	EventStepStart            = "wf_step_start"
	EventStepDone             = "wf_step_done"
	EventStepSkip             = "wf_step_skip"
	EventStepBlock            = "wf_step_block"
	EventCheckpointApproved   = "wf_checkpoint_approved"
	EventCheckpointRejected   = "wf_checkpoint_rejected"
	EventWorkflowRunCompleted = "workflow_run_completed"
	EventWorkflowRunAbandoned = "workflow_run_abandoned"
	EventWorkflowDocChanged   = "workflow_doc_changed"
)

// Event is one append-only entry in progress_events.jsonl.
type Event struct {
	Version    int       `json:"version"`
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	RunID      string    `json:"run_id"`
	WorkflowID string    `json:"workflow_id,omitempty"`
	StepID     string    `json:"step_id,omitempty"`
	StepIndex  int       `json:"step_index,omitempty"`
	Feedback   string    `json:"feedback,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}
