package workflow

import (
	"context"
	"fmt"

	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newRejectCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "reject",
		Short: "Reject checkpoint with feedback",
		Long: `Reject a blocking checkpoint and provide feedback.

When a step's work doesn't meet requirements, use this command
to reject and request revisions.

The feedback will be recorded in the workflow state for reference.`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reason == "" {
				return fmt.Errorf("--reason is required\n\nUsage: fest workflow reject --reason \"your feedback here\"")
			}
			return runReject(cmd.Context(), reason)
		},
	}

	cmd.Flags().StringVarP(&reason, "reason", "r", "", "reason for rejection (required)")
	_ = cmd.MarkFlagRequired("reason")

	return cmd
}

func runReject(ctx context.Context, reason string) error {
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}

	state := nav.GetWorkflowState()
	steps := nav.GetSteps()

	// Check if already complete
	if state.IsComplete() {
		return fmt.Errorf("workflow is already complete")
	}

	// Get current step info
	currentStepNum := state.CurrentStep
	if currentStepNum < 1 || currentStepNum > len(steps) {
		return fmt.Errorf("invalid workflow state: current step %d out of range", currentStepNum)
	}

	step := steps[currentStepNum-1]

	// Verify this is a checkpoint step
	if !step.Checkpoint.IsBlocking() {
		return fmt.Errorf("step %d does not have a blocking checkpoint\n\nReject is only for checkpoint steps", currentStepNum)
	}

	// Reject with feedback
	if err := nav.Reject(ctx, reason); err != nil {
		return fmt.Errorf("rejecting checkpoint: %w", err)
	}

	fmt.Printf("%s Step %d: %s rejected\n", ui.Warning("⚠"), currentStepNum, step.Name)
	fmt.Printf("  %s: %s\n\n", ui.Label("Feedback"), reason)
	fmt.Println("The step is now blocked. Address the feedback and revise the work.")
	fmt.Println("When ready, run " + ui.Accent("fest workflow advance") + " to resubmit.")

	return nil
}
