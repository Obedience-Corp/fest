package watch

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/resolver"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// watchResolver wraps the shared TargetResolver, preserving the watch
// command's picker configuration (cwd-scoped status narrowing + fallback to
// the watchable statuses). All resolution logic lives in the resolver package;
// this file configures it and holds the watch-specific helpers the resolver
// does not own (cycle targets, status narrowing).
type watchResolver struct {
	resolver resolver.TargetResolver
}

func defaultResolver() watchResolver {
	return watchResolver{
		resolver: resolver.DefaultTargetResolver(resolver.TargetResolverOptions{
			PickerOptions: watchPickerOptions,
		}),
	}
}

func (r watchResolver) resolve(ctx context.Context, selector string) (*show.FestivalInfo, error) {
	festival, _, err := r.resolver.Resolve(ctx, selector)
	return festival, err
}

func (r watchResolver) detectFestival(ctx context.Context, path string) (*show.FestivalInfo, error) {
	return show.DetectCurrentFestival(ctx, path, "")
}

// watchPickerStatuses are watch targets, in display priority order. Ritual is a
// template (run into active/ first) and dungeon is terminal, so neither is watched.
var watchPickerStatuses = []string{"active", "ready", "planning"}

// watchPickerOptions returns the picker configuration for a given cwd and
// festivals directory: cwd-scoped status narrowing (so a user browsing
// festivals/active sees only active festivals), with a fallback to the full
// watchable status set.
func watchPickerOptions(cwd, festivalsDir string) shared.FestivalPickerOptions {
	return shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        pickerStatuses(cwd, festivalsDir),
		FallbackStatuses:         watchPickerStatuses,
		OrderByStatusThenRecency: true,
	}
}

func pickerStatuses(cwd, festivalsDir string) []string {
	if narrowed := preferredPickerStatuses(cwd, festivalsDir); len(narrowed) > 0 {
		return narrowed
	}
	return watchPickerStatuses
}

func preferredPickerStatuses(cwd, festivalsDir string) []string {
	rel, err := filepath.Rel(festivalsDir, cwd)
	if err != nil || rel == "." || rel == ".." {
		return nil
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") {
		return nil
	}
	for _, status := range watchPickerStatuses {
		status = filepath.ToSlash(id.ResolveStatusPath(status))
		if rel == status || strings.HasPrefix(rel, status+"/") {
			return []string{status}
		}
	}
	return nil
}

// canonicalWatchPath resolves symlinks and returns an absolute, cleaned path.
// Delegates to the resolver CanonicalPath helper.
func canonicalWatchPath(p string) string {
	return resolver.CanonicalPath(p)
}

func isWatchPickerCancelled(err error) bool {
	return resolver.IsPickerCancelled(err)
}

func defaultListCycleTargets(ctx context.Context, cwd string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return nil, nil
	}
	items := shared.FestivalPickerItems(festivalsDir, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        watchPickerStatuses,
		FallbackStatuses:         watchPickerStatuses,
		OrderByStatusThenRecency: true,
	})
	paths := make([]string, 0, len(items))
	for _, item := range items {
		paths = append(paths, item.Value)
	}
	return paths, nil
}
