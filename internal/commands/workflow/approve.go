package workflow

import (
	"context"
	"fmt"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newApproveCmd() *cobra.Command {
	var actor string
	var summary string

	cmd := &cobra.Command{
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
			decision, err := normalizeDecision("approval", actor, summary, "")
			if err != nil {
				return err
			}
			return runApproveWithDecision(cmd.Context(), decision)
		},
	}

	cmd.Flags().StringVar(&actor, "as", decisionActorUser, "decision actor: user or agent")
	cmd.Flags().StringVar(&summary, "summary", "", "approval summary or rationale")

	return cmd
}

func runApprove(ctx context.Context) error {
	return runApproveWithDecision(ctx, wf.DecisionMetadata{Actor: decisionActorUser})
}

func runApproveWithDecision(ctx context.Context, decision wf.DecisionMetadata) error {
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}

	if err := lifecycle.EnforcePreActive(ctx, nav.Ctx.FestivalPath, lifecycle.EnforceOptions{
		PhasePath: nav.Ctx.PhasePath,
		Reason:    "fest workflow approve",
	}); err != nil {
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
		return festerrors.Validation("invalid workflow state").
			WithField("current_step", currentStepNum).
			WithField("total_steps", len(steps))
	}

	step := steps[currentStepNum-1]

	// Verify this is a checkpoint step
	if !step.Checkpoint.IsBlocking() {
		return festerrors.Validation("step does not have a blocking checkpoint").
			WithField("step", currentStepNum).
			WithHint("Use 'fest workflow advance' for regular steps")
	}

	// Approve and advance
	if err := nav.ApproveWithDecision(ctx, decision); err != nil {
		return festerrors.Wrap(err, "approving checkpoint")
	}

	fmt.Printf("%s Step %d: %s approved\n", ui.Success("✓"), currentStepNum, step.Name)
	if decision.Actor != "" {
		fmt.Printf("  %s: %s\n", ui.Label("Approved by"), decision.Actor)
	}
	if decision.Summary != "" {
		fmt.Printf("  %s: %s\n", ui.Label("Summary"), decision.Summary)
	}
	return showNextStep(ctx, nav, steps)
}
