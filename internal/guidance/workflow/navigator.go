package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/embedded/templates/agent"
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
		return fmt.Errorf("parsing %s: %w", n.filename(), err)
	}

	n.steps = steps

	// Load or initialize state (using stateKey for proper namespacing)
	sk := n.stateKey()
	var state *WorkflowState
	if n.store != nil {
		// Use JSONL-backed store
		state, err = LoadStateFromStore(n.store, sk)
		if err != nil {
			return fmt.Errorf("loading state from store: %w", err)
		}
	} else {
		// Fall back to YAML file
		state, err = LoadState(ctx, n.festivalPath, sk)
		if err != nil {
			return fmt.Errorf("loading state: %w", err)
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
				return fmt.Errorf("saving init events: %w", err)
			}
		}
	}

	n.workflowState = state

	return nil
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

// MarkComplete marks a step as complete.
func (n *Navigator) MarkComplete(ctx context.Context, stepID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	currentStep := n.workflowState.CurrentStep
	n.workflowState.CompleteCurrentStep()

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitStepDoneEvents(sk, currentStep))
		if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
			_ = n.workflowState.Advance()
			n.store.QueueWorkflowEvents(EmitAdvanceEvents(sk, n.workflowState.CurrentStep))
			n.store.QueueWorkflowEvents(EmitStepStartEvents(sk, n.workflowState.CurrentStep))
		}
		return n.store.SaveEvents(ctx)
	}

	if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
		_ = n.workflowState.Advance()
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// MarkSkipped marks a step as skipped.
func (n *Navigator) MarkSkipped(ctx context.Context, stepID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}
	if err := n.validateCurrentStepID(stepID); err != nil {
		return err
	}
	return n.SkipCurrentStep(ctx, StepStatusSkipped, "")
}

func (n *Navigator) validateCurrentStepID(stepID string) error {
	if strings.TrimSpace(stepID) == "" {
		return nil
	}
	expected := fmt.Sprintf("step_%d", n.workflowState.CurrentStep)
	if stepID != expected {
		return fmt.Errorf("step ID mismatch: expected %s, got %s", expected, stepID)
	}
	return nil
}

// SkipCurrentStep marks the current step as skipped/completed with an audit reason.
func (n *Navigator) SkipCurrentStep(ctx context.Context, status StepStatus, reason string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	if status != StepStatusSkipped && status != StepStatusCompleted {
		return fmt.Errorf("invalid terminal status for skip: %s", status)
	}

	currentStep := n.workflowState.CurrentStep
	n.workflowState.MarkCurrentStep(status, reason)

	sk := n.stateKey()
	if n.store != nil {
		if status == StepStatusSkipped {
			n.store.QueueWorkflowEvents(EmitStepSkipEvents(sk, currentStep, reason))
		} else {
			n.store.QueueWorkflowEvents(EmitStepDoneWithFeedbackEvents(sk, currentStep, reason))
		}
		if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
			_ = n.workflowState.Advance()
			n.store.QueueWorkflowEvents(EmitAdvanceEvents(sk, n.workflowState.CurrentStep))
			n.store.QueueWorkflowEvents(EmitStepStartEvents(sk, n.workflowState.CurrentStep))
		}
		return n.store.SaveEvents(ctx)
	}

	if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
		_ = n.workflowState.Advance()
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// MarkFailed marks a step as failed.
func (n *Navigator) MarkFailed(ctx context.Context, stepID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	n.workflowState.Reject("step failed")

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitStepBlockEvents(sk, n.workflowState.CurrentStep, "step failed"))
		return n.store.SaveEvents(ctx)
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// Advance moves to the next step.
func (n *Navigator) Advance(ctx context.Context) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	if n.workflowState.IsComplete() {
		return guidance.ErrAlreadyComplete
	}

	currentStep := n.workflowState.CurrentStep
	n.workflowState.CompleteCurrentStep()

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitStepDoneEvents(sk, currentStep))
		if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
			if err := n.workflowState.Advance(); err != nil {
				return err
			}
			n.store.QueueWorkflowEvents(EmitAdvanceEvents(sk, n.workflowState.CurrentStep))
			n.store.QueueWorkflowEvents(EmitStepStartEvents(sk, n.workflowState.CurrentStep))
		}
		return n.store.SaveEvents(ctx)
	}

	if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
		if err := n.workflowState.Advance(); err != nil {
			return err
		}
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// GetProgress returns workflow progress.
func (n *Navigator) GetProgress(ctx context.Context) (*guidance.Progress, error) {
	if err := n.EnsureInitialized(); err != nil {
		return nil, err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	progress := guidance.NewProgress(n.mode)
	progress.Total = n.workflowState.TotalSteps
	progress.Completed = n.workflowState.CompletedCount()
	progress.Pending = progress.Total - progress.Completed
	progress.Calculate()

	if n.workflowState.CurrentStep >= 1 && n.workflowState.CurrentStep <= len(n.steps) {
		step := n.steps[n.workflowState.CurrentStep-1]
		progress.CurrentTask = step.Name
	}

	return progress, nil
}

// FormatInstructions generates agent-friendly workflow instructions using templates.
func (n *Navigator) FormatInstructions(ctx context.Context) (string, error) {
	if err := n.EnsureInitialized(); err != nil {
		return "", err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}

	// Check if complete
	if n.workflowState.IsComplete() {
		return n.formatComplete(), nil
	}

	// No steps
	if len(n.steps) == 0 {
		return "No workflow steps defined.\n", nil
	}

	currentStepNum := n.workflowState.CurrentStep
	if currentStepNum < 1 || currentStepNum > len(n.steps) {
		return "Invalid workflow state.\n", nil
	}

	step := n.steps[currentStepNum-1]
	stepState := n.workflowState.GetStepState(currentStepNum)

	// Check if this is a blocking step that's currently in progress (awaiting user action)
	// A blocking checkpoint means the user needs to approve before advancing
	if step.Checkpoint.IsBlocking() && stepState.Status == StepStatusInProgress {
		return n.formatCheckpoint(step)
	}

	return n.formatStep(step, stepState)
}

// formatComplete renders the completion message using templates.
func (n *Navigator) formatComplete() string {
	type stepSummary struct {
		Number int
		Name   string
	}

	steps := make([]stepSummary, len(n.steps))
	for i, step := range n.steps {
		steps[i] = stepSummary{
			Number: step.Number,
			Name:   step.Name,
		}
	}

	data := map[string]any{
		"PhaseType":  strings.ToUpper(string(n.mode)),
		"TotalSteps": n.workflowState.TotalSteps,
		"Steps":      steps,
	}

	output, err := agent.Render("workflow/complete", data)
	if err != nil {
		// Fallback to simple message on error
		return "# Workflow Complete\n\nAll steps completed. Run `fest status` to view progress.\n"
	}
	return output
}

// formatCheckpoint renders the checkpoint template for blocking steps.
func (n *Navigator) formatCheckpoint(step WorkflowStep) (string, error) {
	data := map[string]any{
		"InstructionHeader": guidance.InstructionHeader,
		"StepNumber":        step.Number,
		"StepName":          step.Name,
	}

	return agent.Render("workflow/checkpoint", data)
}

// formatStep renders the step template for the current workflow step.
func (n *Navigator) formatStep(step WorkflowStep, stepState *StepState) (string, error) {
	status := string(stepState.Status)
	feedback := stepState.Feedback

	// Show "PHASE GATE" for gate documents, phase type for workflows
	phaseType := strings.ToUpper(string(n.mode))
	isGate := n.docFilename == "GATES.md"
	if isGate {
		phaseType = strings.ToUpper(n.Ctx.PhaseType) + " PHASE GATE"
	}

	data := map[string]any{
		"InstructionHeader": guidance.InstructionHeader,
		"PhaseType":         phaseType,
		"PhaseName":         n.Ctx.PhaseName,
		"StepNumber":        step.Number,
		"TotalSteps":        n.workflowState.TotalSteps,
		"StepName":          step.Name,
		"Goal":              step.Goal,
		"Actions":           step.Actions,
		"Output":            step.Output,
		"IsBlocking":        step.Checkpoint.IsBlocking(),
		"Status":            status,
		"Feedback":          feedback,
		"CurrentStep":       n.workflowState.CurrentStep,
		"IsGate":            isGate,
	}

	return agent.Render("workflow/step", data)
}

// FormatProgress renders the progress template showing workflow status.
func (n *Navigator) FormatProgress(ctx context.Context) (string, error) {
	if err := n.EnsureInitialized(); err != nil {
		return "", err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}

	type stepInfo struct {
		Number     int
		Name       string
		Status     string
		IsBlocking bool
	}

	steps := make([]stepInfo, len(n.steps))
	for i, step := range n.steps {
		state := n.workflowState.GetStepState(step.Number)
		steps[i] = stepInfo{
			Number:     step.Number,
			Name:       step.Name,
			Status:     string(state.Status),
			IsBlocking: step.Checkpoint.IsBlocking(),
		}
	}

	currentStatus := ""
	if n.workflowState.CurrentStep >= 1 && n.workflowState.CurrentStep <= len(n.steps) {
		state := n.workflowState.GetStepState(n.workflowState.CurrentStep)
		currentStatus = string(state.Status)
	}

	data := map[string]any{
		"PhaseName":     n.Ctx.PhaseName,
		"PhaseType":     string(n.mode),
		"Completed":     n.workflowState.CompletedCount(),
		"Total":         n.workflowState.TotalSteps,
		"CurrentStep":   n.workflowState.CurrentStep,
		"CurrentStatus": currentStatus,
		"Steps":         steps,
	}

	return agent.Render("workflow/progress", data)
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

// Approve approves a blocking checkpoint and advances.
func (n *Navigator) Approve(ctx context.Context) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	currentStep := n.workflowState.CurrentStep
	if err := n.workflowState.Approve(); err != nil {
		return err
	}

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitStepDoneEvents(sk, currentStep))
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
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	n.workflowState.Reject(reason)

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitStepBlockEvents(sk, n.workflowState.CurrentStep, reason))
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
