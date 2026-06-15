package watch

import (
	"context"
	"os"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
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
	// Restrict completion to watchable statuses (active, ready, planning).
	// Ritual templates and terminal dungeon/* festivals are never watched.
	// No ordering option here: completion stays alphabetical and avoids the
	// per-candidate filesystem stats the interactive picker performs.
	return shared.CompleteFestivalPickSelectors(ctx, cwd, toComplete, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        watchPickerStatuses,
	})
}

func formatWatchSelectorCompletions(candidates []shared.FestivalPickCandidate, toComplete string) []string {
	return shared.FormatFestivalSelectorCompletions(candidates, toComplete)
}
