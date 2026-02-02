package workflow

import (
	"context"
	"fmt"

	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve",
		Short: "Approve a blocking checkpoint",
		Long: `Approve a blocking checkpoint and proceed to the next step.

Some workflow steps require explicit user approval before proceeding.
This is typically used for review gates or major decision points.

After approval:
  - The current step is marked as approved
  - The workflow advances to the next step`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApprove(cmd.Context())
		},
	}
}

func runApprove(ctx context.Context) error {
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}

	state := nav.GetWorkflowState()
	steps := nav.GetSteps()

	// Check if already complete
	if state.IsComplete() {
		fmt.Println(ui.Success("✓ Workflow already complete!"))
		return nil
	}

	// Get current step info
	currentStepNum := state.CurrentStep
	if currentStepNum < 1 || currentStepNum > len(steps) {
		return fmt.Errorf("invalid workflow state: current step %d out of range", currentStepNum)
	}

	step := steps[currentStepNum-1]

	// Verify this is a checkpoint step
	if !step.Checkpoint.IsBlocking() {
		return fmt.Errorf("step %d does not have a blocking checkpoint\n\nUse 'fest workflow advance' for regular steps", currentStepNum)
	}

	// Approve and advance
	if err := nav.Approve(ctx); err != nil {
		return fmt.Errorf("approving checkpoint: %w", err)
	}

	fmt.Printf("%s Step %d: %s approved\n", ui.Success("✓"), currentStepNum, step.Name)
	return showNextStep(nav, steps)
}
