package guidance

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/yamlutil"
	"gopkg.in/yaml.v3"
)

// GuidanceState represents persisted guidance state.
// It is stored in .fest/guidance_state.yaml within the festival directory.
type GuidanceState struct {
	// Festival identification
	FestivalPath string `yaml:"festival_path" json:"festival_path"`
	FestivalID   string `yaml:"festival_id,omitempty" json:"festival_id,omitempty"`

	// Current mode
	Mode Mode `yaml:"mode" json:"mode"`

	// Position tracking
	CurrentPhase    int `yaml:"current_phase" json:"current_phase"`
	CurrentSequence int `yaml:"current_sequence" json:"current_sequence"`
	CurrentStep     int `yaml:"current_step" json:"current_step"`

	// Task statuses (path -> status)
	TaskStatuses map[string]string `yaml:"task_statuses" json:"task_statuses"`

	// Timestamps
	StartedAt   time.Time `yaml:"started_at" json:"started_at"`
	LastUpdated time.Time `yaml:"last_updated" json:"last_updated"`

	// Progress counters
	TotalTasks     int `yaml:"total_tasks" json:"total_tasks"`
	CompletedTasks int `yaml:"completed_tasks" json:"completed_tasks"`
	SkippedTasks   int `yaml:"skipped_tasks" json:"skipped_tasks"`
	FailedTasks    int `yaml:"failed_tasks" json:"failed_tasks"`

	// Phase-specific state (mode-dependent data)
	// Key is mode name, value is mode-specific state
	PhaseState map[string]any `yaml:"phase_state,omitempty" json:"phase_state,omitempty"`
}

// TaskStatus constants
const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusSkipped    = "skipped"
	StatusFailed     = "failed"
)

// StateManager handles state persistence for all guidance modes.
type StateManager interface {
	// Load retrieves state from disk, creating default if not exists.
	Load(ctx context.Context) (*GuidanceState, error)

	// Save persists current state to disk.
	Save(ctx context.Context) error

	// Clear removes persisted state.
	Clear(ctx context.Context) error

	// GetTaskStatus returns the status of a specific task.
	GetTaskStatus(taskID string) string

	// SetTaskStatus updates the status of a specific task.
	SetTaskStatus(taskID, status string)

	// SetPosition updates the current position.
	SetPosition(phase, sequence, step int)

	// State returns the current state (may be stale, call Load first).
	State() *GuidanceState

	// UpdatePhaseState sets mode-specific state data.
	UpdatePhaseState(mode Mode, key string, value any)
}

// DefaultStateManager is the standard implementation of StateManager.
type DefaultStateManager struct {
	festivalPath string
	statePath    string
	state        *GuidanceState
}

// NewStateManager creates a new state manager for a festival.
func NewStateManager(festivalPath string, mode Mode) *DefaultStateManager {
	statePath := filepath.Join(festivalPath, ".fest", "guidance_state.yaml")
	return &DefaultStateManager{
		festivalPath: festivalPath,
		statePath:    statePath,
		state: &GuidanceState{
			FestivalPath: festivalPath,
			Mode:         mode,
			TaskStatuses: make(map[string]string),
			PhaseState:   make(map[string]any),
		},
	}
}

// Load retrieves state from disk.
func (sm *DefaultStateManager) Load(ctx context.Context) (*GuidanceState, error) {
	data, err := os.ReadFile(sm.statePath)
	if os.IsNotExist(err) {
		// Initialize new state
		sm.state.StartedAt = time.Now()
		sm.state.LastUpdated = time.Now()
		return sm.state, nil
	}
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, sm.state); err != nil {
		return nil, err
	}

	// Ensure maps are initialized
	if sm.state.TaskStatuses == nil {
		sm.state.TaskStatuses = make(map[string]string)
	}
	if sm.state.PhaseState == nil {
		sm.state.PhaseState = make(map[string]any)
	}

	return sm.state, nil
}

// Save persists state to disk.
func (sm *DefaultStateManager) Save(ctx context.Context) error {
	// Ensure directory exists
	dir := filepath.Dir(sm.statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	sm.state.LastUpdated = time.Now()

	data, err := yamlutil.Marshal(sm.state)
	if err != nil {
		return err
	}

	return os.WriteFile(sm.statePath, data, 0644)
}

// Clear removes the state file.
func (sm *DefaultStateManager) Clear(ctx context.Context) error {
	if err := os.Remove(sm.statePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Reset in-memory state
	sm.state = &GuidanceState{
		FestivalPath: sm.festivalPath,
		Mode:         sm.state.Mode,
		TaskStatuses: make(map[string]string),
		PhaseState:   make(map[string]any),
		StartedAt:    time.Now(),
		LastUpdated:  time.Now(),
	}

	return nil
}

// GetTaskStatus returns the status of a task.
func (sm *DefaultStateManager) GetTaskStatus(taskID string) string {
	if status, ok := sm.state.TaskStatuses[taskID]; ok {
		return status
	}
	return StatusPending
}

// SetTaskStatus sets the status of a task.
func (sm *DefaultStateManager) SetTaskStatus(taskID, status string) {
	if sm.state.TaskStatuses == nil {
		sm.state.TaskStatuses = make(map[string]string)
	}

	oldStatus := sm.state.TaskStatuses[taskID]
	sm.state.TaskStatuses[taskID] = status

	// Update counters
	sm.updateCounters(oldStatus, status)
}

// updateCounters adjusts progress counters based on status changes.
func (sm *DefaultStateManager) updateCounters(oldStatus, newStatus string) {
	// Decrement old status counter
	switch oldStatus {
	case StatusCompleted:
		sm.state.CompletedTasks--
	case StatusSkipped:
		sm.state.SkippedTasks--
	case StatusFailed:
		sm.state.FailedTasks--
	}

	// Increment new status counter
	switch newStatus {
	case StatusCompleted:
		sm.state.CompletedTasks++
	case StatusSkipped:
		sm.state.SkippedTasks++
	case StatusFailed:
		sm.state.FailedTasks++
	}
}

// SetPosition updates the current position.
func (sm *DefaultStateManager) SetPosition(phase, sequence, step int) {
	sm.state.CurrentPhase = phase
	sm.state.CurrentSequence = sequence
	sm.state.CurrentStep = step
}

// State returns the current state.
func (sm *DefaultStateManager) State() *GuidanceState {
	return sm.state
}

// UpdatePhaseState sets mode-specific state.
func (sm *DefaultStateManager) UpdatePhaseState(mode Mode, key string, value any) {
	if sm.state.PhaseState == nil {
		sm.state.PhaseState = make(map[string]any)
	}

	modeKey := string(mode)
	modeState, ok := sm.state.PhaseState[modeKey].(map[string]any)
	if !ok {
		modeState = make(map[string]any)
		sm.state.PhaseState[modeKey] = modeState
	}

	modeState[key] = value
}

// Progress returns a Progress struct from current state.
func (s *GuidanceState) Progress() *Progress {
	p := NewProgress(s.Mode)
	p.Total = s.TotalTasks
	p.Completed = s.CompletedTasks
	p.Skipped = s.SkippedTasks
	p.Failed = s.FailedTasks
	p.Pending = s.TotalTasks - s.CompletedTasks - s.SkippedTasks - s.FailedTasks
	p.Calculate()
	return p
}

// ProgressPercent returns the completion percentage.
func (s *GuidanceState) ProgressPercent() float64 {
	if s.TotalTasks == 0 {
		return 0
	}
	return float64(s.CompletedTasks) / float64(s.TotalTasks) * 100
}
