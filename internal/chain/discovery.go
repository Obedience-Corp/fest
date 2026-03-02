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
// Parse errors for individual files are silently skipped (use DiscoverAllStrict
// when parse failures must be surfaced, e.g. for validation).
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
		chains, _, err := discoverInDir(ctx, dir)
		if err != nil {
			continue
		}
		all = append(all, chains...)
	}
	return all, nil
}

// ParseError records a file that failed to parse as a chain.
type ParseError struct {
	File string
	Err  error
}

func (e ParseError) Error() string {
	return fmt.Sprintf("%s: %s", filepath.Base(e.File), e.Err)
}

// DiscoverAllStrict is like DiscoverAll but also returns parse errors for
// files that could not be loaded. Callers that need to surface malformed
// chain files (e.g. validate --cross) should use this variant.
func DiscoverAllStrict(ctx context.Context, festivalsRoot string) ([]*Chain, []ParseError, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	dirs := []string{
		filepath.Join(festivalsRoot, "chains"),
		filepath.Join(festivalsRoot, "dungeon", "completed", "chains"),
	}

	var all []*Chain
	var parseErrors []ParseError
	for _, dir := range dirs {
		chains, errs, err := discoverInDir(ctx, dir)
		if err != nil {
			continue
		}
		all = append(all, chains...)
		parseErrors = append(parseErrors, errs...)
	}
	return all, parseErrors, nil
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
// Returns successfully parsed chains and any parse errors encountered.
func discoverInDir(ctx context.Context, dir string) ([]*Chain, []ParseError, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading chains directory: %w", err)
	}

	var chains []*Chain
	var parseErrors []ParseError
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		c, err := Parse(ctx, path)
		if err != nil {
			parseErrors = append(parseErrors, ParseError{File: path, Err: err})
			continue
		}
		chains = append(chains, c)
	}
	return chains, parseErrors, nil
}
