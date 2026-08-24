package ritual

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

type convertMatch struct {
	path   string
	status string
	name   string
}

// findFestivalForConvert searches all status directories for a matching festival.
// Resolution order: unique exact directory name, unique exact ID, unique
// case-insensitive substring. Multiple hits at the chosen rank are an error.
func findFestivalForConvert(ctx context.Context, festivalsRoot, nameOrID string) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", errors.Wrap(err, "context cancelled")
	}

	var exactName, exactID, substr []convertMatch

	for _, status := range id.StatusDirectories {
		if err := ctx.Err(); err != nil {
			return "", "", errors.Wrap(err, "context cancelled")
		}

		statusPath := workspace.JoinStatus(festivalsRoot, status)
		entries, err := os.ReadDir(statusPath)
		if err != nil {
			continue
		}

		dungeon := strings.HasPrefix(status, "dungeon/")
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			if dungeon && isDateDir(entry.Name()) {
				datePath := filepath.Join(statusPath, entry.Name())
				dateEntries, dateErr := os.ReadDir(datePath)
				if dateErr != nil {
					continue
				}
				for _, de := range dateEntries {
					if !de.IsDir() {
						continue
					}
					classifyConvertCandidate(de.Name(), filepath.Join(datePath, de.Name()), status, nameOrID, &exactName, &exactID, &substr)
				}
				continue
			}

			classifyConvertCandidate(entry.Name(), filepath.Join(statusPath, entry.Name()), status, nameOrID, &exactName, &exactID, &substr)
		}
	}

	return pickConvertMatch(nameOrID, exactName, exactID, substr)
}

func classifyConvertCandidate(name, path, status, query string, exactName, exactID, substr *[]convertMatch) {
	m := convertMatch{path: path, status: status, name: name}
	if strings.EqualFold(name, query) {
		*exactName = append(*exactName, m)
		return
	}
	if convertIDMatches(name, query) {
		*exactID = append(*exactID, m)
		return
	}
	if contains(name, query) {
		*substr = append(*substr, m)
	}
}

func convertIDMatches(dirName, query string) bool {
	if logical, err := id.ExtractLogicalIDFromDirName(dirName); err == nil && strings.EqualFold(logical, query) {
		return true
	}
	if extracted, err := id.ExtractIDFromDirName(dirName); err == nil && strings.EqualFold(extracted, query) {
		return true
	}
	return false
}

func pickConvertMatch(query string, exactName, exactID, substr []convertMatch) (string, string, error) {
	if path, status, err, ok := uniqueConvertMatch(query, "directory name", exactName); ok {
		return path, status, err
	}
	if path, status, err, ok := uniqueConvertMatch(query, "festival ID", exactID); ok {
		return path, status, err
	}
	if path, status, err, ok := uniqueConvertMatch(query, "name", substr); ok {
		return path, status, err
	}
	return "", "", errors.NotFound("festival").
		WithField("query", query).
		WithHint("Run 'fest list --all' to see available festivals")
}

func uniqueConvertMatch(query, kind string, matches []convertMatch) (string, string, error, bool) {
	switch len(matches) {
	case 0:
		return "", "", nil, false
	case 1:
		return matches[0].path, matches[0].status, nil, true
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.name
		}
		sort.Strings(names)
		err := errors.Validation(fmt.Sprintf("festival selector %q is ambiguous", query)).
			WithField("query", query).
			WithHint(fmt.Sprintf("Matches %d festivals by %s: %s. Use an exact directory name or festival ID", len(matches), kind, strings.Join(names, ", ")))
		return "", "", err, true
	}
}

// isDateDir checks if a directory name looks like a date bucket.
func isDateDir(name string) bool {
	if len(name) < 7 {
		return false
	}
	// YYYY-MM or YYYY-MM-DD
	if len(name) == 7 {
		return name[4] == '-' && isDigit(name[5]) && isDigit(name[6])
	}
	if len(name) == 10 {
		return name[4] == '-' && name[7] == '-'
	}
	return false
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
