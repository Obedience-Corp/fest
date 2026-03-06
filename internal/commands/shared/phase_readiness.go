package shared

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/progress"
)

// HasSequenceDirs returns true if the phase directory contains numbered subdirectories (sequences).
func HasSequenceDirs(phasePath string) bool {
	entries, err := os.ReadDir(phasePath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && IsNumberedDir(entry.Name()) {
			return true
		}
	}
	return false
}

// IsPhaseMarkedComplete checks PHASE_GOAL.md frontmatter for fest_status: completed.
func IsPhaseMarkedComplete(phasePath string) bool {
	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")
	data, err := os.ReadFile(goalPath)
	if err != nil {
		return false
	}
	content := string(data)
	// Frontmatter is between --- delimiters at the start of the file
	if !strings.HasPrefix(content, "---") {
		return false
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return false
	}
	fm := content[3 : 3+end]
	return strings.Contains(fm, "fest_status: completed")
}

// IsNumberedDir checks if a directory name starts with a digit.
func IsNumberedDir(name string) bool {
	if len(name) < 2 {
		return false
	}
	return name[0] >= '0' && name[0] <= '9'
}

// ArePhaseTasksAndWorkflowComplete checks if all non-gate work in a phase is done
// (sequences/tasks and any WORKFLOW.md steps) WITHOUT requiring PHASE_GOAL.md frontmatter
// to be marked completed. This avoids the circular dependency where gates are counted
// in progress totals, preventing the phase from ever being marked complete.
func ArePhaseTasksAndWorkflowComplete(ctx context.Context, storeLoaded bool, store *progress.Store, phasePath, phaseName string) bool {
	_ = ctx // reserved for future use

	// Check WORKFLOW.md completion
	workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
	if _, err := os.Stat(workflowPath); err == nil {
		if !storeLoaded {
			return false
		}
		state, ok := store.WorkflowPhaseState(phaseName)
		if !ok || state.TotalSteps == 0 || !state.IsComplete() {
			return false
		}
	}

	// Check if all tasks in phase sequences are complete
	if HasSequenceDirs(phasePath) && !ArePhaseTasksComplete(storeLoaded, store, phasePath, phaseName) {
		return false
	}

	return true
}

// ArePhaseTasksComplete checks if all numbered task files in all sequences have status "complete".
func ArePhaseTasksComplete(storeLoaded bool, store *progress.Store, phasePath, phaseName string) bool {
	if !storeLoaded {
		return false
	}

	entries, err := os.ReadDir(phasePath)
	if err != nil {
		return false
	}

	taskCount := 0
	for _, seqEntry := range entries {
		if !seqEntry.IsDir() || !IsNumberedDir(seqEntry.Name()) {
			continue
		}
		seqPath := filepath.Join(phasePath, seqEntry.Name())
		taskFiles, err := os.ReadDir(seqPath)
		if err != nil {
			continue
		}
		for _, tf := range taskFiles {
			if tf.IsDir() || !strings.HasSuffix(tf.Name(), ".md") {
				continue
			}
			if !isNumberedTaskFile(tf.Name()) {
				continue
			}
			taskCount++
			taskID := filepath.Join(phaseName, seqEntry.Name(), tf.Name())
			task, ok := store.GetTask(taskID)
			if !ok || (task.Status != progress.StatusCompleted && task.Status != "complete") {
				return false
			}
		}
	}
	return taskCount > 0
}

// isNumberedTaskFile checks if a filename starts with digits followed by underscore.
func isNumberedTaskFile(name string) bool {
	for i, c := range name {
		if c == '_' && i > 0 {
			return true
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return false
}
