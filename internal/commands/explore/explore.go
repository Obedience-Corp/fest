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
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/scope"
	tuiexplore "github.com/Obedience-Corp/fest/internal/tui/explore"
	"github.com/Obedience-Corp/fest/internal/workspace"
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

STATUS can be: active, planning, completed, dungeon`,
		Example: `  fest explore              # Auto-detect context and explore
  fest explore active       # Explore active festivals
  fest explore planning     # Explore planning festivals
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
	cmd.AddCommand(newStatusSubcommand("planning", "Explore planning festivals"))
	cmd.AddCommand(newStatusSubcommand("completed", "Explore completed festivals"))
	cmd.AddCommand(newStatusSubcommand("dungeon", "Explore dungeon festivals"))

	return cmd
}

func newStatusSubcommand(status, short string) *cobra.Command {
	opts := &exploreOptions{}

	cmd := &cobra.Command{
		Use:   status,
		Short: short,
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		RunE: func(c *cobra.Command, args []string) error {
			return runExplore(c.Context(), opts, status)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runExplore(ctx context.Context, opts *exploreOptions, status string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("runExplore")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting working directory", err)
	}

	// Try to detect if we're inside a festival
	festivalPath := detectFestivalPath(cwd)
	if festivalPath != "" && status == "" {
		return exploreFestival(ctx, opts, festivalPath)
	}

	// Otherwise, list festivals for the given status.
	// Empty status triggers the status overview in the TUI.
	return exploreFestivalList(ctx, opts, status)
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

// exploreFestival shows the hierarchy of a single festival using the interactive TUI.
func exploreFestival(ctx context.Context, opts *exploreOptions, festivalPath string) error {
	if opts.json {
		return outputFestivalJSON(ctx, festivalPath)
	}

	// Launch TUI pre-navigated into this festival's phase hierarchy
	selected, err := tuiexplore.RunWithFestival(ctx, festivalPath)
	if err != nil {
		return errors.Wrap(err, "running explore TUI")
	}
	if selected != nil {
		campaignRoot, _ := workspace.DetectCampaign(ctx, "")
		fmt.Println(pathutil.DisplayPath(selected.Path, campaignRoot))
	}
	return nil
}

// exploreFestivalList shows festivals for a given status.
// Uses the BubbleTea TUI for interactive mode, JSON for --json.
func exploreFestivalList(ctx context.Context, opts *exploreOptions, status string) error {
	status = id.ResolveStatusPath(status)
	if opts.json {
		return outputExploreJSON(ctx, status)
	}

	// Interactive TUI mode
	selected, err := tuiexplore.Run(ctx, status)
	if err != nil {
		return errors.Wrap(err, "running explore TUI")
	}
	if selected != nil {
		campaignRoot, _ := workspace.DetectCampaign(ctx, "")
		fmt.Println(pathutil.DisplayPath(selected.Path, campaignRoot))
	}
	return nil
}
