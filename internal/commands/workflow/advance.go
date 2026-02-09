package workflow

import (
	"context"
	"fmt"

	"github.com/Obedience-Corp/fest/internal/chaining"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newAdvanceCmd() *cobra.Command {
	var skipFlag bool

	cmd := &cobra.Command{
		Use:   "advance",
		Short: "Complete current step and move to next",
		Long: `Mark the current workflow step as complete and advance to the next step.

This command:
  1. Marks the current step as completed
  2. Advances the workflow to the next step
  3. Saves the updated state

Note: If the current step has a blocking checkpoint, use 'fest workflow approve' instead.`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdvance(cmd.Context(), skipFlag)
		},
	}

	cmd.Flags().BoolVar(&skipFlag, "skip", false, "Skip current step without completing")

	return cmd
}

func runAdvance(ctx context.Context, skip bool) error {
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
	stepState := state.GetCurrentStepState()

	// Handle blocked step
	if stepState != nil && stepState.Status == wf.StepStatusBlocked {
		feedback := ""
		if stepState.Feedback != "" {
			feedback = fmt.Sprintf(": %s", stepState.Feedback)
		}
		return fmt.Errorf("step %d is blocked%s\n\nAddress the feedback and try again, or use 'fest workflow reset' to start over", currentStepNum, feedback)
	}

	// Handle skip
	if skip {
		fmt.Printf("%s Skipping Step %d: %s\n", ui.Warning("⚠"), currentStepNum, step.Name)
		if err := nav.MarkSkipped(ctx, fmt.Sprintf("step_%d", currentStepNum)); err != nil {
			return fmt.Errorf("skipping step: %w", err)
		}
		return showNextStep(ctx, nav, steps)
	}

	// Check for blocking checkpoint
	if step.Checkpoint.IsBlocking() {
		fmt.Printf("%s Step %d: %s — work complete\n", ui.Success("✓"), currentStepNum, step.Name)
		fmt.Printf("%s CHECKPOINT: Awaiting user approval\n\n", ui.Warning("⚠"))
		fmt.Println("When ready to approve: " + ui.Accent("fest workflow approve"))
		fmt.Println("To reject with feedback: " + ui.Accent("fest workflow reject --reason \"...\""))
		return nil
	}

	// Advance to next step
	if err := nav.Advance(ctx); err != nil {
		return fmt.Errorf("advancing workflow: %w", err)
	}

	// Show result
	fmt.Printf("%s Step %d: %s completed\n", ui.Success("✓"), currentStepNum, step.Name)
	return showNextStep(ctx, nav, steps)
}

// showNextStep displays information about the next step or completion message.
func showNextStep(ctx context.Context, nav *wf.Navigator, steps []wf.WorkflowStep) error {
	state := nav.GetWorkflowState()

	if state.IsComplete() {
		fmt.Println()
		fmt.Println(ui.Success("🎉 Workflow complete!"))
		fmt.Println()

		// Check for phase chaining
		gctx := nav.GetContext()
		if gctx.FestivalPath != "" && gctx.PhasePath != "" {
			checkPhaseChaining(ctx, gctx.FestivalPath, gctx.PhasePath)
		}

		fmt.Println("Run " + ui.Accent("fest status") + " to view final state.")
		return nil
	}

	currentStepNum := state.CurrentStep
	if currentStepNum < 1 || currentStepNum > len(steps) {
		return nil
	}

	nextStep := steps[currentStepNum-1]
	fmt.Printf("→ Now on Step %d: %s\n", nextStep.Number, ui.Accent(nextStep.Name))

	if nextStep.Goal != "" {
		fmt.Printf("  %s: %s\n", ui.Dim("Goal"), nextStep.Goal)
	}

	if nextStep.HasCheckpoint() {
		fmt.Printf("  %s\n", ui.Warning("[checkpoint required]"))
	}

	fmt.Println()
	fmt.Println("Run " + ui.Accent("fest next") + " for step details.")

	return nil
}

// checkPhaseChaining checks if a completed phase should trigger creation of the next phase.
func checkPhaseChaining(ctx context.Context, festivalPath, phasePath string) {
	ch := chaining.NewChainer(festivalPath)

	target, err := ch.CheckChaining(ctx, phasePath)
	if err != nil {
		fmt.Printf("%s Phase chaining check failed: %v\n", ui.Warning("!"), err)
		return
	}

	if target == nil {
		return
	}

	fmt.Printf("%s Phase chaining: creating %s phase...\n", ui.Accent("->"), target.PendingPhase.Name)

	result, err := ch.ExecuteChain(ctx, target)
	if err != nil {
		fmt.Printf("%s Failed to create chained phase: %v\n", ui.Warning("!"), err)
		fmt.Printf("   You can create it manually: %s\n", ui.Accent(
			fmt.Sprintf("fest create phase --name %s --type %s", target.PendingPhase.Name, target.PendingPhase.Type),
		))
		return
	}

	fmt.Printf("%s Created phase: %s (type: %s)\n", ui.Success("->"), result.PhaseName, result.PhaseType)
	fmt.Println()
	fmt.Println("Run " + ui.Accent("fest next") + " to continue with the new phase.")
	fmt.Println()
}
