package lifecycle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/progress"
)

// twoPhaseFixture lays out two task phases with one sequence and two tasks each
// and returns the festival path.
func twoPhaseFixture(t *testing.T) string {
	t.Helper()

	festivalPath := t.TempDir()
	for _, phase := range []string{"001_ALPHA", "002_BETA"} {
		seqPath := filepath.Join(festivalPath, phase, "01_seq")
		if err := os.MkdirAll(seqPath, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", seqPath, err)
		}
		for _, name := range []string{"01_task.md", "02_task.md"} {
			if err := os.WriteFile(filepath.Join(seqPath, name), []byte("# Task\n"), 0o644); err != nil {
				t.Fatalf("write task file: %v", err)
			}
		}
	}
	return festivalPath
}

func phaseTaskKey(phase, name string) string {
	return phase + "/01_seq/" + name
}

func TestFindFirstIncompletePhaseMovesPastADeferredBlocker(t *testing.T) {
	cases := []struct {
		name      string
		deferred  bool
		wantPhase string
	}{
		{"deferred blocker opens the next phase", true, "002_BETA"},
		{"open blocker holds the phase", false, "001_ALPHA"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			festivalPath := twoPhaseFixture(t)

			mgr, err := progress.NewManager(ctx, festivalPath)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			if err := mgr.MarkComplete(ctx, phaseTaskKey("001_ALPHA", "01_task.md")); err != nil {
				t.Fatalf("MarkComplete() error = %v", err)
			}
			if err := mgr.ReportBlocker(ctx, phaseTaskKey("001_ALPHA", "02_task.md"),
				"provider API removed", nil); err != nil {
				t.Fatalf("ReportBlocker() error = %v", err)
			}
			if tc.deferred {
				store := mgr.Store()
				store.QueueEvent(&progress.ProgressEvent{
					Timestamp:      time.Now().UTC(),
					Event:          progress.EventBlockerDeferred,
					Task:           phaseTaskKey("001_ALPHA", "02_task.md"),
					DeferralReason: "the vendor replies next week",
				})
				if err := store.Save(ctx); err != nil {
					t.Fatalf("Save() error = %v", err)
				}
			}

			phasePath, isWorkflow, err := FindFirstIncompletePhase(ctx, festivalPath)
			if err != nil {
				t.Fatalf("FindFirstIncompletePhase() error = %v", err)
			}
			if isWorkflow {
				t.Fatal("the fixture has no WORKFLOW.md, so no phase should read as a workflow phase")
			}
			if got := filepath.Base(phasePath); got != tc.wantPhase {
				t.Errorf("first incomplete phase = %q, want %q", got, tc.wantPhase)
			}
		})
	}
}
