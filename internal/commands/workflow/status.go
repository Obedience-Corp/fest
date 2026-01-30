// Package workflow provides commands for managing workflow-based phase execution.
package workflow

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show workflow progress",
		Long: `Display the current progress of the workflow in this phase.

Shows:
  - Current step number and name
  - Completed steps
  - Remaining steps
  - Checkpoint status if applicable`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd.Context())
		},
	}
}

func runStatus(ctx context.Context) error {
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}

	steps := nav.GetSteps()
	state := nav.GetWorkflowState()

	// Handle empty workflow
	if len(steps) == 0 {
		fmt.Println("No workflow steps defined.")
		return nil
	}

	// Build output
	var sb strings.Builder

	// Header
	sb.WriteString(ui.Category("Workflow Status"))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", 40))
	sb.WriteString("\n\n")

	// Current step summary
	sb.WriteString(ui.Label("Current Step: "))
	if state.IsComplete() {
		sb.WriteString(ui.Success("Complete"))
	} else {
		sb.WriteString(fmt.Sprintf("%d of %d", state.CurrentStep, state.TotalSteps))
	}
	sb.WriteString("\n\n")

	// Steps list
	sb.WriteString(ui.Label("Steps:"))
	sb.WriteString("\n")

	for _, step := range steps {
		stepState := state.GetStepState(step.Number)
		status := wf.StepStatusPending
		if stepState != nil {
			status = stepState.Status
		}

		// Build step line
		icon := statusIcon(status)
		isCurrent := step.Number == state.CurrentStep && !state.IsComplete()

		// Step name - highlight current
		stepName := step.Name
		if isCurrent {
			stepName = ui.Accent(step.Name)
		}

		// Checkpoint indicator
		checkpoint := ""
		if step.HasCheckpoint() {
			checkpoint = ui.Warning(" [checkpoint]")
		}

		// Current marker
		marker := "  "
		if isCurrent {
			marker = ui.Accent("→ ")
		}

		sb.WriteString(fmt.Sprintf("%s%s Step %d: %s%s\n",
			marker, icon, step.Number, stepName, checkpoint))

		// Show goal if current step
		if isCurrent && step.Goal != "" {
			sb.WriteString(fmt.Sprintf("     %s: %s\n", ui.Dim("Goal"), step.Goal))
		}

		// Show rejection feedback if blocked
		if stepState != nil && stepState.Status == wf.StepStatusBlocked && stepState.Feedback != "" {
			sb.WriteString(fmt.Sprintf("     %s: %s\n", ui.Error("Feedback"), stepState.Feedback))
		}
	}

	// Progress summary
	sb.WriteString("\n")
	completed := state.CompletedCount()
	percent := state.ProgressPercent()
	sb.WriteString(ui.Label("Progress: "))
	sb.WriteString(fmt.Sprintf("%d/%d (%.0f%%)\n", completed, state.TotalSteps, percent))

	// Completion status
	if state.IsComplete() {
		sb.WriteString("\n")
		sb.WriteString(ui.Success("✓ All steps complete"))
		sb.WriteString("\n")
	}

	fmt.Print(sb.String())
	return nil
}

// statusIcon returns a styled icon for the given step status.
func statusIcon(status wf.StepStatus) string {
	switch status {
	case wf.StepStatusCompleted:
		return ui.Success("✓")
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

// getWorkflowNavigator creates and initializes a workflow navigator for the current context.
// This is a shared helper used by all workflow commands.
func getWorkflowNavigator(ctx context.Context) (*wf.Navigator, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting current directory: %w", err)
	}

	// Resolve festival path from scope context or detection
	festivalPath, ok := scope.FestivalFrom(ctx)
	if !ok {
		// Fall back to manual detection
		festivalPath, err = shared.ResolveFestivalPath(cwd, "")
		if err != nil {
			return nil, fmt.Errorf("not in a festival: %w", err)
		}
	}

	// Resolve phase path
	phasePath := shared.ResolvePhasePath(cwd, festivalPath)
	if phasePath == "" {
		return nil, fmt.Errorf("not inside a phase directory")
	}

	// Detect phase type
	phaseType := guidance.DetectPhaseType(phasePath)
	if !isWorkflowPhase(phaseType) {
		return nil, fmt.Errorf("not in a workflow-based phase (current: %s)\n\nWorkflow commands only work in ingest, research, or planning phases", phaseType)
	}

	// Create guidance context
	gctx := &guidance.GuidanceContext{
		FestivalPath: festivalPath,
		PhasePath:    phasePath,
		PhaseType:    phaseType,
		Mode:         guidance.ModeFromPhaseType(phaseType),
		Config:       guidance.DefaultConfig(),
	}

	// Create workflow navigator
	nav, err := wf.NewNavigator(gctx, gctx.Mode)
	if err != nil {
		return nil, fmt.Errorf("creating navigator: %w", err)
	}

	// Initialize the navigator
	if err := nav.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("initializing navigator: %w", err)
	}

	return nav, nil
}

// isWorkflowPhase returns true if the phase type uses WORKFLOW.md-based navigation.
func isWorkflowPhase(phaseType string) bool {
	switch phaseType {
	case "ingest", "research", "planning":
		return true
	default:
		return false
	}
}
