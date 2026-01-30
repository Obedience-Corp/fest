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
type Navigator struct {
	*guidance.BaseNavigator
	parser        *Parser
	steps         []WorkflowStep
	workflowState *WorkflowState
	phaseDir      string
	mode          guidance.Mode
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

	// Determine phase directory from context
	n.phaseDir = n.Ctx.PhasePath
	if n.phaseDir == "" {
		// If no phase path, try festival path
		n.phaseDir = n.Ctx.FestivalPath
	}

	// Load WORKFLOW.md from phase directory
	workflowPath := filepath.Join(n.phaseDir, "WORKFLOW.md")
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		// No WORKFLOW.md, return empty steps
		n.steps = []WorkflowStep{}
		n.workflowState = NewWorkflowState(0)
		return nil
	}

	steps, err := n.parser.Parse(ctx, workflowPath)
	if err != nil {
		return fmt.Errorf("parsing workflow: %w", err)
	}

	n.steps = steps

	// Load or initialize workflow state
	state, err := LoadState(ctx, n.phaseDir)
	if err != nil {
		return fmt.Errorf("loading workflow state: %w", err)
	}

	// If state is empty, initialize it with parsed steps
	if state.TotalSteps == 0 && len(steps) > 0 {
		state.Initialize(steps)
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

	// Phase WORKFLOW.md
	workflowPath := filepath.Join(n.phaseDir, "WORKFLOW.md")
	if _, err := os.Stat(workflowPath); err == nil {
		files = append(files, workflowPath)
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

	// Complete current step
	n.workflowState.CompleteCurrentStep()

	// Try to advance to next step (ignore error if already at last step)
	if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
		_ = n.workflowState.Advance()
	}

	// Save state
	return n.workflowState.Save(ctx, n.phaseDir)
}

// MarkSkipped marks a step as skipped.
func (n *Navigator) MarkSkipped(ctx context.Context, stepID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	// Mark current as completed (skipped = completed for workflow purposes)
	n.workflowState.CompleteCurrentStep()

	// Try to advance
	if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
		_ = n.workflowState.Advance()
	}

	return n.workflowState.Save(ctx, n.phaseDir)
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
	return n.workflowState.Save(ctx, n.phaseDir)
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

	// Complete current step
	n.workflowState.CompleteCurrentStep()

	// Try to advance to next step (will fail if at last step, which is OK)
	if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
		if err := n.workflowState.Advance(); err != nil {
			return err
		}
	}

	return n.workflowState.Save(ctx, n.phaseDir)
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
		"StepNumber": step.Number,
		"StepName":   step.Name,
	}

	return agent.Render("workflow/checkpoint", data)
}

// formatStep renders the step template for the current workflow step.
func (n *Navigator) formatStep(step WorkflowStep, stepState *StepState) (string, error) {
	status := string(stepState.Status)
	feedback := stepState.Feedback

	data := map[string]any{
		"PhaseType":   strings.ToUpper(string(n.mode)),
		"PhaseName":   n.Ctx.PhaseName,
		"StepNumber":  step.Number,
		"TotalSteps":  n.workflowState.TotalSteps,
		"StepName":    step.Name,
		"Goal":        step.Goal,
		"Actions":     step.Actions,
		"Output":      step.Output,
		"IsBlocking":  step.Checkpoint.IsBlocking(),
		"Status":      status,
		"Feedback":    feedback,
		"CurrentStep": n.workflowState.CurrentStep,
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

	if err := n.workflowState.Approve(); err != nil {
		return err
	}

	return n.workflowState.Save(ctx, n.phaseDir)
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
	return n.workflowState.Save(ctx, n.phaseDir)
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
	return n.workflowState.Save(ctx, n.phaseDir)
}
