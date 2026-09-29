package progress

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

// DeferralAudit carries the operator guard's findings from the command layer
// into the deferral event. It lives here rather than in internal/commands/task
// so the manager can record it without importing the command package.
type DeferralAudit struct {
	Actor        string
	TTY          bool
	AgentMarkers []string
	Ancestry     []string
	DeferredBy   string
}

// MaxCompletedSince caps the "what completed since the deferral" list handed to
// a sweep. A long festival otherwise prints a wall of filenames the executor
// will not read.
const MaxCompletedSince = 10

// completedSince returns the tasks completed strictly after since, in event
// order, capped at MaxCompletedSince entries followed by a count of the rest.
func completedSince(events []ProgressEvent, since time.Time) []string {
	var completed []string
	for _, event := range events {
		if event.Event != EventCompleted || event.Task == "" {
			continue
		}
		if event.Timestamp.After(since) {
			completed = append(completed, event.Task)
		}
	}

	if len(completed) <= MaxCompletedSince {
		return completed
	}
	rest := len(completed) - MaxCompletedSince
	return append(completed[:MaxCompletedSince:MaxCompletedSince],
		"and "+strconv.Itoa(rest)+" more")
}

// CompletedSince lists the tasks completed after a deferral, for the sweep to
// show the executor what might have changed.
func (m *Manager) CompletedSince(ctx context.Context, since time.Time) ([]string, error) {
	events, err := m.store.parseEventsFile(ctx)
	if err != nil {
		return nil, err
	}
	return completedSince(events, since), nil
}

// StartSweep records the start of an end-of-festival sweep. Sweep state is
// derived from these events and never stored (D005).
func (m *Manager) StartSweep(ctx context.Context, sweep int) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if sweep < 1 {
		return errors.Validation("sweep number must be positive")
	}

	return m.store.withExclusiveLock(ctx, func() error {
		if m.store.SweepState().Current >= sweep {
			return nil
		}
		m.store.QueueEvent(&ProgressEvent{
			Timestamp: time.Now().UTC(),
			Event:     EventSweepStarted,
			Sweep:     sweep,
		})
		return m.store.Save(ctx)
	})
}

// RecordBlockerRevisit stamps a deferred task as handed back to the executor in
// this sweep, so the sweep moves on rather than offering it again.
func (m *Manager) RecordBlockerRevisit(ctx context.Context, taskID string, sweep int) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if taskID == "" {
		return errors.Validation("task ID required")
	}
	if sweep < 1 {
		return errors.Validation("sweep number must be positive")
	}

	return m.store.withExclusiveLock(ctx, func() error {
		if m.store.SweepState().LastRevisit[taskID] >= sweep {
			return nil
		}
		m.store.QueueEvent(&ProgressEvent{
			Timestamp: time.Now().UTC(),
			Event:     EventBlockerRevisited,
			Task:      taskID,
			Sweep:     sweep,
		})
		return m.store.Save(ctx)
	})
}

// completionsInSweep counts the task completions recorded inside a sweep. A new
// sweep is only worth starting when the previous one produced at least one, and
// that is what bounds the number of sweeps (D005).
func completionsInSweep(events []ProgressEvent, sweep int) int {
	current := 0
	count := 0
	for _, event := range events {
		switch event.Event {
		case EventSweepStarted:
			current = event.Sweep
		case EventCompleted:
			if current == sweep {
				count++
			}
		}
	}
	return count
}

// CompletionsInSweep reports how many tasks completed during a sweep.
func (m *Manager) CompletionsInSweep(ctx context.Context, sweep int) (int, error) {
	events, err := m.store.parseEventsFile(ctx)
	if err != nil {
		return 0, err
	}
	return completionsInSweep(events, sweep), nil
}

// forcedCompletionInSweep reports whether an operator forced the festival
// complete during a sweep.
func forcedCompletionInSweep(events []ProgressEvent, sweep int) bool {
	current := 0
	for _, event := range events {
		switch event.Event {
		case EventSweepStarted:
			current = event.Sweep
		case EventForcedComplete:
			if current == sweep {
				return true
			}
		}
	}
	return false
}

// SweepMayAdvance reports whether a new sweep is justified after the given one.
// A completion is the ordinary reason: something changed, so a deferred blocker
// may now be resolvable. A forced completion is the other one, because a
// festival that was forced and later reopened must hand its deferred work back
// (design doc 05 H6). Both are finite and neither is reachable by an executor
// without either finishing work or passing the operator guard, so the sweep
// stays bounded.
func (m *Manager) SweepMayAdvance(ctx context.Context, sweep int) (bool, error) {
	events, err := m.store.parseEventsFile(ctx)
	if err != nil {
		return false, err
	}
	return completionsInSweep(events, sweep) > 0 || forcedCompletionInSweep(events, sweep), nil
}

// reopenSweepGates returns the phase's quality gate tasks that were completed
// while taskID's blocker was deferred to pending, so a gate whose inputs have
// just changed is evaluated again before the sweep continues. It runs inside
// the store's exclusive lock and therefore never takes it.
func (m *Manager) reopenSweepGates(now time.Time, taskID string, deferredAt time.Time) []string {
	phase := taskPhase(taskID)
	if phase == "" {
		return nil
	}

	var reopened []string
	for key, candidate := range m.store.AllTasks() {
		if key == taskID || candidate == nil || candidate.Status != StatusCompleted {
			continue
		}
		if candidate.CompletedAt == nil || !candidate.CompletedAt.After(deferredAt) {
			continue
		}
		if taskPhase(key) != phase || !m.isGateTask(key) {
			continue
		}

		candidate.Status = StatusPending
		candidate.Progress = 0
		candidate.StartedAt = nil
		candidate.CompletedAt = nil
		candidate.TimeSpentMinutes = 0

		m.store.QueueEvent(&ProgressEvent{
			Timestamp: now,
			Event:     EventReset,
			Task:      key,
		})
		m.store.SetTask(candidate)
		reopened = append(reopened, key)
	}

	sort.Strings(reopened)
	return reopened
}

func taskPhase(taskID string) string {
	parts := strings.Split(filepath.ToSlash(taskID), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[0]
}

func (m *Manager) isGateTask(taskID string) bool {
	taskPath := filepath.Join(m.store.FestivalPath(), filepath.FromSlash(taskID))
	if !strings.HasSuffix(taskPath, ".md") {
		taskPath += ".md"
	}

	content, err := os.ReadFile(taskPath)
	if err != nil {
		return false
	}
	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		return false
	}
	return fm.Type == frontmatter.TypeGate || fm.Type == frontmatter.TypePhaseGate
}

// DeferredTasks returns every task whose blocker an operator deferred, in task
// ID order. It is the one lookup every caller uses, so the promotion refusal
// and the sweep can never disagree about what is deferred.
func DeferredTasks(store *Store) []*TaskProgress {
	if store == nil {
		return nil
	}

	var deferred []*TaskProgress
	for _, task := range store.AllTasks() {
		if task != nil && task.Status == StatusBlocked && task.BlockerDeferred {
			deferred = append(deferred, task)
		}
	}
	sort.Slice(deferred, func(i, j int) bool {
		return deferred[i].TaskID < deferred[j].TaskID
	})
	return deferred
}

// DeferredTasks returns the festival's deferred blockers.
func (m *Manager) DeferredTasks() []*TaskProgress {
	return DeferredTasks(m.store)
}

// RecordForcedCompletion appends the record of an operator forcing a festival
// complete over open deferred blockers. The deferral fields are deliberately
// left alone: a reopened festival must still sweep (design doc 05 D6 and H6).
func (m *Manager) RecordForcedCompletion(ctx context.Context, audit DeferralAudit, dropped []string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if len(dropped) == 0 {
		return errors.Validation("a forced completion must name the dropped tasks")
	}

	return m.store.withExclusiveLock(ctx, func() error {
		m.store.QueueEvent(&ProgressEvent{
			Timestamp:    time.Now().UTC(),
			Event:        EventForcedComplete,
			Actor:        audit.Actor,
			TTY:          audit.TTY,
			AgentMarkers: audit.AgentMarkers,
			Ancestry:     audit.Ancestry,
			DeferredBy:   audit.DeferredBy,
			DroppedTasks: dropped,
		})
		return m.store.Save(ctx)
	})
}
