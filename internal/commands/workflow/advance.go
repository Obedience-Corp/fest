package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/chaining"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
	"github.com/spf13/cobra"
)

// runAdvanceBootstrap converts an anonymous WORKFLOW.md into tracked mode
// (D007 — fest writes only .workflow/, never .workitem). After bootstrap it
// appends a start+done pair so step 1 reads as completed.
//
// Step 1's checkpoint is honored with the same fail-closed semantics as
// runAdvanceTracked: a blocking checkpoint on step 1 cannot be auto-approved
// by bootstrap, since standalone mode lacks approve/reject/reset. The check
// runs before EnsureTracked so .workflow/ is never created when bootstrap
// would refuse to mutate.
func runAdvanceBootstrap(ctx context.Context, res *standalone.Result) error {
	steps, parseErr := wf.NewParser().Parse(ctx, res.WorkflowDoc)
	if parseErr != nil {
		return festerrors.Wrap(parseErr, "parsing WORKFLOW.md")
	}
	if len(steps) == 0 {
		return festerrors.Validation("WORKFLOW.md has no parseable steps").
			WithField("path", res.WorkflowDoc)
	}
	if steps[0].Checkpoint.IsBlocking() {
		return festerrors.New("standalone workflows do not support blocking checkpoints").
			WithField("step", 1).
			WithField("step_name", steps[0].Name).
			WithHint("remove the blocking checkpoint from step 1 in WORKFLOW.md, or run this workflow inside a festival phase where 'fest workflow approve' is available")
	}
	if _, err := EnsureTracked(ctx, res, BootstrapOptions{}); err != nil {
		return err
	}
	store := localstore.Open(filepath.Join(filepath.Dir(res.WorkflowDoc), ".workflow"), res.WorkflowDoc)
	if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepStart}); err != nil {
		return err
	}
	if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepDone}); err != nil {
		return err
	}
	fmt.Println(ui.Success("✓ Bootstrapped tracked workflow and advanced step 1"))
	fmt.Println("next: " + ui.Accent("fest next"))
	return nil
}

// runAdvanceTracked advances a standalone tracked workflow by one step. It
// honors blocked state and blocking checkpoints with the same semantics as
// the festival navigator. Closes D030 finding #5 — previously this code path
// fell through to getWorkflowNavigator and failed.
//
// Step selection mirrors pickRenderStep so `fest workflow advance` operates
// on the same step `fest next` shows. After bootstrap (start+done for step 1)
// state.CurrentStep == state.CompletedSteps == 1, and the actionable step is
// state.CompletedSteps + 1. When a step is in flight (CurrentStep > CompletedSteps)
// only a Done event is appended; otherwise a Start+Done pair is appended for
// the next unstarted step.
func runAdvanceTracked(ctx context.Context, res *standalone.Result) error {
	store := localstore.Open(res.RuntimeDir, res.WorkflowDoc)
	state, err := store.LoadActive(ctx)
	if err != nil {
		return err
	}
	if state == nil {
		return festerrors.NotFound("no active run").
			WithHint("run 'fest workflow start' first")
	}
	if state.Blocked {
		return festerrors.New("standalone workflow step is blocked").
			WithField("step", state.CurrentStep).
			WithHint("standalone workflows do not support approve/reset yet; remove the blocking checkpoint from WORKFLOW.md or run inside a festival phase where approval commands are available")
	}

	steps, parseErr := wf.NewParser().Parse(ctx, res.WorkflowDoc)
	if parseErr != nil {
		return festerrors.Wrap(parseErr, "parsing WORKFLOW.md")
	}

	actionable := state.CurrentStep
	needStart := false
	if state.CurrentStep <= state.CompletedSteps {
		actionable = state.CompletedSteps + 1
		needStart = true
	}

	if actionable > len(steps) {
		if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventWorkflowRunCompleted}); err != nil {
			return err
		}
		fmt.Println(ui.Success("🎉 Workflow complete!"))
		return nil
	}
	if actionable < 1 {
		return festerrors.New("invalid step index").
			WithField("actionable_step", actionable).
			WithField("total_steps", len(steps))
	}
	step := steps[actionable-1]

	if step.Checkpoint.IsBlocking() {
		// Standalone mode does not yet implement approve/reject/reset for the
		// .workflow/ event stream — those commands are festival-scoped. Rather
		// than appending wf_step_start and routing the user toward a command
		// that cannot mutate this runtime, refuse the advance up-front and
		// keep .workflow/ state consistent. No event is written, so LoadActive
		// continues to report the run as advanceable once the WORKFLOW.md is
		// adjusted.
		return festerrors.New("standalone workflows do not support blocking checkpoints").
			WithField("step", actionable).
			WithField("step_name", step.Name).
			WithHint("remove the blocking checkpoint from this step in WORKFLOW.md, or run this workflow inside a festival phase where 'fest workflow approve' is available")
	}

	if needStart {
		if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepStart}); err != nil {
			return err
		}
	}
	if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepDone}); err != nil {
		return err
	}
	fmt.Printf("%s Step %d: %s completed\n", ui.Success("✓"), actionable, step.Name)

	if actionable >= len(steps) {
		if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventWorkflowRunCompleted}); err != nil {
			return err
		}
		fmt.Println(ui.Success("🎉 Workflow complete!"))
		return nil
	}

	fmt.Println("next: " + ui.Accent("fest next"))
	return nil
}

func newAdvanceCmd() *cobra.Command {
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
			// scope.Global so runAdvance can route to anonymous-bootstrap
			// or tracked standalone runtimes outside a festival.
			"scope": string(scope.Global),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdvance(cmd.Context())
		},
	}

	return cmd
}

func runAdvance(ctx context.Context) error {
	cwd, _ := os.Getwd()
	if res, resErr := standalone.Resolve(ctx, cwd); resErr == nil {
		switch res.Mode {
		case standalone.ModeAnonymous:
			return runAdvanceBootstrap(ctx, res)
		case standalone.ModeTracked:
			return runAdvanceTracked(ctx, res)
		}
		// ModeFestival or ModeNone: fall through to festival navigator.
	}

	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}

	if err := lifecycle.EnforcePreActive(ctx, nav.Ctx.FestivalPath, lifecycle.EnforceOptions{
		PhasePath: nav.Ctx.PhasePath,
		Reason:    "fest workflow advance",
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

		// Propagate phase completion to PHASE_GOAL.md frontmatter
		gctx := nav.GetContext()
		if gctx.FestivalPath != "" && gctx.PhasePath != "" {
			if mgr, mgrErr := progress.NewManagerWithGate(ctx, gctx.FestivalPath,
				lifecycle.NewGateWithReason(gctx.FestivalPath, "fest workflow advance")); mgrErr != nil {
				fmt.Printf("%s %s\n", ui.Dim("Warning: could not initialize progress manager:"), ui.Dim(mgrErr.Error()))
			} else {
				if propErr := mgr.PropagatePhaseCompletion(ctx, gctx.PhasePath); propErr != nil {
					fmt.Printf("%s %s\n", ui.Dim("Warning: could not propagate phase completion:"), ui.Dim(propErr.Error()))
				}
			}
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
