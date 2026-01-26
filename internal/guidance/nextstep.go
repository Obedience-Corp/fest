package guidance

// NextStep represents the next action to take in the guidance workflow.
// It is returned by Guidance.GetNext() and contains all information needed
// to execute or display the next step.
type NextStep struct {
	// Identity
	Mode     Mode   `json:"mode"`
	StepType string `json:"step_type"` // task, gate, planning_step, ingest_step, etc.
	ID       string `json:"id"`
	Title    string `json:"title"`
	Path     string `json:"path"` // File path to the step document

	// Instructions
	Objective         string   `json:"objective,omitempty"`
	Instructions      []string `json:"instructions,omitempty"`
	CompletionCommand string   `json:"completion_command,omitempty"` // CLI command to mark complete

	// Context
	ContextFiles  []string      `json:"context_files,omitempty"`
	AutonomyLevel AutonomyLevel `json:"autonomy_level,omitempty"`

	// Parallel execution support
	Parallel bool        `json:"parallel"`
	Group    []*NextStep `json:"group,omitempty"` // Parallel tasks in this step

	// Dependencies
	Dependencies []string `json:"dependencies,omitempty"`
	BlockedBy    []string `json:"blocked_by,omitempty"`

	// Phase-specific metadata
	Metadata map[string]any `json:"metadata,omitempty"`
}

// StepType constants define the types of steps in guidance workflows.
const (
	// StepTypeTask is a standard implementation task.
	StepTypeTask = "task"

	// StepTypeGate is a quality gate requiring validation.
	StepTypeGate = "gate"

	// StepTypePlanningStep is a planning workflow step.
	StepTypePlanningStep = "planning_step"

	// StepTypeIngestStep is an ingest workflow step.
	StepTypeIngestStep = "ingest_step"

	// StepTypeResearchStep is a research/discovery step.
	StepTypeResearchStep = "research_step"

	// StepTypeReviewStep is a review/QA step.
	StepTypeReviewStep = "review_step"

	// StepTypeActionStep is a deployment/action step.
	StepTypeActionStep = "action_step"

	// StepTypeApproval is a step requiring user approval.
	StepTypeApproval = "approval"
)

// NewNextStep creates a new NextStep with required fields.
func NewNextStep(mode Mode, stepType, id, title, path string) *NextStep {
	return &NextStep{
		Mode:     mode,
		StepType: stepType,
		ID:       id,
		Title:    title,
		Path:     path,
	}
}

// NewTaskStep creates a NextStep for a standard task.
func NewTaskStep(id, title, path string, autonomy AutonomyLevel) *NextStep {
	return &NextStep{
		Mode:          ModeImplementation,
		StepType:      StepTypeTask,
		ID:            id,
		Title:         title,
		Path:          path,
		AutonomyLevel: autonomy,
	}
}

// NewGateStep creates a NextStep for a quality gate.
func NewGateStep(id, title, path string) *NextStep {
	return &NextStep{
		Mode:          ModeImplementation,
		StepType:      StepTypeGate,
		ID:            id,
		Title:         title,
		Path:          path,
		AutonomyLevel: AutonomyLow, // Gates typically require review
	}
}

// WithInstructions adds instructions to the step.
func (s *NextStep) WithInstructions(instructions ...string) *NextStep {
	s.Instructions = append(s.Instructions, instructions...)
	return s
}

// WithObjective sets the objective.
func (s *NextStep) WithObjective(objective string) *NextStep {
	s.Objective = objective
	return s
}

// WithContextFiles adds context files.
func (s *NextStep) WithContextFiles(files ...string) *NextStep {
	s.ContextFiles = append(s.ContextFiles, files...)
	return s
}

// WithDependencies sets dependencies.
func (s *NextStep) WithDependencies(deps ...string) *NextStep {
	s.Dependencies = deps
	return s
}

// WithMetadata adds a metadata key-value pair.
func (s *NextStep) WithMetadata(key string, value any) *NextStep {
	if s.Metadata == nil {
		s.Metadata = make(map[string]any)
	}
	s.Metadata[key] = value
	return s
}

// AddToGroup adds a parallel step to the group.
func (s *NextStep) AddToGroup(step *NextStep) *NextStep {
	s.Parallel = true
	s.Group = append(s.Group, step)
	return s
}

// IsGate returns true if this is a quality gate step.
func (s *NextStep) IsGate() bool {
	return s.StepType == StepTypeGate
}

// IsParallel returns true if this step has parallel tasks.
func (s *NextStep) IsParallel() bool {
	return s.Parallel && len(s.Group) > 0
}

// RequiresHumanReview returns true if human review is recommended.
func (s *NextStep) RequiresHumanReview() bool {
	if s.IsGate() {
		return true
	}
	return s.AutonomyLevel == AutonomyLow
}

// TaskCount returns the number of tasks in this step.
func (s *NextStep) TaskCount() int {
	if s.IsParallel() {
		return len(s.Group)
	}
	return 1
}

// GetCompletionCommand returns the CLI command to complete this step.
func (s *NextStep) GetCompletionCommand() string {
	if s.CompletionCommand != "" {
		return s.CompletionCommand
	}
	return "fest progress --task " + s.Path + " --complete"
}
