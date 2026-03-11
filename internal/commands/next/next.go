// Package next provides the fest next command for task navigation.
package next

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/feedback"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/guidance/selection"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/scope"
	tpl "github.com/Obedience-Corp/fest/internal/template"
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

	// Resolve the next incomplete phase for phase-aware gates.
	// Always use findFirstIncompletePhase rather than cwd — cwd may be inside
	// a later scaffolded phase that isn't the actual next actionable one.
	nextPhasePath := ""
	nextPhaseType := ""
	nextPhaseName := ""
	if found, _, findErr := findFirstIncompletePhase(ctx, festivalPath); findErr == nil && found != "" {
		nextPhasePath = found
		nextPhaseType = guidance.DetectPhaseType(found)
		nextPhaseName = filepath.Base(found)
	}

	// Validation gate: block if festival has errors (phase-aware for markers)
	vResult, vErr := validator.FullValidate(ctx, festivalPath)
	if vErr == nil && hasBlockingIssues(vResult, nextPhaseType, nextPhaseName) {
		return emitValidationBlock(festivalPath, vResult)
	}

	// Block implementation/review phases in pre-active festivals (planning or ready).
	// Use the next incomplete phase, not cwd — cwd may be inside a later scaffolded
	// implementation phase while an earlier preparatory phase is the real next step.
	if err := checkPreActiveStatus(ctx, festivalPath, nextPhasePath); err != nil {
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

	// Check if an earlier phase has an incomplete phase gate (GATES.md)
	// Phase gates run after all tasks and workflows in a phase are complete
	if result.Task != nil {
		earlierGate, egErr := findEarlierIncompletePhaseGate(ctx, festivalPath, result.Task.PhaseName)
		if egErr == nil && earlierGate != "" {
			return runPhaseGateMode(ctx, festivalPath, earlierGate)
		}
	}

	// If selector says festival is complete, check for remaining incomplete workflow phases
	if result.FestivalComplete {
		incompleteWorkflow, wErr := findFirstIncompleteWorkflowPhase(ctx, festivalPath)
		if wErr == nil && incompleteWorkflow != "" {
			return runWorkflowMode(ctx, festivalPath, incompleteWorkflow)
		}
	}

	// If selector says festival is complete, check for remaining incomplete phase gates
	if result.FestivalComplete {
		incompleteGate, gErr := findFirstIncompletePhaseGate(ctx, festivalPath)
		if gErr == nil && incompleteGate != "" {
			return runPhaseGateMode(ctx, festivalPath, incompleteGate)
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
		printChainContext(ctx, festivalPath, result.FestivalComplete)
		printFeedbackReminder(ctx, festivalPath)
		return nil
	}

	fmt.Print(selection.FormatText(result, showInlineContext))
	printChainContext(ctx, festivalPath, result.FestivalComplete)
	printFeedbackReminder(ctx, festivalPath)
	return nil
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

// printChainContext shows chain-awareness context when a festival is complete.
// Best-effort only — any error silently skips.
func printChainContext(ctx context.Context, festivalPath string, festivalComplete bool) {
	if !festivalComplete {
		return
	}

	// Load festival ID from fest.yaml.
	festCfg, cfgErr := config.LoadFestivalConfig(festivalPath, "")
	if cfgErr != nil || !festCfg.Metadata.HasMetadata() {
		return
	}
	festivalID := festCfg.Metadata.ID

	// Find festivals root.
	root, err := tpl.FindFestivalsRoot(festivalPath)
	if err != nil {
		return
	}

	// Find chain containing this festival.
	c, _, findErr := chainpkg.FindForFestival(ctx, festivalID, root)
	if findErr != nil || c == nil {
		return // Not in any chain.
	}

	// Build search dirs for status resolution.
	searchDirs := make([]string, len(id.StatusDirectories))
	for i, d := range id.StatusDirectories {
		searchDirs[i] = filepath.Join(root, d)
	}

	// Resolve live statuses.
	resolved, _ := chainpkg.Resolve(ctx, c, searchDirs)
	statuses := make(map[string]chainpkg.FestivalStatus, len(c.Festivals))
	for _, node := range c.Festivals {
		rf, ok := resolved[node.Ref]
		if !ok || rf.Path == "" {
			statuses[node.Ref] = chainpkg.FestivalPlanning
			continue
		}
		cfg, cfgErr := config.LoadFestivalConfig(rf.Path, "")
		if cfgErr != nil {
			statuses[node.Ref] = chainpkg.FestivalPlanning
			continue
		}
		statuses[node.Ref] = mapChainFestivalStatus(cfg.Metadata.CurrentStatus())
	}

	// Compute progress to find newly unblocked.
	chainProgress, _ := chainpkg.ComputeProgress(ctx, c, statuses)
	if chainProgress == nil || len(chainProgress.Unblocked) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("CHAIN CONTEXT")
	fmt.Println("Completing this festival unblocks:")
	for _, ref := range chainProgress.Unblocked {
		node := c.FestivalByRef(ref)
		if node != nil {
			fmt.Printf("  -> %s (%s) %s [ready to start]\n", ref, node.ID, node.Name)
		}
	}
}

// mapChainFestivalStatus converts a string status to a chain FestivalStatus.
func mapChainFestivalStatus(s string) chainpkg.FestivalStatus {
	switch s {
	case "planning":
		return chainpkg.FestivalPlanning
	case "ready":
		return chainpkg.FestivalReady
	case "active":
		return chainpkg.FestivalActive
	case "completed", "dungeon/completed":
		return chainpkg.FestivalCompleted
	default:
		return chainpkg.FestivalPlanning
	}
}
