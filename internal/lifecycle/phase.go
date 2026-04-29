// Package lifecycle enforces festival lifecycle rules across CLI commands.
//
// The pre-active gate blocks implementation/review work on festivals that
// have not been promoted to active status. The gate is consulted from
// every CLI mutation entry point and from progress.Manager mutation
// methods so the rule cannot be bypassed by switching commands.
package lifecycle

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// FindFirstIncompletePhase scans ALL phases in numerical order and returns
// the first incomplete phase. It respects ordering across both
// workflow-based and task-based phases.
//
// Returns (phasePath, isWorkflow, error). Empty phasePath means all phases
// are complete.
func FindFirstIncompletePhase(ctx context.Context, festivalPath string) (string, bool, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", false, err
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

		workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
		if _, err := os.Stat(workflowPath); err == nil {
			if storeLoaded {
				state, ok := store.WorkflowPhaseState(phaseName)
				if !ok || state.TotalSteps == 0 || !state.IsComplete() {
					return phasePath, true, nil
				}
			} else {
				return phasePath, true, nil
			}
			if HasIncompletePhaseGate(storeLoaded, store, phasePath, phaseName) {
				return phasePath, false, nil
			}
			continue
		}

		if hasSequenceDirs(phasePath) && !shared.ArePhaseTasksComplete(storeLoaded, store, phasePath, phaseName) {
			return phasePath, false, nil
		}

		if HasIncompletePhaseGate(storeLoaded, store, phasePath, phaseName) {
			return phasePath, false, nil
		}
	}

	return "", false, nil
}

// HasIncompletePhaseGate checks if a phase has a GATES.md that is not yet
// complete. Exported because other phase scanners in commands/next also
// need it.
func HasIncompletePhaseGate(storeLoaded bool, store *progress.Store, phasePath, phaseName string) bool {
	gatesPath := filepath.Join(phasePath, "GATES.md")
	if _, err := os.Stat(gatesPath); err != nil {
		return false
	}
	if !storeLoaded {
		return true
	}
	state, ok := store.GatePhaseState(phaseName)
	if !ok || state.TotalSteps == 0 || !state.IsComplete() {
		return true
	}
	return false
}

func hasSequenceDirs(phasePath string) bool {
	entries, err := os.ReadDir(phasePath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && shared.IsNumberedDir(entry.Name()) {
			return true
		}
	}
	return false
}
