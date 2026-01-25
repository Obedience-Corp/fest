package execution

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
)

// mockPlanBuilder is a test implementation of PlanBuilder.
type mockPlanBuilder struct {
	plan *ExecutionPlan
	err  error
}

func (m *mockPlanBuilder) BuildPlan(ctx context.Context) (*ExecutionPlan, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.plan, nil
}

func TestNewNavigator(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: "/tmp/test-festival",
		Config:       guidance.DefaultConfig(),
	}
	pb := &mockPlanBuilder{
		plan: &ExecutionPlan{
			Summary: &ExecutionSummary{TotalTasks: 5},
		},
	}

	nav, err := NewNavigator(gctx, pb)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if nav == nil {
		t.Fatal("NewNavigator() returned nil")
	}

	if nav.planBuilder != pb {
		t.Error("NewNavigator() did not set plan builder")
	}

	if nav.Ctx != gctx {
		t.Error("NewNavigator() did not set context")
	}
}

func TestNewNavigator_NilContext(t *testing.T) {
	_, err := NewNavigator(nil, nil)
	if err == nil {
		t.Error("NewNavigator() expected error for nil context")
	}
}

func TestNavigator_Initialize(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}

	plan := &ExecutionPlan{
		FestivalPath: gctx.FestivalPath,
		Summary:      &ExecutionSummary{TotalTasks: 3},
		Phases: []*PhaseExecution{
			{
				Name:   "001_PHASE",
				Number: 1,
				Sequences: []*SequenceExecution{
					{
						Name:   "01_sequence",
						Number: 1,
						Steps: []*StepGroup{
							{
								Number: 1,
								Tasks: []*TaskInfo{
									{ID: "task-1", Name: "task1", Status: StatusPending},
									{ID: "task-2", Name: "task2", Status: StatusCompleted},
								},
							},
						},
					},
				},
			},
		},
	}

	pb := &mockPlanBuilder{plan: plan}
	nav, err := NewNavigator(gctx, pb)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Verify plan was set
	if nav.GetPlan() != plan {
		t.Error("Initialize() did not set plan")
	}

	// Verify state was synced
	state := nav.GetState()
	if state == nil {
		t.Fatal("GetState() returned nil")
	}

	// Check total tasks was updated
	if state.TotalTasks != 3 {
		t.Errorf("TotalTasks = %d, want 3", state.TotalTasks)
	}

	// Check completed task status was synced
	if status := state.TaskStatuses["task-2"]; status != StatusCompleted {
		t.Errorf("TaskStatuses[task-2] = %q, want %q", status, StatusCompleted)
	}
}

func TestNavigator_Initialize_NoPlanBuilder(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}

	nav, err := NewNavigator(gctx, nil)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	ctx := context.Background()
	err = nav.Initialize(ctx)
	if err == nil {
		t.Error("Initialize() expected error for nil plan builder")
	}
}

func TestNavigator_Initialize_BuildPlanError(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}

	pb := &mockPlanBuilder{err: errors.New("build failed")}
	nav, err := NewNavigator(gctx, pb)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	ctx := context.Background()
	err = nav.Initialize(ctx)
	if err == nil {
		t.Error("Initialize() expected error when BuildPlan fails")
	}
}

func TestNavigator_Initialize_ContextCancelled(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}

	pb := &mockPlanBuilder{
		plan: &ExecutionPlan{Summary: &ExecutionSummary{TotalTasks: 1}},
	}
	nav, err := NewNavigator(gctx, pb)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = nav.Initialize(ctx)
	if err == nil {
		t.Error("Initialize() expected error for cancelled context")
	}
}

func TestNavigator_SetPlanBuilder(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}

	nav, err := NewNavigator(gctx, nil)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if nav.planBuilder != nil {
		t.Error("planBuilder should be nil initially")
	}

	pb := &mockPlanBuilder{
		plan: &ExecutionPlan{Summary: &ExecutionSummary{TotalTasks: 1}},
	}
	nav.SetPlanBuilder(pb)

	if nav.planBuilder != pb {
		t.Error("SetPlanBuilder() did not set plan builder")
	}

	// Now Initialize should work
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error after SetPlanBuilder = %v", err)
	}
}

func TestNavigator_StubMethodsRequireInitialization(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}

	pb := &mockPlanBuilder{
		plan: &ExecutionPlan{Summary: &ExecutionSummary{TotalTasks: 1}},
	}
	nav, _ := NewNavigator(gctx, pb)
	ctx := context.Background()

	// These should all fail with "not initialized" before Initialize() is called
	if _, err := nav.GetNext(ctx); err == nil {
		t.Error("GetNext() should fail before Initialize")
	}

	if err := nav.MarkComplete(ctx, "test"); err == nil {
		t.Error("MarkComplete() should fail before Initialize")
	}

	if err := nav.MarkSkipped(ctx, "test"); err == nil {
		t.Error("MarkSkipped() should fail before Initialize")
	}

	if err := nav.MarkFailed(ctx, "test"); err == nil {
		t.Error("MarkFailed() should fail before Initialize")
	}

	if err := nav.Advance(ctx); err == nil {
		t.Error("Advance() should fail before Initialize")
	}

	if _, err := nav.GetProgress(ctx); err == nil {
		t.Error("GetProgress() should fail before Initialize")
	}

	if _, err := nav.FormatInstructions(ctx); err == nil {
		t.Error("FormatInstructions() should fail before Initialize")
	}

	if _, err := nav.GetContextFiles(ctx); err == nil {
		t.Error("GetContextFiles() should fail before Initialize")
	}
}

func TestNavigator_ImplementsInterface(t *testing.T) {
	// Compile-time check that Navigator implements guidance.Navigator
	var _ guidance.Navigator = (*Navigator)(nil)
}

func TestNavigatorFactoryFunc(t *testing.T) {
	called := false
	planBuilderFactory := func(festivalPath string) PlanBuilder {
		called = true
		return &mockPlanBuilder{
			plan: &ExecutionPlan{Summary: &ExecutionSummary{TotalTasks: 1}},
		}
	}

	factory := NavigatorFactoryFunc(planBuilderFactory)

	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}

	nav, err := factory(gctx)
	if err != nil {
		t.Fatalf("factory() error = %v", err)
	}

	if !called {
		t.Error("planBuilderFactory was not called")
	}

	if nav == nil {
		t.Error("factory() returned nil navigator")
	}
}

// --- GetNext tests ---

func TestNavigator_GetNext_SingleTask(t *testing.T) {
	nav := buildInitializedNavigator(t, buildSingleTaskPlan(t.TempDir()))
	ctx := context.Background()

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step == nil {
		t.Fatal("GetNext() returned nil step")
	}

	if step.ID != "task-1" {
		t.Errorf("GetNext() ID = %q, want %q", step.ID, "task-1")
	}

	if step.Parallel {
		t.Error("GetNext() Parallel = true, want false for single task")
	}

	if len(step.ContextFiles) == 0 {
		t.Error("GetNext() ContextFiles is empty")
	}
}

func TestNavigator_GetNext_ParallelTasks(t *testing.T) {
	nav := buildInitializedNavigator(t, buildParallelTaskPlan(t.TempDir()))
	ctx := context.Background()

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step == nil {
		t.Fatal("GetNext() returned nil step")
	}

	if !step.Parallel {
		t.Error("GetNext() Parallel = false, want true for parallel group")
	}

	if len(step.Group) != 2 {
		t.Errorf("GetNext() Group length = %d, want 2", len(step.Group))
	}
}

func TestNavigator_GetNext_AllComplete(t *testing.T) {
	plan := buildSingleTaskPlan(t.TempDir())
	plan.Phases[0].Sequences[0].Steps[0].Tasks[0].Status = StatusCompleted

	nav := buildInitializedNavigator(t, plan)
	ctx := context.Background()

	// Mark task as complete in state
	nav.StateManager.SetTaskStatus("task-1", StatusCompleted)

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step != nil {
		t.Errorf("GetNext() returned non-nil step when all complete: %+v", step)
	}
}

func TestNavigator_GetNext_MixedStates(t *testing.T) {
	plan := buildMultiTaskPlan(t.TempDir())
	nav := buildInitializedNavigator(t, plan)
	ctx := context.Background()

	// Mark first task as complete
	nav.StateManager.SetTaskStatus("task-1", StatusCompleted)

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	// Should return second task (first pending task after completed one)
	if step == nil {
		t.Fatal("GetNext() returned nil step")
	}

	if step.ID != "task-2" {
		t.Errorf("GetNext() ID = %q, want %q", step.ID, "task-2")
	}
}

func TestNavigator_GetNext_GateTask(t *testing.T) {
	plan := buildPlanWithGate(t.TempDir())
	nav := buildInitializedNavigator(t, plan)
	ctx := context.Background()

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step == nil {
		t.Fatal("GetNext() returned nil step")
	}

	if step.StepType != guidance.StepTypeGate {
		t.Errorf("GetNext() StepType = %q, want %q", step.StepType, guidance.StepTypeGate)
	}
}

func TestNavigator_GetNext_UpdatesPosition(t *testing.T) {
	nav := buildInitializedNavigator(t, buildSingleTaskPlan(t.TempDir()))
	ctx := context.Background()

	_, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	state := nav.GetState()
	if state.CurrentPhase != 0 {
		t.Errorf("CurrentPhase = %d, want 0", state.CurrentPhase)
	}
	if state.CurrentSequence != 0 {
		t.Errorf("CurrentSequence = %d, want 0", state.CurrentSequence)
	}
	if state.CurrentStep != 0 {
		t.Errorf("CurrentStep = %d, want 0", state.CurrentStep)
	}
}

// --- GetProgress tests ---

func TestNavigator_GetProgress(t *testing.T) {
	nav := buildInitializedNavigator(t, buildMultiTaskPlan(t.TempDir()))
	ctx := context.Background()

	// Mark one task complete
	nav.StateManager.SetTaskStatus("task-1", StatusCompleted)

	progress, err := nav.GetProgress(ctx)
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}

	if progress == nil {
		t.Fatal("GetProgress() returned nil")
	}

	if progress.Mode != guidance.ModeExecute {
		t.Errorf("Mode = %v, want %v", progress.Mode, guidance.ModeExecute)
	}

	if progress.Total != 3 {
		t.Errorf("Total = %d, want 3", progress.Total)
	}

	if progress.Completed != 1 {
		t.Errorf("Completed = %d, want 1", progress.Completed)
	}
}

func TestNavigator_GetProgress_PhaseProgress(t *testing.T) {
	nav := buildInitializedNavigator(t, buildMultiTaskPlan(t.TempDir()))
	ctx := context.Background()

	progress, err := nav.GetProgress(ctx)
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}

	if len(progress.PhaseProgress) == 0 {
		t.Error("PhaseProgress is empty")
	}

	phaseInfo, exists := progress.PhaseProgress["001_PHASE"]
	if !exists {
		t.Error("PhaseProgress[001_PHASE] not found")
	}

	if phaseInfo.Total != 3 {
		t.Errorf("PhaseInfo.Total = %d, want 3", phaseInfo.Total)
	}
}

// --- Advance tests ---

func TestNavigator_Advance(t *testing.T) {
	nav := buildInitializedNavigator(t, buildMultiTaskPlan(t.TempDir()))
	ctx := context.Background()

	// Mark first step's tasks as complete
	nav.StateManager.SetTaskStatus("task-1", StatusCompleted)

	err := nav.Advance(ctx)
	if err != nil {
		t.Fatalf("Advance() error = %v", err)
	}

	state := nav.GetState()
	// Should advance to second step
	if state.CurrentStep != 1 {
		t.Errorf("CurrentStep = %d, want 1", state.CurrentStep)
	}
}

func TestNavigator_Advance_AllComplete(t *testing.T) {
	nav := buildInitializedNavigator(t, buildSingleTaskPlan(t.TempDir()))
	ctx := context.Background()

	// Mark all tasks complete
	nav.StateManager.SetTaskStatus("task-1", StatusCompleted)

	err := nav.Advance(ctx)
	if !guidance.IsAlreadyComplete(err) {
		t.Errorf("Advance() error = %v, want ErrAlreadyComplete", err)
	}
}

// --- GetContextFiles tests ---

func TestNavigator_GetContextFiles(t *testing.T) {
	tmpDir := t.TempDir()
	nav := buildInitializedNavigator(t, buildSingleTaskPlan(tmpDir))
	ctx := context.Background()

	// First call GetNext to set position
	_, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	files, err := nav.GetContextFiles(ctx)
	if err != nil {
		t.Fatalf("GetContextFiles() error = %v", err)
	}

	if len(files) < 3 {
		t.Errorf("GetContextFiles() returned %d files, want at least 3", len(files))
	}

	// Check for expected file types
	hasGoal := false
	for _, f := range files {
		if f != "" {
			hasGoal = true
		}
	}
	if !hasGoal {
		t.Error("GetContextFiles() returned no goal files")
	}
}

// --- MarkComplete tests ---

func TestNavigator_MarkComplete(t *testing.T) {
	nav := buildInitializedNavigator(t, buildSingleTaskPlan(t.TempDir()))
	ctx := context.Background()

	err := nav.MarkComplete(ctx, "task-1")
	if err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	status := nav.StateManager.GetTaskStatus("task-1")
	if status != StatusCompleted {
		t.Errorf("GetTaskStatus() = %q, want %q", status, StatusCompleted)
	}
}

func TestNavigator_MarkComplete_NotInitialized(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}
	pb := &mockPlanBuilder{plan: &ExecutionPlan{Summary: &ExecutionSummary{TotalTasks: 1}}}
	nav, _ := NewNavigator(gctx, pb)
	// Don't initialize

	err := nav.MarkComplete(context.Background(), "task-1")
	if err == nil {
		t.Error("MarkComplete() should fail without initialization")
	}
}

func TestNavigator_MarkComplete_CancelledContext(t *testing.T) {
	nav := buildInitializedNavigator(t, buildSingleTaskPlan(t.TempDir()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := nav.MarkComplete(ctx, "task-1")
	if err == nil {
		t.Error("MarkComplete() should fail with cancelled context")
	}
}

// --- MarkSkipped tests ---

func TestNavigator_MarkSkipped(t *testing.T) {
	nav := buildInitializedNavigator(t, buildSingleTaskPlan(t.TempDir()))
	ctx := context.Background()

	err := nav.MarkSkipped(ctx, "task-1")
	if err != nil {
		t.Fatalf("MarkSkipped() error = %v", err)
	}

	status := nav.StateManager.GetTaskStatus("task-1")
	if status != StatusSkipped {
		t.Errorf("GetTaskStatus() = %q, want %q", status, StatusSkipped)
	}
}

func TestNavigator_MarkSkipped_NotInitialized(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}
	pb := &mockPlanBuilder{plan: &ExecutionPlan{Summary: &ExecutionSummary{TotalTasks: 1}}}
	nav, _ := NewNavigator(gctx, pb)

	err := nav.MarkSkipped(context.Background(), "task-1")
	if err == nil {
		t.Error("MarkSkipped() should fail without initialization")
	}
}

// --- MarkFailed tests ---

func TestNavigator_MarkFailed(t *testing.T) {
	nav := buildInitializedNavigator(t, buildSingleTaskPlan(t.TempDir()))
	ctx := context.Background()

	err := nav.MarkFailed(ctx, "task-1")
	if err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	status := nav.StateManager.GetTaskStatus("task-1")
	if status != StatusFailed {
		t.Errorf("GetTaskStatus() = %q, want %q", status, StatusFailed)
	}
}

func TestNavigator_MarkFailed_NotInitialized(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}
	pb := &mockPlanBuilder{plan: &ExecutionPlan{Summary: &ExecutionSummary{TotalTasks: 1}}}
	nav, _ := NewNavigator(gctx, pb)

	err := nav.MarkFailed(context.Background(), "task-1")
	if err == nil {
		t.Error("MarkFailed() should fail without initialization")
	}
}

// --- Marking updates counters tests ---

func TestNavigator_MarkComplete_UpdatesCounters(t *testing.T) {
	nav := buildInitializedNavigator(t, buildMultiTaskPlan(t.TempDir()))
	ctx := context.Background()

	// Mark one task complete
	err := nav.MarkComplete(ctx, "task-1")
	if err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	state := nav.GetState()
	if state.CompletedTasks != 1 {
		t.Errorf("CompletedTasks = %d, want 1", state.CompletedTasks)
	}

	// Mark another task complete
	err = nav.MarkComplete(ctx, "task-2")
	if err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	state = nav.GetState()
	if state.CompletedTasks != 2 {
		t.Errorf("CompletedTasks = %d, want 2", state.CompletedTasks)
	}
}

func TestNavigator_MarkSkipped_UpdatesCounters(t *testing.T) {
	nav := buildInitializedNavigator(t, buildMultiTaskPlan(t.TempDir()))
	ctx := context.Background()

	err := nav.MarkSkipped(ctx, "task-1")
	if err != nil {
		t.Fatalf("MarkSkipped() error = %v", err)
	}

	state := nav.GetState()
	if state.SkippedTasks != 1 {
		t.Errorf("SkippedTasks = %d, want 1", state.SkippedTasks)
	}
}

func TestNavigator_MarkFailed_UpdatesCounters(t *testing.T) {
	nav := buildInitializedNavigator(t, buildMultiTaskPlan(t.TempDir()))
	ctx := context.Background()

	err := nav.MarkFailed(ctx, "task-1")
	if err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	state := nav.GetState()
	if state.FailedTasks != 1 {
		t.Errorf("FailedTasks = %d, want 1", state.FailedTasks)
	}
}

// --- Helper functions for tests ---

func buildInitializedNavigator(t *testing.T, plan *ExecutionPlan) *Navigator {
	t.Helper()

	gctx := &guidance.GuidanceContext{
		FestivalPath: plan.FestivalPath,
		Config:       guidance.DefaultConfig(),
	}

	pb := &mockPlanBuilder{plan: plan}
	nav, err := NewNavigator(gctx, pb)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	return nav
}

func buildSingleTaskPlan(festivalPath string) *ExecutionPlan {
	return &ExecutionPlan{
		FestivalPath: festivalPath,
		Summary:      &ExecutionSummary{TotalTasks: 1},
		Phases: []*PhaseExecution{
			{
				Name:   "001_PHASE",
				Path:   festivalPath + "/001_PHASE",
				Number: 1,
				Sequences: []*SequenceExecution{
					{
						Name:   "01_sequence",
						Path:   festivalPath + "/001_PHASE/01_sequence",
						Number: 1,
						Steps: []*StepGroup{
							{
								Number: 1,
								Tasks: []*TaskInfo{
									{
										ID:            "task-1",
										Name:          "Task One",
										Path:          festivalPath + "/001_PHASE/01_sequence/01_task.md",
										AutonomyLevel: "medium",
										Status:        StatusPending,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func buildParallelTaskPlan(festivalPath string) *ExecutionPlan {
	return &ExecutionPlan{
		FestivalPath: festivalPath,
		Summary:      &ExecutionSummary{TotalTasks: 2},
		Phases: []*PhaseExecution{
			{
				Name:   "001_PHASE",
				Path:   festivalPath + "/001_PHASE",
				Number: 1,
				Sequences: []*SequenceExecution{
					{
						Name:   "01_sequence",
						Path:   festivalPath + "/001_PHASE/01_sequence",
						Number: 1,
						Steps: []*StepGroup{
							{
								Number:   1,
								Parallel: true,
								Tasks: []*TaskInfo{
									{
										ID:            "task-1",
										Name:          "Task One",
										Path:          festivalPath + "/001_PHASE/01_sequence/01_task_a.md",
										AutonomyLevel: "high",
										Status:        StatusPending,
									},
									{
										ID:            "task-2",
										Name:          "Task Two",
										Path:          festivalPath + "/001_PHASE/01_sequence/01_task_b.md",
										AutonomyLevel: "high",
										Status:        StatusPending,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func buildMultiTaskPlan(festivalPath string) *ExecutionPlan {
	return &ExecutionPlan{
		FestivalPath: festivalPath,
		Summary:      &ExecutionSummary{TotalTasks: 3},
		Phases: []*PhaseExecution{
			{
				Name:   "001_PHASE",
				Path:   festivalPath + "/001_PHASE",
				Number: 1,
				Sequences: []*SequenceExecution{
					{
						Name:   "01_sequence",
						Path:   festivalPath + "/001_PHASE/01_sequence",
						Number: 1,
						Steps: []*StepGroup{
							{
								Number: 1,
								Tasks: []*TaskInfo{
									{
										ID:            "task-1",
										Name:          "Task One",
										Path:          festivalPath + "/001_PHASE/01_sequence/01_task.md",
										AutonomyLevel: "medium",
										Status:        StatusPending,
									},
								},
							},
							{
								Number: 2,
								Tasks: []*TaskInfo{
									{
										ID:            "task-2",
										Name:          "Task Two",
										Path:          festivalPath + "/001_PHASE/01_sequence/02_task.md",
										AutonomyLevel: "medium",
										Status:        StatusPending,
									},
								},
							},
							{
								Number: 3,
								Tasks: []*TaskInfo{
									{
										ID:            "task-3",
										Name:          "Task Three",
										Path:          festivalPath + "/001_PHASE/01_sequence/03_task.md",
										AutonomyLevel: "medium",
										Status:        StatusPending,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func buildPlanWithGate(festivalPath string) *ExecutionPlan {
	return &ExecutionPlan{
		FestivalPath: festivalPath,
		Summary:      &ExecutionSummary{TotalTasks: 1},
		Phases: []*PhaseExecution{
			{
				Name:   "001_PHASE",
				Path:   festivalPath + "/001_PHASE",
				Number: 1,
				Sequences: []*SequenceExecution{
					{
						Name:   "01_sequence",
						Path:   festivalPath + "/001_PHASE/01_sequence",
						Number: 1,
						Steps: []*StepGroup{
							{
								Number: 1,
								Tasks: []*TaskInfo{
									{
										ID:            "gate-1",
										Name:          "Testing Gate",
										Path:          festivalPath + "/001_PHASE/01_sequence/01_testing.md",
										AutonomyLevel: "low",
										Status:        StatusPending,
										IsGate:        true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// --- FormatInstructions tests ---

func TestNavigator_FormatInstructions(t *testing.T) {
	tmpDir := t.TempDir()
	plan := buildSingleTaskPlan(tmpDir)
	nav := buildInitializedNavigator(t, plan)

	ctx := context.Background()
	output, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	// Verify the output contains expected sections
	if !strings.Contains(output, "Agent Execution Instructions") {
		t.Error("FormatInstructions() missing header")
	}
	if !strings.Contains(output, "ACTION REQUIRED") {
		t.Error("FormatInstructions() missing action instruction")
	}
	if !strings.Contains(output, "Tasks to Execute") {
		t.Error("FormatInstructions() missing tasks section")
	}
	if !strings.Contains(output, "Progress") {
		t.Error("FormatInstructions() missing progress line")
	}
	if !strings.Contains(output, "Current Position") {
		t.Error("FormatInstructions() missing position section")
	}
	if !strings.Contains(output, "Context Files") {
		t.Error("FormatInstructions() missing context section")
	}
	if !strings.Contains(output, "fest progress") {
		t.Error("FormatInstructions() missing completion command")
	}
}

func TestNavigator_FormatInstructions_AllComplete(t *testing.T) {
	tmpDir := t.TempDir()
	plan := buildSingleTaskPlan(tmpDir)
	// Mark all tasks as complete
	plan.Phases[0].Sequences[0].Steps[0].Tasks[0].Status = StatusCompleted
	nav := buildInitializedNavigator(t, plan)

	// Mark the task as complete in the state manager
	ctx := context.Background()
	nav.StateManager.SetTaskStatus("task-1", StatusCompleted)

	output, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	// Verify it shows completion message
	if !strings.Contains(output, "Execution Complete") {
		t.Error("FormatInstructions() should show completion message when all tasks complete")
	}
	if !strings.Contains(output, "All tasks have been executed") {
		t.Error("FormatInstructions() missing completion message body")
	}
}

func TestNavigator_FormatInstructions_ParallelTasks(t *testing.T) {
	tmpDir := t.TempDir()
	plan := buildParallelTaskPlan(tmpDir)
	nav := buildInitializedNavigator(t, plan)

	ctx := context.Background()
	output, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	// Verify multiple tasks are shown (task names from buildParallelTaskPlan)
	if !strings.Contains(output, "Task One") {
		t.Error("FormatInstructions() missing first parallel task")
	}
	if !strings.Contains(output, "Task Two") {
		t.Error("FormatInstructions() missing second parallel task")
	}
}

func TestNavigator_FormatInstructions_GateTask(t *testing.T) {
	tmpDir := t.TempDir()
	plan := buildPlanWithGate(tmpDir)
	nav := buildInitializedNavigator(t, plan)

	ctx := context.Background()
	output, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	// Verify gate-specific instruction
	if !strings.Contains(output, "gate file") || !strings.Contains(output, "verify") {
		t.Error("FormatInstructions() should show gate-specific instruction")
	}
	// Verify gate indicator
	if !strings.Contains(output, "◆") {
		t.Error("FormatInstructions() missing gate indicator")
	}
}

func TestNavigator_FormatInstructions_NotInitialized(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}

	pb := &mockPlanBuilder{
		plan: &ExecutionPlan{
			Summary: &ExecutionSummary{TotalTasks: 1},
		},
	}

	nav, err := NewNavigator(gctx, pb)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	// Don't initialize - should fail
	ctx := context.Background()
	_, err = nav.FormatInstructions(ctx)
	if err == nil {
		t.Error("FormatInstructions() should fail before Initialize")
	}
}

func TestNavigator_FormatInstructions_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	plan := buildSingleTaskPlan(tmpDir)
	nav := buildInitializedNavigator(t, plan)

	// Use cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := nav.FormatInstructions(ctx)
	if err == nil {
		t.Error("FormatInstructions() should fail with cancelled context")
	}
}

func TestNavigator_FormatInstructions_ShowsTaskPath(t *testing.T) {
	tmpDir := t.TempDir()
	plan := buildSingleTaskPlan(tmpDir)
	nav := buildInitializedNavigator(t, plan)

	ctx := context.Background()
	output, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	// Verify the task path is shown
	expectedPath := tmpDir + "/001_PHASE/01_sequence/01_task.md"
	if !strings.Contains(output, expectedPath) {
		t.Errorf("FormatInstructions() should show task path, got:\n%s", output)
	}
}

func TestNavigator_FormatInstructions_ShowsAutonomyLevel(t *testing.T) {
	tmpDir := t.TempDir()
	plan := buildSingleTaskPlan(tmpDir)
	nav := buildInitializedNavigator(t, plan)

	ctx := context.Background()
	output, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	// Verify autonomy level is shown
	if !strings.Contains(output, "Autonomy") {
		t.Error("FormatInstructions() should show autonomy level")
	}
}

// --- GetPlan backward compatibility tests ---

func TestNavigator_GetPlan(t *testing.T) {
	tmpDir := t.TempDir()
	plan := buildSingleTaskPlan(tmpDir)
	nav := buildInitializedNavigator(t, plan)

	// GetPlan should return the plan used to initialize the navigator
	gotPlan := nav.GetPlan()
	if gotPlan == nil {
		t.Fatal("GetPlan() returned nil after initialization")
	}

	// Verify plan matches what was passed
	if gotPlan.FestivalPath != plan.FestivalPath {
		t.Errorf("GetPlan().FestivalPath = %v, want %v", gotPlan.FestivalPath, plan.FestivalPath)
	}

	if len(gotPlan.Phases) != len(plan.Phases) {
		t.Errorf("GetPlan().Phases length = %d, want %d", len(gotPlan.Phases), len(plan.Phases))
	}

	if gotPlan.Summary.TotalTasks != plan.Summary.TotalTasks {
		t.Errorf("GetPlan().Summary.TotalTasks = %d, want %d", gotPlan.Summary.TotalTasks, plan.Summary.TotalTasks)
	}
}

func TestNavigator_GetPlan_BeforeInitialize(t *testing.T) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Config:       guidance.DefaultConfig(),
	}

	pb := &mockPlanBuilder{
		plan: &ExecutionPlan{
			Summary: &ExecutionSummary{TotalTasks: 1},
		},
	}

	nav, err := NewNavigator(gctx, pb)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	// GetPlan before Initialize should return nil
	gotPlan := nav.GetPlan()
	if gotPlan != nil {
		t.Error("GetPlan() should return nil before Initialize")
	}
}
