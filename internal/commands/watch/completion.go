package watch

import (
	"context"
	"os"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/spf13/cobra"
)

func completeWatchSelector(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	selectors, err := watchSelectorCompletions(context.Background(), cwd, toComplete)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return selectors, cobra.ShellCompDirectiveNoFileComp
}

func watchSelectorCompletions(ctx context.Context, cwd, toComplete string) ([]string, error) {
	// Dungeon festivals are terminal and never watched, so restrict completion
	// to working statuses (planning/ready/active/ritual).
	return shared.CompleteFestivalPickSelectors(ctx, cwd, toComplete, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        id.WorkingStatusDirectories,
	})
}

func formatWatchSelectorCompletions(candidates []shared.FestivalPickCandidate, toComplete string) []string {
	return shared.FormatFestivalSelectorCompletions(candidates, toComplete)
}
