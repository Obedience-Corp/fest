// Package progress implements the fest progress command for tracking execution progress.
package progress

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

const (
	// ProgressBarWidth defines the number of characters in the progress bar
	ProgressBarWidth = 20
)

type progressOptions struct {
	json       bool
	update     string
	complete   bool
	blocker    string
	clear      bool
	taskID     string
	taskPath   string
	phase      string
	sequence   string
	festival   string
	inProgress bool
	watch      bool
	interval   time.Duration
}

var taskFilenamePattern = regexp.MustCompile(`^\d{2}[\._].*\.md$`)

// NewProgressCommand creates the progress command
func NewProgressCommand() *cobra.Command {
	opts := &progressOptions{}

	cmd := &cobra.Command{
		Use:   "progress",
		Short: "Track and display festival execution progress",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Track and display progress for festival execution.

When run without flags, shows an overview of festival progress. 'fest progress'
is the display surface; task state mutations live under 'fest task'.

PROGRESS OVERVIEW:
  fest progress              Show festival progress summary
  fest progress --json       Output progress in JSON format
  fest progress --watch      Continuously refresh the display

DEPRECATED TASK MUTATIONS (use 'fest task' instead):
  fest progress --task <id> --complete   -> fest task completed --yes
  fest progress --task <id> --update 50% -> fest task update 50%
  fest progress --task <id> --blocker "msg" -> fest task blocked --reason "msg" --yes
  fest progress --task <id> --clear      -> fest task unblock

These flags still work for one release and print a deprecation notice.

Task IDs can be festival-relative paths (e.g. 002_FOUNDATION/01_project_scaffold/01_design.md)
or absolute paths. Use --path or --phase/--sequence to disambiguate duplicates.
Use --festival to run outside a festival directory.`,
		Example: `  fest progress                          # Show overall progress
  fest progress --json                   # Overall progress as JSON
  fest progress --watch                  # Live-refreshing progress display
  fest progress --task 01_setup.md       # Show a single task's progress`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProgress(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	cmd.Flags().StringVar(&opts.update, "update", "", "update progress percentage (e.g., 50%)")
	cmd.Flags().BoolVar(&opts.complete, "complete", false, "mark task as complete")
	cmd.Flags().StringVar(&opts.blocker, "blocker", "", "report a blocker with message")
	cmd.Flags().BoolVar(&opts.clear, "clear", false, "clear blocker for task")
	cmd.Flags().StringVar(&opts.taskID, "task", "", "task ID to update")
	cmd.Flags().StringVar(&opts.taskPath, "path", "", "task path (festival-relative or absolute)")
	cmd.Flags().StringVar(&opts.phase, "phase", "", "phase directory name for task path")
	cmd.Flags().StringVar(&opts.sequence, "sequence", "", "sequence directory name for task path")
	cmd.Flags().StringVar(&opts.festival, "festival", "", "festival root path (directory containing fest.yaml)")
	cmd.Flags().BoolVar(&opts.inProgress, "in-progress", false, "mark task as in progress")
	cmd.Flags().BoolVar(&opts.watch, "watch", false, "continuously refresh progress display")
	cmd.Flags().DurationVar(&opts.interval, "interval", 2*time.Second, "refresh interval for watch mode")

	return cmd
}

func runProgress(ctx context.Context, opts *progressOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	if err := validateTaskOptions(opts); err != nil {
		return err
	}

	// Resolve festival path from --festival flag, navigation links, or current directory
	festivalPath := opts.festival
	if festivalPath != "" && !filepath.IsAbs(festivalPath) {
		festivalPath = filepath.Join(cwd, festivalPath)
	}

	// Use shared helper to resolve festival path (supports linked festivals)
	resolvedFestivalPath, resolveErr := shared.ResolveFestivalPath(cwd, festivalPath)
	if resolveErr != nil {
		if opts.festival != "" || opts.taskID != "" || opts.taskPath != "" {
			return errors.Wrap(resolveErr, "detecting festival location")
		}
		picked, pickErr := pickFestivalForProgress(ctx, cwd)
		if pickErr != nil {
			return pickErr
		}
		if picked == "" {
			return nil
		}
		resolvedFestivalPath = picked
	}

	targetPath := cwd
	if resolveErr != nil {
		targetPath = resolvedFestivalPath
	}
	if opts.taskPath != "" {
		resolvedTaskPath, err := resolveTaskPath(opts.taskPath, resolvedFestivalPath, cwd)
		if err != nil {
			return err
		}
		opts.taskPath = resolvedTaskPath
		targetPath = resolvedTaskPath
	}

	// Detect current location based on working directory (or task path if specified)
	loc, err := show.DetectCurrentLocation(ctx, targetPath)
	if err != nil {
		return errors.Wrap(err, "detecting festival location")
	}

	if loc.Festival == nil {
		return errors.NotFound("festival").
			WithField("hint", "run from inside a festival directory")
	}

	// Create progress manager. Mutation flows install the lifecycle gate so
	// pre-active festivals reject task changes; read-only flows do not need it.
	var mgr *progress.Manager
	if opts.taskID != "" || opts.taskPath != "" {
		mgr, err = progress.NewManagerWithGate(ctx, loc.Festival.Path,
			lifecycle.NewGateWithReason(loc.Festival.Path, "fest progress"))
	} else {
		mgr, err = progress.NewManager(ctx, loc.Festival.Path)
	}
	if err != nil {
		return errors.Wrap(err, "initializing progress manager")
	}

	// Handle task updates
	if opts.taskID != "" || opts.taskPath != "" {
		return handleTaskUpdate(ctx, mgr, loc.Festival.Path, opts)
	}

	// Watch mode - continuously refresh progress
	if opts.watch {
		return runWatchMode(ctx, mgr, loc, opts)
	}

	// Show progress overview
	return showProgressOverview(ctx, mgr, loc, opts)
}

func pickFestivalForProgress(ctx context.Context, cwd string) (string, error) {
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return "", errors.NotFound("festival").
			WithHint("navigate to a festival directory or a campaign workspace")
	}
	return shared.PickFestivalPath(ctx, festivalsDir, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        shared.WorkingFestivalPickerStatuses,
		FallbackStatuses:         shared.WorkingFestivalPickerStatuses,
		OrderByStatusThenRecency: true,
	})
}
