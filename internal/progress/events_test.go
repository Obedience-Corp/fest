package progress

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMaterializeState(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-1 * time.Hour)

	tests := []struct {
		name           string
		events         []ProgressEvent
		expectedStatus map[string]string
	}{
		{
			name:           "empty events",
			events:         []ProgressEvent{},
			expectedStatus: map[string]string{},
		},
		{
			name: "started event",
			events: []ProgressEvent{
				{Timestamp: now, Event: EventStarted, Task: "01_task.md"},
			},
			expectedStatus: map[string]string{"01_task.md": StatusInProgress},
		},
		{
			name: "started then completed",
			events: []ProgressEvent{
				{Timestamp: earlier, Event: EventStarted, Task: "01_task.md"},
				{Timestamp: now, Event: EventCompleted, Task: "01_task.md", Minutes: 60},
			},
			expectedStatus: map[string]string{"01_task.md": StatusCompleted},
		},
		{
			name: "blocked event",
			events: []ProgressEvent{
				{Timestamp: earlier, Event: EventStarted, Task: "01_task.md"},
				{Timestamp: now, Event: EventBlocked, Task: "01_task.md", Reason: "Waiting for review"},
			},
			expectedStatus: map[string]string{"01_task.md": StatusBlocked},
		},
		{
			name: "blocked then unblocked",
			events: []ProgressEvent{
				{Timestamp: earlier.Add(-1 * time.Hour), Event: EventStarted, Task: "01_task.md"},
				{Timestamp: earlier, Event: EventBlocked, Task: "01_task.md", Reason: "Waiting"},
				{Timestamp: now, Event: EventUnblocked, Task: "01_task.md"},
			},
			expectedStatus: map[string]string{"01_task.md": StatusInProgress},
		},
		{
			name: "multiple tasks",
			events: []ProgressEvent{
				{Timestamp: earlier, Event: EventStarted, Task: "01_task.md"},
				{Timestamp: earlier, Event: EventStarted, Task: "02_task.md"},
				{Timestamp: now, Event: EventCompleted, Task: "01_task.md", Minutes: 60},
			},
			expectedStatus: map[string]string{
				"01_task.md": StatusCompleted,
				"02_task.md": StatusInProgress,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := materializeState(tt.events)

			if len(tasks) != len(tt.expectedStatus) {
				t.Errorf("got %d tasks, want %d", len(tasks), len(tt.expectedStatus))
			}

			for taskID, expectedStatus := range tt.expectedStatus {
				task, ok := tasks[taskID]
				if !ok {
					t.Errorf("task %q not found", taskID)
					continue
				}
				if task.Status != expectedStatus {
					t.Errorf("task %q status = %q, want %q", taskID, task.Status, expectedStatus)
				}
			}
		})
	}
}

func TestMaterializeState_TimeSpent(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-1 * time.Hour)

	events := []ProgressEvent{
		{Timestamp: earlier, Event: EventStarted, Task: "01_task.md"},
		{Timestamp: now, Event: EventCompleted, Task: "01_task.md", Minutes: 45},
	}

	tasks := materializeState(events)
	task := tasks["01_task.md"]

	if task.TimeSpentMinutes != 45 {
		t.Errorf("TimeSpentMinutes = %d, want 45", task.TimeSpentMinutes)
	}

	if task.Progress != 100 {
		t.Errorf("Progress = %d, want 100", task.Progress)
	}
}

func TestGenerateEventsFromState(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-1 * time.Hour)

	tests := []struct {
		name          string
		tasks         map[string]*TaskProgress
		expectedCount int
	}{
		{
			name:          "empty tasks",
			tasks:         map[string]*TaskProgress{},
			expectedCount: 0,
		},
		{
			name: "completed task generates started and completed events",
			tasks: map[string]*TaskProgress{
				"01_task.md": {
					TaskID:           "01_task.md",
					Status:           StatusCompleted,
					StartedAt:        &earlier,
					CompletedAt:      &now,
					TimeSpentMinutes: 60,
				},
			},
			expectedCount: 2, // started + completed
		},
		{
			name: "in_progress task generates started event",
			tasks: map[string]*TaskProgress{
				"01_task.md": {
					TaskID:    "01_task.md",
					Status:    StatusInProgress,
					StartedAt: &earlier,
				},
			},
			expectedCount: 1, // started only
		},
		{
			name: "blocked task generates started and blocked events",
			tasks: map[string]*TaskProgress{
				"01_task.md": {
					TaskID:         "01_task.md",
					Status:         StatusBlocked,
					StartedAt:      &earlier,
					BlockedAt:      &now,
					BlockerMessage: "Waiting",
				},
			},
			expectedCount: 2, // started + blocked
		},
		{
			name: "pending task with no timestamps generates no events",
			tasks: map[string]*TaskProgress{
				"01_task.md": {
					TaskID: "01_task.md",
					Status: StatusPending,
				},
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := generateEventsFromState(tt.tasks)
			if len(events) != tt.expectedCount {
				t.Errorf("got %d events, want %d", len(events), tt.expectedCount)
			}
		})
	}
}

func TestStore_EventsRoundtrip(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create store and queue multiple events
	store1 := NewStore(tmpDir)
	if err := store1.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	now := time.Now().UTC()

	// Queue and save started event
	store1.QueueEvent(&ProgressEvent{
		Timestamp: now,
		Event:     EventStarted,
		Task:      "01_task.md",
	})
	if err := store1.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Queue and save completed event
	completedTime := now.Add(30 * time.Minute)
	store1.QueueEvent(&ProgressEvent{
		Timestamp: completedTime,
		Event:     EventCompleted,
		Task:      "01_task.md",
		Minutes:   30,
	})
	if err := store1.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load in new store
	store2 := NewStore(tmpDir)
	if err := store2.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify state
	task, found := store2.GetTask("01_task.md")
	if !found {
		t.Fatal("Task not found after reload")
	}

	if task.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", task.Status, StatusCompleted)
	}

	if task.TimeSpentMinutes != 30 {
		t.Errorf("TimeSpentMinutes = %d, want 30", task.TimeSpentMinutes)
	}
}

func TestStore_LegacyYAMLMigration(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a legacy YAML progress file
	legacyYAML := `festival: test-festival
updated_at: 2026-01-15T10:00:00Z
tasks:
  01_task.md:
    task_id: 01_task.md
    status: completed
    progress: 100
    started_at: 2026-01-15T09:00:00Z
    completed_at: 2026-01-15T10:00:00Z
    time_spent_minutes: 60
  02_task.md:
    task_id: 02_task.md
    status: in_progress
    progress: 50
    started_at: 2026-01-15T10:30:00Z
`
	festDir := filepath.Join(tmpDir, ProgressDir)
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("mkdir error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(festDir, legacyProgressFile), []byte(legacyYAML), 0644); err != nil {
		t.Fatalf("write error = %v", err)
	}

	// Load should trigger migration
	store := NewStore(tmpDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify tasks are loaded correctly
	task1, found := store.GetTask("01_task.md")
	if !found {
		t.Fatal("Task 01_task.md not found after migration")
	}
	if task1.Status != StatusCompleted {
		t.Errorf("Task 1 status = %q, want %q", task1.Status, StatusCompleted)
	}

	task2, found := store.GetTask("02_task.md")
	if !found {
		t.Fatal("Task 02_task.md not found after migration")
	}
	if task2.Status != StatusInProgress {
		t.Errorf("Task 2 status = %q, want %q", task2.Status, StatusInProgress)
	}

	// Verify JSONL file was created
	eventsPath := filepath.Join(festDir, ProgressEventsFile)
	if _, err := os.Stat(eventsPath); os.IsNotExist(err) {
		t.Fatal("JSONL events file was not created during migration")
	}

	// Verify YAML file was deleted
	yamlPath := filepath.Join(festDir, legacyProgressFile)
	if _, err := os.Stat(yamlPath); !os.IsNotExist(err) {
		t.Fatal("Legacy YAML file was not deleted during migration")
	}
}

func TestStore_EmptyLegacyMigration(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create an empty legacy YAML progress file (no tasks)
	legacyYAML := `festival: test-festival
updated_at: 2026-01-15T10:00:00Z
tasks: {}
`
	festDir := filepath.Join(tmpDir, ProgressDir)
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("mkdir error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(festDir, legacyProgressFile), []byte(legacyYAML), 0644); err != nil {
		t.Fatalf("write error = %v", err)
	}

	// Load should trigger migration
	store := NewStore(tmpDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify JSONL file was created (even if empty)
	eventsPath := filepath.Join(festDir, ProgressEventsFile)
	if _, err := os.Stat(eventsPath); os.IsNotExist(err) {
		t.Fatal("JSONL events file was not created during migration")
	}

	// Verify YAML file was deleted
	yamlPath := filepath.Join(festDir, legacyProgressFile)
	if _, err := os.Stat(yamlPath); !os.IsNotExist(err) {
		t.Fatal("Legacy YAML file was not deleted during migration")
	}
}

func TestStore_NoSaveWithoutEvent(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store := NewStore(tmpDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Set task without queuing event
	store.SetTask(&TaskProgress{
		TaskID: "01_task.md",
		Status: StatusInProgress,
	})

	// Save should do nothing without pending event
	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Events file should not exist
	eventsPath := filepath.Join(tmpDir, ProgressDir, ProgressEventsFile)
	if _, err := os.Stat(eventsPath); !os.IsNotExist(err) {
		t.Error("Events file should not be created without queued event")
	}
}

func TestStore_MultipleEventsAppend(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store := NewStore(tmpDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Queue and save multiple events
	events := []struct {
		event   EventType
		task    string
		minutes int
	}{
		{EventStarted, "01_task.md", 0},
		{EventProgress, "01_task.md", 0},
		{EventCompleted, "01_task.md", 30},
		{EventStarted, "02_task.md", 0},
	}

	for _, e := range events {
		store.QueueEvent(&ProgressEvent{
			Timestamp: time.Now().UTC(),
			Event:     e.event,
			Task:      e.task,
			Minutes:   e.minutes,
			Percent:   50, // Only used for progress events
		})
		if err := store.Save(ctx); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// Reload and verify
	store2 := NewStore(tmpDir)
	if err := store2.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	task1, found := store2.GetTask("01_task.md")
	if !found {
		t.Fatal("Task 01_task.md not found")
	}
	if task1.Status != StatusCompleted {
		t.Errorf("Task 1 status = %q, want %q", task1.Status, StatusCompleted)
	}

	task2, found := store2.GetTask("02_task.md")
	if !found {
		t.Fatal("Task 02_task.md not found")
	}
	if task2.Status != StatusInProgress {
		t.Errorf("Task 2 status = %q, want %q", task2.Status, StatusInProgress)
	}
}

func TestStore_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Load should fail with cancelled context
	if err := store.Load(ctx); err == nil {
		t.Error("Load() should fail with cancelled context")
	}

	// Save should fail with cancelled context
	store.QueueEvent(&ProgressEvent{
		Timestamp: time.Now().UTC(),
		Event:     EventStarted,
		Task:      "01_task.md",
	})
	if err := store.Save(ctx); err == nil {
		t.Error("Save() should fail with cancelled context")
	}
}
