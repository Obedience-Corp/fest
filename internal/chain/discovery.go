package chain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverAll loads all chain YAML files from the given festivals root.
// It searches both festivals/chains/ and festivals/dungeon/completed/chains/.
func DiscoverAll(ctx context.Context, festivalsRoot string) ([]*Chain, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	dirs := []string{
		filepath.Join(festivalsRoot, "chains"),
		filepath.Join(festivalsRoot, "dungeon", "completed", "chains"),
	}

	var all []*Chain
	for _, dir := range dirs {
		chains, err := discoverInDir(ctx, dir)
		if err != nil {
			continue
		}
		all = append(all, chains...)
	}
	return all, nil
}

// FindForFestival searches all chains in the workspace for one containing the
// given festival ID. Returns the chain, the festival's ref within it, and an error.
// If the festival is not in any chain, returns nil without error.
func FindForFestival(ctx context.Context, festivalID, festivalsRoot string) (*Chain, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}

	chains, err := DiscoverAll(ctx, festivalsRoot)
	if err != nil {
		return nil, "", fmt.Errorf("discovering chains: %w", err)
	}

	for _, c := range chains {
		for _, f := range c.Festivals {
			if f.ID == festivalID {
				return c, f.Ref, nil
			}
		}
	}

	return nil, "", nil
}

// discoverInDir reads all .yaml files in a directory and parses them as chains.
func discoverInDir(ctx context.Context, dir string) ([]*Chain, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading chains directory: %w", err)
	}

	var chains []*Chain
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		c, err := Parse(ctx, filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		chains = append(chains, c)
	}
	return chains, nil
}
