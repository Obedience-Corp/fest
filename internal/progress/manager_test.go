package progress

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

func TestManager_UpdateProgress(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	mgr, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Test initial progress update
	err = mgr.UpdateProgress(ctx, "01_test.md", 25)
	if err != nil {
		t.Fatalf("UpdateProgress() error = %v", err)
	}

	task, found := mgr.GetTaskProgress("01_test.md")
	if !found {
		t.Fatal("Task not found after update")
	}

	if task.Progress != 25 {
		t.Errorf("Progress = %d, want 25", task.Progress)
	}

	if task.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", task.Status, StatusInProgress)
	}

	if task.StartedAt == nil {
		t.Error("StartedAt should be set")
	}
}

func TestManager_UpdateProgress_Complete(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	mgr, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Update to 100%
	err = mgr.UpdateProgress(ctx, "01_test.md", 100)
	if err != nil {
		t.Fatalf("UpdateProgress() error = %v", err)
	}

	task, _ := mgr.GetTaskProgress("01_test.md")

	if task.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", task.Status, StatusCompleted)
	}

	if task.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestManager_UpdateProgress_Invalid(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	mgr, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Test invalid progress values
	err = mgr.UpdateProgress(ctx, "01_test.md", -1)
	if err == nil {
		t.Error("UpdateProgress() should error for negative value")
	}

	err = mgr.UpdateProgress(ctx, "01_test.md", 101)
	if err == nil {
		t.Error("UpdateProgress() should error for value > 100")
	}
}

func TestManager_MarkComplete(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	mgr, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = mgr.MarkComplete(ctx, "01_test.md")
	if err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	task, _ := mgr.GetTaskProgress("01_test.md")

	if task.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", task.Status, StatusCompleted)
	}

	if task.Progress != 100 {
		t.Errorf("Progress = %d, want 100", task.Progress)
	}

	if task.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestManager_ReportBlocker(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	mgr, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = mgr.ReportBlocker(ctx, "01_test.md", "Waiting on API spec")
	if err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}

	task, _ := mgr.GetTaskProgress("01_test.md")

	if task.Status != StatusBlocked {
		t.Errorf("Status = %q, want %q", task.Status, StatusBlocked)
	}

	if task.BlockerMessage != "Waiting on API spec" {
		t.Errorf("BlockerMessage = %q, want %q", task.BlockerMessage, "Waiting on API spec")
	}

	if task.BlockedAt == nil {
		t.Error("BlockedAt should be set")
	}
}

func TestManager_ReportBlocker_EmptyMessage(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	mgr, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = mgr.ReportBlocker(ctx, "01_test.md", "")
	if err == nil {
		t.Error("ReportBlocker() should error for empty message")
	}
}

func TestManager_ClearBlocker(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	mgr, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Report blocker
	if err := mgr.ReportBlocker(ctx, "01_test.md", "Blocker"); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}

	// Clear it
	err = mgr.ClearBlocker(ctx, "01_test.md")
	if err != nil {
		t.Fatalf("ClearBlocker() error = %v", err)
	}

	task, _ := mgr.GetTaskProgress("01_test.md")

	if task.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", task.Status, StatusInProgress)
	}

	if task.BlockerMessage != "" {
		t.Errorf("BlockerMessage should be empty, got %q", task.BlockerMessage)
	}
}

func TestManager_MarkInProgress(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	mgr, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = mgr.MarkInProgress(ctx, "01_test.md")
	if err != nil {
		t.Fatalf("MarkInProgress() error = %v", err)
	}

	task, _ := mgr.GetTaskProgress("01_test.md")

	if task.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", task.Status, StatusInProgress)
	}

	if task.StartedAt == nil {
		t.Error("StartedAt should be set")
	}
}

func TestManager_MarkComplete_UsesCurrentTime(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a task file (modification time doesn't matter anymore)
	taskPath := filepath.Join(tmpDir, "01_test.md")
	if err := os.WriteFile(taskPath, []byte("# Test Task\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Set modification time to 10 minutes ago (should be ignored)
	oldTime := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(taskPath, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set file mod time: %v", err)
	}

	beforeCall := time.Now().UTC()

	mgr, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Mark complete directly without prior MarkInProgress
	err = mgr.MarkComplete(ctx, "01_test.md")
	if err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	afterCall := time.Now().UTC()

	task, _ := mgr.GetTaskProgress("01_test.md")

	// TimeSpentMinutes should be 0 (or very close) since start and complete happen together
	// We track actual work time, not elapsed time since file creation
	if task.TimeSpentMinutes > 1 {
		t.Errorf("TimeSpentMinutes = %d, want 0 (no actual work time tracked)", task.TimeSpentMinutes)
	}

	// StartedAt should be close to current time, NOT the file modification time
	if task.StartedAt == nil {
		t.Fatal("StartedAt should be set")
	}

	if task.StartedAt.Before(beforeCall.Add(-time.Second)) || task.StartedAt.After(afterCall.Add(time.Second)) {
		t.Errorf("StartedAt = %v, expected between %v and %v (current time, not file mod time)", task.StartedAt, beforeCall, afterCall)
	}
}

func TestManager_MarkComplete_PreservesExistingStartTime(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	mgr, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// First mark in progress (sets StartedAt to current time)
	err = mgr.MarkInProgress(ctx, "01_test.md")
	if err != nil {
		t.Fatalf("MarkInProgress() error = %v", err)
	}

	task, _ := mgr.GetTaskProgress("01_test.md")
	originalStartedAt := *task.StartedAt

	// Wait a moment to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Now mark complete
	err = mgr.MarkComplete(ctx, "01_test.md")
	if err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	task, _ = mgr.GetTaskProgress("01_test.md")

	// StartedAt should NOT have changed
	if !task.StartedAt.Equal(originalStartedAt) {
		t.Errorf("StartedAt changed from %v to %v, should be preserved", originalStartedAt, task.StartedAt)
	}
}

// TestManager_PersistenceRoundtrip verifies that Manager operations persist to JSONL
// and can be reloaded by a new Manager instance.
func TestManager_PersistenceRoundtrip(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// First manager: create and modify tasks
	mgr1, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Mark a task in progress
	if err := mgr1.MarkInProgress(ctx, "01_task.md"); err != nil {
		t.Fatalf("MarkInProgress() error = %v", err)
	}

	// Mark another task complete
	if err := mgr1.MarkComplete(ctx, "02_task.md"); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	// Verify JSONL file was created
	eventsPath := filepath.Join(tmpDir, ProgressDir, ProgressEventsFile)
	if _, err := os.Stat(eventsPath); os.IsNotExist(err) {
		t.Fatal("JSONL events file was not created by Manager")
	}

	// Second manager: reload and verify state
	mgr2, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() reload error = %v", err)
	}

	task1, found := mgr2.GetTaskProgress("01_task.md")
	if !found {
		t.Fatal("Task 01_task.md not found after reload")
	}
	if task1.Status != StatusInProgress {
		t.Errorf("Task 1 status = %q, want %q", task1.Status, StatusInProgress)
	}

	task2, found := mgr2.GetTaskProgress("02_task.md")
	if !found {
		t.Fatal("Task 02_task.md not found after reload")
	}
	if task2.Status != StatusCompleted {
		t.Errorf("Task 2 status = %q, want %q", task2.Status, StatusCompleted)
	}
}

// TestManager_BlockerPersistence verifies blocker events persist correctly.
func TestManager_BlockerPersistence(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// First manager: report and clear blocker
	mgr1, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if err := mgr1.ReportBlocker(ctx, "01_task.md", "Waiting for review"); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}

	// Clear the blocker
	if err := mgr1.ClearBlocker(ctx, "01_task.md"); err != nil {
		t.Fatalf("ClearBlocker() error = %v", err)
	}

	// Reload and verify unblocked state
	mgr2, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() reload error = %v", err)
	}

	task, found := mgr2.GetTaskProgress("01_task.md")
	if !found {
		t.Fatal("Task not found after reload")
	}
	if task.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q (should be unblocked)", task.Status, StatusInProgress)
	}
	if task.BlockerMessage != "" {
		t.Errorf("BlockerMessage = %q, want empty", task.BlockerMessage)
	}
}

// TestManager_ProgressUpdatePersistence verifies progress events persist correctly.
func TestManager_ProgressUpdatePersistence(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// First manager: update progress
	mgr1, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if err := mgr1.UpdateProgress(ctx, "01_task.md", 50); err != nil {
		t.Fatalf("UpdateProgress() error = %v", err)
	}

	// Reload and verify progress
	mgr2, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() reload error = %v", err)
	}

	task, found := mgr2.GetTaskProgress("01_task.md")
	if !found {
		t.Fatal("Task not found after reload")
	}
	if task.Progress != 50 {
		t.Errorf("Progress = %d, want 50", task.Progress)
	}
	if task.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", task.Status, StatusInProgress)
	}
}

func TestSyncFrontmatterStatus(t *testing.T) {
	tests := []struct {
		name           string
		taskID         string
		status         string
		initialContent string
		wantStatus     frontmatter.Status
		wantNoChange   bool // true if file should not be changed
		noFile         bool // true if no file should be created
	}{
		{
			name:   "updates pending to completed",
			taskID: "003_EXECUTE/01_sequence/01_task.md",
			status: StatusCompleted,
			initialContent: `---
fest_type: task
fest_id: 01_task
fest_status: pending
fest_created: 2026-02-20T00:00:00Z
---

# Task Content
`,
			wantStatus: frontmatter.StatusCompleted,
		},
		{
			name:   "updates pending to in_progress",
			taskID: "003_EXECUTE/01_sequence/02_task.md",
			status: StatusInProgress,
			initialContent: `---
fest_type: task
fest_id: 02_task
fest_status: pending
fest_created: 2026-02-20T00:00:00Z
---

# Task Content
`,
			wantStatus: frontmatter.StatusInProgress,
		},
		{
			name:   "updates to blocked",
			taskID: "003_EXECUTE/01_sequence/03_task.md",
			status: StatusBlocked,
			initialContent: `---
fest_type: task
fest_id: 03_task
fest_status: in_progress
fest_created: 2026-02-20T00:00:00Z
---

# Task Content
`,
			wantStatus: frontmatter.StatusBlocked,
		},
		{
			name:   "resets to pending",
			taskID: "003_EXECUTE/01_sequence/04_task.md",
			status: StatusPending,
			initialContent: `---
fest_type: task
fest_id: 04_task
fest_status: completed
fest_created: 2026-02-20T00:00:00Z
---

# Task Content
`,
			wantStatus: frontmatter.StatusPending,
		},
		{
			name:         "no-ops when file does not exist",
			taskID:       "003_EXECUTE/01_sequence/nonexistent.md",
			status:       StatusCompleted,
			noFile:       true,
			wantNoChange: true,
		},
		{
			name:   "no-ops when already in sync",
			taskID: "003_EXECUTE/01_sequence/05_task.md",
			status: StatusCompleted,
			initialContent: `---
fest_type: task
fest_id: 05_task
fest_status: completed
fest_created: 2026-02-20T00:00:00Z
---

# Already completed
`,
			wantNoChange: true,
		},
		{
			name:           "no-ops when no frontmatter",
			taskID:         "003_EXECUTE/01_sequence/06_task.md",
			status:         StatusCompleted,
			initialContent: "# Just a plain markdown file\n",
			wantNoChange:   true,
		},
		{
			name:   "appends .md when missing from taskID",
			taskID: "003_EXECUTE/01_sequence/07_task",
			status: StatusCompleted,
			initialContent: `---
fest_type: task
fest_id: 07_task
fest_status: pending
fest_created: 2026-02-20T00:00:00Z
---

# Task Content
`,
			wantStatus: frontmatter.StatusCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()

			mgr, err := NewManager(ctx, tmpDir)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}

			// Resolve the file path, appending .md if needed
			taskPath := filepath.Join(tmpDir, tt.taskID)
			if filepath.Ext(taskPath) != ".md" {
				taskPath += ".md"
			}

			if !tt.noFile {
				// Create directory structure
				if err := os.MkdirAll(filepath.Dir(taskPath), 0o755); err != nil {
					t.Fatalf("MkdirAll() error = %v", err)
				}
				if err := os.WriteFile(taskPath, []byte(tt.initialContent), 0o644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
			}

			mgr.SyncFrontmatterStatus(tt.taskID, tt.status)

			if tt.noFile {
				// File should still not exist
				if _, err := os.Stat(taskPath); err == nil {
					t.Error("File should not have been created")
				}
				return
			}

			content, err := os.ReadFile(taskPath)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}

			if tt.wantNoChange {
				if string(content) != tt.initialContent {
					t.Error("File should not have been modified")
				}
				return
			}

			fm, _, err := frontmatter.Parse(content)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if fm == nil {
				t.Fatal("Expected frontmatter after sync")
			}

			if fm.Status != tt.wantStatus {
				t.Errorf("fest_status = %q, want %q", fm.Status, tt.wantStatus)
			}

			if fm.Updated.IsZero() {
				t.Error("fest_updated should be set after sync")
			}
		})
	}
}

func TestMarkComplete_SyncsFrontmatter(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a task file with pending frontmatter
	taskDir := filepath.Join(tmpDir, "003_EXECUTE", "01_sequence")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	taskContent := `---
fest_type: task
fest_id: 01_task
fest_status: pending
fest_created: 2026-02-20T00:00:00Z
---

# Task: Do something
`
	taskPath := filepath.Join(taskDir, "01_task.md")
	if err := os.WriteFile(taskPath, []byte(taskContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	mgr, err := NewManager(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	taskID := "003_EXECUTE/01_sequence/01_task.md"
	if err := mgr.MarkComplete(ctx, taskID); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	// Verify JSONL was updated
	task, found := mgr.GetTaskProgress(taskID)
	if !found {
		t.Fatal("Task not found in progress store")
	}
	if task.Status != StatusCompleted {
		t.Errorf("progress status = %q, want %q", task.Status, StatusCompleted)
	}

	// Verify frontmatter was updated
	content, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	fm, _, err := frontmatter.Parse(content)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if fm == nil {
		t.Fatal("Expected frontmatter in task file")
	}
	if fm.Status != frontmatter.StatusCompleted {
		t.Errorf("frontmatter fest_status = %q, want %q", fm.Status, frontmatter.StatusCompleted)
	}
}

// setupPropagationFestival creates a minimal festival directory structure for propagation tests.
// Returns the festival root path. Creates:
//
//	festivalRoot/001_PHASE/01_sequence/{01_task.md, 02_task.md, SEQUENCE_GOAL.md}
//	festivalRoot/001_PHASE/PHASE_GOAL.md
func setupPropagationFestival(t *testing.T) (string, string, string) {
	t.Helper()
	festDir := t.TempDir()

	seqDir := filepath.Join(festDir, "001_PHASE", "01_sequence")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	taskFM := `---
fest_type: task
fest_id: %s
fest_status: pending
fest_created: 2026-03-01T00:00:00Z
---

# Task
`
	for _, name := range []string{"01_task.md", "02_task.md"} {
		p := filepath.Join(seqDir, name)
		if err := os.WriteFile(p, []byte(fmt.Sprintf(taskFM, name)), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	seqGoal := `---
fest_type: sequence
fest_id: 01_sequence
fest_status: pending
fest_created: 2026-03-01T00:00:00Z
---

# Sequence Goal
`
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte(seqGoal), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	phaseGoal := `---
fest_type: phase
fest_id: 001_PHASE
fest_status: pending
fest_created: 2026-03-01T00:00:00Z
---

# Phase Goal
`
	if err := os.WriteFile(filepath.Join(festDir, "001_PHASE", "PHASE_GOAL.md"), []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return festDir, "001_PHASE/01_sequence/01_task.md", "001_PHASE/01_sequence/02_task.md"
}

func readStatus(t *testing.T, path string) frontmatter.Status {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		t.Fatalf("Parse frontmatter(%s): %v", path, err)
	}
	return fm.Status
}

func TestPropagateCompletion_SequenceComplete(t *testing.T) {
	ctx := context.Background()
	festDir, task1, task2 := setupPropagationFestival(t)

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Complete both tasks
	if err := mgr.MarkComplete(ctx, task1); err != nil {
		t.Fatalf("MarkComplete(%s): %v", task1, err)
	}
	if err := mgr.MarkComplete(ctx, task2); err != nil {
		t.Fatalf("MarkComplete(%s): %v", task2, err)
	}

	// Verify sequence goal was updated
	seqGoalPath := filepath.Join(festDir, "001_PHASE", "01_sequence", "SEQUENCE_GOAL.md")
	got := readStatus(t, seqGoalPath)
	if got != frontmatter.StatusCompleted {
		t.Errorf("SEQUENCE_GOAL.md status = %q, want %q", got, frontmatter.StatusCompleted)
	}
}

func TestPropagateCompletion_PhaseComplete(t *testing.T) {
	ctx := context.Background()
	festDir, task1, task2 := setupPropagationFestival(t)

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Complete all tasks
	if err := mgr.MarkComplete(ctx, task1); err != nil {
		t.Fatalf("MarkComplete(%s): %v", task1, err)
	}
	if err := mgr.MarkComplete(ctx, task2); err != nil {
		t.Fatalf("MarkComplete(%s): %v", task2, err)
	}

	// Verify phase goal was updated
	phaseGoalPath := filepath.Join(festDir, "001_PHASE", "PHASE_GOAL.md")
	got := readStatus(t, phaseGoalPath)
	if got != frontmatter.StatusCompleted {
		t.Errorf("PHASE_GOAL.md status = %q, want %q", got, frontmatter.StatusCompleted)
	}
}

func TestPropagateCompletion_Partial(t *testing.T) {
	ctx := context.Background()
	festDir, task1, _ := setupPropagationFestival(t)

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Complete only one of two tasks
	if err := mgr.MarkComplete(ctx, task1); err != nil {
		t.Fatalf("MarkComplete(%s): %v", task1, err)
	}

	// Sequence and phase should still be pending
	seqGoalPath := filepath.Join(festDir, "001_PHASE", "01_sequence", "SEQUENCE_GOAL.md")
	got := readStatus(t, seqGoalPath)
	if got != frontmatter.StatusPending {
		t.Errorf("SEQUENCE_GOAL.md status = %q, want %q (partial completion)", got, frontmatter.StatusPending)
	}

	phaseGoalPath := filepath.Join(festDir, "001_PHASE", "PHASE_GOAL.md")
	got = readStatus(t, phaseGoalPath)
	if got != frontmatter.StatusPending {
		t.Errorf("PHASE_GOAL.md status = %q, want %q (partial completion)", got, frontmatter.StatusPending)
	}
}
