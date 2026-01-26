package planning

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
)

func TestDefaultPlanningSteps(t *testing.T) {
	steps := DefaultPlanningSteps()

	if len(steps) != 9 {
		t.Fatalf("Expected 9 planning steps, got %d", len(steps))
	}

	// Verify step order is sequential 1-9
	for i, step := range steps {
		expectedOrder := i + 1
		if step.Order != expectedOrder {
			t.Errorf("Step %d Order = %d, want %d", i, step.Order, expectedOrder)
		}
	}

	// First step should be requirements gathering
	if steps[0].ID != "gather_requirements" {
		t.Errorf("First step ID = %s, want gather_requirements", steps[0].ID)
	}

	// Last step should be planning approval
	last := steps[len(steps)-1]
	if last.ID != "planning_approval" {
		t.Errorf("Last step ID = %s, want planning_approval", last.ID)
	}
}

func TestDefaultPlanningSteps_StepIDs(t *testing.T) {
	expectedIDs := []string{
		"gather_requirements",
		"define_scope",
		"design_architecture",
		"create_phases",
		"create_sequences",
		"create_tasks",
		"add_quality_gates",
		"validate_plan",
		"planning_approval",
	}

	steps := DefaultPlanningSteps()

	for i, expectedID := range expectedIDs {
		if steps[i].ID != expectedID {
			t.Errorf("Step %d ID = %s, want %s", i+1, steps[i].ID, expectedID)
		}
	}
}

func TestDefaultPlanningSteps_AutonomyLevels(t *testing.T) {
	steps := DefaultPlanningSteps()

	// Steps 1-3: Medium autonomy
	for i := 0; i < 3; i++ {
		if steps[i].Autonomy != guidance.AutonomyMedium {
			t.Errorf("Step %d (%s) Autonomy = %s, want Medium", i+1, steps[i].ID, steps[i].Autonomy)
		}
	}

	// Steps 4-7: High autonomy
	for i := 3; i < 7; i++ {
		if steps[i].Autonomy != guidance.AutonomyHigh {
			t.Errorf("Step %d (%s) Autonomy = %s, want High", i+1, steps[i].ID, steps[i].Autonomy)
		}
	}

	// Steps 8-9: Low autonomy (require human review)
	for i := 7; i < 9; i++ {
		if steps[i].Autonomy != guidance.AutonomyLow {
			t.Errorf("Step %d (%s) Autonomy = %s, want Low", i+1, steps[i].ID, steps[i].Autonomy)
		}
	}
}

func TestDefaultPlanningSteps_HasRequiredFields(t *testing.T) {
	steps := DefaultPlanningSteps()

	for i, step := range steps {
		// Each step must have ID
		if step.ID == "" {
			t.Errorf("Step %d has empty ID", i+1)
		}

		// Each step must have Title
		if step.Title == "" {
			t.Errorf("Step %d (%s) has empty Title", i+1, step.ID)
		}

		// Each step must have Description
		if step.Description == "" {
			t.Errorf("Step %d (%s) has empty Description", i+1, step.ID)
		}

		// Each step must have at least one Instruction
		if len(step.Instructions) == 0 {
			t.Errorf("Step %d (%s) has no Instructions", i+1, step.ID)
		}

		// Each step must have at least one Deliverable
		if len(step.Deliverables) == 0 {
			t.Errorf("Step %d (%s) has no Deliverables", i+1, step.ID)
		}
	}
}

func TestNewNavigator(t *testing.T) {
	tests := []struct {
		name    string
		gctx    *guidance.GuidanceContext
		wantErr bool
	}{
		{
			name:    "nil context returns error",
			gctx:    nil,
			wantErr: true,
		},
		{
			name: "valid context creates navigator",
			gctx: &guidance.GuidanceContext{
				FestivalPath: t.TempDir(),
				Mode:         guidance.ModePlan,
				Config:       guidance.DefaultConfig(),
			},
			wantErr: false,
		},
		{
			name: "config is set to default if nil",
			gctx: &guidance.GuidanceContext{
				FestivalPath: t.TempDir(),
				Mode:         guidance.ModePlan,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav, err := NewNavigator(tt.gctx)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if nav == nil {
				t.Fatal("Navigator is nil")
			}

			// Verify steps are initialized
			if len(nav.steps) != 9 {
				t.Errorf("Expected 9 steps, got %d", len(nav.steps))
			}

			// Verify currentStep starts at 0
			if nav.currentStep != 0 {
				t.Errorf("Expected currentStep=0, got %d", nav.currentStep)
			}
		})
	}
}

func TestNavigator_Initialize(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Verify state is initialized
	state := nav.GetState()
	if state == nil {
		t.Fatal("State is nil after initialization")
	}

	// Verify TotalTasks is set
	if state.TotalTasks != 9 {
		t.Errorf("TotalTasks = %d, want 9", state.TotalTasks)
	}
}

func TestNavigator_Initialize_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = nav.Initialize(ctx)
	if err == nil {
		t.Error("Expected error for cancelled context, got nil")
	}
}

func TestNavigator_GetNext(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		FestivalName: "test-festival",
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Get first step
	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step == nil {
		t.Fatal("Expected step, got nil")
	}

	// Verify first step properties
	if step.ID != "gather_requirements" {
		t.Errorf("First step ID = %s, want gather_requirements", step.ID)
	}

	if step.Mode != guidance.ModePlan {
		t.Errorf("Mode = %v, want ModePlan", step.Mode)
	}

	if step.StepType != guidance.StepTypePlanningStep {
		t.Errorf("StepType = %s, want planning_step", step.StepType)
	}

	// Verify metadata
	if step.Metadata == nil {
		t.Fatal("Metadata is nil")
	}

	stepNumber, ok := step.Metadata["step_number"].(int)
	if !ok || stepNumber != 1 {
		t.Errorf("step_number = %v, want 1", step.Metadata["step_number"])
	}

	totalSteps, ok := step.Metadata["total_steps"].(int)
	if !ok || totalSteps != 9 {
		t.Errorf("total_steps = %v, want 9", step.Metadata["total_steps"])
	}
}

func TestNavigator_GetNext_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)

	// Don't initialize
	_, err := nav.GetNext(context.Background())
	if err == nil {
		t.Error("Expected error for uninitialized navigator, got nil")
	}
}

func TestNavigator_GetNext_AllComplete(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Advance past all steps
	nav.currentStep = len(nav.steps)

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step != nil {
		t.Error("Expected nil step when all complete, got step")
	}
}

func TestNavigator_MarkComplete(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Mark first step complete
	err := nav.MarkComplete(ctx, "gather_requirements")
	if err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	// Verify currentStep advanced
	if nav.currentStep != 1 {
		t.Errorf("currentStep = %d, want 1", nav.currentStep)
	}

	// Verify state updated
	status := nav.StateManager.GetTaskStatus("gather_requirements")
	if status != guidance.StatusCompleted {
		t.Errorf("Task status = %s, want completed", status)
	}
}

func TestNavigator_MarkComplete_OutOfOrder(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Try to complete step 2 without completing step 1
	err := nav.MarkComplete(ctx, "define_scope")
	if err == nil {
		t.Error("Expected error for out-of-order completion, got nil")
	}
}

func TestNavigator_MarkComplete_InvalidStep(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	err := nav.MarkComplete(ctx, "nonexistent_step")
	if err == nil {
		t.Error("Expected error for invalid step ID, got nil")
	}
}

func TestNavigator_MarkSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Skip first step
	err := nav.MarkSkipped(ctx, "gather_requirements")
	if err != nil {
		t.Fatalf("MarkSkipped() error = %v", err)
	}

	// Verify currentStep advanced
	if nav.currentStep != 1 {
		t.Errorf("currentStep = %d, want 1", nav.currentStep)
	}

	// Verify state updated
	status := nav.StateManager.GetTaskStatus("gather_requirements")
	if status != guidance.StatusSkipped {
		t.Errorf("Task status = %s, want skipped", status)
	}
}

func TestNavigator_MarkFailed(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Mark first step as failed
	err := nav.MarkFailed(ctx, "gather_requirements")
	if err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	// Verify currentStep did NOT advance (failed step blocks progress)
	if nav.currentStep != 0 {
		t.Errorf("currentStep = %d, want 0 (failed should not advance)", nav.currentStep)
	}

	// Verify state updated
	status := nav.StateManager.GetTaskStatus("gather_requirements")
	if status != guidance.StatusFailed {
		t.Errorf("Task status = %s, want failed", status)
	}
}

func TestNavigator_Advance(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Advance marks current as complete and moves forward
	err := nav.Advance(ctx)
	if err != nil {
		t.Fatalf("Advance() error = %v", err)
	}

	// Verify moved to step 2
	if nav.currentStep != 1 {
		t.Errorf("currentStep = %d, want 1", nav.currentStep)
	}
}

func TestNavigator_Advance_AlreadyComplete(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Move to end
	nav.currentStep = len(nav.steps)

	err := nav.Advance(ctx)
	if err == nil {
		t.Error("Expected ErrAlreadyComplete, got nil")
	}

	if err != guidance.ErrAlreadyComplete {
		t.Errorf("Expected ErrAlreadyComplete, got %v", err)
	}
}

func TestNavigator_GetProgress(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Initial progress
	progress, err := nav.GetProgress(ctx)
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}

	if progress.Mode != guidance.ModePlan {
		t.Errorf("Mode = %v, want ModePlan", progress.Mode)
	}

	if progress.Total != 9 {
		t.Errorf("Total = %d, want 9", progress.Total)
	}

	if progress.Completed != 0 {
		t.Errorf("Completed = %d, want 0", progress.Completed)
	}

	if progress.Pending != 9 {
		t.Errorf("Pending = %d, want 9", progress.Pending)
	}

	// Complete a step
	if err = nav.MarkComplete(ctx, "gather_requirements"); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}
	progress, _ = nav.GetProgress(ctx)

	if progress.Completed != 1 {
		t.Errorf("Completed = %d, want 1", progress.Completed)
	}

	if progress.Pending != 8 {
		t.Errorf("Pending = %d, want 8", progress.Pending)
	}
}

func TestNavigator_GetProgress_CurrentTask(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	progress, _ := nav.GetProgress(ctx)

	// Current task should be first step title
	if progress.CurrentTask != "Gather Requirements" {
		t.Errorf("CurrentTask = %s, want 'Gather Requirements'", progress.CurrentTask)
	}

	// Advance and check again
	if err := nav.MarkComplete(ctx, "gather_requirements"); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}
	progress, _ = nav.GetProgress(ctx)

	if progress.CurrentTask != "Define Scope" {
		t.Errorf("CurrentTask = %s, want 'Define Scope'", progress.CurrentTask)
	}
}

func TestNavigator_FormatInstructions(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	if instructions == "" {
		t.Fatal("Expected non-empty instructions")
	}

	// Check for expected content
	expectedContents := []string{
		"Festival Planning Guidance",
		"Gather Requirements",
		"Step",
		"Instructions",
		"Deliverables",
		"fest plan next",
	}

	for _, expected := range expectedContents {
		if !contains(instructions, expected) {
			t.Errorf("Instructions missing expected content: %s", expected)
		}
	}
}

func TestNavigator_FormatInstructions_Complete(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Complete all steps
	nav.currentStep = len(nav.steps)

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	if !contains(instructions, "Planning Complete") {
		t.Error("Expected 'Planning Complete' in instructions")
	}

	if !contains(instructions, "execution mode") {
		t.Error("Expected reference to execution mode")
	}
}

func TestNavigator_GetContextFiles(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	files, err := nav.GetContextFiles(ctx)
	if err != nil {
		t.Fatalf("GetContextFiles() error = %v", err)
	}

	if len(files) < 2 {
		t.Errorf("Expected at least 2 context files, got %d", len(files))
	}

	// Should include FESTIVAL_GOAL.md
	festivalGoal := filepath.Join(tmpDir, "FESTIVAL_GOAL.md")
	if !containsString(files, festivalGoal) {
		t.Error("Expected FESTIVAL_GOAL.md in context files")
	}

	// Should include input directory
	inputDir := filepath.Join(tmpDir, "input")
	if !containsString(files, inputDir) {
		t.Error("Expected input/ in context files")
	}
}

func TestNavigator_StatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	// First navigator instance
	nav1, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav1.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Complete first step
	if err := nav1.MarkComplete(ctx, "gather_requirements"); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	// Create new navigator instance (simulates new session)
	nav2, _ := NewNavigator(gctx)
	if err := nav2.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Verify state was persisted
	if nav2.currentStep != 1 {
		t.Errorf("After reload, currentStep = %d, want 1", nav2.currentStep)
	}

	// Get next should return second step
	step, _ := nav2.GetNext(ctx)
	if step.ID != "define_scope" {
		t.Errorf("After reload, next step = %s, want define_scope", step.ID)
	}
}

func TestNavigator_Registration(t *testing.T) {
	// Verify the navigator is registered
	modes := guidance.GetRegisteredModes()

	if !slices.Contains(modes, guidance.ModePlan) {
		t.Error("ModePlan not registered with guidance factory")
	}
}

func TestNavigator_FactoryCreation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create navigator via factory
	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, err := guidance.NewNavigator(context.Background(), gctx)
	if err != nil {
		t.Fatalf("Factory creation error: %v", err)
	}

	if nav == nil {
		t.Fatal("Navigator is nil from factory")
	}

	// Verify it's the right type
	if nav.GetContext().Mode != guidance.ModePlan {
		t.Errorf("Mode = %v, want ModePlan", nav.GetContext().Mode)
	}
}

func TestNavigator_getContextFiles_ReturnsCorrectPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create actual files to verify paths
	festivalGoal := filepath.Join(tmpDir, "FESTIVAL_GOAL.md")
	festivalOverview := filepath.Join(tmpDir, "FESTIVAL_OVERVIEW.md")
	inputDir := filepath.Join(tmpDir, "input")

	if err := os.WriteFile(festivalGoal, []byte("# Goal"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(festivalOverview, []byte("# Overview"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModePlan,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	files := nav.getContextFiles()

	if len(files) != 3 {
		t.Errorf("Expected 3 context files, got %d", len(files))
	}

	if files[0] != festivalGoal {
		t.Errorf("Expected %s, got %s", festivalGoal, files[0])
	}

	if files[1] != festivalOverview {
		t.Errorf("Expected %s, got %s", festivalOverview, files[1])
	}

	if files[2] != inputDir {
		t.Errorf("Expected %s, got %s", inputDir, files[2])
	}
}

// Helper functions

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func containsString(slice []string, s string) bool {
	return slices.Contains(slice, s)
}
