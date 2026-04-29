// Package next provides the fest next command for task navigation.
package next

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// findSequencePath finds the sequence path from current directory
func findSequencePath(cwd, festivalPath string) string {
	// Walk up from cwd looking for a sequence directory
	current := cwd
	for {
		// Check if current is a sequence (numbered dir inside a numbered phase dir)
		parent := filepath.Dir(current)
		if parent == festivalPath {
			// Current is a phase, not a sequence
			return ""
		}
		grandparent := filepath.Dir(parent)
		if grandparent == festivalPath {
			// Parent is a phase, current might be a sequence
			if shared.IsNumberedDir(filepath.Base(parent)) && shared.IsNumberedDir(filepath.Base(current)) {
				return current
			}
		}
		if current == festivalPath || current == "/" || current == "." {
			break
		}
		current = parent
	}
	return ""
}

// findFirstIncompleteWorkflowPhase scans phases in numerical order for the first with incomplete workflow.
// Used as a fallback when the selector reports all tasks complete but workflow phases remain.
func findFirstIncompleteWorkflowPhase(ctx context.Context, festivalPath string) (string, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", err
	}

	var phases []string
	for _, entry := range entries {
		if entry.IsDir() && shared.IsNumberedDir(entry.Name()) {
			phases = append(phases, filepath.Join(festivalPath, entry.Name()))
		}
	}

	sort.Strings(phases)

	store := progress.NewStore(festivalPath)
	storeLoaded := store.Load(ctx) == nil

	for _, phasePath := range phases {
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, err := os.Stat(workflowPath); err != nil {
			continue
		}

		phaseName := filepath.Base(phasePath)
		if !storeLoaded {
			return phasePath, nil
		}
		state, ok := store.WorkflowPhaseState(phaseName)
		if !ok || state.TotalSteps == 0 || !state.IsComplete() {
			return phasePath, nil
		}
	}

	return "", nil
}

// findEarlierIncompleteAfterWorkflow scans phases in numerical order before the given phase
// and returns the first one with an incomplete after-position workflow. This handles hybrid
// phases where sequences are complete but the after-workflow steps remain.
func findEarlierIncompleteAfterWorkflow(ctx context.Context, festivalPath, currentPhaseName string) (string, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", err
	}

	var phases []string
	for _, entry := range entries {
		if entry.IsDir() && shared.IsNumberedDir(entry.Name()) {
			phases = append(phases, entry.Name())
		}
	}
	sort.Strings(phases)

	store := progress.NewStore(festivalPath)
	storeLoaded := store.Load(ctx) == nil

	for _, phaseName := range phases {
		// Stop before the current task's phase
		if phaseName >= currentPhaseName {
			break
		}

		phasePath := filepath.Join(festivalPath, phaseName)
		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, statErr := os.Stat(workflowPath); statErr != nil {
			continue
		}

		// Only consider after-position workflows (default)
		position := shared.WorkflowPositionForPhase(phasePath)
		if position == frontmatter.WorkflowPositionBefore {
			continue
		}

		// Check if workflow is incomplete
		if storeLoaded {
			state, ok := store.WorkflowPhaseState(phaseName)
			if ok && state.TotalSteps > 0 && state.IsComplete() {
				continue // Workflow is complete
			}
		}

		return phasePath, nil
	}

	return "", nil
}

// findEarlierIncompletePhaseGate scans phases in numerical order before the given phase
// and returns the first one with an incomplete phase gate (GATES.md).
// Phase gates run after all tasks, sequence gates, and workflows in a phase are complete.
func findEarlierIncompletePhaseGate(ctx context.Context, festivalPath, currentPhaseName string) (string, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", err
	}

	var phases []string
	for _, entry := range entries {
		if entry.IsDir() && shared.IsNumberedDir(entry.Name()) {
			phases = append(phases, entry.Name())
		}
	}
	sort.Strings(phases)

	store := progress.NewStore(festivalPath)
	storeLoaded := store.Load(ctx) == nil

	for _, phaseName := range phases {
		// Stop before the current task's phase
		if phaseName >= currentPhaseName {
			break
		}

		phasePath := filepath.Join(festivalPath, phaseName)

		// Only check gates for phases where all other work is complete
		// (workflows done, sequences done)
		if !isPhaseWorkAndWorkflowComplete(ctx, storeLoaded, store, phasePath, phaseName) {
			continue
		}

		if lifecycle.HasIncompletePhaseGate(storeLoaded, store, phasePath, phaseName) {
			return phasePath, nil
		}
	}

	return "", nil
}

// findFirstIncompletePhaseGate scans phases in numerical order for the first with an incomplete phase gate.
// Used as a fallback when the selector reports all tasks and workflows complete.
func findFirstIncompletePhaseGate(ctx context.Context, festivalPath string) (string, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", err
	}

	var phases []string
	for _, entry := range entries {
		if entry.IsDir() && shared.IsNumberedDir(entry.Name()) {
			phases = append(phases, filepath.Join(festivalPath, entry.Name()))
		}
	}
	sort.Strings(phases)

	store := progress.NewStore(festivalPath)
	storeLoaded := store.Load(ctx) == nil

	for _, phasePath := range phases {
		phaseName := filepath.Base(phasePath)
		// Only surface gates for phases where all other work is complete
		if !isPhaseWorkAndWorkflowComplete(ctx, storeLoaded, store, phasePath, phaseName) {
			continue
		}
		if lifecycle.HasIncompletePhaseGate(storeLoaded, store, phasePath, phaseName) {
			return phasePath, nil
		}
	}

	return "", nil
}

// isPhaseWorkAndWorkflowComplete checks if all non-gate work in a phase is done
// (sequences, tasks, and any WORKFLOW.md steps).
func isPhaseWorkAndWorkflowComplete(ctx context.Context, storeLoaded bool, store *progress.Store, phasePath, phaseName string) bool {
	return shared.ArePhaseTasksAndWorkflowComplete(ctx, storeLoaded, store, phasePath, phaseName)
}
