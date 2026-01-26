// Package next provides the fest next command for task navigation.
package next

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/guidance/selection"
	"github.com/spf13/cobra"

	// Import all navigator packages to trigger their registration.
	_ "github.com/Obedience-Corp/fest/internal/guidance/action"
	_ "github.com/Obedience-Corp/fest/internal/guidance/ingest"
	_ "github.com/Obedience-Corp/fest/internal/guidance/planning"
	_ "github.com/Obedience-Corp/fest/internal/guidance/research"
	_ "github.com/Obedience-Corp/fest/internal/guidance/review"
)

var (
	jsonOutput    bool
	verboseOutput bool
	shortOutput   bool
	cdOutput      bool
	sequenceOnly  bool
	modeFlag      string
	useNavigator  bool
)

// NewNextCommand creates the next command
func NewNextCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Find the next task to work on",
		Long: `Determine the next task to work on based on dependencies and progress.

The command analyzes the festival structure, checks task completion status,
and recommends the next task following the priority order:

1. Tasks in current sequence with satisfied dependencies
2. Next incomplete task in current phase
3. First incomplete task in earliest phase
4. Quality gates before phase transitions

Examples:
  fest next                    # Find next task in festival
  fest next --sequence         # Only consider current sequence
  fest next --json             # Output as JSON
  fest next --verbose          # Detailed output
  fest next --short            # Just the task path
  fest next --cd               # Output directory for shell cd`,
		RunE: runNext,
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&verboseOutput, "verbose", false, "show detailed information")
	cmd.Flags().BoolVar(&shortOutput, "short", false, "output only the task path")
	cmd.Flags().BoolVar(&cdOutput, "cd", false, "output directory path for cd command")
	cmd.Flags().BoolVar(&sequenceOnly, "sequence", false, "only consider current sequence")
	cmd.Flags().StringVarP(&modeFlag, "mode", "m", "", "override phase type detection (execute|plan|research|review|action|ingest)")
	cmd.Flags().BoolVar(&useNavigator, "navigator", false, "use guidance navigator for output formatting")

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

	// Resolve festival path (supports linked festivals via fest link)
	festivalPath, err := shared.ResolveFestivalPath(cwd, "")
	if err != nil {
		return errors.Wrap(err, "not inside a festival")
	}

	// If mode flag is provided or --navigator flag is set, use guidance navigator
	if modeFlag != "" || useNavigator {
		return runNavigatorMode(ctx, cwd, festivalPath)
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
		fmt.Print(selection.FormatVerbose(result))
		return nil
	}

	fmt.Print(selection.FormatText(result))
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
