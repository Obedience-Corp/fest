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
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/feedback"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/guidance/selection"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/validator"
	"github.com/spf13/cobra"

	// Import all navigator packages to trigger their registration.
	_ "github.com/Obedience-Corp/fest/internal/guidance/action"
	_ "github.com/Obedience-Corp/fest/internal/guidance/ingest"
	_ "github.com/Obedience-Corp/fest/internal/guidance/orchestration"
	_ "github.com/Obedience-Corp/fest/internal/guidance/planning"
	_ "github.com/Obedience-Corp/fest/internal/guidance/research"
	_ "github.com/Obedience-Corp/fest/internal/guidance/review"
	"github.com/Obedience-Corp/fest/internal/progress"
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

	// Validation gate: block if festival has errors or warnings
	vResult, vErr := validator.FullValidate(ctx, festivalPath)
	if vErr == nil && hasBlockingIssues(vResult) {
		return emitValidationBlock(festivalPath, vResult)
	}

	// Block implementation/review phases in planning festivals
	if err := checkPlanningStatus(festivalPath, shared.ResolvePhasePath(cwd, festivalPath)); err != nil {
		return err
	}

	// If mode flag is provided or --navigator flag is set, use guidance navigator
	if modeFlag != "" || useNavigator {
		return runNavigatorMode(ctx, cwd, festivalPath)
	}

	// Auto-detect WORKFLOW.md in current phase and route to workflow navigator
	// Only route immediately if workflow_position is "before" (workflow runs before sequences)
	phasePath := shared.ResolvePhasePath(cwd, festivalPath)
	if phasePath != "" {
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, err := os.Stat(workflowPath); err == nil {
			position := shared.WorkflowPositionForPhase(phasePath)
			if position == frontmatter.WorkflowPositionBefore {
				// WORKFLOW.md exists with position=before - check if workflow is complete before routing
				phaseName := filepath.Base(phasePath)
				store := progress.NewStore(festivalPath)
				if loadErr := store.Load(ctx); loadErr != nil {
					return runWorkflowMode(ctx, festivalPath, phasePath)
				}
				state, ok := store.WorkflowPhaseState(phaseName)
				if !ok || state.TotalSteps == 0 || !state.IsComplete() {
					return runWorkflowMode(ctx, festivalPath, phasePath)
				}
			}
			// position=after: fall through to selector; workflow runs after sequences complete
			// Workflow complete: fall through to selector for next task
		}
	}

	// If at festival root (no phase detected), find the first incomplete phase respecting numerical order
	if phasePath == "" {
		nextPhase, isWorkflow, fipErr := findFirstIncompletePhase(ctx, festivalPath)
		if fipErr == nil && nextPhase != "" && isWorkflow {
			// Only route to workflow immediately if position=before
			position := shared.WorkflowPositionForPhase(nextPhase)
			if position == frontmatter.WorkflowPositionBefore {
				return runWorkflowMode(ctx, festivalPath, nextPhase)
			}
			// position=after: fall through to selector; workflow runs after sequences
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

	// Override graph-based progress with authoritative progress from the Manager,
	// which includes workflow phase steps (INGEST, PLAN) for consistency with fest show/progress.
	mgr, mgrErr := progress.NewManager(ctx, festivalPath)
	if mgrErr == nil {
		festProgress, fpErr := mgr.GetFestivalProgress(ctx, festivalPath)
		if fpErr == nil && festProgress.Overall != nil {
			result.Progress = &selection.ProgressInfo{
				TotalTasks:     festProgress.Overall.Total,
				CompletedTasks: festProgress.Overall.Completed,
				Percentage:     float64(festProgress.Overall.Percentage),
			}
		}
	}

	// Check if an earlier phase has incomplete after-workflow steps
	// that should run before jumping to later phases
	if result.Task != nil {
		earlierWorkflow, ewErr := findEarlierIncompleteAfterWorkflow(ctx, festivalPath, result.Task.PhaseName)
		if ewErr == nil && earlierWorkflow != "" {
			return runWorkflowMode(ctx, festivalPath, earlierWorkflow)
		}
	}

	// If selector says festival is complete, check for remaining incomplete workflow phases
	if result.FestivalComplete {
		incompleteWorkflow, wErr := findFirstIncompleteWorkflowPhase(ctx, festivalPath)
		if wErr == nil && incompleteWorkflow != "" {
			return runWorkflowMode(ctx, festivalPath, incompleteWorkflow)
		}
	}

	// Populate feedback criteria for JSON output
	feedbackCriteria := loadFeedbackCriteria(ctx, festivalPath)
	if len(feedbackCriteria) > 0 {
		result.FeedbackCriteria = feedbackCriteria
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
		printFeedbackReminder(ctx, festivalPath)
		return nil
	}

	fmt.Print(selection.FormatText(result, showInlineContext))
	printFeedbackReminder(ctx, festivalPath)
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
	printFeedbackReminder(ctx, festivalPath)
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

	// Create and load progress Store, inject into navigator for JSONL-backed state
	store := progress.NewStore(festivalPath)
	if err := store.Load(ctx); err != nil {
		return errors.Wrap(err, "loading progress store")
	}
	if wfNav, ok := nav.(*wf.Navigator); ok {
		wfNav.SetStateStore(store)
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
	printFeedbackReminder(ctx, festivalPath)
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

	sort.Strings(phases)

	// Load Store once for all workflow state lookups
	store := progress.NewStore(festivalPath)
	storeLoaded := store.Load(ctx) == nil

	for _, phasePath := range phases {
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, err := os.Stat(workflowPath); err == nil {
			phaseName := filepath.Base(phasePath)
			if storeLoaded {
				state, ok := store.WorkflowPhaseState(phaseName)
				if !ok || state.TotalSteps == 0 || !state.IsComplete() {
					return phasePath, true, nil
				}
			} else {
				return phasePath, true, nil // Can't load store, assume incomplete
			}
			continue
		}

		if hasSequenceDirs(phasePath) && !isPhaseMarkedComplete(phasePath) {
			return phasePath, false, nil
		}
	}

	return "", false, nil
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

	store := progress.NewStore(festivalPath)
	storeLoaded := store.Load(ctx) == nil

	for _, phasePath := range phases {
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, err := os.Stat(workflowPath); err != nil {
			continue
		}

		phaseName := filepath.Base(phasePath)
		if !storeLoaded {
			return phasePath, nil
		}
		state, ok := store.WorkflowPhaseState(phaseName)
		if !ok || state.TotalSteps == 0 || !state.IsComplete() {
			return phasePath, nil
		}
	}

	return "", nil
}

// findEarlierIncompleteAfterWorkflow scans phases in numerical order before the given phase
// and returns the first one with an incomplete after-position workflow. This handles hybrid
// phases where sequences are complete but the after-workflow steps remain.
func findEarlierIncompleteAfterWorkflow(ctx context.Context, festivalPath, currentPhaseName string) (string, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", err
	}

	var phases []string
	for _, entry := range entries {
		if entry.IsDir() && isNumberedDir(entry.Name()) {
			phases = append(phases, entry.Name())
		}
	}
	sort.Strings(phases)

	store := progress.NewStore(festivalPath)
	storeLoaded := store.Load(ctx) == nil

	for _, phaseName := range phases {
		// Stop before the current task's phase
		if phaseName >= currentPhaseName {
			break
		}

		phasePath := filepath.Join(festivalPath, phaseName)
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, statErr := os.Stat(workflowPath); statErr != nil {
			continue
		}

		// Only consider after-position workflows (default)
		position := shared.WorkflowPositionForPhase(phasePath)
		if position == frontmatter.WorkflowPositionBefore {
			continue
		}

		// Check if workflow is incomplete
		if storeLoaded {
			state, ok := store.WorkflowPhaseState(phaseName)
			if ok && state.TotalSteps > 0 && state.IsComplete() {
				continue // Workflow is complete
			}
		}

		return phasePath, nil
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

// hasBlockingIssues returns true if the result contains errors or warnings.
// Only LevelInfo issues pass through without blocking.
func hasBlockingIssues(result *validator.Result) bool {
	for _, issue := range result.Issues {
		if issue.Level == validator.LevelError || issue.Level == validator.LevelWarning {
			return true
		}
	}
	return false
}

// emitValidationBlock prints a blocking message when the festival fails validation.
func emitValidationBlock(festivalPath string, result *validator.Result) error {
	var sb strings.Builder
	sb.WriteString("STOP — FESTIVAL VALIDATION FAILED\n")
	sb.WriteString("──────────────────────────────────\n")
	sb.WriteString("This festival has issues that must be fixed before continuing.\n\n")

	// Collect errors and warnings separately
	var errs, warns []validator.Issue
	for _, issue := range result.Issues {
		switch issue.Level {
		case validator.LevelError:
			errs = append(errs, issue)
		case validator.LevelWarning:
			warns = append(warns, issue)
		}
	}

	if len(errs) > 0 {
		sb.WriteString("Errors:\n")
		for _, issue := range errs {
			path := issue.Path
			if rel, err := filepath.Rel(festivalPath, path); err == nil {
				path = rel
			}
			fmt.Fprintf(&sb, "  ✗ %s: %s\n", path, issue.Message)
		}
	}

	if len(warns) > 0 {
		if len(errs) > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Warnings:\n")
		for _, issue := range warns {
			path := issue.Path
			if rel, err := filepath.Rel(festivalPath, path); err == nil {
				path = rel
			}
			fmt.Fprintf(&sb, "  ⚠ %s: %s\n", path, issue.Message)
		}
	}

	sb.WriteString("\nRun 'fest validate' for full details, then fix the issues.\n")
	sb.WriteString("Do not proceed with tasks until the festival passes validation.\n")
	fmt.Print(sb.String())
	return errors.Validation("festival has unfixed validation errors").
		WithField("festival", filepath.Base(festivalPath))
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

// loadFeedbackCriteria checks if feedback is configured for the festival and returns criteria names.
func loadFeedbackCriteria(ctx context.Context, festivalPath string) []string {
	store := feedback.NewStore(festivalPath)
	if !store.IsInitialized() {
		return nil
	}

	config, err := store.LoadConfig(ctx)
	if err != nil {
		return nil
	}

	names := make([]string, len(config.Criteria))
	for i, c := range config.Criteria {
		names[i] = c.Name
	}
	return names
}

// checkPlanningStatus blocks implementation/review phases when the festival is still in planning status.
func checkPlanningStatus(festivalPath, phasePath string) error {
	festCfg, err := config.LoadFestivalConfig(festivalPath, "")
	if err != nil {
		return nil // Can't load config — don't block
	}

	status := festCfg.Metadata.CurrentStatus()
	if status != "planning" {
		return nil
	}

	// Only block if we can detect the phase type
	if phasePath == "" {
		// At festival root — find the first incomplete phase to check its type
		entries, err := os.ReadDir(festivalPath)
		if err != nil {
			return nil
		}
		for _, entry := range entries {
			if entry.IsDir() && isNumberedDir(entry.Name()) {
				phasePath = filepath.Join(festivalPath, entry.Name())
				break
			}
		}
		if phasePath == "" {
			return nil
		}
	}

	phaseType := guidance.DetectPhaseType(phasePath)
	if phaseType == "implementation" || phaseType == "review" {
		return errors.Validation("Festival must be in active status to execute implementation phases. Run: fest status set active")
	}

	return nil
}

// printFeedbackReminder appends the full feedback reminder (criteria + recording instructions)
// to the output when feedback is initialized for the festival.
func printFeedbackReminder(ctx context.Context, festivalPath string) {
	store := feedback.NewStore(festivalPath)
	text, err := store.GetReminderText(ctx)
	if err != nil || text == "" {
		return
	}
	fmt.Printf("\n%s%s", strings.Repeat("─", 60), text)
}
