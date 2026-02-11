// Package explore implements the fest explore command for interactive
// festival hierarchy browsing with vim-style navigation.
package explore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/scope"
	tuiexplore "github.com/Obedience-Corp/fest/internal/tui/explore"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

type exploreOptions struct {
	json bool
}

// NewExploreCommand creates the explore command with status subcommands.
func NewExploreCommand() *cobra.Command {
	opts := &exploreOptions{}

	cmd := &cobra.Command{
		Use:   "explore [status]",
		Short: "Interactive festival explorer with hierarchy drilldown",
		Long: `Explore festivals interactively with vim-style navigation.

If inside a festival directory, shows that festival's phase/sequence/task hierarchy.
If in the festivals/ directory, shows a list of festivals for the detected status.
Otherwise, shows all active festivals by default.

STATUS can be: active, planned, completed, dungeon`,
		Example: `  fest explore              # Auto-detect context and explore
  fest explore active       # Explore active festivals
  fest explore planned      # Explore planned festivals
  fest explore completed    # Explore completed festivals
  fest explore dungeon      # Explore dungeon festivals`,
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			status := ""
			if len(args) > 0 {
				status = strings.ToLower(args[0])
			}
			return runExplore(cmd.Context(), opts, status)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	// Add status subcommands for discoverability
	cmd.AddCommand(newStatusSubcommand("active", "Explore active festivals"))
	cmd.AddCommand(newStatusSubcommand("planned", "Explore planned festivals"))
	cmd.AddCommand(newStatusSubcommand("completed", "Explore completed festivals"))
	cmd.AddCommand(newStatusSubcommand("dungeon", "Explore dungeon festivals"))

	return cmd
}

func newStatusSubcommand(status, short string) *cobra.Command {
	opts := &exploreOptions{}

	return &cobra.Command{
		Use:   status,
		Short: short,
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExplore(cmd.Context(), opts, status)
		},
	}
}

func runExplore(ctx context.Context, opts *exploreOptions, status string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("runExplore")
	}

	display := ui.New(false, false)
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting working directory", err)
	}

	// Try to detect if we're inside a festival
	festivalPath := detectFestivalPath(cwd)
	if festivalPath != "" && status == "" {
		return exploreFestival(ctx, opts, display, festivalPath)
	}

	// Otherwise, list festivals for the given status
	if status == "" {
		status = "active"
	}

	return exploreFestivalList(ctx, opts, display, status)
}

// detectFestivalPath checks if cwd is inside a festival directory.
// Returns the festival root path, or empty string if not inside a festival.
func detectFestivalPath(cwd string) string {
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "fest.yaml")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "FESTIVAL_GOAL.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// exploreFestival shows the hierarchy of a single festival.
func exploreFestival(ctx context.Context, opts *exploreOptions, display *ui.UI, festivalPath string) error {
	if opts.json {
		return outputFestivalJSON(ctx, festivalPath)
	}

	parser := festival.NewParser()
	phases, err := parser.ParsePhases(ctx, festivalPath)
	if err != nil {
		return errors.Wrap(err, "parsing festival phases")
	}

	festivalName := filepath.Base(festivalPath)
	display.Success("Festival: %s", festivalName)
	fmt.Println()

	if len(phases) == 0 {
		display.Info("  No phases found")
		return nil
	}

	for _, phase := range phases {
		fmt.Printf("  %s %s\n", ui.Accent(phase.FullName), ui.Dim(fmt.Sprintf("(%s)", phase.Name)))

		// List sequences within the phase
		phaseDir := phase.Path
		sequences, seqErr := parser.ParseSequences(ctx, phaseDir)
		if seqErr != nil || len(sequences) == 0 {
			continue
		}
		for _, seq := range sequences {
			fmt.Printf("    %s %s\n", ui.Value(seq.FullName), ui.Dim(seq.Name))

			// List tasks within the sequence
			seqDir := seq.Path
			tasks, taskErr := parser.ParseTasks(ctx, seqDir)
			if taskErr != nil || len(tasks) == 0 {
				continue
			}
			for _, task := range tasks {
				fmt.Printf("      %s %s\n", ui.Dim("[ ]"), task.FullName)
			}
		}
	}

	return nil
}

// exploreFestivalList shows festivals for a given status.
// Uses the BubbleTea TUI for interactive mode, JSON for --json.
func exploreFestivalList(ctx context.Context, opts *exploreOptions, _ *ui.UI, status string) error {
	if opts.json {
		return outputExploreJSON(ctx, status)
	}

	// Interactive TUI mode
	selected, err := tuiexplore.Run(ctx, status)
	if err != nil {
		return errors.Wrap(err, "running explore TUI")
	}
	if selected != nil {
		fmt.Println(selected.Path)
	}
	return nil
}
