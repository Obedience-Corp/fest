package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	stateFileName = "workflow_state.yaml"
	stateDirName  = ".fest"
)

// StepState tracks the state of a single workflow step.
type StepState struct {
	// Number is the step number.
	Number int `yaml:"number" json:"number"`

	// Status is the current execution status.
	Status StepStatus `yaml:"status" json:"status"`

	// StartedAt is when the step was started.
	StartedAt *time.Time `yaml:"started_at,omitempty" json:"started_at,omitempty"`

	// CompletedAt is when the step was completed.
	CompletedAt *time.Time `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`

	// Feedback stores rejection reasons or notes.
	Feedback string `yaml:"feedback,omitempty" json:"feedback,omitempty"`
}

// WorkflowState tracks the overall workflow progress.
type WorkflowState struct {
	// Version for future migrations.
	Version int `yaml:"version" json:"version"`

	// CurrentStep is the step number currently being executed.
	CurrentStep int `yaml:"current_step" json:"current_step"`

	// TotalSteps is the total number of steps in the workflow.
	TotalSteps int `yaml:"total_steps" json:"total_steps"`

	// Steps maps step number to step state.
	Steps map[int]*StepState `yaml:"steps" json:"steps"`

	// UpdatedAt is the last time the state was modified.
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`

	// CreatedAt is when the workflow was started.
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
}

// NewWorkflowState creates a new workflow state with the given total steps.
func NewWorkflowState(totalSteps int) *WorkflowState {
	now := time.Now().UTC()
	return &WorkflowState{
		Version:     1,
		CurrentStep: 1,
		TotalSteps:  totalSteps,
		Steps:       make(map[int]*StepState),
		UpdatedAt:   now,
		CreatedAt:   now,
	}
}

// LoadState loads workflow state from the phase directory.
// Returns a new state if the file doesn't exist.
func LoadState(ctx context.Context, phaseDir string) (*WorkflowState, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	statePath := filepath.Join(phaseDir, stateDirName, stateFileName)

	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return NewWorkflowState(0), nil
	}
	if err != nil {
		return nil, err
	}

	var state WorkflowState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	// Ensure maps are initialized
	if state.Steps == nil {
		state.Steps = make(map[int]*StepState)
	}

	return &state, nil
}

// Save persists the workflow state to the phase directory.
func (s *WorkflowState) Save(ctx context.Context, phaseDir string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.UpdatedAt = time.Now().UTC()

	stateDir := filepath.Join(phaseDir, stateDirName)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	statePath := filepath.Join(stateDir, stateFileName)
	return os.WriteFile(statePath, data, 0644)
}

// Initialize sets up the workflow state with step information.
func (s *WorkflowState) Initialize(steps []WorkflowStep) {
	s.TotalSteps = len(steps)
	s.Steps = make(map[int]*StepState)

	for _, step := range steps {
		s.Steps[step.Number] = &StepState{
			Number: step.Number,
			Status: StepStatusPending,
		}
	}
}

// GetCurrentStepState returns the state for the current step.
// Returns nil if no state exists for the current step.
func (s *WorkflowState) GetCurrentStepState() *StepState {
	return s.Steps[s.CurrentStep]
}

// GetStepState returns the state for a specific step number.
func (s *WorkflowState) GetStepState(stepNum int) *StepState {
	return s.Steps[stepNum]
}

// StartCurrentStep marks the current step as in progress.
func (s *WorkflowState) StartCurrentStep() {
	state := s.getOrCreateStepState(s.CurrentStep)
	if state.Status == StepStatusPending {
		state.Status = StepStatusInProgress
		now := time.Now().UTC()
		state.StartedAt = &now
	}
}

// CompleteCurrentStep marks the current step as completed.
func (s *WorkflowState) CompleteCurrentStep() {
	state := s.getOrCreateStepState(s.CurrentStep)
	state.Status = StepStatusCompleted
	now := time.Now().UTC()
	state.CompletedAt = &now
}

// Advance moves to the next step if current is completed.
// Returns error if current step is not completed or if already at the last step.
func (s *WorkflowState) Advance() error {
	state := s.GetCurrentStepState()
	if state == nil || state.Status != StepStatusCompleted {
		return errors.New("current step must be completed before advancing")
	}

	if s.CurrentStep >= s.TotalSteps {
		return errors.New("already at the last step")
	}

	s.CurrentStep++
	s.StartCurrentStep()
	return nil
}

// Approve approves a blocking checkpoint and advances to the next step.
func (s *WorkflowState) Approve() error {
	s.CompleteCurrentStep()
	if s.CurrentStep < s.TotalSteps {
		return s.Advance()
	}
	return nil
}

// Reject rejects the current step with feedback and marks it as blocked.
func (s *WorkflowState) Reject(feedback string) {
	state := s.getOrCreateStepState(s.CurrentStep)
	state.Status = StepStatusBlocked
	state.Feedback = feedback
}

// Reset resets the workflow to step 1 and clears all step states.
func (s *WorkflowState) Reset() {
	s.CurrentStep = 1
	for _, state := range s.Steps {
		state.Status = StepStatusPending
		state.StartedAt = nil
		state.CompletedAt = nil
		state.Feedback = ""
	}
	s.UpdatedAt = time.Now().UTC()
}

// IsComplete returns true if all steps are completed.
func (s *WorkflowState) IsComplete() bool {
	if s.TotalSteps == 0 {
		return false
	}

	for i := 1; i <= s.TotalSteps; i++ {
		state := s.Steps[i]
		if state == nil || state.Status != StepStatusCompleted {
			return false
		}
	}
	return true
}

// CompletedCount returns the number of completed steps.
func (s *WorkflowState) CompletedCount() int {
	count := 0
	for _, state := range s.Steps {
		if state.Status == StepStatusCompleted {
			count++
		}
	}
	return count
}

// ProgressPercent returns the completion percentage.
func (s *WorkflowState) ProgressPercent() float64 {
	if s.TotalSteps == 0 {
		return 0
	}
	return float64(s.CompletedCount()) / float64(s.TotalSteps) * 100
}

// getOrCreateStepState returns existing step state or creates a new one.
func (s *WorkflowState) getOrCreateStepState(stepNum int) *StepState {
	if s.Steps == nil {
		s.Steps = make(map[int]*StepState)
	}

	if state, ok := s.Steps[stepNum]; ok {
		return state
	}

	state := &StepState{
		Number: stepNum,
		Status: StepStatusPending,
	}
	s.Steps[stepNum] = state
	return state
}
