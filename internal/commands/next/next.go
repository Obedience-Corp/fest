// Package next provides the fest next command for task navigation.
package next

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/guidance/selection"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"

	// Import all navigator packages to trigger their registration.
	_ "github.com/Obedience-Corp/fest/internal/guidance/action"
	_ "github.com/Obedience-Corp/fest/internal/guidance/ingest"
	_ "github.com/Obedience-Corp/fest/internal/guidance/orchestration"
	_ "github.com/Obedience-Corp/fest/internal/guidance/planning"
	_ "github.com/Obedience-Corp/fest/internal/guidance/research"
	_ "github.com/Obedience-Corp/fest/internal/guidance/review"
	"github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

var (
	jsonOutput      bool
	verboseOutput   bool
	shortOutput     bool
	cdOutput        bool
	pathFlag        bool
	sequenceOnly    bool
	modeFlag        string
	useNavigator    bool
	noInlineContext bool
)

// NewNextCommand creates the next command
func NewNextCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Find the next task to work on",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Determine the next task to work on based on dependencies and progress.

The command analyzes the festival structure, checks task completion status,
and recommends the next task following the priority order:

1. Tasks in current sequence with satisfied dependencies
2. Next incomplete task in current phase
3. First incomplete task in earliest phase
4. Quality gates before phase transitions

By default, shows layered goals and full task content inline to provide
complete context for execution.

Output Modes:
  (default)      Layered goals + full task content inline
  --no-context   Hide inline content, show minimal output
  --path         Just the task file path (relative, for piping)
  --short        Task path with status message
  --cd           Directory path for shell cd
  --json         Full result as JSON
  --verbose      Detailed human-readable output

Examples:
  fest next                    # Find next task with full context
  fest next --no-context       # Minimal output without task content
  fest next --sequence         # Only consider current sequence
  fest next --json             # Output as JSON
  fest next --verbose          # Detailed output
  fest next --short            # Just the task path
  fest next --cd               # Output directory for shell cd
  fest next --path             # Just the relative file path`,
		RunE: runNext,
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&verboseOutput, "verbose", false, "show detailed information")
	cmd.Flags().BoolVar(&shortOutput, "short", false, "output only the task path")
	cmd.Flags().BoolVar(&cdOutput, "cd", false, "output directory path for cd command")
	cmd.Flags().BoolVar(&pathFlag, "path", false, "output only the relative task file path")
	cmd.Flags().BoolVar(&sequenceOnly, "sequence", false, "only consider current sequence")
	cmd.Flags().StringVarP(&modeFlag, "mode", "m", "", "override phase type detection (implementation|plan|research|review|action|ingest)")
	cmd.Flags().BoolVar(&useNavigator, "navigator", false, "use guidance navigator for output formatting")
	cmd.Flags().BoolVar(&noInlineContext, "no-context", false, "hide inline content (show minimal output)")

	return cmd
}

func runNext(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	// Context is shown by default, --no-context disables it
	showInlineContext := !noInlineContext

	// Resolve festival path (supports linked festivals via fest link)
	festivalPath, err := shared.ResolveFestivalPath(cwd, "")
	if err != nil {
		return errors.Wrap(err, "not inside a festival")
	}

	// If mode flag is provided or --navigator flag is set, use guidance navigator
	if modeFlag != "" || useNavigator {
		return runNavigatorMode(ctx, cwd, festivalPath)
	}

	// Auto-detect WORKFLOW.md in current phase and route to workflow navigator
	phasePath := shared.ResolvePhasePath(cwd, festivalPath)
	if phasePath != "" {
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, err := os.Stat(workflowPath); err == nil {
			// WORKFLOW.md exists - check if workflow is complete before routing
			phaseName := filepath.Base(phasePath)
			state, loadErr := workflow.LoadState(ctx, festivalPath, phaseName)
			if loadErr != nil || state.TotalSteps == 0 || !state.IsComplete() {
				return runWorkflowMode(ctx, festivalPath, phasePath)
			}
			// Workflow complete - fall through to selector for next task
		}
	}

	// If at festival root (no phase detected), find the first incomplete phase respecting numerical order
	if phasePath == "" {
		nextPhase, isWorkflow, fipErr := findFirstIncompletePhase(ctx, festivalPath)
		if fipErr == nil && nextPhase != "" && isWorkflow {
			return runWorkflowMode(ctx, festivalPath, nextPhase)
		}
		// If nextPhase is task-based or empty, fall through to selector
	}

	// Fall back to selector-based navigation
	selector := selection.NewSelector(festivalPath)

	var result *selection.NextTaskResult
	if sequenceOnly {
		seqPath := findSequencePath(cwd, festivalPath)
		if seqPath == "" {
			return errors.NotFound("not inside a sequence directory")
		}
		result, err = selector.FindNextInSequence(ctx, seqPath)
	} else {
		result, err = selector.FindNext(ctx, cwd)
	}

	if err != nil {
		return errors.Wrap(err, "finding next task")
	}

	// If selector says festival is complete, check for remaining incomplete workflow phases
	if result.FestivalComplete {
		incompleteWorkflow, wErr := findFirstIncompleteWorkflowPhase(ctx, festivalPath)
		if wErr == nil && incompleteWorkflow != "" {
			return runWorkflowMode(ctx, festivalPath, incompleteWorkflow)
		}
	}

	// Output formatting
	if pathFlag {
		if result.Task == nil {
			return errors.NotFound("no task available")
		}
		// Output relative path from festival root
		relPath := filepath.Join(result.Task.PhaseName, result.Task.SequenceName, result.Task.Name+".md")
		fmt.Println(relPath)
		return nil
	}

	if cdOutput {
		output := selection.FormatCD(result)
		if output == "" {
			return errors.NotFound("no task available to navigate to")
		}
		fmt.Println(output)
		return nil
	}

	if shortOutput {
		fmt.Println(selection.FormatShort(result))
		return nil
	}

	if jsonOutput {
		output, err := selection.FormatJSON(result)
		if err != nil {
			return errors.Parse("formatting JSON", err)
		}
		fmt.Println(output)
		return nil
	}

	if verboseOutput {
		fmt.Print(selection.FormatVerbose(result, showInlineContext))
		return nil
	}

	fmt.Print(selection.FormatText(result, showInlineContext))
	return nil
}

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
	return nil
}

// runWorkflowMode uses the workflow navigator for WORKFLOW.md-based navigation.
func runWorkflowMode(ctx context.Context, festivalPath, phasePath string) error {
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

	// Initialize the navigator
	if err := nav.Initialize(ctx); err != nil {
		return errors.Wrap(err, "initializing workflow navigator")
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
	return nil
}

// findSequencePath finds the sequence path from current directory
func findSequencePath(cwd, festivalPath string) string {
	// Walk up from cwd looking for a sequence directory
	current := cwd
	for {
		// Check if current is a sequence (numbered dir inside a numbered phase dir)
		parent := filepath.Dir(current)
		if parent == festivalPath {
			// Current is a phase, not a sequence
			return ""
		}
		grandparent := filepath.Dir(parent)
		if grandparent == festivalPath {
			// Parent is a phase, current might be a sequence
			if isNumberedDir(filepath.Base(parent)) && isNumberedDir(filepath.Base(current)) {
				return current
			}
		}
		if current == festivalPath || current == "/" || current == "." {
			break
		}
		current = parent
	}
	return ""
}

// isNumberedDir checks if directory name starts with a number
func isNumberedDir(name string) bool {
	if len(name) < 2 {
		return false
	}
	return name[0] >= '0' && name[0] <= '9'
}

// findFirstIncompletePhase scans ALL phases in numerical order and returns the first incomplete phase.
// It respects ordering across both workflow-based and task-based phases.
// Returns (phasePath, isWorkflow, error). Empty phasePath means all phases are complete.
func findFirstIncompletePhase(ctx context.Context, festivalPath string) (string, bool, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", false, err
	}

	var phases []string
	for _, entry := range entries {
		if entry.IsDir() && isNumberedDir(entry.Name()) {
			phases = append(phases, filepath.Join(festivalPath, entry.Name()))
		}
	}

	// Sort to ensure numerical order (001_, 002_, etc.)
	sort.Strings(phases)

	for _, phasePath := range phases {
		// Check for WORKFLOW.md first
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, err := os.Stat(workflowPath); err == nil {
			// Workflow-based phase - check workflow state
			phaseName := filepath.Base(phasePath)
			state, loadErr := workflow.LoadState(ctx, festivalPath, phaseName)
			if loadErr != nil {
				return phasePath, true, nil // Can't load state, assume incomplete
			}
			if state.TotalSteps == 0 || !state.IsComplete() {
				return phasePath, true, nil
			}
			continue // Workflow complete, check next phase
		}

		// Task-based phase - check if phase is complete via PHASE_GOAL.md frontmatter
		if hasSequenceDirs(phasePath) && !isPhaseMarkedComplete(phasePath) {
			return phasePath, false, nil
		}
	}

	return "", false, nil // All phases complete
}

// findFirstIncompleteWorkflowPhase scans phases in numerical order for the first with incomplete workflow.
// Used as a fallback when the selector reports all tasks complete but workflow phases remain.
func findFirstIncompleteWorkflowPhase(ctx context.Context, festivalPath string) (string, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", err
	}

	var phases []string
	for _, entry := range entries {
		if entry.IsDir() && isNumberedDir(entry.Name()) {
			phases = append(phases, filepath.Join(festivalPath, entry.Name()))
		}
	}

	sort.Strings(phases)

	for _, phasePath := range phases {
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, err := os.Stat(workflowPath); err != nil {
			continue
		}

		phaseName := filepath.Base(phasePath)
		state, err := workflow.LoadState(ctx, festivalPath, phaseName)
		if err != nil {
			return phasePath, nil
		}

		if state.TotalSteps == 0 || !state.IsComplete() {
			return phasePath, nil
		}
	}

	return "", nil
}

// hasSequenceDirs checks if a phase directory contains numbered subdirectories (sequences).
func hasSequenceDirs(phasePath string) bool {
	entries, err := os.ReadDir(phasePath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && isNumberedDir(entry.Name()) {
			return true
		}
	}
	return false
}

// isPhaseMarkedComplete checks PHASE_GOAL.md frontmatter for fest_status: completed.
func isPhaseMarkedComplete(phasePath string) bool {
	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")
	data, err := os.ReadFile(goalPath)
	if err != nil {
		return false
	}
	content := string(data)
	// Frontmatter is between --- delimiters at the start of the file
	if !strings.HasPrefix(content, "---") {
		return false
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return false
	}
	fm := content[3 : 3+end]
	return strings.Contains(fm, "fest_status: completed")
}
