package task

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance/selection"
	"github.com/Obedience-Corp/fest/internal/progress"
)

var taskFilenamePattern = regexp.MustCompile(`^\d{2}[\._].*\.md$`)

// resolveTask resolves the task argument to a normalized task ID and file path.
// When arg is empty, auto-detects by checking in_progress tasks then falling back to selector.
func resolveTask(ctx context.Context, festivalPath, arg string) (taskID string, taskFilePath string, err error) {
	if arg != "" {
		return resolveExplicitTask(festivalPath, arg)
	}
	return resolveAutoTask(ctx, festivalPath)
}

// resolveExplicitTask resolves a user-provided task argument.
func resolveExplicitTask(festivalPath, arg string) (string, string, error) {
	// Try as relative path first (contains /)
	if strings.Contains(arg, "/") {
		taskID, err := progress.NormalizeTaskID(festivalPath, arg)
		if err != nil {
			return "", "", err
		}
		filePath := filepath.Join(festivalPath, taskID)
		if !strings.HasSuffix(filePath, ".md") {
			filePath += ".md"
		}
		return taskID, filePath, nil
	}

	// Bare filename - search for unique match
	matches, err := findTaskMatches(festivalPath, arg)
	if err != nil {
		return "", "", err
	}

	if len(matches) == 0 {
		return "", "", errors.NotFound("task").
			WithField("task", arg).
			WithHint("No task matching '" + arg + "' found in this festival")
	}

	if len(matches) > 1 {
		return "", "", errors.Validation("task name is ambiguous; provide a full path").
			WithField("task", arg).
			WithField("matches", strings.Join(matches, ", "))
	}

	taskID := matches[0]
	filePath := filepath.Join(festivalPath, taskID)
	return taskID, filePath, nil
}

// resolveAutoTask auto-detects the current task from progress state or selector.
func resolveAutoTask(ctx context.Context, festivalPath string) (string, string, error) {
	// Step 1: Check for in_progress task in progress store
	mgr, err := progress.NewManager(ctx, festivalPath)
	if err == nil {
		allTasks := mgr.AllTaskProgress()
		for taskID, tp := range allTasks {
			if tp.Status == progress.StatusInProgress {
				filePath := filepath.Join(festivalPath, taskID)
				if !strings.HasSuffix(filePath, ".md") {
					filePath += ".md"
				}
				return taskID, filePath, nil
			}
		}
	}

	// Step 2: Fall back to selector (next pending task)
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		cwd = festivalPath
	}

	selector := selection.NewSelector(festivalPath)
	result, selErr := selector.FindNext(ctx, cwd)
	if selErr != nil {
		return "", "", errors.Wrap(selErr, "auto-detecting current task")
	}

	if result.Task == nil {
		if result.FestivalComplete {
			return "", "", errors.Validation("festival is complete; no tasks to operate on")
		}
		return "", "", errors.Validation("no task found; specify a task path or filename")
	}

	taskRelPath := filepath.Join(result.Task.PhaseName, result.Task.SequenceName, result.Task.Name+".md")
	taskID, normErr := progress.NormalizeTaskID(festivalPath, taskRelPath)
	if normErr != nil {
		return "", "", normErr
	}

	filePath := filepath.Join(festivalPath, taskID)
	return taskID, filePath, nil
}

// findTaskMatches searches for tasks matching the given filename across the festival.
func findTaskMatches(festivalPath, name string) ([]string, error) {
	if festivalPath == "" {
		return nil, errors.Validation("festival path required")
	}

	exact := name
	withExt := name
	if !strings.HasSuffix(name, ".md") {
		withExt = name + ".md"
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

		fileName := entry.Name()
		if !taskFilenamePattern.MatchString(fileName) {
			return nil
		}

		if fileName != exact && fileName != withExt {
			return nil
		}

		rel, relErr := filepath.Rel(festivalPath, path)
		if relErr != nil {
			return relErr
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
