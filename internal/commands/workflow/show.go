package workflow

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [step]",
		Short: "Display current step details",
		Long: `Display detailed information about the current workflow step.

If a step number is provided, shows that specific step.
Otherwise, shows the current step.

Shows:
  - Step number and name
  - Goal/objective
  - Actions to complete
  - Expected output
  - Checkpoint type if applicable`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stepNum := 0
			if len(args) > 0 {
				var err error
				stepNum, err = strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("invalid step number: %s", args[0])
				}
			}
			return runShow(cmd.Context(), stepNum)
		},
	}
}

func runShow(ctx context.Context, stepNum int) error {
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

	// Default to current step
	if stepNum == 0 {
		stepNum = state.CurrentStep
	}

	// Validate step number
	if stepNum < 1 || stepNum > len(steps) {
		return fmt.Errorf("step %d not found (workflow has %d steps)", stepNum, len(steps))
	}

	step := steps[stepNum-1]
	stepState := state.GetStepState(stepNum)

	// Build output
	var sb strings.Builder

	// Header
	isCurrent := stepNum == state.CurrentStep && !state.IsComplete()
	if isCurrent {
		sb.WriteString(ui.Accent("→ "))
	}
	sb.WriteString(ui.Category(fmt.Sprintf("Step %d: %s", step.Number, step.Name)))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", 40))
	sb.WriteString("\n\n")

	// Status
	sb.WriteString(ui.Label("Status: "))
	if stepState != nil {
		sb.WriteString(formatStepStatus(stepState.Status))
	} else {
		sb.WriteString(formatStepStatus(wf.StepStatusPending))
	}
	sb.WriteString("\n\n")

	// Goal
	if step.Goal != "" {
		sb.WriteString(ui.Label("Goal:"))
		sb.WriteString("\n  ")
		sb.WriteString(step.Goal)
		sb.WriteString("\n\n")
	}

	// Actions
	if len(step.Actions) > 0 {
		sb.WriteString(ui.Label("Actions:"))
		sb.WriteString("\n")
		for i, action := range step.Actions {
			fmt.Fprintf(&sb, "  %d. %s\n", i+1, action)
		}
		sb.WriteString("\n")
	}

	// Output
	if step.Output != "" {
		sb.WriteString(ui.Label("Expected Output:"))
		sb.WriteString("\n  ")
		sb.WriteString(step.Output)
		sb.WriteString("\n\n")
	}

	// Checkpoint
	if step.HasCheckpoint() {
		sb.WriteString(ui.Label("Checkpoint: "))
		if step.Checkpoint.IsBlocking() {
			sb.WriteString(ui.Warning("⚠ Approval Required"))
		} else {
			sb.WriteString(string(step.Checkpoint))
		}
		sb.WriteString("\n\n")
	}

	// Show feedback/note metadata for blocked and skipped/completed-with-note states.
	if stepState != nil && stepState.Feedback != "" {
		label := ui.Error("Rejection Feedback:")
		if stepState.Status == wf.StepStatusSkipped || stepState.Status == wf.StepStatusCompleted {
			label = ui.Warning("Operator Note:")
		}
		sb.WriteString(label)
		sb.WriteString("\n  ")
		sb.WriteString(stepState.Feedback)
		sb.WriteString("\n\n")
	}

	// Navigation hint
	if isCurrent {
		sb.WriteString(ui.Dim("When complete: "))
		if step.Checkpoint.IsBlocking() {
			sb.WriteString(ui.Accent("fest workflow approve"))
		} else {
			sb.WriteString(ui.Accent("fest workflow advance"))
		}
		sb.WriteString("\n")
	}

	fmt.Print(sb.String())
	return nil
}

// formatStepStatus returns a styled status string.
func formatStepStatus(status wf.StepStatus) string {
	switch status {
	case wf.StepStatusCompleted:
		return ui.Success("completed")
	case wf.StepStatusSkipped:
		return ui.Warning("skipped")
	case wf.StepStatusInProgress:
		return ui.ColoredText("in progress", ui.InProgressColor)
	case wf.StepStatusBlocked:
		return ui.Error("blocked")
	case wf.StepStatusPending:
		return ui.Dim("pending")
	default:
		return string(status)
	}
}
