package selection

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/deps"
	"github.com/Obedience-Corp/fest/internal/progress"
)

const (
	callSitePhase    = "001_PHASE"
	callSiteSequence = "01_seq"
)

// writeSequenceFixture lays out one phase with one sequence holding the named
// task files and returns the festival path and the sequence path.
func writeSequenceFixture(t *testing.T, names ...string) (string, string) {
	t.Helper()

	festivalPath := t.TempDir()
	seqPath := filepath.Join(festivalPath, callSitePhase, callSiteSequence)
	if err := os.MkdirAll(seqPath, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	for _, name := range names {
		body := "# " + name + "\n\nWork.\n"
		if err := os.WriteFile(filepath.Join(seqPath, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return festivalPath, seqPath
}

func taskKey(name string) string {
	return callSitePhase + "/" + callSiteSequence + "/" + name
}

// seedProgress writes the given statuses through the real manager, then appends
// a deferral event for each name in deferred.
func seedProgress(t *testing.T, festivalPath string, completed []string, blocked []string, deferred []string) {
	t.Helper()
	ctx := context.Background()

	mgr, err := progress.NewManager(ctx, festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	for _, name := range completed {
		if err := mgr.MarkComplete(ctx, taskKey(name)); err != nil {
			t.Fatalf("MarkComplete(%s) error = %v", name, err)
		}
	}
	for _, name := range blocked {
		if err := mgr.ReportBlocker(ctx, taskKey(name), "provider API removed", nil); err != nil {
			t.Fatalf("ReportBlocker(%s) error = %v", name, err)
		}
	}
	if len(deferred) == 0 {
		return
	}
	store := mgr.Store()
	for _, name := range deferred {
		store.QueueEvent(&progress.ProgressEvent{
			Timestamp:      time.Now().UTC(),
			Event:          progress.EventBlockerDeferred,
			Task:           taskKey(name),
			DeferralReason: "the vendor replies next week",
		})
	}
	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

func TestSettledSequenceReportsComplete(t *testing.T) {
	ctx := t.Context()
	names := []string{"01_first.md", "02_second.md", "03_third.md"}

	t.Run("deferred blocker settles the sequence", func(t *testing.T) {
		festivalPath, seqPath := writeSequenceFixture(t, names...)
		seedProgress(t, festivalPath,
			[]string{"01_first.md", "03_third.md"},
			[]string{"02_second.md"},
			[]string{"02_second.md"})

		result, err := NewSelector(festivalPath).FindNextInSequence(ctx, seqPath)
		if err != nil {
			t.Fatalf("FindNextInSequence() error = %v", err)
		}
		if result.Task != nil {
			t.Fatalf("Task = %+v, want none", result.Task)
		}
		if result.Reason != "All tasks in sequence are complete" {
			t.Errorf("Reason = %q, want the sequence to read as finished", result.Reason)
		}
	})

	t.Run("an open blocker still stalls the sequence", func(t *testing.T) {
		festivalPath, seqPath := writeSequenceFixture(t, names...)
		seedProgress(t, festivalPath,
			[]string{"01_first.md", "03_third.md"},
			[]string{"02_second.md"},
			nil)

		result, err := NewSelector(festivalPath).FindNextInSequence(ctx, seqPath)
		if err != nil {
			t.Fatalf("FindNextInSequence() error = %v", err)
		}
		if result.Task != nil {
			t.Fatalf("Task = %+v, want none", result.Task)
		}
		if result.Reason != "No tasks are ready (dependencies not satisfied)" {
			t.Errorf("Reason = %q, want the stall", result.Reason)
		}
	})
}

func TestDeferralDoesNotPassASequenceGate(t *testing.T) {
	ctx := t.Context()
	festivalPath, seqPath := writeSequenceFixture(t,
		"01_first.md", "02_second.md", "03_testing.md")
	seedProgress(t, festivalPath,
		[]string{"01_first.md"},
		[]string{"02_second.md"},
		[]string{"02_second.md"})

	result, err := NewSelector(festivalPath).FindNextInSequence(ctx, seqPath)
	if err != nil {
		t.Fatalf("FindNextInSequence() error = %v", err)
	}
	if result.Task == nil {
		t.Fatalf("Reason = %q, want the unmet gate handed out rather than the sequence finishing", result.Reason)
	}
	if filepath.Base(result.Task.Path) != "03_testing.md" {
		t.Errorf("Task = %q, want the gate", result.Task.Path)
	}
}

func TestDoneFestivalNotCompleteWithDeferral(t *testing.T) {
	graph := deps.NewGraph()
	graph.AddTask(&deps.Task{ID: "01", Number: 1, Status: "complete"})
	graph.AddTask(&deps.Task{ID: "02", Number: 2,
		Status: progress.StatusBlocked, BlockerDeferred: true})

	selector := NewSelector(t.TempDir())
	if selector.isFestivalComplete(graph) {
		t.Error("a deferred blocker must not let the festival report complete")
	}

	allDone := deps.NewGraph()
	allDone.AddTask(&deps.Task{ID: "01", Number: 1, Status: "complete"})
	allDone.AddTask(&deps.Task{ID: "02", Number: 2, Status: "complete"})
	if !selector.isFestivalComplete(allDone) {
		t.Error("a festival with every task complete must still report complete")
	}
}

func TestCrossPhaseDependentBecomesReadyWhenItsBlockerIsDeferred(t *testing.T) {
	cases := []struct {
		name      string
		deferred  bool
		wantReady bool
	}{
		{"deferred blocker admits the dependent in the next phase", true, true},
		{"open blocker still holds the dependent", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			festivalPath := t.TempDir()

			alphaSeq := filepath.Join(festivalPath, "001_ALPHA", "01_seq")
			betaSeq := filepath.Join(festivalPath, "002_BETA", "01_seq")
			for _, dir := range []string{alphaSeq, betaSeq} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", dir, err)
				}
			}
			if err := os.WriteFile(filepath.Join(alphaSeq, "01_blocker.md"),
				[]byte("# Blocker\n"), 0o644); err != nil {
				t.Fatalf("write blocker: %v", err)
			}
			dependent := "---\nfest_type: task\nfest_dependencies:\n" +
				"  - ../../001_ALPHA/01_seq/01_blocker.md\n---\n\n# Dependent\n"
			if err := os.WriteFile(filepath.Join(betaSeq, "01_dependent.md"),
				[]byte(dependent), 0o644); err != nil {
				t.Fatalf("write dependent: %v", err)
			}

			mgr, err := progress.NewManager(ctx, festivalPath)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			const blockerKey = "001_ALPHA/01_seq/01_blocker.md"
			if err := mgr.ReportBlocker(ctx, blockerKey, "provider API removed", nil); err != nil {
				t.Fatalf("ReportBlocker() error = %v", err)
			}
			if tc.deferred {
				store := mgr.Store()
				store.QueueEvent(&progress.ProgressEvent{
					Timestamp:      time.Now().UTC(),
					Event:          progress.EventBlockerDeferred,
					Task:           blockerKey,
					DeferralReason: "the vendor replies next week",
				})
				if err := store.Save(ctx); err != nil {
					t.Fatalf("Save() error = %v", err)
				}
			}

			graph, err := deps.NewResolver(festivalPath).ResolveFestival()
			if err != nil {
				t.Fatalf("ResolveFestival() error = %v", err)
			}
			dependentID := filepath.Join(betaSeq, "01_dependent.md")
			if _, ok := graph.GetTask(dependentID); !ok {
				t.Fatalf("dependent %q missing from the resolved graph", dependentID)
			}
			if got := graph.GetDependencies(dependentID); len(got) != 1 {
				t.Fatalf("dependent has %d dependencies, want the cross phase blocker", len(got))
			}

			selector := NewSelector(festivalPath)
			if err := selector.updateTaskStatusesFromProgress(ctx, graph); err != nil {
				t.Fatalf("updateTaskStatusesFromProgress() error = %v", err)
			}

			ready := false
			for _, task := range graph.GetReadyTasks() {
				if task.ID == dependentID {
					ready = true
				}
				if task.ID == filepath.Join(alphaSeq, "01_blocker.md") {
					t.Error("the blocked task must never be ready")
				}
			}
			if ready != tc.wantReady {
				t.Errorf("dependent ready = %v, want %v", ready, tc.wantReady)
			}
		})
	}
}
