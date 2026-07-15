// Package show implements the fest show command for displaying festival information.
package show

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type showOptions struct {
	json         bool
	summary      bool // Show aggregate summary instead of tree view
	watch        bool // Continuously refresh display
	goals        bool // Show goals for phases and sequences
	showFeedback bool // Show blocked-step feedback
	collapsed    bool // Show collapsed tree with counters only
	inProgress   bool // Expand only in_progress phases/sequences
	roadmap      bool // Show full execution roadmap with task statuses
	festival     string
}

// NewShowCommand creates the show command with all subcommands.
func NewShowCommand() *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:   "show [festival-name]",
		Short: "Display festival information",
		Long: `Display festival information for a single festival.

When run inside a festival directory, shows the current festival's details.
When run outside a festival in an interactive campaign workspace, opens a
cyclable view; use ←/→ to move between festivals and q/Ctrl+C to exit.

  fest show                        Show current festival, or cycle festivals in a workspace
  fest show <name>                 Show details of a specific festival by name
  fest show --festival <selector>  Show a festival by explicit selector (campaign workspace)

To list festivals by status, use 'fest list' (e.g. 'fest list active',
'fest list all', 'fest list dungeon/completed').`,
		Example: `  fest show
  fest show launch-readiness
  fest show --festival LR0001`,
		Annotations: map[string]string{
			"scope": string(scope.Global),
		},
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeShowFestivalSelector,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.festival != "" && len(args) > 0 {
				return errors.Validation("cannot use positional target with --festival")
			}

			if opts.festival != "" {
				return runShowBySelector(cmd.Context(), opts.festival, opts)
			}

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
	cmd.Flags().BoolVar(&opts.showFeedback, "show-feedback", false, "show blocked-step feedback")
	cmd.Flags().BoolVar(&opts.collapsed, "collapsed", false, "show collapsed tree with counters only")
	cmd.Flags().BoolVar(&opts.inProgress, "inprogress", false, "expand only in-progress phases and sequences")
	cmd.Flags().BoolVar(&opts.roadmap, "roadmap", false, "show full execution roadmap with task statuses")
	cmd.Flags().StringVar(&opts.festival, "festival", "", "festival selector (name or ID) from within a campaign workspace")
	_ = cmd.RegisterFlagCompletionFunc("festival", completeShowFestivalSelector)

	// Hidden, deprecated status aliases: the listing surface now lives on
	// `fest list <status>`. They keep working (fest has production users) but no
	// longer occupy the tab-completion surface, which is reserved for festivals.
	cmd.AddCommand(newShowActiveCommand(opts))
	cmd.AddCommand(newShowPlannedCommand(opts))
	cmd.AddCommand(newShowCompletedCommand(opts))
	cmd.AddCommand(newShowDungeonCommand(opts))
	cmd.AddCommand(newShowAllCommand(opts))

	// Hidden helpers backing the zsh `fest show <tab>` widget.
	cmd.AddCommand(newShowCompletionsCommand())
	cmd.AddCommand(newShowPickCommand())

	return cmd
}

func newShowActiveCommand(opts *showOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "active",
		Short:      "List festivals in active/ directory",
		Hidden:     true,
		Deprecated: "use 'fest list active' instead",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShowStatus(cmd.Context(), "active", opts)
		},
	}
	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	return cmd
}

func newShowPlannedCommand(opts *showOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "planning",
		Short:      "List festivals in planning/ directory",
		Hidden:     true,
		Deprecated: "use 'fest list planning' instead",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShowStatus(cmd.Context(), "planning", opts)
		},
	}
	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	return cmd
}

func newShowCompletedCommand(opts *showOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "completed",
		Short:      "List completed festivals",
		Hidden:     true,
		Deprecated: "use 'fest list completed' instead",
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
		Hidden:     true,
		Deprecated: "use 'fest list dungeon' instead",
		Args:       cobra.MaximumNArgs(1),
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

	if err := workspace.CheckDungeonConflict(festivalsDir); err != nil {
		return err
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
		Use:        "all",
		Short:      "List all festivals grouped by status",
		Hidden:     true,
		Deprecated: "use 'fest list all' instead",
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

	handled, err := runShowStandaloneCurrent(ctx, cwd, opts)
	if err != nil || handled {
		return err
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
			return runShowCycle(ctx, cwd, opts, campaignRoot)
		}
		return err
	}

	// Watch mode - continuously refresh display
	return emitShowFestival(ctx, festival, opts, campaignRoot)
}

// runShowCycle enters the arrow-key cycle over browseable festivals when bare
// `fest show` is run outside any festival in an interactive terminal. Outside a
// terminal, or when no festivals exist, it returns the standard not-found hint.
func runShowCycle(ctx context.Context, cwd string, opts *showOptions, campaignRoot string) error {
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return notInFestivalError()
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return notInFestivalError()
	}
	paths := browseCycleTargets(festivalsDir)
	if len(paths) == 0 {
		return notInFestivalError()
	}
	return RunCycle(ctx, paths, 0, CycleOptions{
		Detect: func(ctx context.Context, path string) (*FestivalInfo, error) {
			return DetectCurrentFestival(ctx, path, campaignRoot)
		},
		Render: func(ctx context.Context, festival *FestivalInfo, cycling bool, frame *FrameState) error {
			watchOpts := showCycleWatchOptions(opts, cycling)
			watchOpts.Frame = frame
			return WatchFestival(ctx, festival, watchOpts)
		},
		RenderFallback: func(_ context.Context, festival *FestivalInfo) error {
			return emitFestivalText(festival, opts, campaignRoot)
		},
		ExtraKeys: showCycleExtraKeys(),
	})
}

// browseCycleTargets returns the festival directories cycled by bare `fest show`,
// ordered by status (active, ready, planning, ritual) then recency.
func browseCycleTargets(festivalsDir string) []string {
	items := shared.FestivalPickerItems(festivalsDir, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        shared.BrowseFestivalPickerStatuses,
		FallbackStatuses:         shared.BrowseFestivalPickerStatuses,
		OrderByStatusThenRecency: true,
	})
	paths := make([]string, 0, len(items))
	for _, item := range items {
		if item.Value != "" {
			paths = append(paths, item.Value)
		}
	}
	return paths
}

// showCycleWatchOptions renders each cycle frame with show's display flags
// (default full tree, not watch's in-progress view) and no promote hint.
func showCycleWatchOptions(opts *showOptions, cycling bool) WatchOptions {
	return WatchOptions{
		Summary:      opts.summary,
		Goals:        opts.goals,
		ShowFeedback: opts.showFeedback,
		Collapsed:    opts.collapsed,
		InProgress:   opts.inProgress,
		CycleHint:    true,
		Cycling:      cycling,
		CyclePromote: false,
	}
}

// showCycleExtraKeys adds q/Q as quit keys for show's cycle. Promote is
// deliberately absent so the engine stays free of the promote package.
func showCycleExtraKeys() map[byte]ExtraKeyHandler {
	quit := func(context.Context, *FestivalInfo, *term.State) (string, bool) {
		return "", false
	}
	return map[byte]ExtraKeyHandler{'q': quit, 'Q': quit}
}

func notInFestivalError() error {
	return errors.NotFound("festival").WithOp("show").
		WithHint("navigate to a festival directory, use 'fest link' to link a project, pass --festival <selector>, or run from a terminal in a campaign workspace to cycle festivals")
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

	return emitShowFestival(ctx, festival, opts, campaignRoot)
}

func runShowBySelector(ctx context.Context, selector string, opts *showOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	festivalPath, err := shared.ResolveFestivalSelector(ctx, cwd, selector)
	if err != nil {
		if opts.json {
			return emitShowErrorJSON(err.Error())
		}
		return err
	}

	campaignRoot, _ := workspace.DetectCampaign(ctx, "")
	festival, err := DetectCurrentFestival(ctx, festivalPath, campaignRoot)
	if err != nil {
		if opts.json {
			return emitShowErrorJSON(err.Error())
		}
		return err
	}

	return emitShowFestival(ctx, festival, opts, campaignRoot)
}

func emitShowFestival(ctx context.Context, festival *FestivalInfo, opts *showOptions, campaignRoot string) error {
	// Watch mode - continuously refresh display
	if opts.watch {
		return runWatchMode(ctx, festival, opts, false, false, false, nil)
	}

	// Roadmap mode - full execution plan
	if opts.roadmap {
		if opts.json {
			return emitRoadmapJSON(ctx, festival, campaignRoot)
		}
		return emitRoadmapText(ctx, festival)
	}

	if opts.json {
		return emitFestivalJSON(ctx, festival, opts, campaignRoot)
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

	if strings.HasPrefix(status, "dungeon/") {
		if err := workspace.CheckDungeonConflict(festivalsDir); err != nil {
			return err
		}
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

	if err := workspace.CheckDungeonConflict(festivalsDir); err != nil {
		return err
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

func completeShowFestivalSelector(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	selectors, err := shared.CompleteFestivalSelector(context.Background(), cwd, toComplete)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return selectors, cobra.ShellCompDirectiveNoFileComp
}

// showPickerOptions is the candidate set for `fest show completions` and
// `fest show pick`: browseable festivals ordered by status then recency, matching
// the bare-`fest show` cycle.
func showPickerOptions() shared.FestivalPickerOptions {
	return shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        shared.BrowseFestivalPickerStatuses,
		FallbackStatuses:         shared.BrowseFestivalPickerStatuses,
		OrderByStatusThenRecency: true,
	}
}

// newShowCompletionsCommand creates the hidden subcommand that emits colorized,
// status-ordered festival completions for the zsh `fest show <tab>` widget,
// mirroring `fest promote completions --color`.
func newShowCompletionsCommand() *cobra.Command {
	var color bool
	cmd := &cobra.Command{
		Use:    "completions",
		Short:  "Output show completion words for shell integration",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runShowCompletions(cmd.Context(), color)
		},
	}
	cmd.Flags().BoolVar(&color, "color", false, "output value\\tcolorized_display for zsh compadd")
	return cmd
}

func runShowCompletions(ctx context.Context, color bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	candidates, err := shared.ListFestivalPickCandidates(ctx, cwd, showPickerOptions())
	if err != nil {
		return nil
	}
	if color {
		for _, line := range shared.ColorSelectorCompletions(candidates, "") {
			fmt.Println(line)
		}
		return nil
	}
	for _, name := range shared.OrderedSelectorNames(candidates, "") {
		fmt.Println(name)
	}
	return nil
}

// newShowPickCommand creates the hidden subcommand backing the zsh tab widget: it
// runs the festival picker TUI (rendered on the tty) and prints the picked
// festival's directory name to stdout. --print-path emits the absolute path
// instead. Outside a terminal it exits non-zero with a hint and empty stdout; a
// cancelled pick exits 0 with empty stdout.
func newShowPickCommand() *cobra.Command {
	var printPath bool
	cmd := &cobra.Command{
		Use:    "pick",
		Short:  "Pick a festival interactively and print its name",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runShowPick(cmd.Context(), printPath)
		},
	}
	cmd.Flags().BoolVar(&printPath, "print-path", false, "print the festival's absolute path instead of its name")
	return cmd
}

func runShowPick(ctx context.Context, printPath bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return errors.NotFound("festivals directory").
			WithHint("run from a campaign workspace with a festivals/ directory")
	}

	picked, outcome, err := shared.PickFestival(ctx, festivalsDir, showPickerOptions())
	if err != nil {
		return err
	}
	switch outcome {
	case shared.FestivalPicked:
		if printPath {
			fmt.Println(picked)
			return nil
		}
		fmt.Println(filepath.Base(picked))
		return nil
	case shared.FestivalPickCancelled:
		return nil
	default:
		return errors.NotFound("festival").WithOp("show pick").
			WithHint("run from an interactive terminal in a campaign workspace to pick a festival")
	}
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
	return errors.ErrAlreadyPrinted
}

type festivalShowJSON struct {
	*FestivalInfo
	View *festivalShowView `json:"view,omitempty"`
}

type festivalShowView struct {
	Mode    string                  `json:"mode"`
	Options festivalShowViewOptions `json:"options"`
	Tree    *DisplayNode            `json:"tree,omitempty"`
	Error   string                  `json:"error,omitempty"`
}

type festivalShowViewOptions struct {
	Summary      bool `json:"summary"`
	ShowGoals    bool `json:"show_goals"`
	ShowFeedback bool `json:"show_feedback"`
	Collapsed    bool `json:"collapsed"`
	InProgress   bool `json:"in_progress"`
}

func emitFestivalJSON(ctx context.Context, festival *FestivalInfo, opts *showOptions, campaignRoot string) error {
	output := festival
	if campaignRoot != "" {
		output = toDisplayFestival(festival, campaignRoot)
	}

	result := &festivalShowJSON{
		FestivalInfo: output,
		View:         buildFestivalShowView(ctx, festival, opts),
	}
	if err := shared.EncodeJSON(os.Stdout, result); err != nil {
		return errors.Wrap(err, "encoding JSON output")
	}
	return nil
}

func buildFestivalShowView(ctx context.Context, festival *FestivalInfo, opts *showOptions) *festivalShowView {
	if opts == nil {
		opts = &showOptions{}
	}

	view := &festivalShowView{
		Mode: "tree",
		Options: festivalShowViewOptions{
			Summary:      opts.summary,
			ShowGoals:    opts.goals,
			ShowFeedback: opts.showFeedback,
			Collapsed:    opts.collapsed,
			InProgress:   opts.inProgress,
		},
	}

	if opts.summary {
		view.Mode = "summary"
		return view
	}

	tree, err := BuildFestivalTree(ctx, festival.Path)
	if err != nil {
		view.Mode = "summary"
		view.Error = err.Error()
		return view
	}

	if opts.inProgress {
		markNextPending(tree)
	}
	if !opts.showFeedback {
		hideTreeFeedback(tree)
	}
	view.Tree = tree
	return view
}

func hideTreeFeedback(node *DisplayNode) {
	if node == nil {
		return
	}
	node.Feedback = ""
	for _, child := range node.Children {
		hideTreeFeedback(child)
	}
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
	opts.ShowFeedback = showOpts.showFeedback
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
