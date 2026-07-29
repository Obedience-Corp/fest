package chain

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/workspace"
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
			return nil, errors.Wrap(err, "resolving festival ref").
				WithField("ref", node.Ref).WithField("id", node.ID)
		}
		resolved[node.Ref] = ResolvedFestival{Node: node, Path: path}
	}

	return resolved, nil
}

// ResolveAvailable is like Resolve but best-effort: unresolved refs are omitted
// instead of failing the whole resolution, so one missing festival does not blank
// out the rest of the chain. Use Resolve when every ref must resolve.
func ResolveAvailable(ctx context.Context, c *Chain, searchDirs []string) (map[string]ResolvedFestival, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	resolved := make(map[string]ResolvedFestival, len(c.Festivals))
	for _, node := range c.Festivals {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path, err := findFestivalDir(ctx, node.ID, searchDirs)
		if err != nil {
			continue // best-effort: omit unresolved refs
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
		dungeon := isDungeonStatusDir(dir)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if matchesFestivalID(entry.Name(), festivalID) {
				return filepath.Join(dir, entry.Name()), nil
			}
			// Dungeon dirs nest festivals under a date bucket; descend one level.
			if dungeon && isDateBucket(entry.Name()) {
				if path, ok := matchFestivalInDir(filepath.Join(dir, entry.Name()), festivalID); ok {
					return path, nil
				}
			}
		}
	}

	return "", errors.NotFound("festival").WithField("festivalID", festivalID)
}

func matchFestivalInDir(dir, festivalID string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if entry.IsDir() && matchesFestivalID(entry.Name(), festivalID) {
			return filepath.Join(dir, entry.Name()), true
		}
	}
	return "", false
}

func isDungeonStatusDir(dir string) bool {
	return workspace.IsDungeonDirName(filepath.Base(filepath.Dir(dir)))
}

// isDateBucket matches a dungeon date bucket: YYYY-MM-DD or legacy YYYY-MM
// (mirrors show.LooksLikeDateDir; kept local to avoid importing a commands pkg).
func isDateBucket(name string) bool {
	// YYYY-MM (len 7) or YYYY-MM-DD (len 10).
	if len(name) != 7 && len(name) != 10 {
		return false
	}
	if name[4] != '-' {
		return false
	}
	if len(name) == 10 && name[7] != '-' {
		return false
	}
	for i := 0; i < len(name); i++ {
		if i == 4 || (len(name) == 10 && i == 7) {
			continue
		}
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	return true
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
