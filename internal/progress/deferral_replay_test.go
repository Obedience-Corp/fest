package progress

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

const replayTask = "001_PHASE/01_seq/01_task.md"

func replayAt(offset int) time.Time {
	return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC).Add(time.Duration(offset) * time.Minute)
}

func TestReplayRestoresDeferral(t *testing.T) {
	tasks, _ := materializeState([]ProgressEvent{
		{Timestamp: replayAt(0), Event: EventBlocked, Task: replayTask, Reason: "provider API removed"},
		{
			Timestamp:      replayAt(1),
			Event:          EventBlockerDeferred,
			Task:           replayTask,
			DeferredBy:     "Ada Lovelace",
			DeferralReason: "the vendor replies next week",
		},
	})

	task := tasks[replayTask]
	if task.Status != StatusBlocked {
		t.Errorf("Status = %q, want %q", task.Status, StatusBlocked)
	}
	if !task.BlockerDeferred {
		t.Error("BlockerDeferred should be true")
	}
	if task.BlockerDeferredAt == nil || !task.BlockerDeferredAt.Equal(replayAt(1)) {
		t.Errorf("BlockerDeferredAt = %v, want %v", task.BlockerDeferredAt, replayAt(1))
	}
	if task.BlockerDeferredBy != "Ada Lovelace" {
		t.Errorf("BlockerDeferredBy = %q, want %q", task.BlockerDeferredBy, "Ada Lovelace")
	}
	if task.DeferralReason != "the vendor replies next week" {
		t.Errorf("DeferralReason = %q", task.DeferralReason)
	}
	if !task.IsSettled() || task.IsDone() {
		t.Error("a deferred blocker must be settled and not done")
	}
}

func TestReplayOrderIsAuthoritative(t *testing.T) {
	tasks, _ := materializeState([]ProgressEvent{
		{Timestamp: replayAt(0), Event: EventBlocked, Task: replayTask, Reason: "first"},
		{Timestamp: replayAt(1), Event: EventBlockerDeferred, Task: replayTask, DeferralReason: "defer it"},
		{Timestamp: replayAt(2), Event: EventUnblocked, Task: replayTask},
		{Timestamp: replayAt(3), Event: EventBlocked, Task: replayTask, Reason: "second"},
	})

	task := tasks[replayTask]
	if task.Status != StatusBlocked {
		t.Errorf("Status = %q, want %q", task.Status, StatusBlocked)
	}
	if task.BlockerDeferred {
		t.Error("the re-block must leave the blocker open and undeferred")
	}
	if task.BlockerMessage != "second" {
		t.Errorf("BlockerMessage = %q, want %q", task.BlockerMessage, "second")
	}
	if task.IsSettled() {
		t.Error("an open blocker is not settled")
	}
}

func TestReplayOperatorNotes(t *testing.T) {
	first, _ := materializeState([]ProgressEvent{
		{Timestamp: replayAt(0), Event: EventBlocked, Task: replayTask, Reason: "stuck"},
		{Timestamp: replayAt(1), Event: EventUnblocked, Task: replayTask, Note: "split the call in two"},
	})
	if got := first[replayTask].OperatorNotes; !slices.Equal(got, []string{"split the call in two"}) {
		t.Errorf("OperatorNotes = %q, want the one note", got)
	}

	second, _ := materializeState([]ProgressEvent{
		{Timestamp: replayAt(0), Event: EventBlocked, Task: replayTask, Reason: "stuck"},
		{Timestamp: replayAt(1), Event: EventUnblocked, Task: replayTask, Note: "split the call in two"},
		{Timestamp: replayAt(2), Event: EventBlocked, Task: replayTask, Reason: "still stuck"},
		{Timestamp: replayAt(3), Event: EventUnblocked, Task: replayTask, Note: "ask the vendor first"},
	})
	if got := second[replayTask].OperatorNotes; !slices.Equal(got, []string{"ask the vendor first"}) {
		t.Errorf("OperatorNotes = %q, want only the newest note", got)
	}

	reset, _ := materializeState([]ProgressEvent{
		{Timestamp: replayAt(0), Event: EventBlocked, Task: replayTask, Reason: "stuck"},
		{Timestamp: replayAt(1), Event: EventUnblocked, Task: replayTask, Note: "split the call in two"},
		{Timestamp: replayAt(2), Event: EventReset, Task: replayTask},
	})
	if got := reset[replayTask].OperatorNotes; got != nil {
		t.Errorf("OperatorNotes = %q, want nil after a reset", got)
	}

	completed, _ := materializeState([]ProgressEvent{
		{Timestamp: replayAt(0), Event: EventBlocked, Task: replayTask, Reason: "stuck"},
		{Timestamp: replayAt(1), Event: EventUnblocked, Task: replayTask, Note: "split the call in two"},
		{Timestamp: replayAt(2), Event: EventCompleted, Task: replayTask},
	})
	if got := completed[replayTask].OperatorNotes; got != nil {
		t.Errorf("OperatorNotes = %q, want nil after a completion", got)
	}
}

func TestReplayDerivesSweepState(t *testing.T) {
	events := []ProgressEvent{
		{Timestamp: replayAt(0), Event: EventSweepStarted, Sweep: 1},
		{Timestamp: replayAt(1), Event: EventBlockerRevisited, Task: "a", Sweep: 1},
		{Timestamp: replayAt(2), Event: EventSweepStarted, Sweep: 2},
		{Timestamp: replayAt(3), Event: EventBlockerRevisited, Task: "b", Sweep: 2},
	}

	_, sweep := materializeState(events)
	if sweep.Current != 2 {
		t.Errorf("Current = %d, want 2", sweep.Current)
	}
	if sweep.LastRevisit["a"] != 1 {
		t.Errorf("LastRevisit[a] = %d, want 1", sweep.LastRevisit["a"])
	}
	if sweep.LastRevisit["b"] != 2 {
		t.Errorf("LastRevisit[b] = %d, want 2", sweep.LastRevisit["b"])
	}

	outOfOrder := append(slices.Clone(events),
		ProgressEvent{Timestamp: replayAt(4), Event: EventSweepStarted, Sweep: 1},
		ProgressEvent{Timestamp: replayAt(5), Event: EventBlockerRevisited, Task: "b", Sweep: 1},
	)
	_, later := materializeState(outOfOrder)
	if later.Current != 2 {
		t.Errorf("Current = %d after a stale sweep event, want 2", later.Current)
	}
	if later.LastRevisit["b"] != 2 {
		t.Errorf("LastRevisit[b] = %d after a stale revisit, want 2", later.LastRevisit["b"])
	}
}

func TestReplayDroppedRevisitEventRevisitsAgain(t *testing.T) {
	_, sweep := materializeState([]ProgressEvent{
		{Timestamp: replayAt(0), Event: EventSweepStarted, Sweep: 1},
		{Timestamp: replayAt(2), Event: EventSweepStarted, Sweep: 2},
		{Timestamp: replayAt(3), Event: EventBlockerRevisited, Task: "b", Sweep: 2},
	})

	if sweep.LastRevisit["a"] != 0 {
		t.Errorf("LastRevisit[a] = %d, want 0 so the task is revisited again", sweep.LastRevisit["a"])
	}
	if sweep.Current != 2 {
		t.Errorf("Current = %d, want 2", sweep.Current)
	}
}

func TestLiveMutationAndReplayProduceIdenticalState(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()

	mgr, err := NewManager(ctx, dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	const (
		blockedTaskID   = "001_PHASE/01_seq/01_task.md"
		completedTaskID = "001_PHASE/01_seq/02_task.md"
		resetTaskID     = "001_PHASE/01_seq/03_task.md"
	)

	if err := mgr.MarkInProgress(ctx, blockedTaskID); err != nil {
		t.Fatalf("MarkInProgress() error = %v", err)
	}
	if err := mgr.ReportBlocker(ctx, blockedTaskID, "provider API removed",
		[]string{"checked the changelog", "asked the vendor"}); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}

	if err := mgr.ReportBlocker(ctx, completedTaskID, "waiting", []string{"retried once"}); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}
	if err := mgr.MarkComplete(ctx, completedTaskID); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	if err := mgr.ReportBlocker(ctx, resetTaskID, "waiting", []string{"retried once"}); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}
	if err := mgr.ClearBlocker(ctx, resetTaskID); err != nil {
		t.Fatalf("ClearBlocker() error = %v", err)
	}
	if err := mgr.ResetTask(ctx, resetTaskID); err != nil {
		t.Fatalf("ResetTask() error = %v", err)
	}

	live := mgr.AllTaskProgress()

	reloaded := NewStore(dir)
	if err := reloaded.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	replayed := reloaded.AllTasks()

	if len(live) != len(replayed) {
		t.Fatalf("live has %d tasks, replay has %d", len(live), len(replayed))
	}
	for id, liveTask := range live {
		replayedTask, ok := replayed[id]
		if !ok {
			t.Errorf("task %q missing from replay", id)
			continue
		}
		if !reflect.DeepEqual(liveTask, replayedTask) {
			t.Errorf("task %q diverged\nlive:   %+v\nreplay: %+v", id, liveTask, replayedTask)
		}
	}
}

func TestManagerExposesSweepState(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()

	store := NewStore(dir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	store.QueueEvent(&ProgressEvent{Timestamp: replayAt(0), Event: EventSweepStarted, Sweep: 3})
	store.QueueEvent(&ProgressEvent{Timestamp: replayAt(1), Event: EventBlockerRevisited, Task: replayTask, Sweep: 3})
	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	mgr, err := NewManager(ctx, dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	sweep := mgr.SweepState()
	if sweep.Current != 3 {
		t.Errorf("Current = %d, want 3", sweep.Current)
	}
	if sweep.LastRevisit[replayTask] != 3 {
		t.Errorf("LastRevisit[%s] = %d, want 3", replayTask, sweep.LastRevisit[replayTask])
	}

	empty := NewStore(t.TempDir())
	if got := empty.SweepState(); got.Current != 0 || got.LastRevisit == nil {
		t.Errorf("an unloaded store must report sweep 0 with a usable map, got %+v", got)
	}
}

func TestSweepStateIsCopiedOut(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()

	store := NewStore(dir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	store.QueueEvent(&ProgressEvent{Timestamp: replayAt(0), Event: EventSweepStarted, Sweep: 1})
	store.QueueEvent(&ProgressEvent{Timestamp: replayAt(1), Event: EventBlockerRevisited, Task: replayTask, Sweep: 1})
	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded := NewStore(dir)
	if err := loaded.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	first := loaded.SweepState()
	first.LastRevisit[replayTask] = 99
	first.Current = 99

	second := loaded.SweepState()
	if second.LastRevisit[replayTask] != 1 {
		t.Errorf("LastRevisit[%s] = %d after a caller wrote to its copy, want 1", replayTask, second.LastRevisit[replayTask])
	}
	if second.Current != 1 {
		t.Errorf("Current = %d, want 1", second.Current)
	}
}
