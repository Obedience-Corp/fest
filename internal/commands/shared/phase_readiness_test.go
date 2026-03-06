package shared

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
)

func TestArePhaseTasksComplete_UsesSlashNormalizedTaskIDs(t *testing.T) {
	t.Parallel()

	festivalPath := t.TempDir()
	phaseName := "001_IMPLEMENT"
	sequenceName := "01_build"
	taskName := "01_task.md"

	phasePath := filepath.Join(festivalPath, phaseName)
	sequencePath := filepath.Join(phasePath, sequenceName)
	if err := os.MkdirAll(sequencePath, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sequencePath, taskName), []byte("# Task\n"), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	store := progress.NewStore(festivalPath)
	store.SetTask(&progress.TaskProgress{
		TaskID: phaseName + "/" + sequenceName + "/" + taskName,
		Status: progress.StatusCompleted,
	})

	if !ArePhaseTasksComplete(true, store, phasePath, phaseName) {
		t.Fatal("expected tasks to be complete when store uses slash-normalized task IDs")
	}
}

func TestArePhaseTasksComplete_ReturnsFalseForIncompleteTask(t *testing.T) {
	t.Parallel()

	festivalPath := t.TempDir()
	phaseName := "001_IMPLEMENT"
	sequenceName := "01_build"
	taskName := "01_task.md"

	phasePath := filepath.Join(festivalPath, phaseName)
	sequencePath := filepath.Join(phasePath, sequenceName)
	if err := os.MkdirAll(sequencePath, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sequencePath, taskName), []byte("# Task\n"), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	store := progress.NewStore(festivalPath)
	store.SetTask(&progress.TaskProgress{
		TaskID: phaseName + "/" + sequenceName + "/" + taskName,
		Status: progress.StatusPending,
	})

	if ArePhaseTasksComplete(true, store, phasePath, phaseName) {
		t.Fatal("expected tasks to be incomplete when task status is pending")
	}
}
