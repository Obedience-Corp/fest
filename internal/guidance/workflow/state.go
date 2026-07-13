package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Obedience-Corp/fest/internal/yamlutil"
	"gopkg.in/yaml.v3"
)

const (
	// StateFileName is the legacy YAML workflow state file name.
	// Exported for migration detection in the progress package.
	StateFileName = "workflow_state.yaml"
	stateDirName  = ".fest"
	stateVersion  = 2 // Version 2 uses festival-level storage
)

// StateStore is an interface for persisting workflow state via the progress event log.
// The progress.Store implements this interface, allowing workflow state to be stored
// as events in progress_events.jsonl instead of a separate YAML file.
type StateStore interface {
	// LoadWorkflowPhaseState returns the materialized workflow state for a phase.
	// Returns nil, false if no workflow state exists for the phase.
	LoadWorkflowPhaseState(phaseName string) (*WorkflowState, bool)

	// QueueWorkflowEvents queues workflow events to be written on next save.
	QueueWorkflowEvents(events []WorkflowEvent)

	// SaveEvents persists all queued events to disk.
	SaveEvents(ctx context.Context) error
}

// WorkflowEvent is a lightweight event representation used to communicate
// workflow state changes from the workflow package to the progress store
// without importing the progress package (avoiding import cycles).
type WorkflowEvent struct {
	EventType        string
	Phase            string
	Step             int
	TotalSteps       int
	Feedback         string
	RemediationPhase string
	DecisionActor    string
	DecisionSummary  string
	JudgeStatus      string
	JudgeCommand     string
	JudgeDetail      string
	JudgePid         int
	JudgeRunID       string
}

// DecisionMetadata records who made a checkpoint decision and the rationale.
type DecisionMetadata struct {
	Actor   string
	Summary string
}

// Judge lifecycle status values recorded on StepState.Judge.
const (
	JudgeRunning  = "running"
	JudgeApproved = "approved"
	JudgeRejected = "rejected"
	JudgeFailed   = "failed"
	JudgeCanceled = "canceled"
)

// JudgeState tracks the lifecycle of a delegated approval judge run for a
// blocking checkpoint. It is persisted (and event-sourced) so concurrent
// watchers like 'fest show --watch' can render a waiting-on-judge indicator
// while the judge command runs, and so timeouts or judge crashes leave a
// durable failed record instead of vanishing.
type JudgeState struct {
	// Status is one of running, approved, rejected, failed.
	Status string `yaml:"status" json:"status"`

	// Command is the judge command that was invoked.
	Command string `yaml:"command,omitempty" json:"command,omitempty"`

	// Detail carries the judge reason (approved/rejected) or the error text (failed).
	Detail string `yaml:"detail,omitempty" json:"detail,omitempty"`

	// StartedAt is when the judge command was invoked.
	StartedAt *time.Time `yaml:"started_at,omitempty" json:"started_at,omitempty"`

	// Pid is the process id of the detached judge runner, used to detect a
	// stale running record after a crash so the judge can be relaunched.
	Pid int `yaml:"pid,omitempty" json:"pid,omitempty"`

	// RunID uniquely identifies one delegated evaluation. Detached runners
	// must still own this ID before they may update or decide the checkpoint.
	RunID string `yaml:"run_id,omitempty" json:"run_id,omitempty"`

	// FinishedAt is when the judge outcome was recorded.
	FinishedAt *time.Time `yaml:"finished_at,omitempty" json:"finished_at,omitempty"`
}

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

	// RemediationPhase is the linked phase name for a step in
	// StepStatusFailedRemediation. Empty for any other status.
	RemediationPhase string `yaml:"remediation_phase,omitempty" json:"remediation_phase,omitempty"`

	// DecisionActor identifies who made the checkpoint decision.
	DecisionActor string `yaml:"decision_actor,omitempty" json:"decision_actor,omitempty"`

	// DecisionSummary records the decision rationale or approval summary.
	DecisionSummary string `yaml:"decision_summary,omitempty" json:"decision_summary,omitempty"`

	// DecisionAt is when the checkpoint decision was recorded.
	DecisionAt *time.Time `yaml:"decision_at,omitempty" json:"decision_at,omitempty"`

	// Judge tracks the delegated approval judge run for this checkpoint, when
	// one was invoked via 'fest workflow approve --auto'.
	Judge *JudgeState `yaml:"judge,omitempty" json:"judge,omitempty"`
}

// FestivalWorkflowState contains workflow state for all phases in a festival.
// Stored at <festival>/.fest/workflow_state.yaml
type FestivalWorkflowState struct {
	// Version for future migrations.
	Version int `yaml:"version" json:"version"`

	// Phases maps phase directory name to phase workflow state.
	Phases map[string]*WorkflowState `yaml:"phases" json:"phases"`

	// UpdatedAt is the last time any phase state was modified.
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`

	// CreatedAt is when the festival workflow state was created.
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
}

// WorkflowState tracks the workflow progress for a single phase.
type WorkflowState struct {
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

// festivalStateMu protects concurrent access to festival workflow state files
var festivalStateMu sync.Mutex

// NewWorkflowState creates a new workflow state with the given total steps.
func NewWorkflowState(totalSteps int) *WorkflowState {
	now := time.Now().UTC()
	return &WorkflowState{
		CurrentStep: 1,
		TotalSteps:  totalSteps,
		Steps:       make(map[int]*StepState),
		UpdatedAt:   now,
		CreatedAt:   now,
	}
}

// NewFestivalWorkflowState creates a new festival-level workflow state.
func NewFestivalWorkflowState() *FestivalWorkflowState {
	now := time.Now().UTC()
	return &FestivalWorkflowState{
		Version:   stateVersion,
		Phases:    make(map[string]*WorkflowState),
		UpdatedAt: now,
		CreatedAt: now,
	}
}

// loadFestivalState loads the festival-level workflow state file.
// Returns a new state if the file doesn't exist.
func loadFestivalState(ctx context.Context, festivalPath string) (*FestivalWorkflowState, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	statePath := filepath.Join(festivalPath, stateDirName, StateFileName)

	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return NewFestivalWorkflowState(), nil
	}
	if err != nil {
		return nil, err
	}

	var state FestivalWorkflowState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	// Ensure maps are initialized
	if state.Phases == nil {
		state.Phases = make(map[string]*WorkflowState)
	}
	for _, phaseState := range state.Phases {
		if phaseState.Steps == nil {
			phaseState.Steps = make(map[int]*StepState)
		}
	}

	return &state, nil
}

// saveFestivalState persists the festival-level workflow state file.
func saveFestivalState(ctx context.Context, festivalPath string, state *FestivalWorkflowState) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	state.UpdatedAt = time.Now().UTC()

	stateDir := filepath.Join(festivalPath, stateDirName)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}

	data, err := yamlutil.Marshal(state)
	if err != nil {
		return err
	}

	statePath := filepath.Join(stateDir, StateFileName)
	return os.WriteFile(statePath, data, 0644)
}

// LoadState loads workflow state for a specific phase from the festival-level state file.
// Returns a new state if no state exists for this phase.
func LoadState(ctx context.Context, festivalPath, phaseName string) (*WorkflowState, error) {
	festivalStateMu.Lock()
	defer festivalStateMu.Unlock()

	festState, err := loadFestivalState(ctx, festivalPath)
	if err != nil {
		return nil, err
	}

	phaseState, ok := festState.Phases[phaseName]
	if !ok {
		return NewWorkflowState(0), nil
	}

	// Ensure maps are initialized
	if phaseState.Steps == nil {
		phaseState.Steps = make(map[int]*StepState)
	}

	return phaseState, nil
}

// LoadStateFromStore loads workflow state for a specific phase from a StateStore.
// This reads from the JSONL-backed store instead of the YAML file.
// Returns a new state if no state exists for this phase.
func LoadStateFromStore(store StateStore, phaseName string) (*WorkflowState, error) {
	state, ok := store.LoadWorkflowPhaseState(phaseName)
	if !ok || state == nil {
		return NewWorkflowState(0), nil
	}

	if state.Steps == nil {
		state.Steps = make(map[int]*StepState)
	}

	return state, nil
}

// Save persists the workflow state for a specific phase to the festival-level state file.
func (s *WorkflowState) Save(ctx context.Context, festivalPath, phaseName string) error {
	festivalStateMu.Lock()
	defer festivalStateMu.Unlock()

	s.UpdatedAt = time.Now().UTC()

	// Load existing festival state
	festState, err := loadFestivalState(ctx, festivalPath)
	if err != nil {
		return err
	}

	// Update phase entry
	festState.Phases[phaseName] = s

	// Save back to file
	return saveFestivalState(ctx, festivalPath, festState)
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
	state := s.GetOrCreateStepState(s.CurrentStep)
	if state.Status == StepStatusPending {
		state.Status = StepStatusInProgress
		now := time.Now().UTC()
		state.StartedAt = &now
	}
}

// BeginJudge records a delegated judge run starting on a step.
func (s *WorkflowState) BeginJudge(step int, command, runID string, pid int, at time.Time) {
	state := s.GetOrCreateStepState(step)
	started := at
	state.Judge = &JudgeState{
		Status:    JudgeRunning,
		Command:   command,
		StartedAt: &started,
		Pid:       pid,
		RunID:     runID,
	}
}

// ClaimJudge records the detached runner PID only while the supplied run still
// owns the checkpoint. Repeating the same claim is idempotent.
func (s *WorkflowState) ClaimJudge(step int, runID string, pid int) bool {
	state := s.GetStepState(step)
	if state == nil || state.Judge == nil || state.Judge.Status != JudgeRunning ||
		state.Judge.RunID != runID {
		return false
	}
	if state.Judge.Pid != 0 && state.Judge.Pid != pid {
		return false
	}
	state.Judge.Pid = pid
	return true
}

// JudgeOwned reports whether a running judge still owns the current decision.
func (s *WorkflowState) JudgeOwned(step int, runID string) bool {
	state := s.GetStepState(step)
	return state != nil && state.Judge != nil && state.Judge.Status == JudgeRunning &&
		state.Judge.RunID == runID && s.CurrentStep == step &&
		(state.Status == StepStatusPending || state.Status == StepStatusInProgress)
}

// RecordJudgeOutcome records how an owned judge run ended. It returns false
// without changing state when a manual decision, reset, or newer run has
// superseded the caller.
func (s *WorkflowState) RecordJudgeOutcome(step int, runID, status, detail string, at time.Time) bool {
	if runID != "" && !s.JudgeOwned(step, runID) {
		return false
	}
	state := s.GetOrCreateStepState(step)
	if state.Judge == nil {
		state.Judge = &JudgeState{}
	}
	finished := at
	state.Judge.Status = status
	state.Judge.Detail = detail
	state.Judge.FinishedAt = &finished
	return true
}

// CompleteCurrentStep marks the current step as completed.
func (s *WorkflowState) CompleteCurrentStep() {
	s.MarkCurrentStep(StepStatusCompleted, "")
}

// MarkCurrentStep marks the current step with a terminal status and optional note.
func (s *WorkflowState) MarkCurrentStep(status StepStatus, note string) {
	state := s.GetOrCreateStepState(s.CurrentStep)
	state.Status = status
	state.Feedback = note
	now := time.Now().UTC()
	state.CompletedAt = &now
}

// Advance moves to the next step if current is completed.
// Returns error if current step is not terminal or if already at the last step.
func (s *WorkflowState) Advance() error {
	state := s.GetCurrentStepState()
	if state == nil || !state.Status.IsTerminal() {
		return errors.New("current step must be terminal before advancing")
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
	return s.ApproveWithAudit("", DecisionMetadata{})
}

// ApproveWithDecision approves a blocking checkpoint with decision metadata.
func (s *WorkflowState) ApproveWithDecision(decision DecisionMetadata) error {
	return s.ApproveWithAudit("", decision)
}

// ApproveWithAudit approves a blocking checkpoint, recording durable audit
// text and decision metadata, then advances to the next step.
func (s *WorkflowState) ApproveWithAudit(feedback string, decision DecisionMetadata) error {
	s.cancelCurrentJudge("superseded by manual approval")
	s.MarkCurrentStep(StepStatusCompleted, feedback)
	s.recordDecision(s.CurrentStep, decision)
	if s.CurrentStep < s.TotalSteps {
		return s.Advance()
	}
	return nil
}

// Reject rejects the current step with feedback and marks it as blocked.
func (s *WorkflowState) Reject(feedback string) {
	s.RejectWithDecision(feedback, DecisionMetadata{})
}

// RejectWithDecision rejects the current step with feedback and decision metadata.
func (s *WorkflowState) RejectWithDecision(feedback string, decision DecisionMetadata) {
	state := s.GetOrCreateStepState(s.CurrentStep)
	s.cancelCurrentJudge("superseded by manual rejection")
	state.Status = StepStatusBlocked
	state.Feedback = feedback
	state.RemediationPhase = ""
	s.recordDecision(s.CurrentStep, decision)
}

// RejectWithRemediation records a non-passing gate decision with a linked
// remediation phase. The step enters StepStatusFailedRemediation and remains
// non-terminal so the gate must be re-evaluated after the remediation phase
// completes.
func (s *WorkflowState) RejectWithRemediation(feedback, remediationPhase string) {
	s.RejectWithRemediationDecision(feedback, remediationPhase, DecisionMetadata{})
}

// RejectWithRemediationDecision records a non-passing gate decision with metadata.
func (s *WorkflowState) RejectWithRemediationDecision(feedback, remediationPhase string, decision DecisionMetadata) {
	state := s.GetOrCreateStepState(s.CurrentStep)
	s.cancelCurrentJudge("superseded by manual remediation decision")
	state.Status = StepStatusFailedRemediation
	state.Feedback = feedback
	state.RemediationPhase = remediationPhase
	s.recordDecision(s.CurrentStep, decision)
}

func (s *WorkflowState) cancelCurrentJudge(detail string) {
	state := s.GetOrCreateStepState(s.CurrentStep)
	if state.Judge == nil || state.Judge.Status != JudgeRunning {
		return
	}
	now := time.Now().UTC()
	state.Judge.Status = JudgeCanceled
	state.Judge.Detail = detail
	state.Judge.FinishedAt = &now
}

func (s *WorkflowState) recordDecision(step int, decision DecisionMetadata) {
	if decision.Actor == "" && decision.Summary == "" {
		return
	}
	state := s.GetOrCreateStepState(step)
	state.DecisionActor = decision.Actor
	state.DecisionSummary = decision.Summary
	now := time.Now().UTC()
	state.DecisionAt = &now
}

// ClearFailedRemediation transitions the current step out of the failed
// remediation state, returning it to in-progress for re-evaluation. Called
// when the linked remediation phase has completed and the gate is ready to
// be rechecked.
func (s *WorkflowState) ClearFailedRemediation() {
	state := s.GetOrCreateStepState(s.CurrentStep)
	if state.Status != StepStatusFailedRemediation {
		return
	}
	state.Status = StepStatusInProgress
	state.RemediationPhase = ""
	state.DecisionActor = ""
	state.DecisionSummary = ""
	state.DecisionAt = nil
}

// Reset resets the workflow to step 1 and clears all step states.
func (s *WorkflowState) Reset() {
	s.CurrentStep = 1
	for _, state := range s.Steps {
		state.Status = StepStatusPending
		state.StartedAt = nil
		state.CompletedAt = nil
		state.Feedback = ""
		state.DecisionActor = ""
		state.DecisionSummary = ""
		state.DecisionAt = nil
		state.Judge = nil
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
		if state == nil || !state.Status.IsTerminal() {
			return false
		}
	}
	return true
}

// CompletedCount returns the number of terminal steps (completed or skipped).
func (s *WorkflowState) CompletedCount() int {
	count := 0
	for _, state := range s.Steps {
		if state.Status.IsTerminal() {
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

// GetOrCreateStepState returns existing step state or creates a new one.
func (s *WorkflowState) GetOrCreateStepState(stepNum int) *StepState {
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

// --- Event generation helpers for StateStore integration ---

// EmitInitEvents generates events for workflow initialization.
func EmitInitEvents(phaseName string, totalSteps int) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType:  "wf_init",
		Phase:      phaseName,
		TotalSteps: totalSteps,
	}}
}

// EmitStepStartEvents generates events for starting a step.
func EmitStepStartEvents(phaseName string, step int) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType: "wf_step_start",
		Phase:     phaseName,
		Step:      step,
	}}
}

// EmitStepDoneEvents generates events for completing a step.
func EmitStepDoneEvents(phaseName string, step int) []WorkflowEvent {
	return EmitStepDoneWithFeedbackEvents(phaseName, step, "")
}

// EmitStepDoneWithFeedbackEvents generates completion events with optional note metadata.
func EmitStepDoneWithFeedbackEvents(phaseName string, step int, feedback string) []WorkflowEvent {
	return EmitStepDoneWithDecisionEvents(phaseName, step, feedback, DecisionMetadata{})
}

// EmitStepDoneWithDecisionEvents generates completion events with decision metadata.
func EmitStepDoneWithDecisionEvents(phaseName string, step int, feedback string, decision DecisionMetadata) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType:       "wf_step_done",
		Phase:           phaseName,
		Step:            step,
		Feedback:        feedback,
		DecisionActor:   decision.Actor,
		DecisionSummary: decision.Summary,
	}}
}

// EmitStepSkipEvents generates events for intentionally skipped steps.
func EmitStepSkipEvents(phaseName string, step int, feedback string) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType: "wf_step_skip",
		Phase:     phaseName,
		Step:      step,
		Feedback:  feedback,
	}}
}

// EmitStepBlockEvents generates events for blocking a step.
func EmitStepBlockEvents(phaseName string, step int, feedback string) []WorkflowEvent {
	return EmitStepBlockWithDecisionEvents(phaseName, step, feedback, DecisionMetadata{})
}

// EmitStepBlockWithDecisionEvents generates blocking events with decision metadata.
func EmitStepBlockWithDecisionEvents(phaseName string, step int, feedback string, decision DecisionMetadata) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType:       "wf_step_block",
		Phase:           phaseName,
		Step:            step,
		Feedback:        feedback,
		DecisionActor:   decision.Actor,
		DecisionSummary: decision.Summary,
	}}
}

// EmitStepFailRemediationEvents generates events for marking a step as
// failed with a linked remediation phase.
func EmitStepFailRemediationEvents(phaseName string, step int, feedback, remediationPhase string) []WorkflowEvent {
	return EmitStepFailRemediationWithDecisionEvents(phaseName, step, feedback, remediationPhase, DecisionMetadata{})
}

// EmitStepFailRemediationWithDecisionEvents generates failed-remediation events with decision metadata.
func EmitStepFailRemediationWithDecisionEvents(phaseName string, step int, feedback, remediationPhase string, decision DecisionMetadata) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType:        "wf_step_fail_remediation",
		Phase:            phaseName,
		Step:             step,
		Feedback:         feedback,
		RemediationPhase: remediationPhase,
		DecisionActor:    decision.Actor,
		DecisionSummary:  decision.Summary,
	}}
}

// EmitStepRecheckEvents generates events for re-entering a failed
// remediation step once the remediation phase has completed.
func EmitStepRecheckEvents(phaseName string, step int) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType: "wf_step_recheck",
		Phase:     phaseName,
		Step:      step,
	}}
}

// EmitAdvanceEvents generates events for advancing to a step.
func EmitAdvanceEvents(phaseName string, step int) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType: "wf_advance",
		Phase:     phaseName,
		Step:      step,
	}}
}

// EmitResetEvents generates events for resetting a workflow.
func EmitResetEvents(phaseName string) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType: "wf_reset",
		Phase:     phaseName,
	}}
}

// EmitJudgeStartedEvents generates events for a delegated judge run starting
// on a blocking checkpoint.
func EmitJudgeStartedEvents(phaseName string, step int, command, runID string, pid int) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType:    "wf_judge_started",
		Phase:        phaseName,
		Step:         step,
		JudgeStatus:  JudgeRunning,
		JudgeCommand: command,
		JudgePid:     pid,
		JudgeRunID:   runID,
	}}
}

// EmitJudgeClaimedEvents binds a durable judge lease to the detached process
// that may evaluate it.
func EmitJudgeClaimedEvents(phaseName string, step int, runID string, pid int) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType:  "wf_judge_claimed",
		Phase:      phaseName,
		Step:       step,
		JudgePid:   pid,
		JudgeRunID: runID,
	}}
}

// EmitJudgeReturnedEvents generates events recording how a delegated judge
// run ended: approved, rejected, or failed (timeout, missing command,
// malformed verdict).
func EmitJudgeReturnedEvents(phaseName string, step int, runID, status, detail string) []WorkflowEvent {
	return []WorkflowEvent{{
		EventType:   "wf_judge_returned",
		Phase:       phaseName,
		Step:        step,
		JudgeStatus: status,
		JudgeDetail: detail,
		JudgeRunID:  runID,
	}}
}
