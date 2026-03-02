package chain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/id"
	tpl "github.com/Obedience-Corp/fest/internal/template"
)

// resolveChainStatuses resolves live festival statuses for a chain by reading
// each festival's fest.yaml from the filesystem. Returns a map keyed by ref.
// Unresolvable festivals default to FestivalPlanning with a printed warning.
func resolveChainStatuses(ctx context.Context, c *chainpkg.Chain) (map[string]chainpkg.FestivalStatus, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	root, err := tpl.FindFestivalsRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("finding festivals root: %w", err)
	}

	// Build search directories from all lifecycle status paths.
	searchDirs := make([]string, len(id.StatusDirectories))
	for i, d := range id.StatusDirectories {
		searchDirs[i] = filepath.Join(root, d)
	}

	// Resolve festival refs to filesystem paths.
	resolved, resolveErr := chainpkg.Resolve(ctx, c, searchDirs)

	statuses := make(map[string]chainpkg.FestivalStatus, len(c.Festivals))

	for _, node := range c.Festivals {
		rf, ok := resolved[node.Ref]
		if !ok || rf.Path == "" {
			// Unresolvable — default to planning with warning.
			fmt.Fprintf(os.Stderr, "warning: could not resolve festival %s (%s), defaulting to planning\n", node.Ref, node.ID)
			statuses[node.Ref] = chainpkg.FestivalPlanning
			continue
		}

		cfg, cfgErr := config.LoadFestivalConfig(rf.Path, "")
		if cfgErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load config for %s (%s): %s\n", node.Ref, node.ID, cfgErr)
			statuses[node.Ref] = chainpkg.FestivalPlanning
			continue
		}

		statuses[node.Ref] = mapStatus(cfg.Metadata.CurrentStatus())
	}

	// If Resolve returned a hard error and we got zero resolved entries, propagate it.
	if resolveErr != nil && len(resolved) == 0 {
		return statuses, resolveErr
	}

	return statuses, nil
}

// mapStatus converts a string status from fest.yaml to a chain FestivalStatus constant.
func mapStatus(s string) chainpkg.FestivalStatus {
	switch s {
	case "planning":
		return chainpkg.FestivalPlanning
	case "ready":
		return chainpkg.FestivalReady
	case "active":
		return chainpkg.FestivalActive
	case "completed", "dungeon/completed":
		return chainpkg.FestivalCompleted
	default:
		return chainpkg.FestivalPlanning
	}
}

// statusIcon returns a short icon for a festival status.
func statusIcon(s chainpkg.FestivalStatus) string {
	switch s {
	case chainpkg.FestivalCompleted:
		return "[done]"
	case chainpkg.FestivalActive:
		return "[active]"
	case chainpkg.FestivalReady:
		return "[ready]"
	default:
		return "[planning]"
	}
}
