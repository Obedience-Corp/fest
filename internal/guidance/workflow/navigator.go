package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
)

// StepTypeWorkflowStep is the step type for workflow-based steps.
const StepTypeWorkflowStep = "workflow_step"

// Navigator provides workflow-based navigation for non-implementation phases.
// It parses WORKFLOW.md files and tracks step progress.
// Also supports GATES.md files via SetDocFilename and SetStateKeyPrefix.
type Navigator struct {
	*guidance.BaseNavigator
	parser         *Parser
	steps          []WorkflowStep
	workflowState  *WorkflowState
	festivalPath   string
	phaseName      string
	phaseDir       string
	mode           guidance.Mode
	store          StateStore // optional: if set, use JSONL events instead of YAML
	docFilename    string     // defaults to "WORKFLOW.md", can be overridden for GATES.md
	stateKeyPrefix string     // prefix for state key, e.g. "gate:" for phase gates
}

// Ensure Navigator implements guidance.Navigator.
var _ guidance.Navigator = (*Navigator)(nil)

// NewNavigator creates a navigator for workflow-based phases.
func NewNavigator(gctx *guidance.GuidanceContext, mode guidance.Mode) (*Navigator, error) {
	base, err := guidance.NewBaseNavigator(gctx, mode)
	if err != nil {
		return nil, err
	}

	return &Navigator{
		BaseNavigator: base,
		parser:        NewParser(),
		mode:          mode,
	}, nil
}

// SetStateStore sets the StateStore for JSONL-backed workflow state persistence.
// When set, the navigator uses the progress event log instead of workflow_state.yaml.
func (n *Navigator) SetStateStore(store StateStore) {
	n.store = store
}

// HasStateStore reports whether workflow state is backed by an injected event
// store instead of the legacy YAML state file. Reloading must preserve this
// persistence mode so injected navigators do not silently change backends.
func (n *Navigator) HasStateStore() bool {
	return n.store != nil
}

// SetDocFilename overrides the document filename to parse (default: "WORKFLOW.md").
// Use "GATES.md" for phase-level gates.
func (n *Navigator) SetDocFilename(name string) {
	n.docFilename = name
}

// SetStateKeyPrefix sets a prefix for the progress state key.
// For phase gates, use "gate:" so state is tracked separately from workflows.
func (n *Navigator) SetStateKeyPrefix(prefix string) {
	n.stateKeyPrefix = prefix
}

// IsGate reports whether this navigator targets a phase gate (GATES.md) rather
// than a regular WORKFLOW.md checkpoint.
func (n *Navigator) IsGate() bool {
	return n.docFilename == "GATES.md"
}

// stateKey returns the key used for progress state lookups.
func (n *Navigator) stateKey() string {
	return n.stateKeyPrefix + n.phaseName
}

// filename returns the document filename to parse.
func (n *Navigator) filename() string {
	if n.docFilename != "" {
		return n.docFilename
	}
	return "WORKFLOW.md"
}

// Initialize loads the workflow and state.
func (n *Navigator) Initialize(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := n.BaseNavigator.Initialize(ctx); err != nil {
		return err
	}

	// Store festival path for state management
	n.festivalPath = n.Ctx.FestivalPath

	// Determine phase directory from context
	n.phaseDir = n.Ctx.PhasePath
	if n.phaseDir == "" {
		// If no phase path, try festival path
		n.phaseDir = n.Ctx.FestivalPath
	}

	// Extract phase name from phase directory
	n.phaseName = filepath.Base(n.phaseDir)

	// Load document from phase directory (WORKFLOW.md or GATES.md)
	docPath := filepath.Join(n.phaseDir, n.filename())
	if _, err := os.Stat(docPath); os.IsNotExist(err) {
		// No document found, return empty steps
		n.steps = []WorkflowStep{}
		n.workflowState = NewWorkflowState(0)
		return nil
	}

	steps, err := n.parser.Parse(ctx, docPath)
	if err != nil {
		return errors.Parse("parsing workflow document", err).WithField("file", n.filename())
	}
	if n.IsGate() {
		normalizeLegacyGateCheckpointClasses(steps)
	}

	n.steps = steps

	// Load or initialize state (using stateKey for proper namespacing)
	sk := n.stateKey()
	var state *WorkflowState
	if n.store != nil {
		// Use JSONL-backed store
		state, err = LoadStateFromStore(n.store, sk)
		if err != nil {
			return errors.Wrap(err, "loading state from store")
		}
	} else {
		// Fall back to YAML file
		state, err = LoadState(ctx, n.festivalPath, sk)
		if err != nil {
			return errors.Wrap(err, "loading state")
		}
	}

	// If state is empty, initialize it with parsed steps
	if state.TotalSteps == 0 && len(steps) > 0 {
		state.Initialize(steps)

		// Emit init event if using store
		if n.store != nil {
			n.store.QueueWorkflowEvents(EmitInitEvents(sk, len(steps)))
			// Also emit step_start for step 1
			n.store.QueueWorkflowEvents(EmitStepStartEvents(sk, 1))
			if err := n.store.SaveEvents(ctx); err != nil {
				return errors.Wrap(err, "saving init events")
			}
		}
	}

	n.workflowState = state

	return nil
}

// normalizeLegacyGateCheckpointClasses preserves auto-judge behavior for
// phase gates created before checkpoint classes were added to the GATES.md
// template. Phase gates are artifact-oriented quality checks by convention,
// so an omitted class on a blocking gate is compatible with artifact_review.
// Explicit operator_attestation remains human-only, and regular WORKFLOW.md
// steps continue to fail closed through ClassifyCheckpoint.
func normalizeLegacyGateCheckpointClasses(steps []WorkflowStep) {
	for i := range steps {
		if steps[i].Checkpoint.IsBlocking() &&
			steps[i].CheckpointClass == CheckpointClassUnspecified {
			steps[i].CheckpointClass = CheckpointClassArtifactReview
		}
	}
}

// GetNext returns the next workflow step.
func (n *Navigator) GetNext(ctx context.Context) (*guidance.NextStep, error) {
	if err := n.EnsureInitialized(); err != nil {
		return nil, err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	// Check if workflow is complete
	if n.workflowState.IsComplete() {
		return nil, nil
	}

	// No steps to process
	if len(n.steps) == 0 {
		return nil, nil
	}

	currentStepNum := n.workflowState.CurrentStep
	if currentStepNum < 1 || currentStepNum > len(n.steps) {
		return nil, nil
	}

	step := n.steps[currentStepNum-1]
	return n.buildNextStep(step), nil
}

// buildNextStep creates a NextStep from a WorkflowStep.
func (n *Navigator) buildNextStep(step WorkflowStep) *guidance.NextStep {
	nextStep := &guidance.NextStep{
		Mode:          n.mode,
		StepType:      StepTypeWorkflowStep,
		ID:            fmt.Sprintf("step_%d", step.Number),
		Title:         fmt.Sprintf("Step %d: %s", step.Number, step.Name),
		Objective:     step.Goal,
		Instructions:  step.Actions,
		ContextFiles:  n.getContextFiles(),
		AutonomyLevel: n.getAutonomyLevel(step),
		Metadata: map[string]any{
			"step_number": step.Number,
			"step_name":   step.Name,
			"output":      step.Output,
			"checkpoint":  step.Checkpoint,
			"total_steps": n.workflowState.TotalSteps,
		},
	}

	// Set completion command
	nextStep.CompletionCommand = "fest workflow advance"

	return nextStep
}

// getAutonomyLevel determines autonomy based on checkpoint type.
func (n *Navigator) getAutonomyLevel(step WorkflowStep) guidance.AutonomyLevel {
	if step.Checkpoint.IsBlocking() {
		return guidance.AutonomyLow
	}
	return guidance.AutonomyMedium
}

// getContextFiles returns relevant context files for the workflow.
func (n *Navigator) getContextFiles() []string {
	var files []string

	// Festival goal
	festivalGoal := filepath.Join(n.Ctx.FestivalPath, "FESTIVAL_GOAL.md")
	if _, err := os.Stat(festivalGoal); err == nil {
		files = append(files, festivalGoal)
	}

	// Phase document (WORKFLOW.md or GATES.md)
	docPath := filepath.Join(n.phaseDir, n.filename())
	if _, err := os.Stat(docPath); err == nil {
		files = append(files, docPath)
	}

	return files
}

// GetContextFiles returns context files.
func (n *Navigator) GetContextFiles(ctx context.Context) ([]string, error) {
	if err := n.EnsureInitialized(); err != nil {
		return nil, err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	return n.getContextFiles(), nil
}

// BeginJudge durably records that a delegated approval judge run started on
// the current step, before the judge command executes, so concurrent watchers
// (fest show --watch, fest progress) can render the waiting-on-judge state
// and a crashed or timed-out judge still leaves a trace.
func (n *Navigator) BeginJudge(ctx context.Context, step int, command, runID string, pid int) error {
	return n.recordJudge(ctx, EmitJudgeStartedEvents(n.stateKey(), step, command, runID, pid), func(at time.Time) bool {
		n.workflowState.BeginJudge(step, command, runID, pid, at)
		return true
	})
}

// ClaimJudge binds a previously persisted run lease to its detached process.
// It returns false when another transition superseded the run first.
func (n *Navigator) ClaimJudge(ctx context.Context, step int, runID string, pid int) (bool, error) {
	claimed := false
	err := n.recordJudge(ctx, EmitJudgeClaimedEvents(n.stateKey(), step, runID, pid), func(_ time.Time) bool {
		claimed = n.workflowState.ClaimJudge(step, runID, pid)
		return claimed
	})
	return claimed, err
}

// RecordJudgeOutcome durably records how the judge run ended: JudgeApproved,
// JudgeRejected, or JudgeFailed with the reason or error text in detail.
func (n *Navigator) RecordJudgeOutcome(ctx context.Context, step int, runID, status, detail string) (bool, error) {
	recorded := false
	err := n.recordJudge(ctx, EmitJudgeReturnedEvents(n.stateKey(), step, runID, status, detail), func(at time.Time) bool {
		recorded = n.workflowState.RecordJudgeOutcome(step, runID, status, detail, at)
		return recorded
	})
	return recorded, err
}

// RecordJudgeFailure durably marks an owned judge lease as failed. Unlike a
// normal verdict, stale-run cleanup remains valid after the step is blocked.
func (n *Navigator) RecordJudgeFailure(ctx context.Context, step int, runID, detail string) (bool, error) {
	recorded := false
	err := n.recordJudge(ctx, EmitJudgeReturnedEvents(n.stateKey(), step, runID, JudgeFailed, detail), func(at time.Time) bool {
		recorded = n.workflowState.RecordJudgeFailure(step, runID, detail, at)
		return recorded
	})
	return recorded, err
}

func (n *Navigator) recordJudge(ctx context.Context, events []WorkflowEvent, apply func(at time.Time) bool) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	if !apply(time.Now().UTC()) {
		return nil
	}

	if n.store != nil {
		n.store.QueueWorkflowEvents(events)
		return n.store.SaveEvents(ctx)
	}
	return n.workflowState.Save(ctx, n.festivalPath, n.stateKey())
}

// Approve approves a blocking checkpoint and advances.
func (n *Navigator) Approve(ctx context.Context) error {
	return n.ApproveWithAudit(ctx, "", DecisionMetadata{})
}

// ApproveWithDecision approves a blocking checkpoint and records decision metadata.
func (n *Navigator) ApproveWithDecision(ctx context.Context, decision DecisionMetadata) error {
	return n.ApproveWithAudit(ctx, "", decision)
}

// ApproveWithAudit approves a blocking checkpoint, recording durable audit
// text and decision metadata, then advances.
func (n *Navigator) ApproveWithAudit(ctx context.Context, feedback string, decision DecisionMetadata) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	currentStep := n.workflowState.CurrentStep
	stepState := n.workflowState.GetStepState(currentStep)
	judgeRunID := runningJudgeRunID(stepState)
	judgeClearRunID := terminalJudgeRunID(stepState, decision)
	if err := n.workflowState.ApproveWithAudit(feedback, decision); err != nil {
		return err
	}

	sk := n.stateKey()
	if n.store != nil {
		if judgeRunID != "" {
			n.store.QueueWorkflowEvents(EmitJudgeReturnedEvents(sk, currentStep, judgeRunID, JudgeCanceled, "superseded by manual approval"))
		}
		if judgeClearRunID != "" {
			n.store.QueueWorkflowEvents(EmitJudgeClearedEvents(sk, currentStep, judgeClearRunID))
		}
		n.store.QueueWorkflowEvents(EmitStepDoneWithDecisionEvents(sk, currentStep, feedback, decision))
		if n.workflowState.CurrentStep > currentStep {
			n.store.QueueWorkflowEvents(EmitAdvanceEvents(sk, n.workflowState.CurrentStep))
			n.store.QueueWorkflowEvents(EmitStepStartEvents(sk, n.workflowState.CurrentStep))
		}
		return n.store.SaveEvents(ctx)
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// Reject rejects the current step with feedback.
func (n *Navigator) Reject(ctx context.Context, reason string) error {
	return n.RejectWithDecision(ctx, reason, DecisionMetadata{})
}

// RejectWithDecision rejects the current step with feedback and decision metadata.
func (n *Navigator) RejectWithDecision(ctx context.Context, reason string, decision DecisionMetadata) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	currentStep := n.workflowState.CurrentStep
	stepState := n.workflowState.GetStepState(currentStep)
	judgeRunID := runningJudgeRunID(stepState)
	judgeClearRunID := terminalJudgeRunID(stepState, decision)
	n.workflowState.RejectWithDecision(reason, decision)

	sk := n.stateKey()
	if n.store != nil {
		if judgeRunID != "" {
			n.store.QueueWorkflowEvents(EmitJudgeReturnedEvents(sk, currentStep, judgeRunID, JudgeCanceled, "superseded by manual rejection"))
		}
		if judgeClearRunID != "" {
			n.store.QueueWorkflowEvents(EmitJudgeClearedEvents(sk, currentStep, judgeClearRunID))
		}
		n.store.QueueWorkflowEvents(EmitStepBlockWithDecisionEvents(sk, currentStep, reason, decision))
		return n.store.SaveEvents(ctx)
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// RejectWithRemediation records the current step as failed with a linked
// remediation phase. The step remains non-terminal so the workflow does not
// silently complete past a real failure; the gate must be re-evaluated once
// the remediation phase finishes.
func (n *Navigator) RejectWithRemediation(ctx context.Context, reason, remediationPhase string) error {
	return n.RejectWithRemediationDecision(ctx, reason, remediationPhase, DecisionMetadata{})
}

// RejectWithRemediationDecision records a failed gate with decision metadata.
func (n *Navigator) RejectWithRemediationDecision(ctx context.Context, reason, remediationPhase string, decision DecisionMetadata) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	currentStep := n.workflowState.CurrentStep
	stepState := n.workflowState.GetStepState(currentStep)
	judgeRunID := runningJudgeRunID(stepState)
	judgeClearRunID := terminalJudgeRunID(stepState, decision)
	n.workflowState.RejectWithRemediationDecision(reason, remediationPhase, decision)

	sk := n.stateKey()
	if n.store != nil {
		if judgeRunID != "" {
			n.store.QueueWorkflowEvents(EmitJudgeReturnedEvents(sk, currentStep, judgeRunID, JudgeCanceled, "superseded by manual remediation decision"))
		}
		if judgeClearRunID != "" {
			n.store.QueueWorkflowEvents(EmitJudgeClearedEvents(sk, currentStep, judgeClearRunID))
		}
		n.store.QueueWorkflowEvents(EmitStepFailRemediationWithDecisionEvents(sk, currentStep, reason, remediationPhase, decision))
		return n.store.SaveEvents(ctx)
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

func runningJudgeRunID(state *StepState) string {
	if state == nil || state.Judge == nil || state.Judge.Status != JudgeRunning {
		return ""
	}
	return state.Judge.RunID
}

func terminalJudgeRunID(state *StepState, decision DecisionMetadata) string {
	if decision.Actor == "agent" || state == nil || state.Judge == nil ||
		state.Judge.Status == JudgeRunning || state.Judge.Status == JudgeCanceled {
		return ""
	}
	return state.Judge.RunID
}

// ApplyJudgeApproval atomically records an owned verdict and advances the
// checkpoint. The caller must hold the checkpoint's cross-process lock.
func (n *Navigator) ApplyJudgeApproval(ctx context.Context, step int, runID, audit string, decision DecisionMetadata) (bool, error) {
	if err := n.EnsureInitialized(); err != nil {
		return false, err
	}
	if !n.workflowState.JudgeOwned(step, runID) {
		return false, nil
	}
	now := time.Now().UTC()
	if !n.workflowState.RecordJudgeOutcome(step, runID, JudgeApproved, decision.Summary, now) {
		return false, nil
	}
	if err := n.workflowState.ApproveWithAudit(decision.Summary, decision); err != nil {
		return false, err
	}

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitJudgeReturnedEvents(sk, step, runID, JudgeApproved, decision.Summary))
		n.store.QueueWorkflowEvents(EmitStepDoneWithDecisionEvents(sk, step, decision.Summary, decision))
		if n.workflowState.CurrentStep > step {
			n.store.QueueWorkflowEvents(EmitAdvanceEvents(sk, n.workflowState.CurrentStep))
			n.store.QueueWorkflowEvents(EmitStepStartEvents(sk, n.workflowState.CurrentStep))
		}
		return true, n.store.SaveEvents(ctx)
	}
	return true, n.workflowState.Save(ctx, n.festivalPath, sk)
}

// ApplyJudgeRejection atomically records an owned verdict and blocks the
// checkpoint. The caller must hold the checkpoint's cross-process lock.
func (n *Navigator) ApplyJudgeRejection(ctx context.Context, step int, runID, audit string, decision DecisionMetadata) (bool, error) {
	if err := n.EnsureInitialized(); err != nil {
		return false, err
	}
	if !n.workflowState.JudgeOwned(step, runID) {
		return false, nil
	}
	now := time.Now().UTC()
	if !n.workflowState.RecordJudgeOutcome(step, runID, JudgeRejected, decision.Summary, now) {
		return false, nil
	}
	n.workflowState.RejectWithDecision(decision.Summary, decision)

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitJudgeReturnedEvents(sk, step, runID, JudgeRejected, decision.Summary))
		n.store.QueueWorkflowEvents(EmitStepBlockWithDecisionEvents(sk, step, decision.Summary, decision))
		return true, n.store.SaveEvents(ctx)
	}
	return true, n.workflowState.Save(ctx, n.festivalPath, sk)
}

// Recheck transitions the current step out of failed_with_remediation back
// to in-progress so the operator can re-evaluate (approve or reject) the
// gate after the linked remediation phase has completed.
func (n *Navigator) Recheck(ctx context.Context) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	n.workflowState.ClearFailedRemediation()

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitStepRecheckEvents(sk, n.workflowState.CurrentStep))
		return n.store.SaveEvents(ctx)
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// ReopenJudgeRejection moves the current judge-owned rejection back to
// in-progress so revised evidence can be submitted with the approval judge.
// Ordinary operator rejections are intentionally left untouched.
func (n *Navigator) ReopenJudgeRejection(ctx context.Context, step int) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if n.workflowState.CurrentStep != step {
		return errors.Validation("approval judge can only re-run the current step").
			WithField("expected_step", n.workflowState.CurrentStep).
			WithField("step", step)
	}
	if !n.workflowState.ReopenJudgeRejection(step) {
		return errors.Validation("current step is not blocked by an approval judge rejection").
			WithHint("address the current feedback first, then run 'fest workflow judge'; ordinary operator rejections require 'fest workflow approve'")
	}

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitJudgeRecheckEvents(sk, step))
		return n.store.SaveEvents(ctx)
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// GetWorkflowState returns the current workflow state.
func (n *Navigator) GetWorkflowState() *WorkflowState {
	return n.workflowState
}

// GetSteps returns the parsed workflow steps.
func (n *Navigator) GetSteps() []WorkflowStep {
	return n.steps
}

// Reset resets the workflow to step 1 and clears all step states.
func (n *Navigator) Reset(ctx context.Context) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	n.workflowState.Reset()

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitResetEvents(sk))
		return n.store.SaveEvents(ctx)
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}
