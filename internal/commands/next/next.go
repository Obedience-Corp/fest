// Package next provides the fest next command for task navigation.
package next

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
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
	pathOnly        bool
	sequenceOnly    bool
	modeFlag        string
	useNavigator    bool
	inlineContext   bool
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

Output Modes:
  --path-only    Just the task file path (for piping)
  --short        Task path with status message
  --cd           Directory path for shell cd
  --json         Full result as JSON
  --verbose      Detailed human-readable output
  --context      Layered goals + full task content inline

Examples:
  fest next                    # Find next task in festival
  fest next --sequence         # Only consider current sequence
  fest next --json             # Output as JSON
  fest next --verbose          # Detailed output
  fest next --short            # Just the task path
  fest next --cd               # Output directory for shell cd
  fest next --path-only        # Just the file path, nothing else
  fest next --context          # Show layered goals and full task content`,
		RunE: runNext,
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&verboseOutput, "verbose", false, "show detailed information")
	cmd.Flags().BoolVar(&shortOutput, "short", false, "output only the task path")
	cmd.Flags().BoolVar(&cdOutput, "cd", false, "output directory path for cd command")
	cmd.Flags().BoolVar(&pathOnly, "path-only", false, "output only the task file path")
	cmd.Flags().BoolVar(&sequenceOnly, "sequence", false, "only consider current sequence")
	cmd.Flags().StringVarP(&modeFlag, "mode", "m", "", "override phase type detection (implementation|plan|research|review|action|ingest)")
	cmd.Flags().BoolVar(&useNavigator, "navigator", false, "use guidance navigator for output formatting")
	cmd.Flags().BoolVar(&inlineContext, "context", false, "show layered goals and full task content inline (override config)")
	cmd.Flags().BoolVar(&noInlineContext, "no-context", false, "hide inline content (override config)")

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

	// Load config for inline context default
	cfg, err := config.Load(ctx, "")
	if err != nil {
		return errors.Wrap(err, "loading config")
	}

	// Determine inline context setting: flags override config
	showInlineContext := cfg.Behavior.InlineContextDefault
	if inlineContext {
		showInlineContext = true
	} else if noInlineContext {
		showInlineContext = false
	}

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
			// WORKFLOW.md exists - use workflow navigator
			return runWorkflowMode(ctx, festivalPath, phasePath)
		}
	}

	// If at festival root (no phase detected), check for incomplete workflow phases in order
	if phasePath == "" {
		incompletePhase, err := findFirstIncompleteWorkflowPhase(ctx, festivalPath)
		if err == nil && incompletePhase != "" {
			return runWorkflowMode(ctx, festivalPath, incompletePhase)
		}
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

	// Output formatting
	if pathOnly {
		if result.Task == nil {
			return errors.NotFound("no task available")
		}
		fmt.Println(result.Task.Path)
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

// findFirstIncompleteWorkflowPhase scans phases in numerical order for the first with incomplete workflow.
// Returns the phase path if found, empty string if all workflow phases are complete or no workflow phases exist.
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

	// Sort to ensure numerical order (001_, 002_, etc.)
	sort.Strings(phases)

	for _, phasePath := range phases {
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, err := os.Stat(workflowPath); err != nil {
			continue // No WORKFLOW.md, skip (selector handles task-based)
		}

		state, err := workflow.LoadState(ctx, phasePath)
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
