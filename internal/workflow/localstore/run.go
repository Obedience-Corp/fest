package localstore

import (
	"time"
)

// RunManifest is .workflow/runs/<run-id>/run.yaml.
type RunManifest struct {
	Version     int        `yaml:"version"`
	Kind        string     `yaml:"kind"`
	RunID       string     `yaml:"run_id"`
	WorkflowID  string     `yaml:"workflow_id"`
	Status      string     `yaml:"status"`
	StartedAt   time.Time  `yaml:"started_at"`
	CompletedAt time.Time  `yaml:"completed_at,omitempty"`
	StartedBy   string     `yaml:"started_by,omitempty"`
	Executor    string     `yaml:"executor,omitempty"`
	Source      RunSource  `yaml:"source"`
	Summary     RunSummary `yaml:"summary"`
}

// RunSource captures pointers to the authored workflow document.
type RunSource struct {
	WorkflowPath string `yaml:"workflow_path"`
	WorkflowHash string `yaml:"workflow_hash"`
	WorkitemPath string `yaml:"workitem_path"`
}

// RunSummary is the cached current state. Event replay is authoritative;
// mutators refresh this cache when they append events.
type RunSummary struct {
	CurrentStep    int  `yaml:"current_step"`
	TotalSteps     int  `yaml:"total_steps"`
	CompletedSteps int  `yaml:"completed_steps"`
	Blocked        bool `yaml:"blocked"`
}

// RunState is the replayed view of an active or completed run.
type RunState struct {
	RunID          string
	Status         string // "active" | "completed" | "abandoned" | "blocked"
	StartedAt      time.Time
	CompletedAt    time.Time
	CurrentStep    int
	TotalSteps     int
	CompletedSteps int
	Blocked        bool
	DocHashChanged bool
	// BlockReason says why a blocked run is blocked, taken from the last
	// event that blocked it: BlockReasonCheckpoint when the step reached its
	// checkpoint and is waiting on a decision, BlockReasonRejected when that
	// decision was a rejection and the step is waiting on a revision. Empty
	// when the run is not blocked.
	BlockReason string
	// Feedback is the reason recorded with the rejection that currently blocks
	// the run. Empty otherwise.
	Feedback string
}

// Why a blocked standalone run is blocked.
const (
	BlockReasonCheckpoint = "checkpoint"
	BlockReasonRejected   = "rejected"
)

// AwaitingApproval reports whether the run is stopped at a checkpoint waiting
// on a decision, as opposed to waiting on a revision after a rejection.
func (s *RunState) AwaitingApproval() bool {
	return s != nil && s.Blocked && s.BlockReason == BlockReasonCheckpoint
}

const (
	RunVersion = 1
	RunKind    = "workflow-run"
)
