package progress

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	aggPhase    = "001_PHASE"
	aggSequence = "01_seq"
)

// aggregateFixture writes one phase with one sequence holding the named task
// files and returns the festival path and the sequence path.
func aggregateFixture(t *testing.T, names ...string) (string, string) {
	t.Helper()

	festivalPath := t.TempDir()
	seqPath := filepath.Join(festivalPath, aggPhase, aggSequence)
	if err := os.MkdirAll(seqPath, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(seqPath, name), []byte("# Task\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return festivalPath, seqPath
}

func aggKey(name string) string {
	return aggPhase + "/" + aggSequence + "/" + name
}

// seedAggregate marks the named tasks complete or blocked and defers the ones
// listed, all through the real manager and its event log.
func seedAggregate(t *testing.T, festivalPath string, completed, blocked, deferred []string) *Manager {
	t.Helper()
	ctx := t.Context()

	mgr, err := NewManager(ctx, festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	for _, name := range completed {
		if err := mgr.MarkComplete(ctx, aggKey(name)); err != nil {
			t.Fatalf("MarkComplete(%s) error = %v", name, err)
		}
	}
	for _, name := range blocked {
		if err := mgr.ReportBlocker(ctx, aggKey(name), "provider API removed", nil); err != nil {
			t.Fatalf("ReportBlocker(%s) error = %v", name, err)
		}
	}
	if len(deferred) > 0 {
		for _, name := range deferred {
			mgr.Store().QueueEvent(&ProgressEvent{
				Timestamp:      time.Now().UTC(),
				Event:          EventBlockerDeferred,
				Task:           aggKey(name),
				DeferralReason: "the vendor replies next week",
			})
		}
		if err := mgr.Store().Save(ctx); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		reloaded, err := NewManager(ctx, festivalPath)
		if err != nil {
			t.Fatalf("NewManager() reload error = %v", err)
		}
		return reloaded
	}
	return mgr
}

func TestSettledSequenceCountsADeferredBlocker(t *testing.T) {
	ctx := t.Context()
	names := []string{"01_first.md", "02_second.md", "03_third.md"}

	t.Run("deferred", func(t *testing.T) {
		festivalPath, seqPath := aggregateFixture(t, names...)
		mgr := seedAggregate(t, festivalPath,
			[]string{"01_first.md", "03_third.md"},
			[]string{"02_second.md"},
			[]string{"02_second.md"})

		seq, err := mgr.GetSequenceProgress(ctx, seqPath)
		if err != nil {
			t.Fatalf("GetSequenceProgress() error = %v", err)
		}
		p := seq.Progress
		if p.Total != 3 || p.Completed != 2 || p.Blocked != 1 {
			t.Fatalf("Total/Completed/Blocked = %d/%d/%d, want 3/2/1", p.Total, p.Completed, p.Blocked)
		}
		if p.Settled != 3 {
			t.Errorf("Settled = %d, want 3", p.Settled)
		}
		if p.DeferredBlocked != 1 {
			t.Errorf("DeferredBlocked = %d, want 1", p.DeferredBlocked)
		}
		if len(p.DeferredBlockers) != 1 || p.DeferredBlockers[0].TaskID != aggKey("02_second.md") {
			t.Errorf("DeferredBlockers = %+v, want the one deferred task", p.DeferredBlockers)
		}
		if len(p.Blockers) != 1 {
			t.Errorf("Blockers = %+v, want the deferred task still counted as a blocker", p.Blockers)
		}
	})

	t.Run("open", func(t *testing.T) {
		festivalPath, seqPath := aggregateFixture(t, names...)
		mgr := seedAggregate(t, festivalPath,
			[]string{"01_first.md", "03_third.md"},
			[]string{"02_second.md"},
			nil)

		seq, err := mgr.GetSequenceProgress(ctx, seqPath)
		if err != nil {
			t.Fatalf("GetSequenceProgress() error = %v", err)
		}
		p := seq.Progress
		if p.Settled != 2 {
			t.Errorf("Settled = %d, want 2", p.Settled)
		}
		if p.DeferredBlocked != 0 || len(p.DeferredBlockers) != 0 {
			t.Errorf("DeferredBlocked = %d, DeferredBlockers = %+v, want none", p.DeferredBlocked, p.DeferredBlockers)
		}
	})
}

func TestPercentageUnchangedByDeferral(t *testing.T) {
	ctx := t.Context()
	names := []string{"01_first.md", "02_second.md", "03_third.md"}

	read := func(deferred []string) *AggregateProgress {
		t.Helper()
		festivalPath, seqPath := aggregateFixture(t, names...)
		mgr := seedAggregate(t, festivalPath,
			[]string{"01_first.md", "03_third.md"},
			[]string{"02_second.md"},
			deferred)
		seq, err := mgr.GetSequenceProgress(ctx, seqPath)
		if err != nil {
			t.Fatalf("GetSequenceProgress() error = %v", err)
		}
		return seq.Progress
	}

	before := read(nil)
	after := read([]string{"02_second.md"})

	if before.Percentage != after.Percentage {
		t.Errorf("Percentage moved from %d to %d when a blocker was deferred", before.Percentage, after.Percentage)
	}
	if before.Completed != after.Completed {
		t.Errorf("Completed moved from %d to %d", before.Completed, after.Completed)
	}
	if before.Blocked != after.Blocked {
		t.Errorf("Blocked moved from %d to %d", before.Blocked, after.Blocked)
	}
	if before.Pending != after.Pending || before.InProgress != after.InProgress {
		t.Errorf("Pending or InProgress moved: %+v then %+v", before, after)
	}
}

func TestPhaseAndFestivalAggregationSumTheNewCounts(t *testing.T) {
	ctx := t.Context()
	festivalPath := t.TempDir()

	seqOne := filepath.Join(festivalPath, aggPhase, "01_seq")
	seqTwo := filepath.Join(festivalPath, aggPhase, "02_seq")
	for _, dir := range []string{seqOne, seqTwo} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		for _, name := range []string{"01_task.md", "02_task.md"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("# Task\n"), 0o644); err != nil {
				t.Fatalf("write task: %v", err)
			}
		}
	}

	mgr, err := NewManager(ctx, festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	key := func(seq, name string) string { return aggPhase + "/" + seq + "/" + name }
	for _, seq := range []string{"01_seq", "02_seq"} {
		if err := mgr.MarkComplete(ctx, key(seq, "01_task.md")); err != nil {
			t.Fatalf("MarkComplete() error = %v", err)
		}
		if err := mgr.ReportBlocker(ctx, key(seq, "02_task.md"), "provider API removed", nil); err != nil {
			t.Fatalf("ReportBlocker() error = %v", err)
		}
	}
	// Queued after every manager mutation: each one takes the exclusive lock,
	// which reloads the store and drops anything still pending.
	for _, seq := range []string{"01_seq", "02_seq"} {
		mgr.Store().QueueEvent(&ProgressEvent{
			Timestamp:      time.Now().UTC(),
			Event:          EventBlockerDeferred,
			Task:           key(seq, "02_task.md"),
			DeferralReason: "the vendor replies next week",
		})
	}
	if err := mgr.Store().Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	mgr, err = NewManager(ctx, festivalPath)
	if err != nil {
		t.Fatalf("NewManager() reload error = %v", err)
	}

	phase, err := mgr.GetPhaseProgress(ctx, filepath.Join(festivalPath, aggPhase))
	if err != nil {
		t.Fatalf("GetPhaseProgress() error = %v", err)
	}
	if phase.Progress.Settled != 4 || phase.Progress.DeferredBlocked != 2 {
		t.Errorf("phase Settled/DeferredBlocked = %d/%d, want 4/2",
			phase.Progress.Settled, phase.Progress.DeferredBlocked)
	}
	if len(phase.Progress.DeferredBlockers) != 2 {
		t.Errorf("phase DeferredBlockers = %d, want 2", len(phase.Progress.DeferredBlockers))
	}

	festival, err := mgr.GetFestivalProgress(ctx, festivalPath)
	if err != nil {
		t.Fatalf("GetFestivalProgress() error = %v", err)
	}
	if festival.Overall.Settled != 4 || festival.Overall.DeferredBlocked != 2 {
		t.Errorf("festival Settled/DeferredBlocked = %d/%d, want 4/2",
			festival.Overall.Settled, festival.Overall.DeferredBlocked)
	}
	if len(festival.Overall.DeferredBlockers) != 2 {
		t.Errorf("festival DeferredBlockers = %d, want 2", len(festival.Overall.DeferredBlockers))
	}
	if festival.Overall.Completed != 2 {
		t.Errorf("festival Completed = %d, want 2", festival.Overall.Completed)
	}
}

func TestSettledSequenceGoalPropagates(t *testing.T) {
	ctx := t.Context()
	names := []string{"01_first.md", "02_second.md"}

	run := func(deferred []string) bool {
		t.Helper()
		festivalPath, seqPath := aggregateFixture(t, names...)
		goalPath := filepath.Join(seqPath, "SEQUENCE_GOAL.md")
		if err := os.WriteFile(goalPath,
			[]byte("---\nfest_type: sequence\nfest_status: pending\n---\n\n# Sequence Goal\n"), 0o644); err != nil {
			t.Fatalf("write goal: %v", err)
		}

		mgr := seedAggregate(t, festivalPath,
			[]string{"01_first.md"},
			[]string{"02_second.md"},
			deferred)

		seq, err := mgr.GetSequenceProgress(ctx, seqPath)
		if err != nil {
			t.Fatalf("GetSequenceProgress() error = %v", err)
		}
		return seq.Progress.Total > 0 && seq.Progress.Settled >= seq.Progress.Total
	}

	if !run([]string{"02_second.md"}) {
		t.Error("a sequence whose only open task is a deferred blocker must meet the propagation precondition")
	}
	if run(nil) {
		t.Error("a sequence with an open blocker must not meet the propagation precondition")
	}
}

func TestDoneCompletedAtNilWithDeferral(t *testing.T) {
	at := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

	deferredEvents := []ProgressEvent{
		{Timestamp: at, Event: EventCompleted, Task: "01_task.md", Minutes: 10},
		{Timestamp: at.Add(time.Minute), Event: EventBlocked, Task: "02_task.md", Reason: "stuck"},
		{Timestamp: at.Add(2 * time.Minute), Event: EventBlockerDeferred, Task: "02_task.md"},
	}
	tasks, _ := materializeState(deferredEvents)
	if got := materializeTimeMetrics(deferredEvents, tasks).CompletedAt; got != nil {
		t.Errorf("CompletedAt = %v with a deferred blocker, want nil", got)
	}

	doneEvents := []ProgressEvent{
		{Timestamp: at, Event: EventCompleted, Task: "01_task.md", Minutes: 10},
		{Timestamp: at.Add(time.Minute), Event: EventCompleted, Task: "02_task.md", Minutes: 5},
	}
	doneTasks, _ := materializeState(doneEvents)
	if got := materializeTimeMetrics(doneEvents, doneTasks).CompletedAt; got == nil {
		t.Error("CompletedAt should be set when every task is done")
	}
}
