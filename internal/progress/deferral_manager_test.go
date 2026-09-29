package progress

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const deferTestTaskID = "001_PHASE/01_sequence/01_task.md"

func deferTestAudit() DeferralAudit {
	return DeferralAudit{
		Actor:        "operator",
		TTY:          true,
		AgentMarkers: []string{"OBEY_AGENT", "CLAUDE_CODE", "CODEX_TASK", "OBEY_SESSION_ID"},
		Ancestry:     []string{"zsh", "login"},
		DeferredBy:   "Ada Lovelace",
	}
}

func snapshotProgressDir(t *testing.T, festivalPath string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	root := filepath.Join(festivalPath, ProgressDir)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel == ProgressLockFile {
			return nil
		}
		snapshot[rel] = string(data)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("snapshotting %s: %v", root, err)
	}
	return snapshot
}

func readDeferralEvents(t *testing.T, festivalPath string) []ProgressEvent {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(festivalPath, ProgressDir, ProgressEventsFile))
	if err != nil {
		t.Fatalf("reading the event log: %v", err)
	}

	var events []ProgressEvent
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var event ProgressEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decoding event %q: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}

func blockedDeferFixture(t *testing.T) (string, *Manager) {
	t.Helper()
	ctx := t.Context()
	festivalPath := createTestFestivalWithTasks(t)

	mgr, err := NewManager(ctx, festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := mgr.ReportBlocker(ctx, deferTestTaskID, "provider API removed", []string{"checked the changelog"}); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}
	return festivalPath, mgr
}

func TestDeferBlockerRecordsTheDeferralAndLeavesTheStatusBlocked(t *testing.T) {
	ctx := t.Context()
	festivalPath, mgr := blockedDeferFixture(t)

	taskFile := filepath.Join(festivalPath, "001_PHASE", "01_sequence", "01_task.md")
	beforeFrontmatter, err := os.ReadFile(taskFile)
	if err != nil {
		t.Fatalf("reading task file: %v", err)
	}

	if err := mgr.DeferBlocker(ctx, deferTestTaskID, "the vendor replies next week", deferTestAudit()); err != nil {
		t.Fatalf("DeferBlocker() error = %v", err)
	}

	task, ok := mgr.GetTaskProgress(deferTestTaskID)
	if !ok {
		t.Fatal("task missing from the store after DeferBlocker")
	}
	if task.Status != StatusBlocked {
		t.Errorf("Status = %q, want %q", task.Status, StatusBlocked)
	}
	if !task.BlockerDeferred {
		t.Error("BlockerDeferred = false, want true")
	}
	if task.BlockerDeferredAt == nil {
		t.Error("BlockerDeferredAt = nil, want a timestamp")
	}
	if task.BlockerDeferredBy != "Ada Lovelace" {
		t.Errorf("BlockerDeferredBy = %q, want %q", task.BlockerDeferredBy, "Ada Lovelace")
	}
	if task.DeferralReason != "the vendor replies next week" {
		t.Errorf("DeferralReason = %q, want the operator reason", task.DeferralReason)
	}
	if task.BlockerMessage != "provider API removed" {
		t.Errorf("BlockerMessage = %q, want the executor blocker to survive", task.BlockerMessage)
	}

	events := readDeferralEvents(t, festivalPath)
	var deferred *ProgressEvent
	for i := range events {
		if events[i].Event == EventBlockerDeferred {
			deferred = &events[i]
		}
	}
	if deferred == nil {
		t.Fatal("no EventBlockerDeferred in the event log")
	}
	if deferred.DeferralReason != "the vendor replies next week" || deferred.DeferredBy != "Ada Lovelace" {
		t.Errorf("event reason/deferred_by = %q/%q, want the operator values", deferred.DeferralReason, deferred.DeferredBy)
	}
	if deferred.Actor != "operator" || !deferred.TTY {
		t.Errorf("event actor/tty = %q/%v, want operator/true", deferred.Actor, deferred.TTY)
	}
	if strings.Join(deferred.AgentMarkers, ",") != "OBEY_AGENT,CLAUDE_CODE,CODEX_TASK,OBEY_SESSION_ID" {
		t.Errorf("event agent_markers = %v, want the four checked markers", deferred.AgentMarkers)
	}
	if strings.Join(deferred.Ancestry, ",") != "zsh,login" {
		t.Errorf("event ancestry = %v, want the recorded chain", deferred.Ancestry)
	}
	if deferred.Reason != "provider API removed" {
		t.Errorf("event reason = %q, want the executor blocker message", deferred.Reason)
	}

	afterFrontmatter, err := os.ReadFile(taskFile)
	if err != nil {
		t.Fatalf("reading task file: %v", err)
	}
	if string(afterFrontmatter) != string(beforeFrontmatter) {
		t.Errorf("task frontmatter changed; deferral must never be mirrored into the task file\nbefore:\n%s\nafter:\n%s",
			beforeFrontmatter, afterFrontmatter)
	}
}

func TestDeferBlockerRefusesEveryNonBlockedStatusAndWritesNothing(t *testing.T) {
	seed := map[string]func(ctx context.Context, mgr *Manager) error{
		StatusPending: func(ctx context.Context, mgr *Manager) error {
			if err := mgr.ReportBlocker(ctx, deferTestTaskID, "provider API removed", nil); err != nil {
				return err
			}
			return mgr.ResetTask(ctx, deferTestTaskID)
		},
		StatusInProgress: func(ctx context.Context, mgr *Manager) error {
			return mgr.MarkInProgress(ctx, deferTestTaskID)
		},
		StatusCompleted: func(ctx context.Context, mgr *Manager) error {
			return mgr.MarkComplete(ctx, deferTestTaskID)
		},
	}

	for status, seedTask := range seed {
		t.Run(status, func(t *testing.T) {
			ctx := t.Context()
			festivalPath := createTestFestivalWithTasks(t)

			mgr, err := NewManager(ctx, festivalPath)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			if err := seedTask(ctx, mgr); err != nil {
				t.Fatalf("seeding a %s task: %v", status, err)
			}

			seeded, ok := mgr.GetTaskProgress(deferTestTaskID)
			if !ok {
				t.Fatalf("seeded task missing from the store")
			}
			if seeded.Status != status {
				t.Fatalf("seeded status = %q, want %q", seeded.Status, status)
			}

			before := snapshotProgressDir(t, festivalPath)
			if len(before) == 0 {
				t.Fatal("expected the seeded state to be on disk before the refusal")
			}

			err = mgr.DeferBlocker(ctx, deferTestTaskID, "the vendor replies next week", deferTestAudit())
			if err == nil {
				t.Fatalf("DeferBlocker() on a %s task must fail", status)
			}
			if !strings.Contains(err.Error(), "only a blocked task can be deferred") {
				t.Errorf("error = %q, want the blocked-only refusal", err)
			}

			after := snapshotProgressDir(t, festivalPath)
			if len(before) != len(after) {
				t.Fatalf("progress dir file count changed: %d before, %d after", len(before), len(after))
			}
			for name, content := range before {
				if after[name] != content {
					t.Errorf("%s changed after a refused deferral", name)
				}
			}

			task, ok := mgr.GetTaskProgress(deferTestTaskID)
			if !ok {
				t.Fatal("task missing from the store")
			}
			if task.BlockerDeferred {
				t.Error("BlockerDeferred = true after a refused deferral")
			}
		})
	}
}

func TestDeferBlockerRequiresAReasonAndWritesNothing(t *testing.T) {
	ctx := t.Context()
	festivalPath, mgr := blockedDeferFixture(t)
	before := snapshotProgressDir(t, festivalPath)

	err := mgr.DeferBlocker(ctx, deferTestTaskID, "", deferTestAudit())
	if err == nil {
		t.Fatal("DeferBlocker() with no reason must fail")
	}
	if !strings.Contains(err.Error(), "deferral reason required") {
		t.Errorf("error = %q, want the missing-reason refusal", err)
	}

	after := snapshotProgressDir(t, festivalPath)
	for name, content := range before {
		if after[name] != content {
			t.Errorf("%s changed after a refused deferral", name)
		}
	}
}

func TestDeferBlockerRefusesACancelledContextAndWritesNothing(t *testing.T) {
	festivalPath, mgr := blockedDeferFixture(t)
	before := snapshotProgressDir(t, festivalPath)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := mgr.DeferBlocker(ctx, deferTestTaskID, "the vendor replies next week", deferTestAudit())
	if err == nil {
		t.Fatal("DeferBlocker() with a cancelled context must fail")
	}

	after := snapshotProgressDir(t, festivalPath)
	for name, content := range before {
		if after[name] != content {
			t.Errorf("%s changed after a cancelled deferral", name)
		}
	}
}

func TestDeferRefusesWhenUnblockedConcurrently(t *testing.T) {
	ctx := t.Context()
	festivalPath, _ := blockedDeferFixture(t)

	deferrer, err := NewManager(ctx, festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	unblocker, err := NewManager(ctx, festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	start := make(chan struct{})
	var deferErr, clearErr error

	var wg sync.WaitGroup
	wg.Go(func() {
		<-start
		deferErr = deferrer.DeferBlocker(ctx, deferTestTaskID, "the vendor replies next week", deferTestAudit())
	})
	wg.Go(func() {
		<-start
		clearErr = unblocker.ClearBlocker(ctx, deferTestTaskID, "")
	})
	close(start)
	wg.Wait()

	if clearErr != nil {
		t.Fatalf("ClearBlocker() error = %v", clearErr)
	}
	if deferErr != nil && !strings.Contains(deferErr.Error(), "only a blocked task can be deferred") {
		t.Fatalf("DeferBlocker() error = %v, want nil or the blocked-only refusal", deferErr)
	}

	reader, err := NewManager(ctx, festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	task, ok := reader.GetTaskProgress(deferTestTaskID)
	if !ok {
		t.Fatal("task missing from the store")
	}
	if task.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q after the unblock", task.Status, StatusInProgress)
	}
	if task.BlockerDeferred {
		t.Error("BlockerDeferred = true on an unblocked task; the unblock must clear any deferral that raced it")
	}
	if task.DeferralReason != "" || task.BlockerDeferredAt != nil || task.BlockerDeferredBy != "" {
		t.Errorf("deferral fields survived the unblock: %+v", task)
	}
}

func TestDeferResolvesBothKeyForms(t *testing.T) {
	for _, form := range []string{deferTestTaskID, "01_task.md"} {
		t.Run(form, func(t *testing.T) {
			ctx := t.Context()
			festivalPath := createTestFestivalWithTasks(t)

			key, err := NormalizeTaskID(festivalPath, form)
			if err != nil {
				t.Fatalf("NormalizeTaskID(%q) error = %v", form, err)
			}

			mgr, err := NewManager(ctx, festivalPath)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			if err := mgr.ReportBlocker(ctx, key, "provider API removed", nil); err != nil {
				t.Fatalf("ReportBlocker() error = %v", err)
			}
			if err := mgr.DeferBlocker(ctx, key, "the vendor replies next week", deferTestAudit()); err != nil {
				t.Fatalf("DeferBlocker() error = %v", err)
			}

			taskPath := filepath.Join(festivalPath, deferTestTaskID)
			task, ok := ResolveTaskProgress(mgr.Store(), festivalPath, taskPath)
			if !ok {
				t.Fatalf("ResolveTaskProgress could not find the task deferred as %q", form)
			}
			if !task.BlockerDeferred || task.DeferralReason != "the vendor replies next week" {
				t.Errorf("deferral not visible through the resolved record: %+v", task)
			}
		})
	}
}
