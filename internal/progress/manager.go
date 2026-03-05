package progress

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
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
	if err := m.store.Save(ctx); err != nil {
		return err
	}
	m.SyncFrontmatterStatus(taskID, task.Status)
	return nil
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
	if err := m.store.Save(ctx); err != nil {
		return err
	}
	m.SyncFrontmatterStatus(taskID, task.Status)
	// Propagation is best-effort: a failure here should not block
	// the task completion that already succeeded above.
	_ = m.PropagateCompletion(ctx, taskID)
	return nil
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
	if err := m.store.Save(ctx); err != nil {
		return err
	}
	m.SyncFrontmatterStatus(taskID, task.Status)
	return nil
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
	if err := m.store.Save(ctx); err != nil {
		return err
	}
	m.SyncFrontmatterStatus(taskID, task.Status)
	return nil
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
	if err := m.store.Save(ctx); err != nil {
		return err
	}
	m.SyncFrontmatterStatus(taskID, task.Status)
	return nil
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
	if err := m.store.Save(ctx); err != nil {
		return err
	}
	m.SyncFrontmatterStatus(taskID, task.Status)
	return nil
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

// SyncFrontmatterStatus updates the fest_status field in a task file's
// YAML frontmatter to match the progress store's status. This keeps the
// file's frontmatter in sync with the JSONL event log.
func (m *Manager) SyncFrontmatterStatus(taskID, status string) {
	taskPath := filepath.Join(m.store.FestivalPath(), taskID)
	if !strings.HasSuffix(taskPath, ".md") {
		taskPath += ".md"
	}

	content, err := os.ReadFile(taskPath)
	if err != nil {
		return // File doesn't exist or can't be read — no-op
	}

	fm, remaining, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		return // No frontmatter or parse error — no-op
	}

	fmStatus := frontmatter.Status(status)
	if fm.Status == fmStatus {
		return // Already in sync
	}

	fm.Status = fmStatus
	fm.Updated = time.Now()

	newContent, err := frontmatter.Inject(remaining, fm)
	if err != nil {
		return
	}

	_ = os.WriteFile(taskPath, newContent, 0o644)
}

// PropagateCompletion checks if the parent sequence and phase of a task
// are fully complete, and updates their frontmatter status if so.
// Errors are non-fatal — propagation is best-effort to avoid blocking
// the primary task completion flow.
func (m *Manager) PropagateCompletion(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled before propagation")
	}

	festPath := m.store.FestivalPath()
	taskPath := filepath.Join(festPath, taskID)
	if !strings.HasSuffix(taskPath, ".md") {
		taskPath += ".md"
	}

	seqPath := filepath.Dir(taskPath)
	phasePath := filepath.Dir(seqPath)

	if err := m.propagateSequenceCompletion(ctx, seqPath); err != nil {
		return errors.Wrap(err, "propagating sequence completion")
	}

	if err := m.propagatePhaseCompletion(ctx, phasePath); err != nil {
		return errors.Wrap(err, "propagating phase completion")
	}

	return nil
}

// PropagatePhaseCompletion checks if a phase is fully complete and updates
// its PHASE_GOAL.md frontmatter. Use this after workflow completion.
func (m *Manager) PropagatePhaseCompletion(ctx context.Context, phasePath string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled before phase propagation")
	}
	return m.propagatePhaseCompletion(ctx, phasePath)
}

// propagateSequenceCompletion updates SEQUENCE_GOAL.md if all tasks are done.
func (m *Manager) propagateSequenceCompletion(ctx context.Context, seqPath string) error {
	seqProgress, err := m.GetSequenceProgress(ctx, seqPath)
	if err != nil {
		return errors.Wrap(err, "getting sequence progress")
	}

	if seqProgress.Progress.Total == 0 || seqProgress.Progress.Completed < seqProgress.Progress.Total {
		return nil
	}

	goalPath := filepath.Join(seqPath, "SEQUENCE_GOAL.md")
	return m.updateGoalStatus(ctx, goalPath, frontmatter.StatusCompleted)
}

// propagatePhaseCompletion updates PHASE_GOAL.md if all sequences/workflows are done.
func (m *Manager) propagatePhaseCompletion(ctx context.Context, phasePath string) error {
	phaseProgress, err := m.GetPhaseProgress(ctx, phasePath)
	if err != nil {
		return errors.Wrap(err, "getting phase progress")
	}

	if phaseProgress.Progress.Total == 0 || phaseProgress.Progress.Completed < phaseProgress.Progress.Total {
		return nil
	}

	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")
	return m.updateGoalStatus(ctx, goalPath, frontmatter.StatusCompleted)
}

// updateGoalStatus updates the fest_status field in a goal file's YAML frontmatter.
// Returns nil if the file doesn't exist (goal files are optional).
func (m *Manager) updateGoalStatus(ctx context.Context, goalPath string, status frontmatter.Status) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	content, err := os.ReadFile(goalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Goal file is optional
		}
		return errors.IO("reading goal file", err)
	}

	fm, remaining, err := frontmatter.Parse(content)
	if err != nil {
		return errors.Wrap(err, "parsing goal frontmatter")
	}
	if fm == nil {
		return nil // No frontmatter to update
	}

	if fm.Status == status {
		return nil // Already in sync
	}

	fm.Status = status
	fm.Updated = time.Now()

	newContent, err := frontmatter.Inject(remaining, fm)
	if err != nil {
		return errors.Wrap(err, "injecting updated frontmatter")
	}

	if err := os.WriteFile(goalPath, newContent, 0o644); err != nil {
		return errors.IO("writing updated goal file", err)
	}
	return nil
}
