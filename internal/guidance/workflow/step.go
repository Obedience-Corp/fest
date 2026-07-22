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

// CheckpointClass separates artifact review (auto-judgeable) from operator
// attestation (human-only). See fest workflow approve --auto hardening.
type CheckpointClass string

const (
	// CheckpointClassUnspecified means the class was not set on the step and
	// should be resolved via ClassifyCheckpoint.
	CheckpointClassUnspecified CheckpointClass = ""

	// CheckpointClassArtifactReview means deliverables can answer the checkpoint
	// from packaged evidence; approve --auto is allowed when ready.
	CheckpointClassArtifactReview CheckpointClass = "artifact_review"

	// CheckpointClassOperatorAttestation means a human must affirm intent or
	// acceptance; approve --auto is refused.
	CheckpointClassOperatorAttestation CheckpointClass = "operator_attestation"
)

// String returns the checkpoint class string.
func (c CheckpointClass) String() string {
	return string(c)
}

// IsValid reports whether c is a known checkpoint class (including empty).
func (c CheckpointClass) IsValid() bool {
	switch c {
	case CheckpointClassUnspecified, CheckpointClassArtifactReview, CheckpointClassOperatorAttestation:
		return true
	default:
		return false
	}
}

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

	// CheckpointText is the raw checkpoint line from WORKFLOW.md / GATES.md.
	CheckpointText string `json:"checkpoint_text,omitempty" yaml:"checkpoint_text,omitempty"`

	// CheckpointClass is the explicit class when annotated on the step.
	// Empty means resolve via ClassifyCheckpoint.
	CheckpointClass CheckpointClass `json:"checkpoint_class,omitempty" yaml:"checkpoint_class,omitempty"`

	// EvidencePaths are phase-relative deliverable paths listed under **Evidence:**.
	EvidencePaths []string `json:"evidence_paths,omitempty" yaml:"evidence_paths,omitempty"`

	// Hooks are the pre/post hook name bindings for this step (spec 03).
	Hooks StepHooks `json:"hooks,omitempty" yaml:"hooks,omitempty"`

	// Approval, when "human-required", marks a non-bypassable human gate (sequence 04).
	Approval string `json:"approval,omitempty" yaml:"approval,omitempty"`
}

// StepHooks binds already-declared hooks to a step's lifecycle by name.
// Structurally matches frontmatter.StepHooks to avoid import cycles.
type StepHooks struct {
	Pre  []string `json:"pre,omitempty" yaml:"pre,omitempty"`
	Post []string `json:"post,omitempty" yaml:"post,omitempty"`
}

// HasCheckpoint returns true if this step has a checkpoint.
func (w WorkflowStep) HasCheckpoint() bool {
	return w.Checkpoint != CheckpointNone
}
