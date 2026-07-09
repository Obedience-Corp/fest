package workflow

import (
	"context"
	"strings"

	"github.com/Obedience-Corp/fest/embedded/templates/agent"
	"github.com/Obedience-Corp/fest/internal/guidance"
)

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

// displayPhaseType returns the uppercased phase type for use in rendered headers.
// It prefers the context's PhaseType (e.g. "ingest", "planning") over the mode
// string, which is always "workflow" for workflow-based phases and gives no
// useful identity when multiple workflow phases appear in sequence.
func (n *Navigator) displayPhaseType() string {
	if n.Ctx != nil && n.Ctx.PhaseType != "" {
		return strings.ToUpper(n.Ctx.PhaseType)
	}
	return strings.ToUpper(string(n.mode))
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
		"PhaseType":  n.displayPhaseType(),
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
		"Goal":              step.Goal,
		"Actions":           step.Actions,
	}

	return agent.Render("workflow/checkpoint", data)
}

// formatStep renders the step template for the current workflow step.
func (n *Navigator) formatStep(step WorkflowStep, stepState *StepState) (string, error) {
	status := string(stepState.Status)
	feedback := stepState.Feedback

	isGate := n.docFilename == "GATES.md"
	phaseType := n.displayPhaseType()
	if isGate {
		phaseType = phaseType + " PHASE GATE"
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
		"PhaseType":     n.displayPhaseType(),
		"Completed":     n.workflowState.CompletedCount(),
		"Total":         n.workflowState.TotalSteps,
		"CurrentStep":   n.workflowState.CurrentStep,
		"CurrentStatus": currentStatus,
		"Steps":         steps,
	}

	return agent.Render("workflow/progress", data)
}
