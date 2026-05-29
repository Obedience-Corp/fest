// Package workflow provides parsing and state management for WORKFLOW.md files.
// It enables step-by-step guidance for non-implementation phases (ingest, research, planning).
package workflow

// StepStatus represents the execution state of a workflow step.
type StepStatus string

const (
	// StepStatusPending indicates the step has not been started.
	StepStatusPending StepStatus = "pending"

	// StepStatusInProgress indicates the step is currently being executed.
	StepStatusInProgress StepStatus = "in_progress"

	// StepStatusCompleted indicates the step has been successfully completed.
	StepStatusCompleted StepStatus = "completed"

	// StepStatusSkipped indicates the step was intentionally bypassed by an operator.
	StepStatusSkipped StepStatus = "skipped"

	// StepStatusBlocked indicates the step cannot proceed due to a dependency or issue.
	StepStatusBlocked StepStatus = "blocked"

	// StepStatusFailedRemediation indicates the step (typically a phase gate)
	// did not pass and a remediation phase has been linked to correct the
	// underlying issues. The step remains non-terminal so the gate must be
	// re-evaluated after the remediation phase completes.
	StepStatusFailedRemediation StepStatus = "failed_with_remediation"
)

// String returns the string representation of the step status.
func (s StepStatus) String() string {
	return string(s)
}

// IsValid returns true if the status is a known valid status.
func (s StepStatus) IsValid() bool {
	switch s {
	case StepStatusPending, StepStatusInProgress, StepStatusCompleted, StepStatusSkipped, StepStatusBlocked, StepStatusFailedRemediation:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the status represents a final state.
func (s StepStatus) IsTerminal() bool {
	return s == StepStatusCompleted || s == StepStatusSkipped
}

// CheckpointType represents the type of approval checkpoint for a step.
type CheckpointType string

const (
	// CheckpointNone indicates no checkpoint is required.
	CheckpointNone CheckpointType = ""

	// CheckpointUserApproval requires explicit user approval before proceeding.
	CheckpointUserApproval CheckpointType = "user_approval"

	// CheckpointDocumentation requires documentation to be created/updated.
	CheckpointDocumentation CheckpointType = "documentation"

	// CheckpointVerification requires verification of outputs before proceeding.
	CheckpointVerification CheckpointType = "verification"
)

// String returns the string representation of the checkpoint type.
func (c CheckpointType) String() string {
	return string(c)
}

// IsValid returns true if the checkpoint type is a known valid type.
func (c CheckpointType) IsValid() bool {
	switch c {
	case CheckpointNone, CheckpointUserApproval, CheckpointDocumentation, CheckpointVerification:
		return true
	default:
		return false
	}
}

// IsBlocking returns true if the checkpoint requires human intervention.
func (c CheckpointType) IsBlocking() bool {
	return c == CheckpointUserApproval
}

// WorkflowStep represents a single step in a WORKFLOW.md file.
type WorkflowStep struct {
	// Number is the step number (1, 2, 3...).
	Number int `json:"number" yaml:"number"`

	// Name is the step name from the header (e.g., "REVIEW", "ANALYZE").
	Name string `json:"name" yaml:"name"`

	// Goal is the objective text describing what this step accomplishes.
	Goal string `json:"goal" yaml:"goal"`

	// Actions is the list of actions to perform in this step.
	Actions []string `json:"actions" yaml:"actions"`

	// Output describes the expected output from this step.
	Output string `json:"output" yaml:"output"`

	// Checkpoint is the type of checkpoint required (if any).
	Checkpoint CheckpointType `json:"checkpoint,omitempty" yaml:"checkpoint,omitempty"`
}

// HasCheckpoint returns true if this step has a checkpoint.
func (w WorkflowStep) HasCheckpoint() bool {
	return w.Checkpoint != CheckpointNone
}
