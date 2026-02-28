package chain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ResolvedFestival holds a festival node paired with its resolved filesystem path.
type ResolvedFestival struct {
	Node FestivalNode
	Path string
}

// Resolve resolves all festival references in a chain to absolute filesystem
// paths under the given festival root directories. It searches each provided
// search directory for a directory whose name contains the festival ID.
func Resolve(ctx context.Context, c *Chain, searchDirs []string) (map[string]ResolvedFestival, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	resolved := make(map[string]ResolvedFestival, len(c.Festivals))

	for _, node := range c.Festivals {
		path, err := findFestivalDir(ctx, node.ID, searchDirs)
		if err != nil {
			return nil, fmt.Errorf("resolving festival ref %q (id=%s): %w", node.Ref, node.ID, err)
		}
		resolved[node.Ref] = ResolvedFestival{Node: node, Path: path}
	}

	return resolved, nil
}

// findFestivalDir searches the given directories for a subdirectory whose name
// contains the festival ID (e.g., "hedera-foundation-HF0001" contains "HF0001").
func findFestivalDir(ctx context.Context, festivalID string, searchDirs []string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // skip unreadable directories
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if matchesFestivalID(entry.Name(), festivalID) {
				return filepath.Join(dir, entry.Name()), nil
			}
		}
	}

	return "", fmt.Errorf("festival %s not found in search directories", festivalID)
}

// matchesFestivalID checks whether a directory name matches a festival ID.
// Festival directories follow the pattern "<name>-<ID>", so we check for
// the ID as a suffix after a hyphen.
func matchesFestivalID(dirName, festivalID string) bool {
	// Check if the directory name ends with "-<festivalID>"
	suffix := "-" + festivalID
	if len(dirName) > len(suffix) && dirName[len(dirName)-len(suffix):] == suffix {
		return true
	}
	// Also match exact ID (for cases where the dir is just the ID)
	return dirName == festivalID
}
