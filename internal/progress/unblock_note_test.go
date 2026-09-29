package progress

import (
	"strings"
	"testing"
)

func reloadManager(t *testing.T, festivalPath string) *Manager {
	t.Helper()
	mgr, err := NewManager(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return mgr
}

func TestUnblockNotePersistsAndSurvivesAReload(t *testing.T) {
	ctx := t.Context()
	festivalPath, mgr := blockedDeferFixture(t)

	if err := mgr.DeferBlocker(ctx, deferTestTaskID, "the vendor replies next week", deferTestAudit()); err != nil {
		t.Fatalf("DeferBlocker() error = %v", err)
	}
	if err := mgr.ClearBlocker(ctx, deferTestTaskID, "the v2 endpoint does the same thing, try that"); err != nil {
		t.Fatalf("ClearBlocker() error = %v", err)
	}

	task, ok := mgr.GetTaskProgress(deferTestTaskID)
	if !ok {
		t.Fatal("task missing from the store")
	}
	if len(task.OperatorNotes) != 1 || task.OperatorNotes[0] != "the v2 endpoint does the same thing, try that" {
		t.Fatalf("OperatorNotes = %v, want exactly the new note", task.OperatorNotes)
	}
	if task.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", task.Status, StatusInProgress)
	}
	if task.BlockerDeferred || task.BlockerDeferredAt != nil || task.BlockerDeferredBy != "" || task.DeferralReason != "" {
		t.Errorf("deferral fields survived the unblock: %+v", task)
	}
	if task.BlockerMessage != "" || task.BlockedAt != nil {
		t.Errorf("blocker fields survived the unblock: %+v", task)
	}

	replayed, ok := reloadManager(t, festivalPath).GetTaskProgress(deferTestTaskID)
	if !ok {
		t.Fatal("task missing after a store reload")
	}
	if len(replayed.OperatorNotes) != 1 || replayed.OperatorNotes[0] != task.OperatorNotes[0] {
		t.Errorf("OperatorNotes after replay = %v, want %v", replayed.OperatorNotes, task.OperatorNotes)
	}

	events := readDeferralEvents(t, festivalPath)
	last := events[len(events)-1]
	if last.Event != EventUnblocked {
		t.Fatalf("last event = %q, want %q", last.Event, EventUnblocked)
	}
	if last.Note != "the v2 endpoint does the same thing, try that" {
		t.Errorf("event note = %q, want the operator note", last.Note)
	}
}

func TestUnblockWithoutANoteLeavesNoNotes(t *testing.T) {
	ctx := t.Context()
	festivalPath, mgr := blockedDeferFixture(t)

	if err := mgr.ClearBlocker(ctx, deferTestTaskID, ""); err != nil {
		t.Fatalf("ClearBlocker() error = %v", err)
	}

	task, ok := mgr.GetTaskProgress(deferTestTaskID)
	if !ok {
		t.Fatal("task missing from the store")
	}
	if task.OperatorNotes != nil {
		t.Errorf("OperatorNotes = %v, want nil", task.OperatorNotes)
	}

	events := readDeferralEvents(t, festivalPath)
	last := events[len(events)-1]
	if last.Note != "" {
		t.Errorf("event note = %q, want empty", last.Note)
	}
}

func TestUnblockReplacesAnEarlierNote(t *testing.T) {
	ctx := t.Context()
	festivalPath, mgr := blockedDeferFixture(t)

	if err := mgr.ClearBlocker(ctx, deferTestTaskID, "try the v2 endpoint"); err != nil {
		t.Fatalf("ClearBlocker() error = %v", err)
	}
	if err := mgr.ReportBlocker(ctx, deferTestTaskID, "v2 is rate limited", nil); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}
	if err := mgr.ClearBlocker(ctx, deferTestTaskID, "request a quota bump first"); err != nil {
		t.Fatalf("ClearBlocker() error = %v", err)
	}

	task, ok := reloadManager(t, festivalPath).GetTaskProgress(deferTestTaskID)
	if !ok {
		t.Fatal("task missing from the store")
	}
	if len(task.OperatorNotes) != 1 || task.OperatorNotes[0] != "request a quota bump first" {
		t.Errorf("OperatorNotes = %v, want only the latest note", task.OperatorNotes)
	}
}

// TestUnblockNoteOnATaskWithNoBlockerIsANoOp pins the deliberate choice to leave
// ClearBlocker's early return alone: a note passed for a task that is not
// blocked is dropped rather than recorded, because there is no blocker to
// answer and no unblocked event to carry it.
func TestUnblockNoteOnATaskWithNoBlockerIsANoOp(t *testing.T) {
	ctx := t.Context()
	festivalPath := createTestFestivalWithTasks(t)

	mgr, err := NewManager(ctx, festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := mgr.MarkInProgress(ctx, deferTestTaskID); err != nil {
		t.Fatalf("MarkInProgress() error = %v", err)
	}

	before := snapshotProgressDir(t, festivalPath)

	if err := mgr.ClearBlocker(ctx, deferTestTaskID, "try the v2 endpoint"); err != nil {
		t.Fatalf("ClearBlocker() error = %v, want the no-blocker no-op", err)
	}

	task, ok := mgr.GetTaskProgress(deferTestTaskID)
	if !ok {
		t.Fatal("task missing from the store")
	}
	if task.OperatorNotes != nil {
		t.Errorf("OperatorNotes = %v, want nil on a task that was never blocked", task.OperatorNotes)
	}

	after := snapshotProgressDir(t, festivalPath)
	for name, content := range before {
		if after[name] != content {
			t.Errorf("%s changed although there was no blocker to clear", name)
		}
	}

	events := readDeferralEvents(t, festivalPath)
	for _, event := range events {
		if event.Event == EventUnblocked {
			t.Errorf("an unblocked event was written for a task with no blocker: %+v", event)
		}
	}
	if strings.Contains(after[ProgressEventsFile], "try the v2 endpoint") {
		t.Error("the dropped note reached the event log")
	}
}
