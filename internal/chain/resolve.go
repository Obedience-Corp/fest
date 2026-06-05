package chain

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
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

// ResolveAvailable resolves festival references best-effort: refs whose
// directory cannot be found are omitted from the result rather than failing the
// entire resolution (as Resolve does). Use this when partial chain state is
// acceptable — computing live statuses for display or dependency gates, where
// one missing or not-yet-created festival should not blank out the rest of the
// chain. Use Resolve when every ref must resolve (e.g. chain validation).
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
			// Dungeon status dirs interpose a dated bucket between the status
			// dir and the festival (dungeon/<status>/YYYY-MM-DD/<festival>), so
			// descend one level into date buckets to match festivals there.
			if dungeon && isDateBucket(entry.Name()) {
				if path, ok := matchFestivalInDir(filepath.Join(dir, entry.Name()), festivalID); ok {
					return path, nil
				}
			}
		}
	}

	return "", errors.NotFound("festival").WithField("festivalID", festivalID)
}

// matchFestivalInDir returns the path of a festival directory matching
// festivalID located directly inside dir, if present.
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

// isDungeonStatusDir reports whether dir is a dungeon status directory
// (festivals/dungeon/{completed,archived,someday}), which holds festivals one
// level deeper inside dated YYYY-MM-DD buckets.
func isDungeonStatusDir(dir string) bool {
	return filepath.Base(filepath.Dir(dir)) == "dungeon"
}

// isDateBucket reports whether name is a YYYY-MM-DD dungeon bucket directory.
func isDateBucket(name string) bool {
	if len(name) != 10 || name[4] != '-' || name[7] != '-' {
		return false
	}
	for i := 0; i < len(name); i++ {
		if i == 4 || i == 7 {
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
