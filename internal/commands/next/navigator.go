// Package next provides the fest next command for task navigation.
package next

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	wfcmd "github.com/Obedience-Corp/fest/internal/commands/workflow"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// runNavigatorMode uses the guidance navigator for mode-aware output.
func runNavigatorMode(ctx context.Context, cwd, festivalPath string) error {
	// Validate mode flag if provided
	var modeOverride guidance.Mode
	if modeFlag != "" {
		modeOverride = guidance.Mode(modeFlag)
		if !modeOverride.IsValid() {
			validModes := []string{}
			for _, m := range guidance.AllModes() {
				validModes = append(validModes, string(m))
			}
			return errors.Validation("invalid mode").
				WithField("mode", modeFlag).
				WithField("valid_modes", validModes)
		}
	}

	// Detect if we're within a phase directory
	phasePath := shared.ResolvePhasePath(cwd, festivalPath)

	// Create navigator via factory
	var nav guidance.Navigator
	var err error
	if phasePath != "" {
		// Within a phase - use NewNavigatorForPath for phase-type-aware navigation
		nav, err = guidance.NewNavigatorForPath(ctx, festivalPath, phasePath, guidance.DefaultConfig())
		if err != nil {
			return errors.Wrap(err, "creating navigator for phase").
				WithField("phase_path", phasePath)
		}
	} else {
		// At festival root - use NewNavigator with default execution mode
		mode := guidance.ModeImplementation
		if modeOverride != "" {
			mode = modeOverride
		}
		gctx := &guidance.GuidanceContext{
			FestivalPath: festivalPath,
			FestivalName: filepath.Base(festivalPath),
			Mode:         mode,
			Config:       guidance.DefaultConfig(),
		}
		nav, err = guidance.NewNavigator(ctx, gctx)
		if err != nil {
			return errors.Wrap(err, "creating navigator")
		}
	}

	// Apply mode override if flag was provided (overrides auto-detection)
	if modeOverride != "" && nav.GetContext().Mode != modeOverride {
		gctx := nav.GetContext().WithMode(modeOverride)
		nav, err = guidance.NewNavigator(ctx, gctx)
		if err != nil {
			return errors.Wrap(err, "creating navigator with mode override").
				WithField("mode", modeOverride)
		}
	}

	// Initialize the navigator
	if err := nav.Initialize(ctx); err != nil {
		return errors.Wrap(err, "initializing navigator")
	}

	// Use Navigator.FormatInstructions() for output
	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		return errors.Wrap(err, "formatting instructions")
	}
	fmt.Print(instructions)
	printFeedbackReminder(ctx, festivalPath)
	return nil
}

// runWorkflowMode uses the workflow navigator for WORKFLOW.md-based navigation.
func runWorkflowMode(ctx context.Context, festivalPath, phasePath string) error {
	return runWorkflowModeWithStore(ctx, festivalPath, phasePath, nil)
}

func runWorkflowModeWithStore(ctx context.Context, festivalPath, phasePath string, store *progress.Store) error {
	// Create guidance context for workflow navigation
	gctx := &guidance.GuidanceContext{
		FestivalPath: festivalPath,
		FestivalName: filepath.Base(festivalPath),
		PhasePath:    phasePath,
		PhaseName:    filepath.Base(phasePath),
		PhaseType:    guidance.DetectPhaseType(phasePath),
		Mode:         guidance.ModeWorkflow,
		Config:       guidance.DefaultConfig(),
	}

	// Create workflow navigator
	nav, err := guidance.NewNavigator(ctx, gctx)
	if err != nil {
		return errors.Wrap(err, "creating workflow navigator").
			WithField("phase_path", phasePath)
	}

	// Create and load progress Store when the caller has not already loaded it,
	// then inject it into the workflow navigator for JSONL-backed state.
	if store == nil {
		store = progress.NewStore(festivalPath)
		if err := store.Load(ctx); err != nil {
			return errors.Wrap(err, "loading progress store")
		}
	}
	if wfNav, ok := nav.(*wf.Navigator); ok {
		wfNav.SetStateStore(store)
	}

	// Initialize the navigator
	if err := nav.Initialize(ctx); err != nil {
		return errors.Wrap(err, "initializing workflow navigator")
	}

	// Operator opt-in via hooks.approval_judge: auto-run the judge on
	// consecutive blocking checkpoints so fest next does not wait for a human.
	if wfNav, ok := nav.(*wf.Navigator); ok {
		if err := wfcmd.AutoDelegateBlockingCheckpoints(ctx, wfNav); err != nil {
			return err
		}
	}

	// Check if we got a valid next step
	nextStep, err := nav.GetNext(ctx)
	if err != nil {
		return errors.Wrap(err, "getting next workflow step")
	}

	// If no next step and not complete, WORKFLOW.md may be malformed
	if nextStep == nil {
		progress, _ := nav.GetProgress(ctx)
		if progress != nil && progress.Completed == progress.Total && progress.Total > 0 {
			// Workflow is complete
			fmt.Println("WORKFLOW COMPLETE")
			fmt.Println("─────────────────")
			fmt.Printf("All %d steps have been completed.\n", progress.Total)
			return nil
		}

		// No steps found - likely malformed WORKFLOW.md
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		return errors.Validation("WORKFLOW.md has no valid steps").
			WithField("path", workflowPath).
			WithHint("WORKFLOW.md must contain step headers in format:\n" +
				"  ## Step 1: STEP_NAME\n" +
				"  **Goal:** Description of what this step achieves\n" +
				"  **Actions:**\n" +
				"  1. First action\n" +
				"  2. Second action\n" +
				"  **Output:** Expected output from this step")
	}

	// Use Navigator.FormatInstructions() for output
	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		return errors.Wrap(err, "formatting workflow instructions")
	}
	fmt.Print(instructions)
	printFeedbackReminder(ctx, festivalPath)
	return nil
}

// runPhaseGateMode uses the workflow navigator configured for GATES.md navigation.
func runPhaseGateMode(ctx context.Context, festivalPath, phasePath string) error {
	// Create guidance context for phase gate navigation
	gctx := &guidance.GuidanceContext{
		FestivalPath: festivalPath,
		FestivalName: filepath.Base(festivalPath),
		PhasePath:    phasePath,
		PhaseName:    filepath.Base(phasePath),
		PhaseType:    guidance.DetectPhaseType(phasePath),
		Mode:         guidance.ModeWorkflow,
		Config:       guidance.DefaultConfig(),
	}

	// Create workflow navigator configured for GATES.md
	nav, err := guidance.NewNavigator(ctx, gctx)
	if err != nil {
		return errors.Wrap(err, "creating phase gate navigator").
			WithField("phase_path", phasePath)
	}

	// Configure for GATES.md with gate: state prefix
	if wfNav, ok := nav.(*wf.Navigator); ok {
		wfNav.SetDocFilename("GATES.md")
		wfNav.SetStateKeyPrefix("gate:")
	}

	// Create and load progress Store, inject into navigator
	store := progress.NewStore(festivalPath)
	if err := store.Load(ctx); err != nil {
		return errors.Wrap(err, "loading progress store")
	}
	if wfNav, ok := nav.(*wf.Navigator); ok {
		wfNav.SetStateStore(store)
	}

	// Initialize the navigator
	if err := nav.Initialize(ctx); err != nil {
		return errors.Wrap(err, "initializing phase gate navigator")
	}

	// Operator opt-in via hooks.approval_judge: auto-run the judge on
	// consecutive blocking gate steps so fest next does not wait for a human.
	if wfNav, ok := nav.(*wf.Navigator); ok {
		if err := wfcmd.AutoDelegateBlockingCheckpoints(ctx, wfNav); err != nil {
			return err
		}
	}

	// Check if we got a valid next step
	nextStep, err := nav.GetNext(ctx)
	if err != nil {
		return errors.Wrap(err, "getting next phase gate step")
	}

	// If no next step, gate may be complete or malformed
	if nextStep == nil {
		gateProgress, _ := nav.GetProgress(ctx)
		if gateProgress != nil && gateProgress.Completed == gateProgress.Total && gateProgress.Total > 0 {
			fmt.Println("PHASE GATE COMPLETE")
			fmt.Println("───────────────────")
			fmt.Printf("All %d gate steps have been completed.\n", gateProgress.Total)
			return nil
		}

		gatesPath := filepath.Join(phasePath, "GATES.md")
		return errors.Validation("GATES.md has no valid steps").
			WithField("path", gatesPath).
			WithHint("GATES.md must contain step headers in format:\n" +
				"  ## Step 1: STEP_NAME\n" +
				"  **Question:** What to verify\n" +
				"  **Checkpoint:** APPROVAL REQUIRED")
	}

	// Format and display instructions
	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		return errors.Wrap(err, "formatting phase gate instructions")
	}
	fmt.Print(instructions)
	printFeedbackReminder(ctx, festivalPath)
	return nil
}
