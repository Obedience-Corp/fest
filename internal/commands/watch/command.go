// Package watch implements the dev-gated fest watch command.
package watch

import (
	"context"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

type options struct {
	goals     bool
	collapsed bool
	summary   bool
}

type commandDeps struct {
	resolve func(context.Context, string) (*show.FestivalInfo, error)
	watch   func(context.Context, *show.FestivalInfo, show.WatchOptions) error
}

// NewWatchCommand creates the watch command.
func NewWatchCommand() *cobra.Command {
	return newWatchCommand(defaultCommandDeps())
}

func defaultCommandDeps() commandDeps {
	return commandDeps{
		resolve: func(ctx context.Context, selector string) (*show.FestivalInfo, error) {
			return defaultResolver().resolve(ctx, selector)
		},
		watch: show.WatchFestival,
	}
}

func newWatchCommand(deps commandDeps) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "watch [festival-selector]",
		Short: "Watch a festival's in-progress work",
		Long: `Watch the in-progress state of a festival.

With no selector, fest watch resolves the current festival context or linked
project context. From broader workspace context it opens a festival picker.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeWatchSelector,
		Annotations: map[string]string{
			"scope":        string(scope.Workspace),
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

	festival, err := deps.resolve(ctx, selector)
	if err != nil {
		if isWatchPickerCancelled(err) {
			return nil
		}
		return err
	}
	return watchFestival(ctx, festival, *opts, deps)
}

func watchFestival(ctx context.Context, festival *show.FestivalInfo, opts options, deps commandDeps) error {
	if festival == nil {
		return nil
	}
	return deps.watch(ctx, festival, showWatchOptions(opts))
}

func showWatchOptions(opts options) show.WatchOptions {
	return show.WatchOptions{
		Summary:    opts.summary,
		Goals:      opts.goals,
		Collapsed:  opts.collapsed,
		InProgress: true,
	}
}
