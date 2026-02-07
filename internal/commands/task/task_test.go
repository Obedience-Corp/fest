package task

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
)

// setupTestFestival creates a minimal festival directory structure for testing.
func setupTestFestival(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Create fest.yaml (festival marker)
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte("name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create phase/sequence/task structure
	seqDir := filepath.Join(dir, "001_PHASE", "01_sequence")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create task files
	tasks := []string{"01_first.md", "02_second.md"}
	for _, name := range tasks {
		content := "---\nfest_type: task\n---\n# " + name + "\nTask content here.\n"
		if err := os.WriteFile(filepath.Join(seqDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create second sequence for ambiguity tests
	seq2Dir := filepath.Join(dir, "002_PHASE2", "01_other")
	if err := os.MkdirAll(seq2Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seq2Dir, "01_first.md"), []byte("# dup\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .fest directory for progress tracking
	if err := os.MkdirAll(filepath.Join(dir, ".fest"), 0755); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestResolveExplicitTask_FullPath(t *testing.T) {
	dir := setupTestFestival(t)

	taskID, filePath, err := resolveExplicitTask(dir, "001_PHASE/01_sequence/01_first.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskID != "001_PHASE/01_sequence/01_first.md" {
		t.Errorf("expected taskID '001_PHASE/01_sequence/01_first.md', got %q", taskID)
	}
	expectedPath := filepath.Join(dir, "001_PHASE/01_sequence/01_first.md")
	if filePath != expectedPath {
		t.Errorf("expected filePath %q, got %q", expectedPath, filePath)
	}
}

func TestResolveExplicitTask_BareFilename(t *testing.T) {
	dir := setupTestFestival(t)

	// 02_second.md is unique across the festival
	taskID, _, err := resolveExplicitTask(dir, "02_second.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskID != "001_PHASE/01_sequence/02_second.md" {
		t.Errorf("expected '001_PHASE/01_sequence/02_second.md', got %q", taskID)
	}
}

func TestResolveExplicitTask_AmbiguousFilename(t *testing.T) {
	dir := setupTestFestival(t)

	// 01_first.md exists in both sequences
	_, _, err := resolveExplicitTask(dir, "01_first.md")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	if !containsString(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguity error, got: %v", err)
	}
}

func TestResolveExplicitTask_NotFound(t *testing.T) {
	dir := setupTestFestival(t)

	_, _, err := resolveExplicitTask(dir, "99_nonexistent.md")
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

func TestFindTaskMatches(t *testing.T) {
	dir := setupTestFestival(t)

	tests := []struct {
		name      string
		query     string
		wantCount int
	}{
		{"unique match", "02_second.md", 1},
		{"duplicate match", "01_first.md", 2},
		{"no match", "99_nope.md", 0},
		{"without extension", "02_second", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := findTaskMatches(dir, tt.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(matches) != tt.wantCount {
				t.Errorf("expected %d matches, got %d: %v", tt.wantCount, len(matches), matches)
			}
		})
	}
}

func TestResetTaskEvent_MaterializesCorrectly(t *testing.T) {
	events := []progress.ProgressEvent{
		{Event: progress.EventStarted, Task: "t1"},
		{Event: progress.EventCompleted, Task: "t1", Minutes: 5},
		{Event: progress.EventReset, Task: "t1"},
	}

	// Use unexported materializeState via a higher-level path: create a store
	// Instead, test via the Manager API which is the public interface.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte("name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	mgr, err := progress.NewManager(ctx, dir)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	// Mark task complete first
	if err := mgr.MarkComplete(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	task, ok := mgr.GetTaskProgress("t1")
	if !ok {
		t.Fatal("task not found after completion")
	}
	if task.Status != progress.StatusCompleted {
		t.Fatalf("expected completed, got %s", task.Status)
	}

	// Reset it
	if err := mgr.ResetTask(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	task, ok = mgr.GetTaskProgress("t1")
	if !ok {
		t.Fatal("task not found after reset")
	}
	if task.Status != progress.StatusPending {
		t.Errorf("expected pending after reset, got %s", task.Status)
	}
	if task.Progress != 0 {
		t.Errorf("expected 0%% after reset, got %d%%", task.Progress)
	}
	if task.StartedAt != nil {
		t.Error("expected nil StartedAt after reset")
	}
	if task.CompletedAt != nil {
		t.Error("expected nil CompletedAt after reset")
	}
	if task.TimeSpentMinutes != 0 {
		t.Errorf("expected 0 minutes after reset, got %d", task.TimeSpentMinutes)
	}
	if task.BlockerMessage != "" {
		t.Errorf("expected empty blocker after reset, got %q", task.BlockerMessage)
	}

	// Verify event was persisted by reloading
	mgr2, err := progress.NewManager(ctx, dir)
	if err != nil {
		t.Fatalf("unexpected error reloading manager: %v", err)
	}
	task2, ok := mgr2.GetTaskProgress("t1")
	if !ok {
		t.Fatal("task not found after reload")
	}
	if task2.Status != progress.StatusPending {
		t.Errorf("expected pending after reload, got %s", task2.Status)
	}
	// Note: EventReset events are not tested here directly; we verify via
	// the materializeState() that underlies loadFromEvents().
	_ = events // Reference the events variable to document our intent
}

func TestResetBlockedTask(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte("name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	mgr, err := progress.NewManager(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	// Block a task
	if err := mgr.ReportBlocker(ctx, "blocked_task", "waiting for API key"); err != nil {
		t.Fatal(err)
	}
	task, _ := mgr.GetTaskProgress("blocked_task")
	if task.Status != progress.StatusBlocked {
		t.Fatalf("expected blocked, got %s", task.Status)
	}

	// Reset it
	if err := mgr.ResetTask(ctx, "blocked_task"); err != nil {
		t.Fatal(err)
	}
	task, _ = mgr.GetTaskProgress("blocked_task")
	if task.Status != progress.StatusPending {
		t.Errorf("expected pending, got %s", task.Status)
	}
	if task.BlockerMessage != "" {
		t.Errorf("expected empty blocker, got %q", task.BlockerMessage)
	}
	if task.BlockedAt != nil {
		t.Error("expected nil BlockedAt")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
