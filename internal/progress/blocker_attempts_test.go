package progress

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestReportBlockerRecordsAttempts(t *testing.T) {
	ctx := t.Context()
	mgr, err := NewManager(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	attempts := []string{"checked the changelog", "asked the vendor, no replacement"}
	if err := mgr.ReportBlocker(ctx, "01_test.md", "provider API removed", attempts); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}

	task, found := mgr.GetTaskProgress("01_test.md")
	if !found {
		t.Fatal("task not found after ReportBlocker")
	}
	if !slices.Equal(task.BlockerAttempts, attempts) {
		t.Errorf("BlockerAttempts = %q, want %q", task.BlockerAttempts, attempts)
	}
}

func TestReportBlockerReplacesAttemptsOnReblock(t *testing.T) {
	ctx := t.Context()
	mgr, err := NewManager(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if err := mgr.ReportBlocker(ctx, "01_test.md", "first", []string{"first attempt"}); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}
	second := []string{"second attempt", "third attempt"}
	if err := mgr.ReportBlocker(ctx, "01_test.md", "second", second); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}

	task, _ := mgr.GetTaskProgress("01_test.md")
	if !slices.Equal(task.BlockerAttempts, second) {
		t.Errorf("BlockerAttempts = %q, want %q", task.BlockerAttempts, second)
	}
}

func TestReportBlockerWithoutAttemptsMatchesPreChangeRecord(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	mgr, err := NewManager(ctx, dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if err := mgr.ReportBlocker(ctx, "01_test.md", "waiting on upstream", nil); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}

	task, _ := mgr.GetTaskProgress("01_test.md")
	if task.BlockerAttempts != nil {
		t.Errorf("BlockerAttempts = %q, want nil", task.BlockerAttempts)
	}

	raw, err := os.ReadFile(filepath.Join(dir, ProgressDir, ProgressEventsFile))
	if err != nil {
		t.Fatalf("reading events file: %v", err)
	}
	ts := regexp.MustCompile(`"ts":"[^"]*"`)
	got := strings.TrimSpace(ts.ReplaceAllString(string(raw), `"ts":"TS"`))
	const want = `{"ts":"TS","event":"blocked","task":"01_test.md","reason":"waiting on upstream"}`
	if got != want {
		t.Errorf("events record =\n%s\nwant\n%s", got, want)
	}
}

func TestBlockedEventReplaysAttempts(t *testing.T) {
	at := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	attempts := []string{"checked the changelog", "asked the vendor, no replacement"}

	tasks, _ := materializeState([]ProgressEvent{
		{Timestamp: at, Event: EventBlocked, Task: "01_test.md", Reason: "provider API removed", Attempts: attempts},
	})

	task, ok := tasks["01_test.md"]
	if !ok {
		t.Fatal("task not materialized from blocked event")
	}
	if task.Status != StatusBlocked {
		t.Errorf("Status = %q, want %q", task.Status, StatusBlocked)
	}
	if !slices.Equal(task.BlockerAttempts, attempts) {
		t.Errorf("BlockerAttempts = %q, want %q", task.BlockerAttempts, attempts)
	}

	cleared, _ := materializeState([]ProgressEvent{
		{Timestamp: at, Event: EventBlocked, Task: "01_test.md", Reason: "provider API removed", Attempts: attempts},
		{Timestamp: at.Add(time.Minute), Event: EventUnblocked, Task: "01_test.md"},
	})
	if cleared["01_test.md"].BlockerAttempts != nil {
		t.Errorf("BlockerAttempts = %q, want nil after unblocked replay", cleared["01_test.md"].BlockerAttempts)
	}

	reset, _ := materializeState([]ProgressEvent{
		{Timestamp: at, Event: EventBlocked, Task: "01_test.md", Reason: "provider API removed", Attempts: attempts},
		{Timestamp: at.Add(time.Minute), Event: EventReset, Task: "01_test.md"},
	})
	if reset["01_test.md"].BlockerAttempts != nil {
		t.Errorf("BlockerAttempts = %q, want nil after reset replay", reset["01_test.md"].BlockerAttempts)
	}

	completed, _ := materializeState([]ProgressEvent{
		{Timestamp: at, Event: EventBlocked, Task: "01_test.md", Reason: "provider API removed", Attempts: attempts},
		{Timestamp: at.Add(time.Minute), Event: EventCompleted, Task: "01_test.md"},
	})
	if completed["01_test.md"].BlockerAttempts != nil {
		t.Errorf("BlockerAttempts = %q, want nil after completed replay", completed["01_test.md"].BlockerAttempts)
	}
}
