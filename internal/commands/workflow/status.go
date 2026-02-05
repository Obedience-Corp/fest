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
// It supports:
//   - --phase flag to specify a particular phase
//   - Running from inside a phase directory
//   - Running from festival root (auto-detects first incomplete workflow phase)
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

		// Priority 3: Auto-detect first incomplete workflow phase
		if phasePath == "" {
			detected, err := findFirstIncompleteWorkflowPhase(ctx, festivalPath)
			if err != nil {
				return nil, fmt.Errorf("scanning for workflow phases: %w", err)
			}
			if detected == "" {
				// Check if there are any workflow phases at all
				phases, _ := findAllWorkflowPhases(festivalPath)
				if len(phases) == 0 {
					return nil, fmt.Errorf("no workflow phases found in this festival\n\nWorkflow commands only work with phases that have a WORKFLOW.md file")
				}
				return nil, fmt.Errorf("all workflow phases are complete\n\nUse --phase to specify a particular phase, or 'fest workflow reset --phase <name>' to restart one")
			}
			phasePath = detected
		}
	}

	// Detect phase type
	phaseType := guidance.DetectPhaseType(phasePath)
	if !isWorkflowPhase(phaseType) {
		return nil, fmt.Errorf("not a workflow-based phase (detected: %s)\n\nWorkflow commands only work in ingest, research, or planning phases", phaseType)
	}

	// Create guidance context
	gctx := &guidance.GuidanceContext{
		FestivalPath: festivalPath,
		PhasePath:    phasePath,
		PhaseName:    filepath.Base(phasePath),
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

// findAllWorkflowPhases returns all phase directories that have a WORKFLOW.md file.
func findAllWorkflowPhases(festivalPath string) ([]string, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return nil, err
	}

	var phases []string
	for _, entry := range entries {
		if !entry.IsDir() || !isNumberedDir(entry.Name()) {
			continue
		}

		phasePath := filepath.Join(festivalPath, entry.Name())
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, err := os.Stat(workflowPath); err == nil {
			phases = append(phases, phasePath)
		}
	}

	sort.Strings(phases)
	return phases, nil
}

// findFirstIncompleteWorkflowPhase scans phases in numerical order for the first with incomplete workflow.
// Returns the phase path if found, empty string if all workflow phases are complete.
func findFirstIncompleteWorkflowPhase(ctx context.Context, festivalPath string) (string, error) {
	phases, err := findAllWorkflowPhases(festivalPath)
	if err != nil {
		return "", err
	}

	for _, phasePath := range phases {
		phaseName := filepath.Base(phasePath)
		state, err := wf.LoadState(ctx, festivalPath, phaseName)
		if err != nil {
			return phasePath, nil // Can't load state, assume incomplete
		}

		// A workflow phase is incomplete if it has no steps initialized yet or isn't complete
		if state.TotalSteps == 0 || !state.IsComplete() {
			return phasePath, nil
		}
	}

	return "", nil // All workflow phases complete (or no workflow phases exist)
}

// isNumberedDir checks if directory name starts with a number.
func isNumberedDir(name string) bool {
	if len(name) < 1 {
		return false
	}
	return name[0] >= '0' && name[0] <= '9'
}
