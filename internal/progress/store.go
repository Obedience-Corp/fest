// Package progress provides progress tracking for festival execution.
package progress

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"gopkg.in/yaml.v3"
)

const (
	// ProgressDir is the directory within festival for progress data
	ProgressDir = ".fest"

	// ProgressEventsFile is the JSONL event log format.
	// This is the only supported format for progress tracking.
	ProgressEventsFile = "progress_events.jsonl"

	// legacyProgressFile is the old YAML format.
	// Only used for migration - converted to JSONL on first access.
	// Unexported to discourage external use.
	legacyProgressFile = "progress.yaml"

	// ProgressFileName is kept for backward compatibility in tests.
	// Deprecated: Use ProgressEventsFile for new code.
	ProgressFileName = legacyProgressFile

	// Status values
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusBlocked    = "blocked"
	StatusCompleted  = "completed"
)

// TaskProgress represents the progress state of a single task
type TaskProgress struct {
	TaskID           string     `yaml:"task_id"`
	Status           string     `yaml:"status"`
	Progress         int        `yaml:"progress"`
	StartedAt        *time.Time `yaml:"started_at,omitempty"`
	CompletedAt      *time.Time `yaml:"completed_at,omitempty"`
	TimeSpentMinutes int        `yaml:"time_spent_minutes,omitempty"`
	BlockerMessage   string     `yaml:"blocker_message,omitempty"`
	BlockedAt        *time.Time `yaml:"blocked_at,omitempty"`
}

// FestivalTimeMetrics tracks festival-level time metrics separate from task-level tracking.
// This enables displaying both "how long agents worked" (TotalWorkMinutes) and
// "how long the festival has existed" (lifecycle from CreatedAt to CompletedAt).
//
// Note: This is distinct from FestivalProgress in aggregate.go which tracks
// task counts and completion status. FestivalTimeMetrics focuses on time data.
//
// Example usage:
//
//	ftm := &FestivalTimeMetrics{
//	    CreatedAt:        time.Now(),
//	    TotalWorkMinutes: 0,
//	}
//	// Later, when festival completes:
//	now := time.Now()
//	ftm.CompletedAt = &now
//	ftm.LifecycleDuration = int(now.Sub(ftm.CreatedAt).Hours() / 24)
type FestivalTimeMetrics struct {
	// CreatedAt is when the festival was created (from fest.yaml or directory creation time)
	CreatedAt time.Time `yaml:"created_at"`
	// CompletedAt is when the last task was completed (nil if festival is ongoing)
	CompletedAt *time.Time `yaml:"completed_at,omitempty"`
	// LifecycleDuration is the number of days from creation to completion (0 if ongoing)
	LifecycleDuration int `yaml:"lifecycle_duration_days,omitempty"`
	// TotalWorkMinutes is the sum of all task TimeSpentMinutes (agent work time)
	TotalWorkMinutes int `yaml:"total_work_minutes"`
}

// FestivalProgressData is the persisted progress state for a festival
type FestivalProgressData struct {
	Festival    string                   `yaml:"festival"`
	UpdatedAt   time.Time                `yaml:"updated_at"`
	TimeMetrics *FestivalTimeMetrics     `yaml:"time_metrics,omitempty"`
	Tasks       map[string]*TaskProgress `yaml:"tasks"`
}

// Store manages progress persistence
type Store struct {
	festivalPath  string
	data          *FestivalProgressData
	workflowData  *wf.FestivalWorkflowState
	pendingEvents []*ProgressEvent // Events to append on next Save()
}

// Compile-time check that Store implements wf.StateStore.
var _ wf.StateStore = (*Store)(nil)

// NewStore creates a new progress store for a festival
func NewStore(festivalPath string) *Store {
	return &Store{
		festivalPath: festivalPath,
	}
}

// eventsFilePath returns the path to the JSONL events file
func (s *Store) eventsFilePath() string {
	return filepath.Join(s.festivalPath, ProgressDir, ProgressEventsFile)
}

// legacyFilePath returns the path to the legacy YAML progress file
func (s *Store) legacyFilePath() string {
	return filepath.Join(s.festivalPath, ProgressDir, legacyProgressFile)
}

// Load loads progress data from disk.
// Uses JSONL event format as primary, with convert-on-access for legacy YAML.
// Also migrates workflow_state.yaml into the JSONL event stream if present.
func (s *Store) Load(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	eventsPath := s.eventsFilePath()
	legacyPath := s.legacyFilePath()

	// Preferred: JSONL exists - load from events
	if fileExists(eventsPath) {
		if err := s.loadFromEvents(ctx); err != nil {
			return err
		}
		// Migrate workflow_state.yaml if it exists alongside JSONL
		if err := s.migrateWorkflowYAML(ctx); err != nil {
			return errors.Wrap(err, "migrating workflow state")
		}
		return nil
	}

	// Legacy: YAML exists - convert it to JSONL
	if fileExists(legacyPath) {
		if err := s.migrateFromLegacy(ctx); err != nil {
			return errors.Wrap(err, "migrating legacy progress format")
		}
		// Also migrate workflow_state.yaml if present
		if err := s.migrateWorkflowYAML(ctx); err != nil {
			return errors.Wrap(err, "migrating workflow state")
		}
		return nil
	}

	// No progress data exists - create empty state
	s.initializeEmptyState()
	// Still check for workflow_state.yaml (could exist without progress data)
	if err := s.migrateWorkflowYAML(ctx); err != nil {
		return errors.Wrap(err, "migrating workflow state")
	}
	return nil
}

// LoadReadOnly materializes progress state without mutating disk. Unlike Load,
// it never converts legacy YAML to JSONL or removes workflow_state.yaml; legacy
// formats are parsed into synthetic events in memory and materialized the same
// way Load would after migration. Use for orientation-only callers (walk/inspect).
func (s *Store) LoadReadOnly(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	var events []ProgressEvent

	switch {
	case fileExists(s.eventsFilePath()):
		parsed, err := s.parseEventsFile(ctx)
		if err != nil {
			return err
		}
		events = parsed
	case fileExists(s.legacyFilePath()):
		if err := s.loadLegacyYAML(ctx); err != nil {
			return errors.Wrap(err, "loading legacy progress format")
		}
		events = generateEventsFromState(s.data.Tasks)
	}

	if fileExists(s.workflowYAMLPath()) {
		workflowEvents, err := s.readonlyWorkflowEvents(ctx)
		if err != nil {
			return err
		}
		events = append(events, workflowEvents...)
	}

	s.materializeFrom(events)
	return nil
}

func (s *Store) readonlyWorkflowEvents(ctx context.Context) ([]ProgressEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}
	festState, err := s.parseWorkflowYAML()
	if err != nil {
		return nil, err
	}
	return generateWorkflowEventsFromYAML(festState), nil
}

// fileExists checks if a file exists and is not a directory
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

// initializeEmptyState creates empty progress data with initialized time metrics
func (s *Store) initializeEmptyState() {
	now := time.Now().UTC()
	s.data = &FestivalProgressData{
		Festival:  filepath.Base(s.festivalPath),
		UpdatedAt: now,
		TimeMetrics: &FestivalTimeMetrics{
			CreatedAt: now,
		},
		Tasks: make(map[string]*TaskProgress),
	}
	s.workflowData = wf.NewFestivalWorkflowState()
}

// migrateFromLegacy converts a legacy YAML progress file to JSONL format.
// After successful migration, the YAML file is deleted.
func (s *Store) migrateFromLegacy(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// 1. Load YAML
	if err := s.loadLegacyYAML(ctx); err != nil {
		return err
	}

	// 2. Generate synthetic events from current state
	events := generateEventsFromState(s.data.Tasks)

	// 3. Write JSONL (only if we have events to write)
	if len(events) > 0 {
		if err := s.writeEvents(ctx, events); err != nil {
			return err
		}
	} else {
		// Create empty events file to mark migration complete
		eventsPath := s.eventsFilePath()
		if err := os.MkdirAll(filepath.Dir(eventsPath), 0755); err != nil {
			return errors.IO("creating progress directory", err).
				WithField("path", filepath.Dir(eventsPath))
		}
		if err := os.WriteFile(eventsPath, []byte{}, 0644); err != nil {
			return errors.IO("creating empty events file", err).
				WithField("path", eventsPath)
		}
	}

	// 4. Remove legacy file (ignore errors - JSONL is written, YAML removal is cleanup)
	legacyPath := s.legacyFilePath()
	_ = os.Remove(legacyPath)

	return nil
}

// loadLegacyYAML reads the legacy YAML progress file.
func (s *Store) loadLegacyYAML(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	legacyPath := s.legacyFilePath()

	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return errors.IO("reading legacy progress file", err).
			WithField("path", legacyPath)
	}

	var progressData FestivalProgressData
	if err := yaml.Unmarshal(data, &progressData); err != nil {
		return errors.Parse("parsing legacy progress file", err).
			WithField("path", legacyPath)
	}

	// Initialize maps if nil
	if progressData.Tasks == nil {
		progressData.Tasks = make(map[string]*TaskProgress)
	}

	// Initialize TimeMetrics for legacy files that don't have it
	if progressData.TimeMetrics == nil {
		progressData.TimeMetrics = &FestivalTimeMetrics{
			CreatedAt: progressData.UpdatedAt, // Use first known timestamp as fallback
		}
	}

	s.data = &progressData
	return nil
}

// Save writes progress data to disk by appending pending events to JSONL.
// Only JSONL is written - no YAML output.
func (s *Store) Save(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	if len(s.pendingEvents) == 0 {
		return nil
	}

	// Append all pending events to JSONL
	if err := s.appendEvents(ctx, s.pendingEvents); err != nil {
		return errors.Wrap(err, "appending progress events")
	}

	// Clear pending events after successful write
	s.pendingEvents = nil
	s.data.UpdatedAt = time.Now().UTC()

	return nil
}

// QueueEvent queues an event to be written on the next Save() call.
// The event will be appended to the JSONL file when Save() is called.
func (s *Store) QueueEvent(event *ProgressEvent) {
	s.pendingEvents = append(s.pendingEvents, event)
}

// Data returns the current progress data
func (s *Store) Data() *FestivalProgressData {
	return s.data
}

// GetTask retrieves progress for a specific task
func (s *Store) GetTask(taskID string) (*TaskProgress, bool) {
	if s.data == nil || s.data.Tasks == nil {
		return nil, false
	}
	task, ok := s.data.Tasks[taskID]
	return task, ok
}

// SetTask updates or creates progress for a task
func (s *Store) SetTask(task *TaskProgress) {
	if s.data == nil {
		s.data = &FestivalProgressData{
			Festival:  filepath.Base(s.festivalPath),
			UpdatedAt: time.Now().UTC(),
			Tasks:     make(map[string]*TaskProgress),
		}
	}
	s.data.Tasks[task.TaskID] = task
}

// AllTasks returns all task progress entries
func (s *Store) AllTasks() map[string]*TaskProgress {
	if s.data == nil {
		return nil
	}
	return s.data.Tasks
}

// GetTimeMetrics returns the festival time metrics (may be nil for legacy data)
func (s *Store) GetTimeMetrics() *FestivalTimeMetrics {
	if s.data == nil {
		return nil
	}
	return s.data.TimeMetrics
}

// SetTimeMetrics sets the festival time metrics
func (s *Store) SetTimeMetrics(metrics *FestivalTimeMetrics) {
	if s.data == nil {
		s.data = &FestivalProgressData{
			Festival:  filepath.Base(s.festivalPath),
			UpdatedAt: time.Now().UTC(),
			Tasks:     make(map[string]*TaskProgress),
		}
	}
	s.data.TimeMetrics = metrics
}

// EnsureTimeMetrics ensures TimeMetrics is initialized, creating it if nil
func (s *Store) EnsureTimeMetrics() *FestivalTimeMetrics {
	if s.data == nil {
		s.data = &FestivalProgressData{
			Festival:  filepath.Base(s.festivalPath),
			UpdatedAt: time.Now().UTC(),
			Tasks:     make(map[string]*TaskProgress),
		}
	}
	if s.data.TimeMetrics == nil {
		s.data.TimeMetrics = &FestivalTimeMetrics{
			CreatedAt: time.Now().UTC(),
		}
	}
	return s.data.TimeMetrics
}

// MarkFestivalCompleted sets the completion timestamp and calculates lifecycle duration
func (s *Store) MarkFestivalCompleted() {
	metrics := s.EnsureTimeMetrics()
	now := time.Now().UTC()
	metrics.CompletedAt = &now
	// Calculate lifecycle duration in days
	if !metrics.CreatedAt.IsZero() {
		metrics.LifecycleDuration = int(now.Sub(metrics.CreatedAt).Hours() / 24)
	}
}

// UpdateTotalWorkMinutes recalculates TotalWorkMinutes from all tasks
func (s *Store) UpdateTotalWorkMinutes() {
	if s.data == nil {
		return
	}
	total := 0
	for _, task := range s.data.Tasks {
		total += task.TimeSpentMinutes
	}
	metrics := s.EnsureTimeMetrics()
	metrics.TotalWorkMinutes = total
}

// LazyPopulateTimeData infers time data for completed tasks that don't have it.
// This provides a seamless experience for legacy festivals - time data appears
// automatically on first access without requiring users to run migration.
func (s *Store) LazyPopulateTimeData(festivalPath string) bool {
	if s.data == nil || len(s.data.Tasks) == 0 {
		return false
	}

	modified := false
	for _, task := range s.data.Tasks {
		if task.Status != StatusCompleted {
			continue
		}

		// Skip tasks that already have time data
		if task.TimeSpentMinutes > 0 {
			continue
		}

		// Build full task path and infer time
		taskPath := filepath.Join(festivalPath, task.TaskID)
		if InferTaskTime(taskPath, task) {
			modified = true
		}
	}

	if modified {
		s.UpdateTotalWorkMinutes()
	}

	return modified
}

// IsFestivalComplete returns true if all tracked tasks are completed
func (s *Store) IsFestivalComplete() bool {
	if s.data == nil || len(s.data.Tasks) == 0 {
		return false // Empty festivals are not complete
	}
	for _, task := range s.data.Tasks {
		if task.Status != StatusCompleted {
			return false
		}
	}
	return true
}

// CheckAndSetCompletion checks if festival is complete and marks it if so
// This is idempotent - calling multiple times will not reset CompletedAt
func (s *Store) CheckAndSetCompletion() bool {
	if !s.IsFestivalComplete() {
		return false
	}
	metrics := s.GetTimeMetrics()
	if metrics != nil && metrics.CompletedAt != nil {
		return false // Already marked complete
	}
	s.MarkFestivalCompleted()
	s.UpdateTotalWorkMinutes()
	return true
}

// CalculateLifecycleDuration computes days from creation to completion.
// Returns -1 for ongoing festivals (not yet completed).
// No arbitrary caps - if festival took 365 days, returns 365.
func (m *FestivalTimeMetrics) CalculateLifecycleDuration() int {
	if m == nil || m.CompletedAt == nil {
		return -1 // ongoing
	}

	duration := m.CompletedAt.Sub(m.CreatedAt)
	days := int(duration.Hours() / 24)

	return days
}

// GetLifecycleDuration returns the lifecycle duration, recalculating if needed
func (m *FestivalTimeMetrics) GetLifecycleDuration() int {
	if m == nil {
		return -1
	}
	if m.CompletedAt != nil {
		m.LifecycleDuration = m.CalculateLifecycleDuration()
	}
	return m.LifecycleDuration
}

// FormatLifecycleDuration formats lifecycle duration for display
func FormatLifecycleDuration(days int) string {
	if days < 0 {
		return "ongoing"
	}
	if days == 0 {
		return "< 1 day"
	}
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// GetCurrentDuration calculates days since festival creation (for ongoing festivals)
func (m *FestivalTimeMetrics) GetCurrentDuration() int {
	if m == nil {
		return 0
	}
	duration := time.Since(m.CreatedAt)
	return int(duration.Hours() / 24)
}

// FormatDurationWithStatus formats duration with status indicator for ongoing festivals
// For completed festivals, shows "X days"
// For ongoing festivals, shows "X days (ongoing)" or "< 1 day (ongoing)"
func FormatDurationWithStatus(metrics *FestivalTimeMetrics) string {
	if metrics == nil {
		return "unknown"
	}

	// Completed festival - use stored lifecycle duration
	if metrics.CompletedAt != nil {
		days := metrics.GetLifecycleDuration()
		return FormatLifecycleDuration(days)
	}

	// Ongoing festival - calculate current duration
	days := metrics.GetCurrentDuration()
	if days == 0 {
		return "< 1 day (ongoing)"
	}
	if days == 1 {
		return "1 day (ongoing)"
	}
	return fmt.Sprintf("%d days (ongoing)", days)
}

// --- Workflow state accessors ---

// WorkflowState returns the full festival workflow state materialized from events.
func (s *Store) WorkflowState() *wf.FestivalWorkflowState {
	if s.workflowData == nil {
		s.workflowData = wf.NewFestivalWorkflowState()
	}
	return s.workflowData
}

// WorkflowPhaseState returns the workflow state for a specific phase.
func (s *Store) WorkflowPhaseState(phaseName string) (*wf.WorkflowState, bool) {
	if s.workflowData == nil {
		return nil, false
	}
	state, ok := s.workflowData.Phases[phaseName]
	return state, ok
}

// GatePhaseState returns the phase gate state for a specific phase.
// Phase gates are stored with a "gate:" prefix to distinguish from workflow state.
func (s *Store) GatePhaseState(phaseName string) (*wf.WorkflowState, bool) {
	return s.WorkflowPhaseState("gate:" + phaseName)
}

// FestivalPath returns the festival path for this store.
func (s *Store) FestivalPath() string {
	return s.festivalPath
}

// --- wf.StateStore interface implementation ---

// LoadWorkflowPhaseState returns the materialized workflow state for a phase.
func (s *Store) LoadWorkflowPhaseState(phaseName string) (*wf.WorkflowState, bool) {
	return s.WorkflowPhaseState(phaseName)
}

// QueueWorkflowEvents converts WorkflowEvents into ProgressEvents and queues them.
func (s *Store) QueueWorkflowEvents(events []wf.WorkflowEvent) {
	now := time.Now().UTC()
	for _, we := range events {
		pe := &ProgressEvent{
			Timestamp:        now,
			Event:            EventType(we.EventType),
			Phase:            we.Phase,
			Step:             we.Step,
			TotalSteps:       we.TotalSteps,
			Feedback:         we.Feedback,
			RemediationPhase: we.RemediationPhase,
		}
		s.pendingEvents = append(s.pendingEvents, pe)
	}
}

// SaveEvents persists all queued events to disk. Implements wf.StateStore.
func (s *Store) SaveEvents(ctx context.Context) error {
	return s.Save(ctx)
}

// --- Workflow YAML migration ---

// workflowYAMLPath returns the path to the workflow_state.yaml file.
func (s *Store) workflowYAMLPath() string {
	return filepath.Join(s.festivalPath, ProgressDir, wf.StateFileName)
}

// migrateWorkflowYAML migrates workflow_state.yaml into the JSONL event stream.
// If the YAML file doesn't exist, this is a no-op.
func (s *Store) migrateWorkflowYAML(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	yamlPath := s.workflowYAMLPath()
	if !fileExists(yamlPath) {
		return nil
	}

	festState, err := s.parseWorkflowYAML()
	if err != nil {
		return err
	}

	syntheticEvents := generateWorkflowEventsFromYAML(festState)

	if len(syntheticEvents) > 0 {
		ptrs := make([]*ProgressEvent, len(syntheticEvents))
		for i := range syntheticEvents {
			ptrs[i] = &syntheticEvents[i]
		}
		if err := s.appendEvents(ctx, ptrs); err != nil {
			return errors.Wrap(err, "appending workflow migration events")
		}
	}

	// Reload from events to get the merged state
	if err := s.loadFromEvents(ctx); err != nil {
		return errors.Wrap(err, "reloading after workflow migration")
	}

	// Remove the YAML file
	_ = os.Remove(yamlPath)

	return nil
}

func (s *Store) parseWorkflowYAML() (*wf.FestivalWorkflowState, error) {
	yamlPath := s.workflowYAMLPath()

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, errors.IO("reading workflow state YAML", err).
			WithField("path", yamlPath)
	}

	var festState wf.FestivalWorkflowState
	if err := yaml.Unmarshal(data, &festState); err != nil {
		return nil, errors.Parse("parsing workflow state YAML", err).
			WithField("path", yamlPath)
	}

	if festState.Phases == nil {
		festState.Phases = make(map[string]*wf.WorkflowState)
	}
	for _, phaseState := range festState.Phases {
		if phaseState.Steps == nil {
			phaseState.Steps = make(map[int]*wf.StepState)
		}
	}

	return &festState, nil
}
