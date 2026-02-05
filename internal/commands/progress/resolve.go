// Package progress implements the fest progress command for tracking execution progress.
package progress

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
)

func resolveTaskPath(pathArg, festivalPath, cwd string) (string, error) {
	if pathArg == "" {
		return "", errors.Validation("task path required")
	}

	if filepath.IsAbs(pathArg) {
		return filepath.Clean(pathArg), nil
	}

	if festivalPath != "" {
		return filepath.Clean(filepath.Join(festivalPath, pathArg)), nil
	}

	return filepath.Clean(filepath.Join(cwd, pathArg)), nil
}

func resolveTaskID(festivalPath string, opts *progressOptions) (string, error) {
	if opts.taskPath != "" {
		return progress.NormalizeTaskID(festivalPath, opts.taskPath)
	}

	taskID := strings.TrimSpace(opts.taskID)
	if taskID == "" {
		return "", errors.Validation("task ID required")
	}

	if opts.phase != "" && opts.sequence != "" {
		taskID = ensureMarkdownFilename(taskID)
		taskPath := filepath.Join(opts.phase, opts.sequence, taskID)
		return progress.NormalizeTaskID(festivalPath, taskPath)
	}

	normalized, err := progress.NormalizeTaskID(festivalPath, taskID)
	if err != nil {
		return "", err
	}

	if !strings.Contains(taskID, "/") && !strings.Contains(taskID, "\\") && !filepath.IsAbs(taskID) {
		matches, err := findTaskMatches(festivalPath, taskID)
		if err != nil {
			return "", err
		}

		if len(matches) > 1 {
			return "", errors.Validation("task ID is ambiguous; provide a full task path or use --phase/--sequence").
				WithField("task", taskID).
				WithField("matches", strings.Join(matches, ", "))
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
	}

	return normalized, nil
}

func ensureMarkdownFilename(name string) string {
	if strings.HasSuffix(name, ".md") {
		return name
	}
	return name + ".md"
}

func findTaskMatches(festivalPath, taskID string) ([]string, error) {
	if festivalPath == "" {
		return nil, errors.Validation("festival path required")
	}

	exact := taskID
	withExt := taskID
	if !strings.HasSuffix(taskID, ".md") {
		withExt = taskID + ".md"
	}

	var matches []string
	err := filepath.WalkDir(festivalPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".fest" || entry.Name() == "results" {
				return filepath.SkipDir
			}
			return nil
		}

		name := entry.Name()
		if !isTaskFile(name) {
			return nil
		}

		if name != exact && name != withExt {
			return nil
		}

		rel, err := filepath.Rel(festivalPath, path)
		if err != nil {
			return err
		}

		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 3 {
			return nil
		}

		matches = append(matches, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, errors.IO("walking festival", err).WithField("path", festivalPath)
	}

	sort.Strings(matches)
	return matches, nil
}

func isTaskFile(name string) bool {
	return taskFilenamePattern.MatchString(name)
}

// resolveTaskLocationPaths extracts phase and sequence paths from a task ID path.
// Task IDs are in the format: "phase/sequence/task.md"
func resolveTaskLocationPaths(festivalPath, taskID string) (phasePath, sequencePath string) {
	parts := strings.Split(taskID, "/")
	if len(parts) >= 1 {
		phasePath = filepath.Join(festivalPath, parts[0])
	}
	if len(parts) >= 2 {
		sequencePath = filepath.Join(festivalPath, parts[0], parts[1])
	}
	return phasePath, sequencePath
}
