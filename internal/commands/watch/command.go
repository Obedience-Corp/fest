// Package watch implements the fest watch command.
package watch

import (
	"context"
	"os"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

type options struct {
	goals     bool
	collapsed bool
	summary   bool
}

type commandDeps struct {
	resolve           func(context.Context, string) (*show.FestivalInfo, error)
	resolveStandalone func(context.Context) (*show.StandaloneWorkflowInfo, error)
	watch             func(context.Context, *show.FestivalInfo, show.WatchOptions) error
	watchStandalone   func(context.Context, *show.StandaloneWorkflowInfo, show.WatchOptions) error
	detectFestival    func(context.Context, string) (*show.FestivalInfo, error)
	listCycleTargets  func(context.Context, string) ([]string, error)
}

// NewWatchCommand creates the watch command.
func NewWatchCommand() *cobra.Command {
	return newWatchCommand(defaultCommandDeps())
}

func defaultCommandDeps() commandDeps {
	r := defaultResolver()
	return commandDeps{
		resolve: func(ctx context.Context, selector string) (*show.FestivalInfo, error) {
			return r.resolve(ctx, selector)
		},
		resolveStandalone: defaultResolveStandaloneWorkflow,
		watch:             show.WatchFestival,
		watchStandalone:   show.WatchStandaloneWorkflow,
		detectFestival: func(ctx context.Context, path string) (*show.FestivalInfo, error) {
			return r.detectFestival(ctx, path)
		},
		listCycleTargets: defaultListCycleTargets,
	}
}

func defaultResolveStandaloneWorkflow(ctx context.Context) (*show.StandaloneWorkflowInfo, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.IO("getting current directory", err)
	}
	return show.ResolveStandaloneWorkflow(ctx, cwd)
}

func newWatchCommand(deps commandDeps) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "watch [festival-selector]",
		Short: "Watch a festival's in-progress work",
		Long: `Watch the in-progress state of a festival.

With a selector, fest watch resolves a festival by directory name or logical ID.
Without a selector, it watches the current festival when run from a festival
directory, the linked festival when run from a linked project directory, or a
standalone WORKFLOW.md from that workflow directory.

From a campaign or festivals workspace in an interactive terminal, fest watch
opens a festival picker. Watch mode refreshes in place until you press Ctrl+C.
It does not change your shell directory.`,
		Example: `  fest watch
  fest watch my-festival
  fest watch GS0001`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeWatchSelector,
		Annotations: map[string]string{
			"scope":        string(scope.Global),
			"interactive":  "true",
			"long_running": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatch(cmd.Context(), args, opts, deps)
		},
	}

	cmd.Flags().BoolVar(&opts.goals, "goals", false, "show goals for phases and sequences")
	cmd.Flags().BoolVar(&opts.collapsed, "collapsed", false, "show collapsed tree with counters only")
	cmd.Flags().BoolVar(&opts.summary, "summary", false, "show aggregate summary instead of tree view")

	return cmd
}

func runWatch(ctx context.Context, args []string, opts *options, deps commandDeps) error {
	selector := ""
	if len(args) > 0 {
		selector = args[0]
	}

	if selector == "" && deps.resolveStandalone != nil {
		workflow, err := deps.resolveStandalone(ctx)
		if err != nil {
			return err
		}
		if workflow != nil {
			return watchStandaloneWorkflow(ctx, workflow, *opts, deps)
		}
	}

	if selector == "" && deps.listCycleTargets != nil {
		cwd, err := os.Getwd()
		if err == nil {
			paths, listErr := deps.listCycleTargets(ctx, cwd)
			if listErr == nil && len(paths) > 1 {
				return runWatchCycle(ctx, paths, currentFestivalIndex(ctx, cwd, paths, deps), *opts, deps)
			}
		}
	}

	festival, err := deps.resolve(ctx, selector)
	if err != nil {
		if isWatchPickerCancelled(err) {
			return nil
		}
		return err
	}
	return watchFestival(ctx, festival, *opts, deps)
}

func currentFestivalIndex(ctx context.Context, cwd string, paths []string, deps commandDeps) int {
	if deps.detectFestival == nil {
		return 0
	}
	festival, err := deps.detectFestival(ctx, cwd)
	if err != nil || festival == nil {
		return 0
	}
	target := canonicalWatchPath(festival.Path)
	if target == "" {
		return 0
	}
	for i, path := range paths {
		if canonicalWatchPath(path) == target {
			return i
		}
	}
	return 0
}

func watchFestival(ctx context.Context, festival *show.FestivalInfo, opts options, deps commandDeps) error {
	return deps.watch(ctx, festival, showWatchOptions(opts))
}

func watchStandaloneWorkflow(ctx context.Context, workflow *show.StandaloneWorkflowInfo, opts options, deps commandDeps) error {
	if deps.watchStandalone == nil {
		return errors.Validation("standalone watch delegate is required")
	}
	return deps.watchStandalone(ctx, workflow, showWatchOptions(opts))
}

func showWatchOptions(opts options) show.WatchOptions {
	return show.WatchOptions{
		Summary:    opts.summary,
		Goals:      opts.goals,
		Collapsed:  opts.collapsed,
		InProgress: true,
	}
}
