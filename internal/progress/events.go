// Package progress provides progress tracking for festival execution.
package progress

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// Event type constants for JSONL progress events.
const (
	EventStarted   = "started"
	EventCompleted = "completed"
	EventProgress  = "progress"
	EventBlocked   = "blocked"
	EventUnblocked = "unblocked"
)

// ProgressEvent represents a single progress event in JSONL format.
// Events are append-only and the current state is materialized by
// replaying events in timestamp order.
type ProgressEvent struct {
	Timestamp time.Time `json:"ts"`
	Event     string    `json:"event"`
	Task      string    `json:"task"`

	// Event-specific fields (omitempty)
	Minutes int    `json:"minutes,omitempty"` // completed event
	Percent int    `json:"percent,omitempty"` // progress event
	Reason  string `json:"reason,omitempty"`  // blocked event
}

// appendEvent appends a single event to the JSONL file.
// Uses atomic append to prevent partial writes.
func (s *Store) appendEvent(ctx context.Context, event *ProgressEvent) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	eventsPath := s.eventsFilePath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(eventsPath), 0755); err != nil {
		return errors.IO("creating progress directory", err).
			WithField("path", filepath.Dir(eventsPath))
	}

	// Open for append (create if not exists)
	f, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.IO("opening events file for append", err).
			WithField("path", eventsPath)
	}
	defer f.Close()

	// Marshal and write with newline
	data, err := json.Marshal(event)
	if err != nil {
		return errors.Wrap(err, "marshaling progress event")
	}
	data = append(data, '\n')

	if _, err := f.Write(data); err != nil {
		return errors.IO("appending progress event", err).
			WithField("path", eventsPath)
	}

	return nil
}

// loadFromEvents reads the JSONL file and materializes current state.
func (s *Store) loadFromEvents(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	eventsPath := s.eventsFilePath()

	f, err := os.Open(eventsPath)
	if err != nil {
		return errors.IO("opening events file", err).
			WithField("path", eventsPath)
	}
	defer f.Close()

	var events []ProgressEvent
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		var event ProgressEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Log warning but continue - don't fail on single bad line
			// This makes the format resilient to partial writes
			continue
		}
		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return errors.IO("reading events file", err).
			WithField("path", eventsPath)
	}

	// Materialize state from events
	s.data = &FestivalProgressData{
		Festival:  filepath.Base(s.festivalPath),
		UpdatedAt: time.Now().UTC(),
		Tasks:     materializeState(events),
	}

	// Initialize TimeMetrics from events
	s.data.TimeMetrics = materializeTimeMetrics(events, s.data.Tasks)

	return nil
}

// materializeState builds current task state from a sequence of events.
// Events are processed in order to derive the final state of each task.
func materializeState(events []ProgressEvent) map[string]*TaskProgress {
	tasks := make(map[string]*TaskProgress)

	for _, e := range events {
		task, ok := tasks[e.Task]
		if !ok {
			task = &TaskProgress{
				TaskID: e.Task,
				Status: StatusPending,
			}
			tasks[e.Task] = task
		}

		switch e.Event {
		case EventStarted:
			task.Status = StatusInProgress
			ts := e.Timestamp
			task.StartedAt = &ts

		case EventCompleted:
			task.Status = StatusCompleted
			ts := e.Timestamp
			task.CompletedAt = &ts
			task.Progress = 100
			task.TimeSpentMinutes = e.Minutes
			// Clear any blocker
			task.BlockerMessage = ""
			task.BlockedAt = nil

		case EventProgress:
			task.Progress = e.Percent
			if e.Percent > 0 && task.Status == StatusPending {
				task.Status = StatusInProgress
			}

		case EventBlocked:
			task.Status = StatusBlocked
			task.BlockerMessage = e.Reason
			ts := e.Timestamp
			task.BlockedAt = &ts

		case EventUnblocked:
			if task.Status == StatusBlocked {
				task.Status = StatusInProgress
			}
			task.BlockerMessage = ""
			task.BlockedAt = nil
		}
	}

	return tasks
}

// materializeTimeMetrics builds festival time metrics from events.
func materializeTimeMetrics(events []ProgressEvent, tasks map[string]*TaskProgress) *FestivalTimeMetrics {
	if len(events) == 0 {
		return &FestivalTimeMetrics{
			CreatedAt: time.Now().UTC(),
		}
	}

	// Find earliest event timestamp as creation time
	var earliest time.Time
	var latest time.Time
	for _, e := range events {
		if earliest.IsZero() || e.Timestamp.Before(earliest) {
			earliest = e.Timestamp
		}
		if latest.IsZero() || e.Timestamp.After(latest) {
			latest = e.Timestamp
		}
	}

	metrics := &FestivalTimeMetrics{
		CreatedAt: earliest,
	}

	// Calculate total work minutes from tasks
	for _, task := range tasks {
		metrics.TotalWorkMinutes += task.TimeSpentMinutes
	}

	// Check if all tasks are completed to set completion time
	allComplete := len(tasks) > 0
	for _, task := range tasks {
		if task.Status != StatusCompleted {
			allComplete = false
			break
		}
	}

	if allComplete {
		metrics.CompletedAt = &latest
		metrics.LifecycleDuration = int(latest.Sub(earliest).Hours() / 24)
	}

	return metrics
}

// generateEventsFromState converts current YAML state to synthetic events.
// This is used during migration from legacy YAML format to JSONL.
func generateEventsFromState(tasks map[string]*TaskProgress) []ProgressEvent {
	var events []ProgressEvent

	for _, task := range tasks {
		// For completed tasks, generate started + completed events
		if task.Status == StatusCompleted {
			if task.StartedAt != nil {
				events = append(events, ProgressEvent{
					Timestamp: *task.StartedAt,
					Event:     EventStarted,
					Task:      task.TaskID,
				})
			}
			if task.CompletedAt != nil {
				events = append(events, ProgressEvent{
					Timestamp: *task.CompletedAt,
					Event:     EventCompleted,
					Task:      task.TaskID,
					Minutes:   task.TimeSpentMinutes,
				})
			}
		}

		// For in-progress tasks
		if task.Status == StatusInProgress && task.StartedAt != nil {
			events = append(events, ProgressEvent{
				Timestamp: *task.StartedAt,
				Event:     EventStarted,
				Task:      task.TaskID,
			})
			// Include progress if set
			if task.Progress > 0 && task.Progress < 100 {
				events = append(events, ProgressEvent{
					Timestamp: task.StartedAt.Add(time.Second), // Slightly after start
					Event:     EventProgress,
					Task:      task.TaskID,
					Percent:   task.Progress,
				})
			}
		}

		// For blocked tasks
		if task.Status == StatusBlocked {
			if task.StartedAt != nil {
				events = append(events, ProgressEvent{
					Timestamp: *task.StartedAt,
					Event:     EventStarted,
					Task:      task.TaskID,
				})
			}
			if task.BlockedAt != nil {
				events = append(events, ProgressEvent{
					Timestamp: *task.BlockedAt,
					Event:     EventBlocked,
					Task:      task.TaskID,
					Reason:    task.BlockerMessage,
				})
			}
		}

		// For pending tasks with start time (rare but possible)
		if task.Status == StatusPending && task.StartedAt != nil {
			events = append(events, ProgressEvent{
				Timestamp: *task.StartedAt,
				Event:     EventStarted,
				Task:      task.TaskID,
			})
		}
	}

	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events
}

// writeEvents writes a batch of events to the JSONL file.
// Used during migration from YAML to write all synthetic events at once.
func (s *Store) writeEvents(ctx context.Context, events []ProgressEvent) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	eventsPath := s.eventsFilePath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(eventsPath), 0755); err != nil {
		return errors.IO("creating progress directory", err).
			WithField("path", filepath.Dir(eventsPath))
	}

	// Create/truncate file
	f, err := os.Create(eventsPath)
	if err != nil {
		return errors.IO("creating events file", err).
			WithField("path", eventsPath)
	}
	defer f.Close()

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return errors.Wrap(err, "marshaling progress event")
		}
		data = append(data, '\n')

		if _, err := f.Write(data); err != nil {
			return errors.IO("writing progress event", err).
				WithField("path", eventsPath)
		}
	}

	return nil
}
