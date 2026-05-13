// Package watch implements the dev-gated fest watch command.
package watch

import (
	"context"
	"errors"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

type options struct {
	goals     bool
	collapsed bool
	summary   bool
}

// NewWatchCommand creates the watch command.
func NewWatchCommand() *cobra.Command {
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
			return runWatch(cmd.Context(), args, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.goals, "goals", false, "show goals for phases and sequences")
	cmd.Flags().BoolVar(&opts.collapsed, "collapsed", false, "show collapsed tree with counters only")
	cmd.Flags().BoolVar(&opts.summary, "summary", false, "show aggregate summary instead of tree view")

	return cmd
}

func runWatch(ctx context.Context, args []string, opts *options) error {
	selector := ""
	if len(args) > 0 {
		selector = args[0]
	}

	festival, err := defaultResolver().resolve(ctx, selector)
	if err != nil {
		return err
	}
	return watchFestival(ctx, festival, *opts)
}

func watchFestival(ctx context.Context, _ *show.FestivalInfo, _ options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("fest watch is not implemented yet")
}
