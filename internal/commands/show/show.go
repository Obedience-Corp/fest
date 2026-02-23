// Package show implements the fest show command for displaying festival information.
package show

import (
	"context"
	"fmt"
	"os"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

type showOptions struct {
	json       bool
	summary    bool // Show aggregate summary instead of tree view
	watch      bool // Continuously refresh display
	goals      bool // Show goals for phases and sequences
	collapsed  bool // Show collapsed tree with counters only
	inProgress bool // Expand only in_progress phases/sequences
}

// NewShowCommand creates the show command with all subcommands.
func NewShowCommand() *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:   "show [status|festival-name]",
		Short: "Display festival information",
		Long: `Display festival information by status or show details of a specific festival.

When run inside a festival directory, shows the current festival's details.
When run with a status argument, lists all festivals with that status.

SUBCOMMANDS:
  fest show              Show current festival (detect from cwd)
  fest show active       List festivals in active/ directory
  fest show planning     List festivals in planning/ directory
  fest show completed    List festivals in completed/ directory
  fest show dungeon      List festivals in dungeon/ directory
  fest show all          List all festivals grouped by status
  fest show <name>       Show details of a specific festival by name`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runShowCurrent(cmd.Context(), opts)
			}
			return runShow(cmd.Context(), args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	cmd.Flags().BoolVar(&opts.summary, "summary", false, "show aggregate summary instead of tree view")
	cmd.Flags().BoolVar(&opts.watch, "watch", false, "continuously refresh display")
	cmd.Flags().BoolVar(&opts.goals, "goals", false, "show goals for phases and sequences")
	cmd.Flags().BoolVar(&opts.collapsed, "collapsed", false, "show collapsed tree with counters only")
	cmd.Flags().BoolVar(&opts.inProgress, "inprogress", false, "expand only in-progress phases and sequences")

	// Add subcommands for status directories
	cmd.AddCommand(newShowActiveCommand(opts))
	cmd.AddCommand(newShowPlannedCommand(opts))
	cmd.AddCommand(newShowCompletedCommand(opts))
	cmd.AddCommand(newShowDungeonCommand(opts))
	cmd.AddCommand(newShowAllCommand(opts))

	return cmd
}

func newShowActiveCommand(opts *showOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "active",
		Short: "List festivals in active/ directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShowStatus(cmd.Context(), "active", opts)
		},
	}
	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	return cmd
}

func newShowPlannedCommand(opts *showOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "planning",
		Short: "List festivals in planning/ directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShowStatus(cmd.Context(), "planning", opts)
		},
	}
	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	return cmd
}

func newShowCompletedCommand(opts *showOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completed",
		Short: "List completed festivals",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShowStatus(cmd.Context(), "dungeon/completed", opts)
		},
	}
	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	return cmd
}

func newShowDungeonCommand(opts *showOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dungeon [substatus]",
		Short: "List festivals in dungeon/ directory",
		Long: `List festivals in dungeon/ directory.

Optionally specify a substatus: completed, archived, someday.
Without a substatus, lists all dungeon festivals.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return runShowStatus(cmd.Context(), "dungeon/"+args[0], opts)
			}
			return runShowDungeon(cmd.Context(), opts)
		},
	}
	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	return cmd
}

func runShowDungeon(ctx context.Context, opts *showOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil {
		return errors.Wrap(err, "finding festivals directory")
	}
	if festivalsDir == "" {
		return errors.NotFound("festivals directory")
	}

	campaignRoot, _ := workspace.DetectCampaign(ctx, "")

	allFestivals := make(map[string][]*FestivalInfo)
	dungeonStatuses := []string{"dungeon/completed", "dungeon/archived", "dungeon/someday"}

	for _, status := range dungeonStatuses {
		festivals, err := ListFestivalsByStatus(ctx, festivalsDir, status, campaignRoot)
		if err != nil {
			continue
		}
		allFestivals[status] = festivals
	}

	if opts.json {
		return emitAllFestivalsJSON(allFestivals, dungeonStatuses, campaignRoot)
	}
	return emitAllFestivalsText(allFestivals, dungeonStatuses)
}

func newShowAllCommand(opts *showOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all",
		Short: "List all festivals grouped by status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShowAll(cmd.Context(), opts)
		},
	}
	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	return cmd
}

func runShowCurrent(ctx context.Context, opts *showOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	campaignRoot, _ := workspace.DetectCampaign(ctx, "")

	// Try to resolve festival path using link resolution
	// This handles: 1) explicit path, 2) linked project, 3) festival directory
	festivalPath, resolveErr := shared.ResolveFestivalPath(cwd, "")

	var festival *FestivalInfo
	if resolveErr == nil && festivalPath != "" {
		// Successfully resolved - use the resolved path
		festival, err = DetectCurrentFestival(ctx, festivalPath, campaignRoot)
	} else {
		// Fall back to direct detection from cwd
		festival, err = DetectCurrentFestival(ctx, cwd, campaignRoot)
	}

	if err != nil {
		if errors.Is(err, errors.ErrCodeNotFound) {
			if opts.json {
				return emitShowErrorJSON("not in a festival directory or linked project")
			}
			return errors.NotFound("festival").WithOp("show").
				WithField("hint", "navigate to a festival directory, use 'fest link' to link a project, or specify a festival name")
		}
		return err
	}

	// Watch mode - continuously refresh display
	if opts.watch {
		return runWatchMode(ctx, festival, opts)
	}

	if opts.json {
		return emitFestivalJSON(festival, campaignRoot)
	}
	return emitFestivalText(festival, opts, campaignRoot)
}

func runShow(ctx context.Context, target string, opts *showOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	// Find festivals directory
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil {
		return errors.Wrap(err, "finding festivals directory")
	}
	if festivalsDir == "" {
		return errors.NotFound("festivals directory")
	}

	campaignRoot, _ := workspace.DetectCampaign(ctx, "")

	// Try to find festival by name in any status directory
	festival, err := FindFestivalByName(ctx, festivalsDir, target, campaignRoot)
	if err != nil {
		if opts.json {
			return emitShowErrorJSON(fmt.Sprintf("festival '%s' not found", target))
		}
		return err
	}

	if opts.json {
		return emitFestivalJSON(festival, campaignRoot)
	}
	return emitFestivalText(festival, opts, campaignRoot)
}

func runShowStatus(ctx context.Context, status string, opts *showOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil {
		return errors.Wrap(err, "finding festivals directory")
	}
	if festivalsDir == "" {
		return errors.NotFound("festivals directory")
	}

	campaignRoot, _ := workspace.DetectCampaign(ctx, "")

	festivals, err := ListFestivalsByStatus(ctx, festivalsDir, status, campaignRoot)
	if err != nil {
		return err
	}

	if opts.json {
		return emitFestivalListJSON(status, festivals, campaignRoot)
	}
	return emitFestivalListText(status, festivals)
}

func runShowAll(ctx context.Context, opts *showOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil {
		return errors.Wrap(err, "finding festivals directory")
	}
	if festivalsDir == "" {
		return errors.NotFound("festivals directory")
	}

	campaignRoot, _ := workspace.DetectCampaign(ctx, "")

	allFestivals := make(map[string][]*FestivalInfo)
	statusOrder := id.StatusDirectories

	for _, status := range statusOrder {
		festivals, err := ListFestivalsByStatus(ctx, festivalsDir, status, campaignRoot)
		if err != nil {
			continue // Skip empty or inaccessible directories
		}
		allFestivals[status] = festivals
	}

	if opts.json {
		return emitAllFestivalsJSON(allFestivals, statusOrder, campaignRoot)
	}
	return emitAllFestivalsText(allFestivals, statusOrder)
}

// toDisplayFestival returns a copy of FestivalInfo with campaign-relative paths for serialization.
func toDisplayFestival(f *FestivalInfo, campaignRoot string) *FestivalInfo {
	copy := *f
	copy.Path = pathutil.DisplayPath(f.Path, campaignRoot)
	if f.ProjectPath != "" {
		copy.ProjectPath = pathutil.DisplayPath(f.ProjectPath, campaignRoot)
	}
	return &copy
}

// toDisplayFestivals converts a slice of FestivalInfo to display-ready copies.
func toDisplayFestivals(festivals []*FestivalInfo, campaignRoot string) []*FestivalInfo {
	result := make([]*FestivalInfo, len(festivals))
	for i, f := range festivals {
		result[i] = toDisplayFestival(f, campaignRoot)
	}
	return result
}

func emitShowErrorJSON(message string) error {
	result := map[string]interface{}{
		"error": message,
	}
	if err := shared.EncodeJSON(os.Stdout, result); err != nil {
		return errors.Wrap(err, "encoding JSON output")
	}
	return nil
}

func emitFestivalJSON(festival *FestivalInfo, campaignRoot string) error {
	output := festival
	if campaignRoot != "" {
		output = toDisplayFestival(festival, campaignRoot)
	}
	if err := shared.EncodeJSON(os.Stdout, output); err != nil {
		return errors.Wrap(err, "encoding JSON output")
	}
	return nil
}

func emitFestivalText(festival *FestivalInfo, showOpts *showOptions, campaignRoot string) error {
	verbose := shared.IsVerbose()

	// Use tree view by default, summary view with --summary flag
	if showOpts.summary {
		fmt.Println(FormatFestivalDetails(festival, verbose, campaignRoot))
		return nil
	}

	// Build and render tree view
	tree, err := BuildFestivalTree(context.Background(), festival.Path)
	if err != nil {
		// Fall back to summary view on error
		fmt.Println(FormatFestivalDetails(festival, verbose, campaignRoot))
		return nil
	}

	opts := DefaultTreeOptions()
	opts.ShowGoals = showOpts.goals
	opts.Collapsed = showOpts.collapsed
	opts.InProgress = showOpts.inProgress
	fmt.Println(RenderTree(tree, opts))
	return nil
}

func emitFestivalListJSON(status string, festivals []*FestivalInfo, campaignRoot string) error {
	output := festivals
	if campaignRoot != "" {
		output = toDisplayFestivals(festivals, campaignRoot)
	}
	result := map[string]interface{}{
		"status":    status,
		"count":     len(festivals),
		"festivals": output,
	}
	if err := shared.EncodeJSON(os.Stdout, result); err != nil {
		return errors.Wrap(err, "encoding JSON output")
	}
	return nil
}

func emitFestivalListText(status string, festivals []*FestivalInfo) error {
	fmt.Println(FormatFestivalList(status, festivals))
	return nil
}

func emitAllFestivalsJSON(allFestivals map[string][]*FestivalInfo, statusOrder []string, campaignRoot string) error {
	result := make(map[string]interface{})
	total := 0
	for _, status := range statusOrder {
		festivals := allFestivals[status]
		output := festivals
		if campaignRoot != "" {
			output = toDisplayFestivals(festivals, campaignRoot)
		}
		result[status] = map[string]interface{}{
			"count":     len(festivals),
			"festivals": output,
		}
		total += len(festivals)
	}
	result["total"] = total

	if err := shared.EncodeJSON(os.Stdout, result); err != nil {
		return errors.Wrap(err, "encoding JSON output")
	}
	return nil
}

func emitAllFestivalsText(allFestivals map[string][]*FestivalInfo, statusOrder []string) error {
	fmt.Println(FormatAllFestivals(allFestivals, statusOrder))
	return nil
}
