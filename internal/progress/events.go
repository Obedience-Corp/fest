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
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// EventType represents the type of progress event.
type EventType string

// Event type constants for JSONL progress events.
const (
	EventStarted   EventType = "started"
	EventCompleted EventType = "completed"
	EventProgress  EventType = "progress"
	EventBlocked   EventType = "blocked"
	EventUnblocked EventType = "unblocked"
	EventReset     EventType = "reset"

	// Workflow event types for tracking workflow step progress.
	EventWorkflowInit      EventType = "wf_init"
	EventWorkflowStepStart EventType = "wf_step_start"
	EventWorkflowStepDone  EventType = "wf_step_done"
	EventWorkflowStepSkip  EventType = "wf_step_skip"
	EventWorkflowStepBlock EventType = "wf_step_block"
	EventWorkflowAdvance   EventType = "wf_advance"
	EventWorkflowReset     EventType = "wf_reset"

	// EventWorkflowStepFailRemediation records that a gate step did not pass
	// and is linked to a remediation phase. The step remains non-terminal and
	// must be re-evaluated after the remediation phase completes.
	EventWorkflowStepFailRemediation EventType = "wf_step_fail_remediation"

	// EventWorkflowStepRecheck records that the operator is re-entering a
	// previously failed remediation step for re-evaluation.
	EventWorkflowStepRecheck EventType = "wf_step_recheck"

	// EventWorkflowJudgeRecheck records that a judge-owned rejection is being
	// reopened for a new approval-judge evaluation.
	EventWorkflowJudgeRecheck EventType = "wf_judge_recheck"

	// EventWorkflowJudgeStarted records that a delegated approval judge run
	// was invoked on a blocking checkpoint via 'fest workflow approve --auto'.
	EventWorkflowJudgeStarted EventType = "wf_judge_started"

	// EventWorkflowJudgeClaimed binds a started judge run to the detached PID
	// that is allowed to evaluate it.
	EventWorkflowJudgeClaimed EventType = "wf_judge_claimed"

	// EventWorkflowJudgeReturned records how a delegated judge run ended:
	// approved, rejected, or failed (timeout, missing command, bad verdict).
	EventWorkflowJudgeReturned EventType = "wf_judge_returned"

	// EventWorkflowJudgeCleared removes a prior terminal judge outcome after
	// an operator takes ownership of the checkpoint.
	EventWorkflowJudgeCleared EventType = "wf_judge_cleared"
)

// ProgressEvent represents a single progress event in JSONL format.
// Events are append-only and the current state is materialized by
// replaying events in timestamp order.
type ProgressEvent struct {
	Timestamp time.Time `json:"ts"`
	Event     EventType `json:"event"`
	Task      string    `json:"task,omitempty"`

	// Task event-specific fields (omitempty)
	Minutes int    `json:"minutes,omitempty"` // completed event
	Percent int    `json:"percent,omitempty"` // progress event
	Reason  string `json:"reason,omitempty"`  // blocked event

	// Workflow event-specific fields (omitempty)
	Phase            string   `json:"phase,omitempty"`
	Step             int      `json:"step,omitempty"`
	TotalSteps       int      `json:"total_steps,omitempty"`
	Feedback         string   `json:"feedback,omitempty"`
	Followups        []string `json:"followups,omitempty"`
	RemediationPhase string   `json:"remediation_phase,omitempty"`
	DecisionActor    string   `json:"decision_actor,omitempty"`
	DecisionSummary  string   `json:"decision_summary,omitempty"`
	JudgeStatus      string   `json:"judge_status,omitempty"`
	JudgeCommand     string   `json:"judge_command,omitempty"`
	JudgeDetail      string   `json:"judge_detail,omitempty"`
	JudgePid         int      `json:"judge_pid,omitempty"`
	JudgeRunID       string   `json:"judge_run_id,omitempty"`
}

// loadFromEvents reads the JSONL file and materializes current state.
func (s *Store) loadFromEvents(ctx context.Context) error {
	events, err := s.parseEventsFile(ctx)
	if err != nil {
		return err
	}
	s.materializeFrom(events)
	return nil
}

func (s *Store) parseEventsFile(ctx context.Context) ([]ProgressEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	eventsPath := s.eventsFilePath()

	f, err := os.Open(eventsPath)
	if err != nil {
		return nil, errors.IO("opening events file", err).
			WithField("path", eventsPath)
	}
	defer func() { _ = f.Close() }()

	var events []ProgressEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event ProgressEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.IO("reading events file", err).
			WithField("path", eventsPath)
	}

	return events, nil
}

func (s *Store) materializeFrom(events []ProgressEvent) {
	s.data = &FestivalProgressData{
		Festival:  filepath.Base(s.festivalPath),
		UpdatedAt: time.Now().UTC(),
		Tasks:     materializeState(events),
	}
	s.data.TimeMetrics = materializeTimeMetrics(events, s.data.Tasks)
	s.workflowData = materializeWorkflowState(events)
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

		case EventReset:
			task.Status = StatusPending
			task.Progress = 0
			task.StartedAt = nil
			task.CompletedAt = nil
			task.TimeSpentMinutes = 0
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

// materializeWorkflowState builds a FestivalWorkflowState from workflow events.
// This replays all wf_* events in order to reconstruct per-phase WorkflowState.
func materializeWorkflowState(events []ProgressEvent) *wf.FestivalWorkflowState {
	state := wf.NewFestivalWorkflowState()

	for _, e := range events {
		if e.Phase == "" {
			continue // Not a workflow event
		}

		phaseState, ok := state.Phases[e.Phase]
		if !ok {
			phaseState = wf.NewWorkflowState(0)
			state.Phases[e.Phase] = phaseState
		}

		switch e.Event {
		case EventWorkflowInit:
			phaseState.TotalSteps = e.TotalSteps
			if phaseState.Steps == nil {
				phaseState.Steps = make(map[int]*wf.StepState)
			}
			for i := 1; i <= e.TotalSteps; i++ {
				if _, exists := phaseState.Steps[i]; !exists {
					phaseState.Steps[i] = &wf.StepState{
						Number: i,
						Status: wf.StepStatusPending,
					}
				}
			}
			if phaseState.CurrentStep == 0 {
				phaseState.CurrentStep = 1
			}

		case EventWorkflowStepStart:
			ss := phaseState.GetOrCreateStepState(e.Step)
			ss.Status = wf.StepStatusInProgress
			ss.Followups = nil
			ts := e.Timestamp
			ss.StartedAt = &ts

		case EventWorkflowStepDone:
			ss := phaseState.GetOrCreateStepState(e.Step)
			ss.Status = wf.StepStatusCompleted
			ts := e.Timestamp
			ss.CompletedAt = &ts
			ss.Feedback = e.Feedback
			ss.Followups = nil
			recordDecision(ss, e)

		case EventWorkflowStepSkip:
			ss := phaseState.GetOrCreateStepState(e.Step)
			ss.Status = wf.StepStatusSkipped
			ts := e.Timestamp
			ss.CompletedAt = &ts
			ss.Feedback = e.Feedback
			ss.Followups = nil

		case EventWorkflowStepBlock:
			ss := phaseState.GetOrCreateStepState(e.Step)
			ss.Status = wf.StepStatusBlocked
			ss.Feedback = e.Feedback
			ss.Followups = e.Followups
			recordDecision(ss, e)

		case EventWorkflowStepFailRemediation:
			ss := phaseState.GetOrCreateStepState(e.Step)
			ss.Status = wf.StepStatusFailedRemediation
			ss.Feedback = e.Feedback
			ss.Followups = e.Followups
			ss.RemediationPhase = e.RemediationPhase
			recordDecision(ss, e)

		case EventWorkflowStepRecheck:
			ss := phaseState.GetOrCreateStepState(e.Step)
			if ss.Status == wf.StepStatusFailedRemediation {
				ss.Status = wf.StepStatusInProgress
				ss.RemediationPhase = ""
				ss.Followups = nil
				ss.DecisionActor = ""
				ss.DecisionSummary = ""
				ss.DecisionAt = nil
				ss.Judge = nil
			}

		case EventWorkflowJudgeRecheck:
			ss := phaseState.GetOrCreateStepState(e.Step)
			if wf.IsJudgeRejection(ss) {
				ss.Status = wf.StepStatusInProgress
				ss.Feedback = ""
				ss.Followups = nil
				ss.RemediationPhase = ""
				ss.DecisionActor = ""
				ss.DecisionSummary = ""
				ss.DecisionAt = nil
				ss.Judge = nil
			}

		case EventWorkflowJudgeStarted:
			ss := phaseState.GetOrCreateStepState(e.Step)
			ts := e.Timestamp
			ss.Judge = &wf.JudgeState{
				Status:    wf.JudgeRunning,
				Command:   e.JudgeCommand,
				StartedAt: &ts,
				Pid:       e.JudgePid,
				RunID:     e.JudgeRunID,
			}

		case EventWorkflowJudgeClaimed:
			ss := phaseState.GetOrCreateStepState(e.Step)
			if ss.Judge != nil && ss.Judge.Status == wf.JudgeRunning &&
				ss.Judge.RunID == e.JudgeRunID {
				ss.Judge.Pid = e.JudgePid
			}

		case EventWorkflowJudgeReturned:
			ss := phaseState.GetOrCreateStepState(e.Step)
			// Run IDs make late events from superseded detached processes inert.
			// Empty IDs remain compatible with events written before leases existed.
			if e.JudgeRunID != "" && (ss.Judge == nil || ss.Judge.Status != wf.JudgeRunning ||
				ss.Judge.RunID != e.JudgeRunID) {
				continue
			}
			if ss.Judge == nil {
				ss.Judge = &wf.JudgeState{}
			}
			ts := e.Timestamp
			ss.Judge.Status = e.JudgeStatus
			ss.Judge.Detail = e.JudgeDetail
			ss.Judge.FinishedAt = &ts

		case EventWorkflowJudgeCleared:
			ss := phaseState.GetOrCreateStepState(e.Step)
			// A run ID prevents an older operator cleanup from clearing a
			// newer judge lease that started before the event was replayed.
			if e.JudgeRunID == "" || (ss.Judge != nil && ss.Judge.RunID == e.JudgeRunID) {
				ss.Judge = nil
			}

		case EventWorkflowAdvance:
			phaseState.CurrentStep = e.Step

		case EventWorkflowReset:
			phaseState.CurrentStep = 1
			for _, ss := range phaseState.Steps {
				ss.Status = wf.StepStatusPending
				ss.StartedAt = nil
				ss.CompletedAt = nil
				ss.Feedback = ""
				ss.DecisionActor = ""
				ss.DecisionSummary = ""
				ss.DecisionAt = nil
				ss.Judge = nil
			}
		}

		phaseState.UpdatedAt = e.Timestamp
		state.UpdatedAt = e.Timestamp
	}

	return state
}

func recordDecision(ss *wf.StepState, e ProgressEvent) {
	if e.DecisionActor == "" && e.DecisionSummary == "" {
		return
	}
	ss.DecisionActor = e.DecisionActor
	ss.DecisionSummary = e.DecisionSummary
	ts := e.Timestamp
	ss.DecisionAt = &ts
}

// generateWorkflowEventsFromYAML converts a FestivalWorkflowState (from YAML) into
// synthetic progress events. Used during migration from workflow_state.yaml to JSONL.
func generateWorkflowEventsFromYAML(state *wf.FestivalWorkflowState) []ProgressEvent {
	var events []ProgressEvent
	if state == nil {
		return events
	}

	for phaseName, phaseState := range state.Phases {
		if phaseState.TotalSteps == 0 {
			continue
		}

		// Emit wf_init
		initTS := phaseState.CreatedAt
		if initTS.IsZero() {
			initTS = state.CreatedAt
		}
		events = append(events, ProgressEvent{
			Timestamp:  initTS,
			Event:      EventWorkflowInit,
			Phase:      phaseName,
			TotalSteps: phaseState.TotalSteps,
		})

		// Emit step events in order
		for i := 1; i <= phaseState.TotalSteps; i++ {
			ss := phaseState.GetStepState(i)
			if ss == nil {
				continue
			}

			if ss.StartedAt != nil {
				events = append(events, ProgressEvent{
					Timestamp: *ss.StartedAt,
					Event:     EventWorkflowStepStart,
					Phase:     phaseName,
					Step:      i,
				})
			}

			switch ss.Status {
			case wf.StepStatusCompleted:
				ts := initTS.Add(time.Duration(i) * time.Second) // Fallback ordering
				if ss.CompletedAt != nil {
					ts = *ss.CompletedAt
				}
				events = append(events, ProgressEvent{
					Timestamp:       ts,
					Event:           EventWorkflowStepDone,
					Phase:           phaseName,
					Step:            i,
					Feedback:        ss.Feedback,
					DecisionActor:   ss.DecisionActor,
					DecisionSummary: ss.DecisionSummary,
				})

			case wf.StepStatusSkipped:
				ts := initTS.Add(time.Duration(i) * time.Second) // Fallback ordering
				if ss.CompletedAt != nil {
					ts = *ss.CompletedAt
				}
				events = append(events, ProgressEvent{
					Timestamp: ts,
					Event:     EventWorkflowStepSkip,
					Phase:     phaseName,
					Step:      i,
					Feedback:  ss.Feedback,
				})

			case wf.StepStatusBlocked:
				ts := initTS.Add(time.Duration(i) * time.Second)
				if ss.StartedAt != nil {
					ts = ss.StartedAt.Add(time.Second)
				}
				events = append(events, ProgressEvent{
					Timestamp:       ts,
					Event:           EventWorkflowStepBlock,
					Phase:           phaseName,
					Step:            i,
					Feedback:        ss.Feedback,
					DecisionActor:   ss.DecisionActor,
					DecisionSummary: ss.DecisionSummary,
				})

			case wf.StepStatusFailedRemediation:
				ts := initTS.Add(time.Duration(i) * time.Second)
				if ss.StartedAt != nil {
					ts = ss.StartedAt.Add(time.Second)
				}
				events = append(events, ProgressEvent{
					Timestamp:        ts,
					Event:            EventWorkflowStepFailRemediation,
					Phase:            phaseName,
					Step:             i,
					Feedback:         ss.Feedback,
					RemediationPhase: ss.RemediationPhase,
					DecisionActor:    ss.DecisionActor,
					DecisionSummary:  ss.DecisionSummary,
				})
			}
		}

		// Emit advance event for current step if > 1
		if phaseState.CurrentStep > 1 {
			ts := phaseState.UpdatedAt
			if ts.IsZero() {
				ts = initTS.Add(time.Duration(phaseState.CurrentStep) * time.Second)
			}
			events = append(events, ProgressEvent{
				Timestamp: ts,
				Event:     EventWorkflowAdvance,
				Phase:     phaseName,
				Step:      phaseState.CurrentStep,
			})
		}
	}

	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events
}

// appendEvents appends multiple events to the JSONL file.
func (s *Store) appendEvents(ctx context.Context, events []*ProgressEvent) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	eventsPath := s.eventsFilePath()

	if err := os.MkdirAll(filepath.Dir(eventsPath), 0755); err != nil {
		return errors.IO("creating progress directory", err).
			WithField("path", filepath.Dir(eventsPath))
	}

	f, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.IO("opening events file for append", err).
			WithField("path", eventsPath)
	}

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			_ = f.Close()
			return errors.Wrap(err, "marshaling progress event")
		}
		data = append(data, '\n')

		if _, err := f.Write(data); err != nil {
			_ = f.Close()
			return errors.IO("appending progress event", err).
				WithField("path", eventsPath)
		}
	}

	if err := f.Close(); err != nil {
		return errors.IO("closing events file", err).
			WithField("path", eventsPath)
	}

	return nil
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

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			_ = f.Close()
			return errors.Wrap(err, "marshaling progress event")
		}
		data = append(data, '\n')

		if _, err := f.Write(data); err != nil {
			_ = f.Close()
			return errors.IO("writing progress event", err).
				WithField("path", eventsPath)
		}
	}

	// Close and check for errors (some filesystems report write errors on close)
	if err := f.Close(); err != nil {
		return errors.IO("closing events file", err).
			WithField("path", eventsPath)
	}

	return nil
}
