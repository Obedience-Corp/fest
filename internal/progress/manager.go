package progress

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	"github.com/Obedience-Corp/fest/internal/campledger"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/hooks"
)

// Manager handles progress operations for a festival
type Manager struct {
	store        *Store
	gate         Gate
	festivalPath string
}

// NewManager creates a new progress manager with no lifecycle gate.
// Mutations are not gated. Suitable for read-heavy callers and tests.
// CLI mutation entry points should use NewManagerWithGate instead.
func NewManager(ctx context.Context, festivalPath string) (*Manager, error) {
	return NewManagerWithGate(ctx, festivalPath, NoopGate{})
}

// NewManagerWithGate creates a new progress manager whose mutation
// methods consult the supplied Gate. Pass a real gate at CLI mutation
// sites; pass NoopGate{} for housekeeping paths that legitimately
// bypass enforcement (e.g., promote internals, store fixups).
func NewManagerWithGate(ctx context.Context, festivalPath string, gate Gate) (*Manager, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	if gate == nil {
		gate = NoopGate{}
	}

	store := NewStore(festivalPath)
	if err := store.Load(ctx); err != nil {
		return nil, errors.Wrap(err, "loading progress data")
	}
	return &Manager{store: store, gate: gate, festivalPath: festivalPath}, nil
}

// NewManagerReadOnly creates a progress manager that never mutates disk while
// loading: legacy progress and workflow YAML are materialized in memory rather
// than migrated. Use for orientation-only callers such as walk/inspect.
func NewManagerReadOnly(ctx context.Context, festivalPath string) (*Manager, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	store := NewStore(festivalPath)
	if err := store.LoadReadOnly(ctx); err != nil {
		return nil, errors.Wrap(err, "loading progress data")
	}
	return &Manager{store: store, gate: NoopGate{}, festivalPath: festivalPath}, nil
}

// UpdateProgress updates the progress percentage for a task
func (m *Manager) UpdateProgress(ctx context.Context, taskID string, progress int) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if err := m.gate.EnforceForTask(ctx, taskID); err != nil {
		return err
	}

	if progress < 0 || progress > 100 {
		return errors.Validation("progress must be between 0 and 100").
			WithField("progress", progress)
	}

	return m.store.withExclusiveLock(ctx, func() error {
		task, exists := m.store.GetTask(taskID)
		if !exists {
			task = &TaskProgress{
				TaskID: taskID,
				Status: StatusPending,
			}
		}

		// A 100% update is a completion transition, so the task_complete verb
		// runs here too: without it UpdateProgress(100) would complete a task
		// with its completion bindings silently skipped. Hooks fire only on an
		// actual transition into completed; replaying 100 on an already-completed
		// task never re-fires them.
		var completeStage *taskHookStage
		if progress == 100 && task.Status != StatusCompleted {
			var err error
			completeStage, err = m.beginTaskHooks(ctx, taskID, hooks.VerbTaskComplete)
			if err != nil {
				return err
			}
		}

		// The first progress above zero is the first start signal, so the
		// task_start stage runs here too: without it a progress update would
		// permanently disarm start anchors (StartedAt set, hooks never fired).
		var startStage *taskHookStage
		if task.StartedAt == nil && progress > 0 {
			var err error
			startStage, err = m.beginTaskHooks(ctx, taskID, hooks.VerbTaskStart)
			if err != nil {
				return err
			}
		}

		now := time.Now().UTC()

		// Start tracking time on the first progress update above zero.
		// Use current time - we only track actual work time, not time since
		// file creation. A zero update records no work start.
		if progress > 0 && task.StartedAt == nil {
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
		// The transition is applied; every remaining side effect must still be
		// attempted when a post stage fails. A start post failure must not skip
		// the task_complete post stage, and neither may skip parent propagation:
		// otherwise a completed task leaves its sequence goal permanently stalled.
		// The first post error is returned after all side effects have run.
		postErr := m.finishTaskHooks(ctx, startStage, "started")
		if err := m.finishTaskHooks(ctx, completeStage, "completed"); postErr == nil {
			postErr = err
		}
		if progress == 100 {
			// Propagation is best-effort: a failure here should not block
			// the progress update that already succeeded above.
			_ = m.PropagateCompletion(ctx, taskID)
		}
		return postErr
	})
}

// MarkComplete marks a task as complete
func (m *Manager) MarkComplete(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if err := m.gate.EnforceForTask(ctx, taskID); err != nil {
		return err
	}

	return m.store.withExclusiveLock(ctx, func() error {
		task, exists := m.store.GetTask(taskID)
		if !exists {
			task = &TaskProgress{
				TaskID: taskID,
			}
		}

		// Completion bindings run around the transition here so every completion
		// surface shares them (task completed command, status set, progress 100).
		// Pre ordering preserves the historical 'fest task completed' flow:
		// task_complete pre, then first-start task_start pre, mutation,
		// task_start post, task_complete post. An explicit re-completion of an
		// already-completed task re-fires its bindings, matching the historical
		// command behavior.
		completeStage, err := m.beginTaskHooks(ctx, taskID, hooks.VerbTaskComplete)
		if err != nil {
			return err
		}

		var startStage *taskHookStage
		if task.StartedAt == nil {
			startStage, err = m.beginTaskHooks(ctx, taskID, hooks.VerbTaskStart)
			if err != nil {
				return err
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
		// The completion is applied; every remaining side effect must still be
		// attempted when a post stage fails. A start post failure must not skip
		// the task_complete post stage, and neither may skip the ledger emit or
		// parent propagation: otherwise a completed task leaves its sequence goal
		// permanently stalled. The first post error is returned after all side
		// effects have run, preserving the historical contract that ledger and
		// propagation always follow a successful completion mutation.
		postErr := m.finishTaskHooks(ctx, startStage, "started")
		if err := m.finishTaskHooks(ctx, completeStage, "completed"); postErr == nil {
			postErr = err
		}
		// Campaign ledger: high-intent task completion (D003/D006). Does not
		// alter progress_events.jsonl content beyond the save already done.
		m.emitLedger(ctx, ledgerkit.KindCompleted, taskID, "", map[string]any{
			"target": "task",
			"to":     "completed",
		})
		// Propagation is best-effort: a failure here should not block
		// the task completion that already succeeded above.
		_ = m.PropagateCompletion(ctx, taskID)
		return postErr
	})
}

// MarkInProgress marks a task as in progress. On the first start (no
// StartedAt yet) the task document's start-stage hook bindings run around
// the transition (task_start verb): a blocked pre stage leaves the task
// unstarted; the post stage runs after the status is applied. Re-entering
// in_progress later never re-fires start hooks.
func (m *Manager) MarkInProgress(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if err := m.gate.EnforceForTask(ctx, taskID); err != nil {
		return err
	}

	return m.store.withExclusiveLock(ctx, func() error {
		task, exists := m.store.GetTask(taskID)
		if !exists {
			task = &TaskProgress{
				TaskID: taskID,
			}
		}

		var stage *taskHookStage
		if task.StartedAt == nil {
			var err error
			stage, err = m.beginTaskHooks(ctx, taskID, hooks.VerbTaskStart)
			if err != nil {
				return err
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
		return m.finishTaskHooks(ctx, stage, "started")
	})
}

// taskHookStage carries one task transition's planned lifecycle bindings
// between the pre and post timings.
type taskHookStage struct {
	req     LifecycleHookRequest
	planned []hooks.PlannedHook
	runner  *hooks.Runner
	label   string // human wording for error text, e.g. "task start"
}

// beginTaskHooks plans the task document's bindings for verb and runs the
// pre timing. task_start plans the nested start-stage bindings; task_complete
// plans the bare pre/post bindings. A nil stage with nil error means nothing
// is bound. On a blocked pre the caller must not apply the transition. A
// missing task file yields no bindings: progress can track tasks whose
// documents do not exist.
func (m *Manager) beginTaskHooks(ctx context.Context, taskID string, verb hooks.Verb) (*taskHookStage, error) {
	req := LifecycleHookRequest{
		FestivalPath: m.store.FestivalPath(),
		Phase:        taskPhaseCoordinate(taskID),
		Task:         taskID,
		Level:        hooks.LevelTask,
		Verb:         verb,
	}
	label := "task start"
	if verb == hooks.VerbTaskComplete {
		label = "task completion"
	}
	taskPath := filepath.Join(m.store.FestivalPath(), taskID)
	if !strings.HasSuffix(taskPath, ".md") {
		taskPath += ".md"
	}
	content, err := os.ReadFile(taskPath)
	if err != nil {
		return nil, nil
	}
	if verb == hooks.VerbTaskStart {
		req.Pre, req.Post, req.HumanGate = StartHookBindingsFromFrontmatter(content)
	} else {
		req.Pre, req.Post, req.HumanGate = StepHookBindingsFromFrontmatter(content)
	}
	planned, err := PlanLifecycleHooks(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "planning "+label+" hooks")
	}
	if len(planned) == 0 {
		return nil, nil
	}
	stage := &taskHookStage{req: req, planned: planned, runner: newLifecycleHookRunner(m.store.FestivalPath()), label: label}
	preRuns, blocked, err := RunLifecycleStage(ctx, m.store, stage.runner, stage.req, stage.planned, hooks.TimingPre)
	if saveErr := m.store.SaveEvents(ctx); saveErr != nil && err == nil {
		err = saveErr
	}
	if err != nil {
		return nil, errors.Wrap(err, "running "+label+" pre hooks")
	}
	if blocked {
		return nil, BlockedHookError(verb, preRuns)
	}
	return stage, nil
}

// finishTaskHooks runs the post timing after the transition has been applied.
// A fail-closed post failure keeps the transition applied (appliedState names
// it in the warning); it is recorded in the audit trail and warned to stderr.
func (m *Manager) finishTaskHooks(ctx context.Context, stage *taskHookStage, appliedState string) error {
	if stage == nil {
		return nil
	}
	_, postBlocked, postErr := RunLifecycleStage(ctx, m.store, stage.runner, stage.req, stage.planned, hooks.TimingPost)
	if saveErr := m.store.SaveEvents(ctx); saveErr != nil && postErr == nil {
		postErr = saveErr
	}
	if postErr != nil {
		return errors.Wrap(postErr, "running "+stage.label+" post hooks")
	}
	if postBlocked {
		fmt.Fprintf(os.Stderr, "Warning: a fail-closed %s post hook failed; the task remains %s (see wf_hook_run in the festival audit trail)\n", string(stage.req.Verb), appliedState)
	}
	return nil
}

// taskPhaseCoordinate derives the audit-event phase coordinate from a
// festival-relative task ID (its first path segment; empty for root-level ids).
func taskPhaseCoordinate(taskID string) string {
	parts := strings.Split(filepath.ToSlash(taskID), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[0]
}

// ReportBlocker reports a blocker for a task
func (m *Manager) ReportBlocker(ctx context.Context, taskID, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if err := m.gate.EnforceForTask(ctx, taskID); err != nil {
		return err
	}

	if message == "" {
		return errors.Validation("blocker message required")
	}

	return m.store.withExclusiveLock(ctx, func() error {
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
		m.emitLedger(ctx, ledgerkit.KindTransitioned, taskID, message, map[string]any{
			"from":   string(StatusPending),
			"to":     "blocked",
			"target": "task",
			"action": "block",
		})
		return nil
	})
}

// ResetTask resets a task back to pending status, clearing all progress data.
func (m *Manager) ResetTask(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if err := m.gate.EnforceForTask(ctx, taskID); err != nil {
		return err
	}

	return m.store.withExclusiveLock(ctx, func() error {
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
		m.emitLedger(ctx, ledgerkit.KindTransitioned, taskID, "", map[string]any{
			"to":     "pending",
			"target": "task",
			"action": "reset",
		})
		return nil
	})
}

// emitLedger best-effort appends a campaign ledger event for a high-intent
// task mutation. No-op outside a campaign. Never fails the caller (D003).
func (m *Manager) emitLedger(ctx context.Context, kind ledgerkit.Kind, taskID, why string, payload map[string]any) {
	if m == nil || m.festivalPath == "" {
		return
	}
	e := campledger.NewFromFestival(ctx, m.festivalPath, campledger.WarnToStderr())
	scope := campledger.FestivalScope(m.festivalPath, taskID)
	opts := []campledger.Option{}
	if why != "" {
		opts = append(opts, campledger.WithWhy(why))
	}
	if payload != nil {
		opts = append(opts, campledger.WithPayload(payload))
	}
	e.Emit(ctx, kind, scope, opts...)
}

// ClearBlocker clears a blocker for a task
func (m *Manager) ClearBlocker(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if err := m.gate.EnforceForTask(ctx, taskID); err != nil {
		return err
	}

	return m.store.withExclusiveLock(ctx, func() error {
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
	})
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
	return m.completeGoalWithHooks(ctx, goalPath, hooks.LevelSequence, hooks.VerbSequenceComplete,
		filepath.Base(filepath.Dir(seqPath)), filepath.Base(seqPath))
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
	return m.completeGoalWithHooks(ctx, goalPath, hooks.LevelPhase, hooks.VerbPhaseComplete,
		filepath.Base(phasePath), "")
}

// completeGoalWithHooks marks a goal doc completed, running its frontmatter
// hook bindings around the transition (spec 03: sequence/phase complete
// verbs). A blocked pre list leaves the goal incomplete because the verb
// never applies; a blocked post list leaves the transition applied. Hooks run
// only on the pending -> completed transition, never for goals already
// completed.
func (m *Manager) completeGoalWithHooks(ctx context.Context, goalPath string, level hooks.Level, verb hooks.Verb, phaseName, taskCoord string) error {
	content, err := os.ReadFile(goalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Goal files are optional
		}
		return errors.IO("reading goal file", err)
	}
	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		return m.updateGoalStatus(ctx, goalPath, frontmatter.StatusCompleted)
	}
	if fm.Status == frontmatter.StatusCompleted {
		return nil // Already applied; never re-run completion hooks
	}

	req := LifecycleHookRequest{
		FestivalPath: m.store.FestivalPath(),
		Phase:        phaseName,
		Task:         taskCoord,
		Level:        level,
		Verb:         verb,
	}
	req.Pre, req.Post, req.HumanGate = StepHookBindingsFromFrontmatter(content)
	planned, err := PlanLifecycleHooks(ctx, req)
	if err != nil {
		return errors.Wrap(err, "planning goal hooks")
	}
	runner := newLifecycleHookRunner(m.store.FestivalPath())
	preRuns, blocked, err := RunLifecycleStage(ctx, m.store, runner, req, planned, hooks.TimingPre)
	if saveErr := m.store.SaveEvents(ctx); saveErr != nil && err == nil {
		err = saveErr
	}
	if err != nil {
		return errors.Wrap(err, "running goal pre hooks")
	}
	if blocked {
		return BlockedHookError(verb, preRuns)
	}

	if err := m.updateGoalStatus(ctx, goalPath, frontmatter.StatusCompleted); err != nil {
		return err
	}

	_, _, postErr := RunLifecycleStage(ctx, m.store, runner, req, planned, hooks.TimingPost)
	if saveErr := m.store.SaveEvents(ctx); saveErr != nil && postErr == nil {
		postErr = saveErr
	}
	if postErr != nil {
		return errors.Wrap(postErr, "running goal post hooks")
	}
	return nil
}

// ResetPhaseStatus sets the phase PHASE_GOAL.md frontmatter status back to in_progress.
// Use this when resetting a workflow or gate to reflect that the phase is no longer complete.
func (m *Manager) ResetPhaseStatus(ctx context.Context, phasePath string) error {
	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")
	return m.updateGoalStatus(ctx, goalPath, frontmatter.StatusInProgress)
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
