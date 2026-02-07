package progress

import (
	"context"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// Manager handles progress operations for a festival
type Manager struct {
	store *Store
}

// NewManager creates a new progress manager
func NewManager(ctx context.Context, festivalPath string) (*Manager, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	store := NewStore(festivalPath)
	if err := store.Load(ctx); err != nil {
		return nil, errors.Wrap(err, "loading progress data")
	}
	return &Manager{store: store}, nil
}

// UpdateProgress updates the progress percentage for a task
func (m *Manager) UpdateProgress(ctx context.Context, taskID string, progress int) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	if progress < 0 || progress > 100 {
		return errors.Validation("progress must be between 0 and 100").
			WithField("progress", progress)
	}

	task, exists := m.store.GetTask(taskID)
	if !exists {
		task = &TaskProgress{
			TaskID: taskID,
			Status: StatusPending,
		}
	}

	now := time.Now().UTC()

	// Start tracking time on first progress update
	// Use current time - we only track actual work time, not time since file creation
	if task.StartedAt == nil {
		task.StartedAt = &now
	}

	// If progress > 0, mark as in progress
	if progress > 0 && task.Status == StatusPending {
		task.Status = StatusInProgress
	}

	// If progress is 100, mark as completed
	if progress == 100 {
		task.Status = StatusCompleted
		task.CompletedAt = &now

		// Calculate time spent
		if task.StartedAt != nil {
			task.TimeSpentMinutes = int(now.Sub(*task.StartedAt).Minutes())
		}

		// Queue completed event
		m.store.QueueEvent(&ProgressEvent{
			Timestamp: now,
			Event:     EventCompleted,
			Task:      taskID,
			Minutes:   task.TimeSpentMinutes,
		})
	} else {
		// Queue progress event
		m.store.QueueEvent(&ProgressEvent{
			Timestamp: now,
			Event:     EventProgress,
			Task:      taskID,
			Percent:   progress,
		})
	}

	task.Progress = progress

	// Clear blocker if task is progressing
	if progress > 0 && task.BlockerMessage != "" {
		task.BlockerMessage = ""
		task.BlockedAt = nil
	}

	m.store.SetTask(task)
	return m.store.Save(ctx)
}

// MarkComplete marks a task as complete
func (m *Manager) MarkComplete(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	task, exists := m.store.GetTask(taskID)
	if !exists {
		task = &TaskProgress{
			TaskID: taskID,
		}
	}

	now := time.Now().UTC()

	// Set start time if not already set - use current time
	// We track actual work time, not elapsed time since file creation
	if task.StartedAt == nil {
		task.StartedAt = &now
	}

	task.Status = StatusCompleted
	task.Progress = 100
	task.CompletedAt = &now
	task.TimeSpentMinutes = int(now.Sub(*task.StartedAt).Minutes())

	// Clear any blocker
	task.BlockerMessage = ""
	task.BlockedAt = nil

	// Queue completed event
	m.store.QueueEvent(&ProgressEvent{
		Timestamp: now,
		Event:     EventCompleted,
		Task:      taskID,
		Minutes:   task.TimeSpentMinutes,
	})

	m.store.SetTask(task)
	return m.store.Save(ctx)
}

// MarkInProgress marks a task as in progress
func (m *Manager) MarkInProgress(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	task, exists := m.store.GetTask(taskID)
	if !exists {
		task = &TaskProgress{
			TaskID: taskID,
		}
	}

	now := time.Now().UTC()

	// Set start time if not already set
	// For MarkInProgress, use current time (user explicitly starting work)
	if task.StartedAt == nil {
		task.StartedAt = &now
	}

	task.Status = StatusInProgress

	// Queue started event
	m.store.QueueEvent(&ProgressEvent{
		Timestamp: now,
		Event:     EventStarted,
		Task:      taskID,
	})

	m.store.SetTask(task)
	return m.store.Save(ctx)
}

// ReportBlocker reports a blocker for a task
func (m *Manager) ReportBlocker(ctx context.Context, taskID, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	if message == "" {
		return errors.Validation("blocker message required")
	}

	task, exists := m.store.GetTask(taskID)
	if !exists {
		task = &TaskProgress{
			TaskID: taskID,
			Status: StatusPending,
		}
	}

	now := time.Now().UTC()
	task.Status = StatusBlocked
	task.BlockerMessage = message
	task.BlockedAt = &now

	// Start tracking time if not already
	if task.StartedAt == nil {
		task.StartedAt = &now
	}

	// Queue blocked event
	m.store.QueueEvent(&ProgressEvent{
		Timestamp: now,
		Event:     EventBlocked,
		Task:      taskID,
		Reason:    message,
	})

	m.store.SetTask(task)
	return m.store.Save(ctx)
}

// ResetTask resets a task back to pending status, clearing all progress data.
func (m *Manager) ResetTask(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	task, exists := m.store.GetTask(taskID)
	if !exists {
		task = &TaskProgress{
			TaskID: taskID,
		}
	}

	now := time.Now().UTC()

	task.Status = StatusPending
	task.Progress = 0
	task.StartedAt = nil
	task.CompletedAt = nil
	task.TimeSpentMinutes = 0
	task.BlockerMessage = ""
	task.BlockedAt = nil

	// Queue reset event
	m.store.QueueEvent(&ProgressEvent{
		Timestamp: now,
		Event:     EventReset,
		Task:      taskID,
	})

	m.store.SetTask(task)
	return m.store.Save(ctx)
}

// ClearBlocker clears a blocker for a task
func (m *Manager) ClearBlocker(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	task, exists := m.store.GetTask(taskID)
	if !exists {
		return errors.NotFound("task").WithField("taskID", taskID)
	}

	if task.BlockerMessage == "" {
		return nil // No blocker to clear
	}

	now := time.Now().UTC()
	task.BlockerMessage = ""
	task.BlockedAt = nil

	// Return to in_progress if was blocked
	if task.Status == StatusBlocked {
		task.Status = StatusInProgress
	}

	// Queue unblocked event
	m.store.QueueEvent(&ProgressEvent{
		Timestamp: now,
		Event:     EventUnblocked,
		Task:      taskID,
	})

	m.store.SetTask(task)
	return m.store.Save(ctx)
}

// GetTaskProgress retrieves progress for a specific task
func (m *Manager) GetTaskProgress(taskID string) (*TaskProgress, bool) {
	return m.store.GetTask(taskID)
}

// AllTaskProgress returns all task progress entries
func (m *Manager) AllTaskProgress() map[string]*TaskProgress {
	return m.store.AllTasks()
}

// Store returns the underlying store for advanced operations
func (m *Manager) Store() *Store {
	return m.store
}
