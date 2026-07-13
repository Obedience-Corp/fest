// Package workflow provides commands for managing workflow-based phase execution.
package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show workflow progress",
		Long: `Display the current progress of the workflow in this phase.

Shows:
  - Current step number and name
  - Completed steps
  - Remaining steps
  - Checkpoint status if applicable

Use --json for a stable machine-readable snapshot (schema fest.workflow.status/v1)
that consumers can read without parsing the human-readable output.`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd.Context(), jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	return cmd
}

func runStatus(ctx context.Context, jsonOutput bool) error {
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}

	if jsonOutput {
		out, err := renderWorkflowStatusJSON(nav)
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	}

	steps := nav.GetSteps()
	state := nav.GetWorkflowState()

	// Handle empty workflow
	if len(steps) == 0 {
		fmt.Println("No workflow steps defined.")
		return nil
	}

	// Convert to shared view format
	views := make([]shared.WorkflowStepView, len(steps))
	failedRemediation := make([]*wf.StepState, len(steps))
	for i, step := range steps {
		stepState := state.GetStepState(step.Number)
		status := wf.StepStatusPending
		feedback := ""
		var judge *wf.JudgeState
		if stepState != nil {
			status = stepState.Status
			feedback = stepState.Feedback
			judge = stepState.Judge
			if stepState.Status == wf.StepStatusFailedRemediation {
				failedRemediation[i] = stepState
			}
		}

		views[i] = shared.WorkflowStepView{
			Number:        step.Number,
			Name:          step.Name,
			Status:        status,
			IsCurrent:     step.Number == state.CurrentStep && !state.IsComplete(),
			HasCheckpoint: step.HasCheckpoint(),
			Goal:          step.Goal,
			Feedback:      feedback,
			Judge:         judge,
		}
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
		fmt.Fprintf(&sb, "%d of %d", state.CurrentStep, state.TotalSteps)
	}
	sb.WriteString("\n\n")

	// Steps list using shared renderer
	sb.WriteString(ui.Label("Steps:"))
	sb.WriteString("\n")
	sb.WriteString(shared.RenderWorkflowSteps(views, false))

	// Progress summary
	sb.WriteString("\n")
	sb.WriteString(ui.Label("Progress: "))
	sb.WriteString(shared.RenderWorkflowProgress(state.CompletedCount(), state.TotalSteps))
	sb.WriteString("\n")

	// Completion status
	if state.IsComplete() {
		sb.WriteString("\n")
		sb.WriteString(ui.Success("✓ All steps complete"))
		sb.WriteString("\n")
	}

	for i, ss := range failedRemediation {
		if ss == nil {
			continue
		}
		sb.WriteString("\n")
		sb.WriteString(ui.Warning("⚠ Failed gate (remediation linked)"))
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "  Step %d: %s\n", steps[i].Number, steps[i].Name)
		if ss.Feedback != "" {
			fmt.Fprintf(&sb, "  Reason: %s\n", ss.Feedback)
		}
		if ss.RemediationPhase != "" {
			fmt.Fprintf(&sb, "  Remediation phase: %s\n", ss.RemediationPhase)
		}
		sb.WriteString("  Run 'fest next' to advance into remediation or recheck the gate.\n")
	}

	fmt.Print(sb.String())
	return nil
}

// getWorkflowNavigator creates and initializes a workflow navigator for the current context.
// This is a shared helper used by all workflow commands.
// It supports:
//   - --phase flag to specify a particular phase
//   - Running from inside a phase directory
//   - Running from festival root (auto-detects first incomplete navigable phase)
//   - Automatic gate mode: when a phase's WORKFLOW.md is complete (or absent) and GATES.md
//     is incomplete, the navigator targets GATES.md instead
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
	if err := lifecycle.EnforceRitualRun(ctx, festivalPath, "fest workflow"); err != nil {
		return nil, err
	}

	var phasePath string

	// Priority 1: --phase flag
	if phaseFlag != "" {
		phasePath = filepath.Join(festivalPath, phaseFlag)
		if _, err := os.Stat(phasePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("phase directory not found: %s", phaseFlag)
		}
	} else {
		// Priority 2: Check if we're inside a phase directory
		phasePath = shared.ResolvePhasePath(cwd, festivalPath)

		// Priority 3: Auto-detect first incomplete navigable phase (workflow or gate)
		if phasePath == "" {
			detected, err := findFirstIncompleteNavigablePhase(ctx, festivalPath)
			if err != nil {
				return nil, fmt.Errorf("scanning for navigable phases: %w", err)
			}
			if detected == "" {
				phases, _ := findAllNavigablePhases(festivalPath)
				if len(phases) == 0 {
					return nil, fmt.Errorf("no navigable phases found in this festival\n\nWorkflow commands work with phases that have a WORKFLOW.md or GATES.md file")
				}
				return nil, fmt.Errorf("all navigable phases are complete\n\nUse --phase to specify a particular phase, or 'fest workflow reset --phase <name>' to restart one")
			}
			phasePath = detected
		}
	}

	// Detect phase type and determine navigation mode (workflow vs gate)
	phaseType := guidance.DetectPhaseType(phasePath)
	docFilename, stateKeyPrefix, notReadyMsg := resolveNavigationMode(ctx, festivalPath, phasePath)

	if docFilename == "" {
		if notReadyMsg != "" {
			return nil, fmt.Errorf("%s", notReadyMsg)
		}
		return nil, fmt.Errorf("phase has no WORKFLOW.md or GATES.md (detected type: %s)\n\nWorkflow commands require a navigable document", phaseType)
	}

	// Create guidance context
	gctx := &guidance.GuidanceContext{
		FestivalPath: festivalPath,
		PhasePath:    phasePath,
		PhaseName:    filepath.Base(phasePath),
		PhaseType:    phaseType,
		Mode:         guidance.ModeWorkflow,
		Config:       guidance.DefaultConfig(),
	}

	// Create workflow navigator
	nav, err := wf.NewNavigator(gctx, gctx.Mode)
	if err != nil {
		return nil, fmt.Errorf("creating navigator: %w", err)
	}

	// Configure for gate mode if needed
	if docFilename != "WORKFLOW.md" {
		nav.SetDocFilename(docFilename)
		nav.SetStateKeyPrefix(stateKeyPrefix)
	}

	// Create and load progress Store, inject into navigator for JSONL-backed state
	store := progress.NewStore(festivalPath)
	if err := store.Load(ctx); err != nil {
		return nil, fmt.Errorf("loading progress store: %w", err)
	}
	nav.SetStateStore(store)

	// Initialize the navigator
	if err := nav.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("initializing navigator: %w", err)
	}

	return nav, nil
}

// resolveNavigationMode determines whether workflow commands should target WORKFLOW.md or GATES.md.
// Returns the document filename, state key prefix, and an optional not-ready message.
// Returns empty filename if phase has neither document or if gate is not yet eligible.
//
// Logic:
//  1. If phase has WORKFLOW.md and it's incomplete → target WORKFLOW.md
//  2. If phase has GATES.md and workflow is complete (or absent) AND phase work is done → target GATES.md
//  3. If phase has GATES.md but phase work is incomplete → return not-ready message
//  4. If neither exists → return empty (caller should error)
func resolveNavigationMode(ctx context.Context, festivalPath, phasePath string) (docFilename, stateKeyPrefix, notReadyMsg string) {
	phaseName := filepath.Base(phasePath)
	hasWorkflow := false
	workflowComplete := false

	// Load store once for both workflow and gate checks
	store := progress.NewStore(festivalPath)
	storeLoaded := store.Load(ctx) == nil

	// Check WORKFLOW.md
	workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
	if _, err := os.Stat(workflowPath); err == nil {
		hasWorkflow = true

		// Check if workflow is complete
		if storeLoaded {
			state, ok := store.WorkflowPhaseState(phaseName)
			if ok && state.TotalSteps > 0 && state.IsComplete() {
				workflowComplete = true
			}
		}
	}

	// Check GATES.md
	gatesPath := filepath.Join(phasePath, "GATES.md")
	hasGates := false
	if _, err := os.Stat(gatesPath); err == nil {
		hasGates = true
	}

	// Route: incomplete workflow takes priority
	if hasWorkflow && !workflowComplete {
		return "WORKFLOW.md", "", ""
	}

	// Route: gate mode when workflow is done (or absent) and gates exist
	if hasGates {
		// Phase gate runs AFTER all other phase work. Check actual task/workflow completion
		// rather than PHASE_GOAL.md frontmatter (which creates a circular dependency since
		// gate steps are counted in progress totals).
		if shared.HasSequenceDirs(phasePath) && !shared.ArePhaseTasksAndWorkflowComplete(ctx, storeLoaded, store, phasePath, phaseName) {
			return "", "", "phase gate exists but is not yet eligible\n\nComplete all phase sequences/tasks first, then the gate will become accessible.\nUse 'fest next' to continue working on phase tasks."
		}
		return "GATES.md", "gate:", ""
	}

	// Fallback: workflow phase with completed workflow and no gates
	if hasWorkflow {
		return "WORKFLOW.md", "", ""
	}

	return "", "", ""
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

// findAllNavigablePhases returns all phase directories that have a WORKFLOW.md or GATES.md file.
func findAllNavigablePhases(festivalPath string) ([]string, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return nil, err
	}

	var phases []string
	for _, entry := range entries {
		if !entry.IsDir() || !shared.IsNumberedDir(entry.Name()) {
			continue
		}

		phasePath := filepath.Join(festivalPath, entry.Name())
		hasWorkflow := fileExists(filepath.Join(phasePath, "WORKFLOW.md"))
		hasGates := fileExists(filepath.Join(phasePath, "GATES.md"))
		if hasWorkflow || hasGates {
			phases = append(phases, phasePath)
		}
	}

	sort.Strings(phases)
	return phases, nil
}

// findFirstIncompleteNavigablePhase scans phases for the first with incomplete WORKFLOW.md or GATES.md.
// Checks workflow first (incomplete workflow takes priority), then gates.
func findFirstIncompleteNavigablePhase(ctx context.Context, festivalPath string) (string, error) {
	phases, err := findAllNavigablePhases(festivalPath)
	if err != nil {
		return "", err
	}

	store := progress.NewStore(festivalPath)
	storeLoaded := store.Load(ctx) == nil

	for _, phasePath := range phases {
		phaseName := filepath.Base(phasePath)

		// Check WORKFLOW.md first
		if fileExists(filepath.Join(phasePath, "WORKFLOW.md")) {
			if !storeLoaded {
				return phasePath, nil
			}
			state, ok := store.WorkflowPhaseState(phaseName)
			if !ok || state.TotalSteps == 0 || !state.IsComplete() {
				return phasePath, nil
			}
		}

		// Check GATES.md (only if workflow is complete or absent AND phase work is done)
		if fileExists(filepath.Join(phasePath, "GATES.md")) {
			// Phase gate requires all other phase work to be complete first.
			// Use actual task/workflow completion check, not PHASE_GOAL.md frontmatter,
			// to avoid circular dependency with gate steps in progress totals.
			if shared.HasSequenceDirs(phasePath) && !shared.ArePhaseTasksAndWorkflowComplete(ctx, storeLoaded, store, phasePath, phaseName) {
				continue // Gate not yet eligible, skip to next phase
			}
			if !storeLoaded {
				return phasePath, nil
			}
			gateState, ok := store.GatePhaseState(phaseName)
			if !ok || gateState.TotalSteps == 0 || !gateState.IsComplete() {
				return phasePath, nil
			}
		}
	}

	return "", nil
}

// fileExists returns true if the path exists and is accessible.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
