// Package shared provides shared utilities for fest commands.
package shared

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// WorkflowStepView represents a workflow step ready for display.
type WorkflowStepView struct {
	Number        int
	Name          string
	Status        wf.StepStatus
	IsCurrent     bool
	HasCheckpoint bool
	Goal          string
	Feedback      string
}

// WorkflowStepIcon returns the styled icon for a step status.
func WorkflowStepIcon(status wf.StepStatus) string {
	switch status {
	case wf.StepStatusCompleted:
		return ui.Success("✓")
	case wf.StepStatusSkipped:
		return ui.Warning("⤼")
	case wf.StepStatusInProgress:
		return ui.ColoredText("●", ui.InProgressColor)
	case wf.StepStatusBlocked:
		return ui.Error("✗")
	case wf.StepStatusPending:
		return ui.Dim("○")
	default:
		return ui.Dim("○")
	}
}

// RenderWorkflowStepLine renders a single workflow step line.
// When compact is true, only shows icon, number, name, and checkpoint indicator.
// When compact is false, also shows goal for current step and feedback for blocked steps.
func RenderWorkflowStepLine(step WorkflowStepView, compact bool) string {
	var sb strings.Builder

	icon := WorkflowStepIcon(step.Status)

	// Current marker
	marker := "  "
	if step.IsCurrent {
		marker = ui.Accent("→ ")
	}

	// Step name - highlight current
	stepName := step.Name
	if step.IsCurrent {
		stepName = ui.Accent(step.Name)
	}

	// Checkpoint indicator
	checkpoint := ""
	if step.HasCheckpoint {
		checkpoint = ui.Warning(" [checkpoint]")
	}

	sb.WriteString(fmt.Sprintf("%s%s Step %d: %s%s\n",
		marker, icon, step.Number, stepName, checkpoint))

	// Show goal if current step (and not compact)
	if !compact && step.IsCurrent && step.Goal != "" {
		sb.WriteString(fmt.Sprintf("     %s: %s\n", ui.Dim("Goal"), step.Goal))
	}

	// Show feedback/note metadata for blocked and skipped steps.
	if !compact && step.Feedback != "" && (step.Status == wf.StepStatusBlocked || step.Status == wf.StepStatusSkipped) {
		label := ui.Error("Feedback")
		if step.Status == wf.StepStatusSkipped {
			label = ui.Warning("Note")
		}
		sb.WriteString(fmt.Sprintf("     %s: %s\n", label, step.Feedback))
	}

	// Keep completed notes visible when present (used by workflow skip --as completed).
	if !compact && step.Feedback != "" && step.Status == wf.StepStatusCompleted {
		sb.WriteString(fmt.Sprintf("     %s: %s\n", ui.Warning("Note"), step.Feedback))
	}

	return sb.String()
}

// RenderWorkflowSteps renders a list of workflow steps.
func RenderWorkflowSteps(steps []WorkflowStepView, compact bool) string {
	var sb strings.Builder
	for _, step := range steps {
		sb.WriteString(RenderWorkflowStepLine(step, compact))
	}
	return sb.String()
}

// RenderWorkflowProgress renders the progress summary line.
func RenderWorkflowProgress(completed, total int) string {
	if total == 0 {
		return ui.Dim("0/0 (0%)")
	}
	percent := float64(completed) / float64(total) * 100
	return fmt.Sprintf("%d/%d (%.0f%%)", completed, total, percent)
}

// LoadWorkflowStepsForPhase loads workflow steps and state for a given phase path.
// Returns a slice of WorkflowStepView ready for rendering.
func LoadWorkflowStepsForPhase(ctx context.Context, festivalPath, phasePath string) ([]WorkflowStepView, error) {
	phaseName := filepath.Base(phasePath)

	// Check for WORKFLOW.md
	workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no WORKFLOW.md found in phase %s", phaseName)
	}

	// Parse workflow steps
	parser := wf.NewParser()
	steps, err := parser.Parse(ctx, workflowPath)
	if err != nil {
		return nil, fmt.Errorf("parsing workflow: %w", err)
	}

	if len(steps) == 0 {
		return []WorkflowStepView{}, nil
	}

	// Load workflow state from Store (JSONL-backed)
	store := progress.NewStore(festivalPath)
	if err := store.Load(ctx); err != nil {
		return nil, fmt.Errorf("loading progress store: %w", err)
	}
	state, ok := store.WorkflowPhaseState(phaseName)
	if !ok {
		state = wf.NewWorkflowState(0)
	}

	// Build view models
	views := make([]WorkflowStepView, len(steps))
	for i, step := range steps {
		stepState := state.GetStepState(step.Number)
		status := wf.StepStatusPending
		feedback := ""
		if stepState != nil {
			status = stepState.Status
			feedback = stepState.Feedback
		}

		views[i] = WorkflowStepView{
			Number:        step.Number,
			Name:          step.Name,
			Status:        status,
			IsCurrent:     step.Number == state.CurrentStep && !state.IsComplete(),
			HasCheckpoint: step.HasCheckpoint(),
			Goal:          step.Goal,
			Feedback:      feedback,
		}
	}

	return views, nil
}

// WorkflowStepCounts returns completed and total counts from step views.
func WorkflowStepCounts(steps []WorkflowStepView) (completed, total int) {
	total = len(steps)
	for _, step := range steps {
		if step.Status == wf.StepStatusCompleted || step.Status == wf.StepStatusSkipped {
			completed++
		}
	}
	return
}

// IsWorkflowPhase returns true if the phase type uses WORKFLOW.md-based navigation.
func IsWorkflowPhase(phaseType string) bool {
	switch phaseType {
	case "ingest", "research", "planning":
		return true
	default:
		return false
	}
}

// IsWorkflowPhaseDir checks if a phase directory is a workflow phase by detecting phase type.
func IsWorkflowPhaseDir(phaseDir string) bool {
	phaseType := guidance.DetectPhaseType(phaseDir)
	return IsWorkflowPhase(phaseType)
}

// HasWorkflowFile checks if a phase directory contains a WORKFLOW.md file.
func HasWorkflowFile(phaseDir string) bool {
	workflowPath := filepath.Join(phaseDir, "WORKFLOW.md")
	_, err := os.Stat(workflowPath)
	return err == nil
}

// WorkflowPositionForPhase reads the workflow_position from a phase's WORKFLOW.md frontmatter.
// Returns "after" (default) if not set or on error.
func WorkflowPositionForPhase(phaseDir string) frontmatter.WorkflowPosition {
	workflowPath := filepath.Join(phaseDir, "WORKFLOW.md")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		return frontmatter.WorkflowPositionAfter
	}
	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		return frontmatter.WorkflowPositionAfter
	}
	if fm.WorkflowPosition == frontmatter.WorkflowPositionBefore {
		return frontmatter.WorkflowPositionBefore
	}
	return frontmatter.WorkflowPositionAfter
}
