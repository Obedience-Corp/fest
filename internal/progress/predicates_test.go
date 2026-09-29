package progress

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestTaskProgressPredicates(t *testing.T) {
	cases := []struct {
		name     string
		status   string
		deferred bool
		settled  bool
		done     bool
	}{
		{"pending", StatusPending, false, false, false},
		{"pending with stale deferral flag", StatusPending, true, false, false},
		{"in progress", StatusInProgress, false, false, false},
		{"in progress with stale deferral flag", StatusInProgress, true, false, false},
		{"blocked open", StatusBlocked, false, false, false},
		{"blocked deferred", StatusBlocked, true, true, false},
		{"completed", StatusCompleted, false, true, true},
		{"completed with stale deferral flag", StatusCompleted, true, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &TaskProgress{Status: tc.status, BlockerDeferred: tc.deferred}
			if got := task.IsSettled(); got != tc.settled {
				t.Errorf("IsSettled() = %v, want %v", got, tc.settled)
			}
			if got := task.IsDone(); got != tc.done {
				t.Errorf("IsDone() = %v, want %v", got, tc.done)
			}
		})
	}

	t.Run("nil is neither", func(t *testing.T) {
		var task *TaskProgress
		if task.IsSettled() || task.IsDone() {
			t.Error("nil task must be neither settled nor done")
		}
	})
}

func deferredTaskProgress(taskID string) *TaskProgress {
	at := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	return &TaskProgress{
		TaskID:            taskID,
		Status:            StatusBlocked,
		BlockerMessage:    "waiting on upstream",
		BlockedAt:         &at,
		BlockerDeferred:   true,
		BlockerDeferredAt: &at,
		BlockerDeferredBy: "operator",
		DeferralReason:    "upstream ships next week",
		BlockerAttempts:   []string{"retried the migration", "asked the owner"},
		OperatorNotes:     []string{"pick this up after the release"},
	}
}

func assertDeferralCleared(t *testing.T, task *TaskProgress) {
	t.Helper()
	if task.BlockerDeferred {
		t.Error("BlockerDeferred should be false")
	}
	if task.BlockerDeferredAt != nil {
		t.Errorf("BlockerDeferredAt should be nil, got %v", task.BlockerDeferredAt)
	}
	if task.BlockerDeferredBy != "" {
		t.Errorf("BlockerDeferredBy should be empty, got %q", task.BlockerDeferredBy)
	}
	if task.DeferralReason != "" {
		t.Errorf("DeferralReason should be empty, got %q", task.DeferralReason)
	}
	if task.BlockerAttempts != nil {
		t.Errorf("BlockerAttempts should be nil, got %v", task.BlockerAttempts)
	}
	if task.OperatorNotes != nil {
		t.Errorf("OperatorNotes should be nil, got %v", task.OperatorNotes)
	}
}

func TestTaskProgressClearDeferral(t *testing.T) {
	task := deferredTaskProgress("01_test.md")
	task.clearDeferral()
	assertDeferralCleared(t, task)

	if task.BlockerMessage != "waiting on upstream" {
		t.Errorf("BlockerMessage = %q, want it untouched", task.BlockerMessage)
	}
	if task.BlockedAt == nil {
		t.Error("BlockedAt should be untouched")
	}

	var nilTask *TaskProgress
	nilTask.clearDeferral()
}

func TestManagerClearsDeferralOnLifecycleTransitions(t *testing.T) {
	ctx := t.Context()

	cases := []struct {
		name string
		call func(mgr *Manager, taskID string) error
	}{
		{"ClearBlocker", func(mgr *Manager, taskID string) error { return mgr.ClearBlocker(ctx, taskID, "") }},
		{"MarkComplete", func(mgr *Manager, taskID string) error { return mgr.MarkComplete(ctx, taskID) }},
		{"ResetTask", func(mgr *Manager, taskID string) error { return mgr.ResetTask(ctx, taskID) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, err := NewManager(ctx, t.TempDir())
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}

			const taskID = "01_test.md"
			if err := mgr.ReportBlocker(ctx, taskID, "waiting on upstream", nil); err != nil {
				t.Fatalf("ReportBlocker() error = %v", err)
			}

			blocked, found := mgr.GetTaskProgress(taskID)
			if !found {
				t.Fatal("task not found after ReportBlocker")
			}
			seed := deferredTaskProgress(taskID)
			blocked.BlockerDeferred = seed.BlockerDeferred
			blocked.BlockerDeferredAt = seed.BlockerDeferredAt
			blocked.BlockerDeferredBy = seed.BlockerDeferredBy
			blocked.DeferralReason = seed.DeferralReason
			blocked.BlockerAttempts = seed.BlockerAttempts
			blocked.OperatorNotes = seed.OperatorNotes

			if err := tc.call(mgr, taskID); err != nil {
				t.Fatalf("%s() error = %v", tc.name, err)
			}

			task, found := mgr.GetTaskProgress(taskID)
			if !found {
				t.Fatalf("task not found after %s", tc.name)
			}
			assertDeferralCleared(t, task)
		})
	}
}

func TestTaskProgressLoadsPreDeferralRecord(t *testing.T) {
	const record = `task_id: 001_PHASE/01_seq/01_task.md
status: blocked
progress: 40
started_at: 2026-09-20T10:00:00Z
time_spent_minutes: 12
blocker_message: waiting on upstream
blocked_at: 2026-09-20T10:12:00Z
`

	var task TaskProgress
	if err := yaml.Unmarshal([]byte(record), &task); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if task.Status != StatusBlocked {
		t.Errorf("Status = %q, want %q", task.Status, StatusBlocked)
	}
	if task.BlockerMessage != "waiting on upstream" {
		t.Errorf("BlockerMessage = %q, want it preserved", task.BlockerMessage)
	}
	assertDeferralCleared(t, &task)
	if task.IsSettled() {
		t.Error("a pre-change blocked record must not be settled")
	}
	if task.IsDone() {
		t.Error("a pre-change blocked record must not be done")
	}
}
