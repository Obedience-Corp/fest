package shared

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
)

func TestSettledPhaseTasksComplete(t *testing.T) {
	t.Parallel()

	const (
		phaseName    = "001_IMPLEMENT"
		sequenceName = "01_build"
	)

	cases := []struct {
		name     string
		deferred bool
		want     bool
	}{
		{"deferred blocker settles the phase", true, true},
		{"open blocker does not", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			festivalPath := t.TempDir()
			phasePath := filepath.Join(festivalPath, phaseName)
			sequencePath := filepath.Join(phasePath, sequenceName)
			if err := os.MkdirAll(sequencePath, 0o755); err != nil {
				t.Fatalf("mkdir sequence: %v", err)
			}
			for _, name := range []string{"01_task.md", "02_task.md"} {
				if err := os.WriteFile(filepath.Join(sequencePath, name), []byte("# Task\n"), 0o644); err != nil {
					t.Fatalf("write task file: %v", err)
				}
			}

			store := progress.NewStore(festivalPath)
			store.SetTask(&progress.TaskProgress{
				TaskID: phaseName + "/" + sequenceName + "/01_task.md",
				Status: progress.StatusCompleted,
			})
			store.SetTask(&progress.TaskProgress{
				TaskID:          phaseName + "/" + sequenceName + "/02_task.md",
				Status:          progress.StatusBlocked,
				BlockerMessage:  "provider API removed",
				BlockerDeferred: tc.deferred,
			})

			if got := ArePhaseTasksComplete(true, store, phasePath, phaseName); got != tc.want {
				t.Errorf("ArePhaseTasksComplete() = %v, want %v", got, tc.want)
			}
		})
	}
}
