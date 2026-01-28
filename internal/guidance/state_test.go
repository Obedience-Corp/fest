package guidance

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStateManager(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir, ModeImplementation)

	if sm.festivalPath != dir {
		t.Errorf("festivalPath = %v, want %v", sm.festivalPath, dir)
	}
	expectedPath := filepath.Join(dir, ".fest", "guidance_state.yaml")
	if sm.statePath != expectedPath {
		t.Errorf("statePath = %v, want %v", sm.statePath, expectedPath)
	}
	if sm.state.FestivalPath != dir {
		t.Errorf("state.FestivalPath = %v, want %v", sm.state.FestivalPath, dir)
	}
	if sm.state.Mode != ModeImplementation {
		t.Errorf("state.Mode = %v, want %v", sm.state.Mode, ModeImplementation)
	}
	if sm.state.TaskStatuses == nil {
		t.Error("TaskStatuses should be initialized")
	}
	if sm.state.PhaseState == nil {
		t.Error("PhaseState should be initialized")
	}
}

func TestDefaultStateManager_Load_NewState(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir, ModeImplementation)

	state, err := sm.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if state == nil {
		t.Fatal("Load() returned nil state")
	}
	if state.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}
	if state.LastUpdated.IsZero() {
		t.Error("LastUpdated should be set")
	}
}

func TestDefaultStateManager_LoadSave(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir, ModeImplementation)

	// Load creates new state
	_, err := sm.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Modify state
	sm.SetTaskStatus("task1.md", StatusCompleted)
	sm.SetPosition(1, 2, 3)

	// Save
	if err := sm.Save(context.Background()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	statePath := filepath.Join(dir, ".fest", "guidance_state.yaml")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("State file should exist after Save()")
	}

	// Create new manager and load
	sm2 := NewStateManager(dir, ModeImplementation)
	state2, err := sm2.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify persisted
	if status := sm2.GetTaskStatus("task1.md"); status != StatusCompleted {
		t.Errorf("GetTaskStatus() = %v, want %v", status, StatusCompleted)
	}
	if state2.CurrentPhase != 1 {
		t.Errorf("CurrentPhase = %v, want 1", state2.CurrentPhase)
	}
	if state2.CurrentSequence != 2 {
		t.Errorf("CurrentSequence = %v, want 2", state2.CurrentSequence)
	}
	if state2.CurrentStep != 3 {
		t.Errorf("CurrentStep = %v, want 3", state2.CurrentStep)
	}
}

func TestDefaultStateManager_Counters(t *testing.T) {
	sm := NewStateManager(t.TempDir(), ModeImplementation)
	_, _ = sm.Load(context.Background())

	sm.state.TotalTasks = 10

	// Complete some tasks
	sm.SetTaskStatus("t1", StatusCompleted)
	sm.SetTaskStatus("t2", StatusCompleted)
	sm.SetTaskStatus("t3", StatusSkipped)
	sm.SetTaskStatus("t4", StatusFailed)

	state := sm.State()
	if state.CompletedTasks != 2 {
		t.Errorf("CompletedTasks = %d, want 2", state.CompletedTasks)
	}
	if state.SkippedTasks != 1 {
		t.Errorf("SkippedTasks = %d, want 1", state.SkippedTasks)
	}
	if state.FailedTasks != 1 {
		t.Errorf("FailedTasks = %d, want 1", state.FailedTasks)
	}

	// Check progress
	p := state.Progress()
	if p.Percentage != 20.0 {
		t.Errorf("Percentage = %v, want 20.0", p.Percentage)
	}
}

func TestDefaultStateManager_CounterTransitions(t *testing.T) {
	sm := NewStateManager(t.TempDir(), ModeImplementation)
	_, _ = sm.Load(context.Background())

	// Set initial status
	sm.SetTaskStatus("t1", StatusCompleted)
	if sm.state.CompletedTasks != 1 {
		t.Errorf("CompletedTasks after first complete = %d, want 1", sm.state.CompletedTasks)
	}

	// Change from completed to failed
	sm.SetTaskStatus("t1", StatusFailed)
	if sm.state.CompletedTasks != 0 {
		t.Errorf("CompletedTasks after fail = %d, want 0", sm.state.CompletedTasks)
	}
	if sm.state.FailedTasks != 1 {
		t.Errorf("FailedTasks after fail = %d, want 1", sm.state.FailedTasks)
	}

	// Change from failed to skipped
	sm.SetTaskStatus("t1", StatusSkipped)
	if sm.state.FailedTasks != 0 {
		t.Errorf("FailedTasks after skip = %d, want 0", sm.state.FailedTasks)
	}
	if sm.state.SkippedTasks != 1 {
		t.Errorf("SkippedTasks after skip = %d, want 1", sm.state.SkippedTasks)
	}
}

func TestDefaultStateManager_Clear(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir, ModeImplementation)

	_, _ = sm.Load(context.Background())
	sm.SetTaskStatus("t1", StatusCompleted)
	_ = sm.Save(context.Background())

	// Verify file exists
	statePath := filepath.Join(dir, ".fest", "guidance_state.yaml")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("State file should exist before Clear")
	}

	// Clear
	if err := sm.Clear(context.Background()); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	// File should be removed
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("State file should be removed after Clear")
	}

	// State should be reset
	if len(sm.State().TaskStatuses) != 0 {
		t.Error("TaskStatuses should be empty after Clear")
	}
	if sm.State().FestivalPath != dir {
		t.Error("FestivalPath should be preserved after Clear")
	}
	if sm.State().Mode != ModeImplementation {
		t.Error("Mode should be preserved after Clear")
	}
}

func TestDefaultStateManager_Clear_NoFile(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir, ModeImplementation)
	_, _ = sm.Load(context.Background())

	// Clear without saving first (no file exists)
	if err := sm.Clear(context.Background()); err != nil {
		t.Fatalf("Clear() should not error when file doesn't exist: %v", err)
	}
}

func TestDefaultStateManager_GetTaskStatus(t *testing.T) {
	sm := NewStateManager(t.TempDir(), ModeImplementation)
	_, _ = sm.Load(context.Background())

	// Unknown task returns pending
	if status := sm.GetTaskStatus("unknown"); status != StatusPending {
		t.Errorf("GetTaskStatus(unknown) = %v, want %v", status, StatusPending)
	}

	// Set and get
	sm.SetTaskStatus("task1", StatusCompleted)
	if status := sm.GetTaskStatus("task1"); status != StatusCompleted {
		t.Errorf("GetTaskStatus(task1) = %v, want %v", status, StatusCompleted)
	}
}

func TestDefaultStateManager_SetPosition(t *testing.T) {
	sm := NewStateManager(t.TempDir(), ModeImplementation)
	_, _ = sm.Load(context.Background())

	sm.SetPosition(5, 3, 2)

	state := sm.State()
	if state.CurrentPhase != 5 {
		t.Errorf("CurrentPhase = %d, want 5", state.CurrentPhase)
	}
	if state.CurrentSequence != 3 {
		t.Errorf("CurrentSequence = %d, want 3", state.CurrentSequence)
	}
	if state.CurrentStep != 2 {
		t.Errorf("CurrentStep = %d, want 2", state.CurrentStep)
	}
}

func TestDefaultStateManager_UpdatePhaseState(t *testing.T) {
	sm := NewStateManager(t.TempDir(), ModeImplementation)
	_, _ = sm.Load(context.Background())

	// Update phase state
	sm.UpdatePhaseState(ModeIngest, "iteration_count", 2)
	sm.UpdatePhaseState(ModeIngest, "pending_approval", true)
	sm.UpdatePhaseState(ModePlan, "plan_version", "v1")

	// Verify
	state := sm.State()
	ingestState, ok := state.PhaseState["ingest"].(map[string]any)
	if !ok {
		t.Fatal("Ingest phase state should exist")
	}
	if ingestState["iteration_count"] != 2 {
		t.Errorf("iteration_count = %v, want 2", ingestState["iteration_count"])
	}
	if ingestState["pending_approval"] != true {
		t.Errorf("pending_approval = %v, want true", ingestState["pending_approval"])
	}

	planState, ok := state.PhaseState["plan"].(map[string]any)
	if !ok {
		t.Fatal("Plan phase state should exist")
	}
	if planState["plan_version"] != "v1" {
		t.Errorf("plan_version = %v, want v1", planState["plan_version"])
	}
}

func TestDefaultStateManager_UpdatePhaseState_NilMap(t *testing.T) {
	sm := NewStateManager(t.TempDir(), ModeImplementation)
	sm.state.PhaseState = nil

	// Should not panic
	sm.UpdatePhaseState(ModeIngest, "key", "value")

	if sm.state.PhaseState == nil {
		t.Error("PhaseState should be initialized")
	}
}

func TestGuidanceState_Progress(t *testing.T) {
	state := &GuidanceState{
		Mode:           ModeImplementation,
		TotalTasks:     10,
		CompletedTasks: 5,
		SkippedTasks:   1,
		FailedTasks:    1,
	}

	p := state.Progress()

	if p.Mode != ModeImplementation {
		t.Errorf("Mode = %v, want ModeImplementation", p.Mode)
	}
	if p.Total != 10 {
		t.Errorf("Total = %d, want 10", p.Total)
	}
	if p.Completed != 5 {
		t.Errorf("Completed = %d, want 5", p.Completed)
	}
	if p.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", p.Skipped)
	}
	if p.Failed != 1 {
		t.Errorf("Failed = %d, want 1", p.Failed)
	}
	if p.Pending != 3 {
		t.Errorf("Pending = %d, want 3", p.Pending)
	}
	if p.Percentage != 50.0 {
		t.Errorf("Percentage = %v, want 50.0", p.Percentage)
	}
}

func TestGuidanceState_ProgressPercent(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		completed int
		want      float64
	}{
		{"zero total", 0, 0, 0},
		{"50%", 10, 5, 50.0},
		{"100%", 10, 10, 100.0},
		{"0%", 10, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &GuidanceState{
				TotalTasks:     tt.total,
				CompletedTasks: tt.completed,
			}
			if got := state.ProgressPercent(); got != tt.want {
				t.Errorf("ProgressPercent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultStateManager_Load_CorruptedFile(t *testing.T) {
	dir := t.TempDir()

	// Create corrupted state file
	festDir := filepath.Join(dir, ".fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(festDir, "guidance_state.yaml")
	if err := os.WriteFile(statePath, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatal(err)
	}

	sm := NewStateManager(dir, ModeImplementation)
	_, err := sm.Load(context.Background())

	if err == nil {
		t.Error("Load() should error on corrupted YAML")
	}
}

func TestDefaultStateManager_SetTaskStatus_NilMap(t *testing.T) {
	sm := NewStateManager(t.TempDir(), ModeImplementation)
	sm.state.TaskStatuses = nil

	// Should not panic
	sm.SetTaskStatus("task1", StatusCompleted)

	if sm.state.TaskStatuses == nil {
		t.Error("TaskStatuses should be initialized")
	}
	if sm.state.TaskStatuses["task1"] != StatusCompleted {
		t.Error("Task status should be set")
	}
}

func TestStatusConstants(t *testing.T) {
	// Verify constants have expected values
	tests := []struct {
		constant string
		want     string
	}{
		{StatusPending, "pending"},
		{StatusInProgress, "in_progress"},
		{StatusCompleted, "completed"},
		{StatusSkipped, "skipped"},
		{StatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if tt.constant != tt.want {
				t.Errorf("Status constant = %v, want %v", tt.constant, tt.want)
			}
		})
	}
}
