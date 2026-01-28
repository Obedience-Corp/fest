package ingest

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
)

func TestNavigatorRegistration(t *testing.T) {
	modes := guidance.GetRegisteredModes()
	if !slices.Contains(modes, guidance.ModeIngest) {
		t.Error("ModeIngest not registered with guidance factory")
	}
}

func TestNavigatorFactory_CreateNavigator(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, err := guidance.NewNavigator(context.Background(), gctx)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if nav == nil {
		t.Fatal("NewNavigator() returned nil")
	}

	if nav.GetContext().Mode != guidance.ModeIngest {
		t.Errorf("Navigator mode = %v, want %v", nav.GetContext().Mode, guidance.ModeIngest)
	}
}

func TestDefaultIngestSteps(t *testing.T) {
	steps := DefaultIngestSteps()

	if len(steps) != 5 {
		t.Errorf("DefaultIngestSteps() returned %d steps, want 5", len(steps))
	}

	// Verify step IDs
	expectedIDs := []string{
		"distill_intent",
		"identify_constraints",
		"define_scope",
		"surface_unknowns",
		"ingest_gate",
	}

	for i, expected := range expectedIDs {
		if steps[i].ID != expected {
			t.Errorf("Step %d ID = %q, want %q", i, steps[i].ID, expected)
		}
	}

	// Verify order fields
	for i, step := range steps {
		if step.Order != i+1 {
			t.Errorf("Step %d Order = %d, want %d", i, step.Order, i+1)
		}
	}

	// Verify last step has AutonomyLow
	if steps[4].Autonomy != guidance.AutonomyLow {
		t.Errorf("Gate step Autonomy = %v, want AutonomyLow", steps[4].Autonomy)
	}

	// Verify checklists exist
	for i, step := range steps {
		if len(step.Checklist) == 0 {
			t.Errorf("Step %d (%s) has no checklist items", i, step.ID)
		}
	}

	// Verify output files
	if steps[0].OutputFile != "intent_packet.md" {
		t.Errorf("Step 0 OutputFile = %q, want intent_packet.md", steps[0].OutputFile)
	}
	if steps[3].OutputFile != "risks_and_unknowns.md" {
		t.Errorf("Step 3 OutputFile = %q, want risks_and_unknowns.md", steps[3].OutputFile)
	}
	if steps[4].OutputFile != "" {
		t.Errorf("Gate step OutputFile = %q, want empty", steps[4].OutputFile)
	}
}

func TestNewNavigator(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if nav == nil {
		t.Fatal("NewNavigator() returned nil")
	}

	if nav.loopState == nil {
		t.Fatal("Navigator loopState is nil")
	}

	if nav.loopState.IterationCount != 1 {
		t.Errorf("Initial IterationCount = %d, want 1", nav.loopState.IterationCount)
	}

	if nav.loopState.MaxIterations != 3 {
		t.Errorf("MaxIterations = %d, want 3", nav.loopState.MaxIterations)
	}

	if nav.loopState.Approved {
		t.Error("Initial Approved = true, want false")
	}

	if len(nav.steps) != 5 {
		t.Errorf("Navigator has %d steps, want 5", len(nav.steps))
	}
}

func TestNavigator_Initialize(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
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

	// Should handle nil context by using context.Background internally
	nav2, _ := NewNavigator(gctx)
	if err := nav2.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
}

func TestNavigator_GetNext(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step == nil {
		t.Fatal("GetNext() returned nil")
	}

	if step.ID != "distill_intent" {
		t.Errorf("First step ID = %q, want distill_intent", step.ID)
	}

	if step.Mode != guidance.ModeIngest {
		t.Errorf("Step mode = %v, want ModeIngest", step.Mode)
	}

	if step.StepType != guidance.StepTypeIngestStep {
		t.Errorf("StepType = %v, want StepTypeIngestStep", step.StepType)
	}
}

func TestNavigator_GetNext_ReturnsNilWhenApproved(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Set approved
	nav.loopState.Approved = true

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step != nil {
		t.Error("GetNext() should return nil when approved")
	}
}

func TestNavigator_GetNext_ReturnsApprovalStepAtGate(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Move to gate step
	nav.currentStep = 4 // ingest_gate index

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step == nil {
		t.Fatal("GetNext() returned nil at gate step")
	}

	if step.StepType != guidance.StepTypeApproval {
		t.Errorf("StepType = %v, want StepTypeApproval", step.StepType)
	}

	if step.ID != "ingest_approval" {
		t.Errorf("Approval step ID = %q, want ingest_approval", step.ID)
	}
}

func TestNavigator_MarkComplete(t *testing.T) {
	tmpDir := t.TempDir()
	// Create .fest directory
	festDir := filepath.Join(tmpDir, ".fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Complete first step
	if err := nav.MarkComplete(ctx, "distill_intent"); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	if nav.currentStep != 1 {
		t.Errorf("currentStep = %d, want 1", nav.currentStep)
	}
}

func TestNavigator_MarkComplete_OutOfOrder(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Try to complete second step first
	err := nav.MarkComplete(ctx, "identify_constraints")
	if err == nil {
		t.Error("MarkComplete() should fail for out of order step")
	}
}

func TestNavigator_MarkComplete_IngestApproval(t *testing.T) {
	tmpDir := t.TempDir()
	festDir := filepath.Join(tmpDir, ".fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Move to gate step and set pending approval
	nav.currentStep = 4
	nav.loopState.PendingApproval = true

	// MarkComplete with "ingest_approval" should call Approve
	if err := nav.MarkComplete(ctx, "ingest_approval"); err != nil {
		t.Fatalf("MarkComplete(ingest_approval) error = %v", err)
	}

	if !nav.loopState.Approved {
		t.Error("Approved should be true after MarkComplete(ingest_approval)")
	}
}

func TestNavigator_MarkSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	festDir := filepath.Join(tmpDir, ".fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if err := nav.MarkSkipped(ctx, "distill_intent"); err != nil {
		t.Fatalf("MarkSkipped() error = %v", err)
	}

	if nav.currentStep != 1 {
		t.Errorf("currentStep = %d, want 1", nav.currentStep)
	}
}

func TestNavigator_MarkFailed(t *testing.T) {
	tmpDir := t.TempDir()
	festDir := filepath.Join(tmpDir, ".fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if err := nav.MarkFailed(ctx, "distill_intent"); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	status := nav.StateManager.GetTaskStatus("distill_intent")
	if status != guidance.StatusFailed {
		t.Errorf("Task status = %q, want %q", status, guidance.StatusFailed)
	}
}

func TestNavigator_Advance(t *testing.T) {
	tmpDir := t.TempDir()
	festDir := filepath.Join(tmpDir, ".fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("Advance() error = %v", err)
	}

	if nav.currentStep != 1 {
		t.Errorf("currentStep = %d, want 1", nav.currentStep)
	}
}

func TestNavigator_Advance_WhenComplete(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Set as approved
	nav.loopState.Approved = true

	err := nav.Advance(ctx)
	if err != guidance.ErrAlreadyComplete {
		t.Errorf("Advance() error = %v, want ErrAlreadyComplete", err)
	}
}

func TestNavigator_Approve(t *testing.T) {
	tmpDir := t.TempDir()
	festDir := filepath.Join(tmpDir, ".fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Move to gate step
	nav.currentStep = 4
	nav.loopState.PendingApproval = true

	if err := nav.Approve(ctx); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if !nav.loopState.Approved {
		t.Error("Approved should be true")
	}

	if nav.loopState.PendingApproval {
		t.Error("PendingApproval should be false")
	}

	if !nav.loopState.LastGatePassed {
		t.Error("LastGatePassed should be true")
	}
}

func TestNavigator_Approve_NotAtGate(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// At first step, not gate
	err := nav.Approve(ctx)
	if err == nil {
		t.Error("Approve() should fail when not at gate step")
	}
}

func TestNavigator_Reject(t *testing.T) {
	tmpDir := t.TempDir()
	festDir := filepath.Join(tmpDir, ".fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Move to gate step
	nav.currentStep = 4
	nav.loopState.PendingApproval = true

	if err := nav.Reject(ctx, "needs more detail"); err != nil {
		t.Fatalf("Reject() error = %v", err)
	}

	if nav.loopState.IterationCount != 2 {
		t.Errorf("IterationCount = %d, want 2", nav.loopState.IterationCount)
	}

	if nav.loopState.CurrentStep != 0 {
		t.Errorf("CurrentStep = %d, want 0", nav.loopState.CurrentStep)
	}

	if nav.currentStep != 0 {
		t.Errorf("Navigator currentStep = %d, want 0", nav.currentStep)
	}

	if nav.loopState.PendingApproval {
		t.Error("PendingApproval should be false")
	}

	if nav.loopState.RejectReason != "needs more detail" {
		t.Errorf("RejectReason = %q, want 'needs more detail'", nav.loopState.RejectReason)
	}
}

func TestNavigator_Reject_NotAtGate(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// At first step, not gate
	err := nav.Reject(ctx, "test")
	if err == nil {
		t.Error("Reject() should fail when not at gate step")
	}
}

func TestNavigator_Reject_MaxIterations(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Move to gate and set at max iterations
	nav.currentStep = 4
	nav.loopState.PendingApproval = true
	nav.loopState.IterationCount = 3 // MaxIterations is 3

	err := nav.Reject(ctx, "test")
	if err == nil {
		t.Error("Reject() should fail at max iterations")
	}
}

func TestNavigator_GetLoopState(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)

	ls := nav.GetLoopState()
	if ls == nil {
		t.Fatal("GetLoopState() returned nil")
	}

	if ls != nav.loopState {
		t.Error("GetLoopState() should return the same LoopState pointer")
	}
}

func TestNavigator_GetProgress(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	progress, err := nav.GetProgress(ctx)
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}

	if progress == nil {
		t.Fatal("GetProgress() returned nil")
	}

	if progress.Mode != guidance.ModeIngest {
		t.Errorf("Progress mode = %v, want ModeIngest", progress.Mode)
	}

	if progress.Total != 5 {
		t.Errorf("Progress total = %d, want 5", progress.Total)
	}

	if progress.Completed != 0 {
		t.Errorf("Progress completed = %d, want 0", progress.Completed)
	}
}

func TestNavigator_FormatInstructions(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
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
		t.Error("FormatInstructions() returned empty string")
	}

	// Should contain step info
	if !contains(instructions, "Distill Core Intent") {
		t.Error("Instructions should contain step title")
	}
}

func TestNavigator_FormatInstructions_WhenComplete(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Set as complete
	nav.loopState.Approved = true

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("FormatInstructions() error = %v", err)
	}

	if !contains(instructions, "Ingest Complete") {
		t.Error("Should show completion message when approved")
	}
}

func TestNavigator_GetContextFiles(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
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

	if len(files) == 0 {
		t.Error("GetContextFiles() returned empty slice")
	}
}

func TestNavigator_FullWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	festDir := filepath.Join(tmpDir, ".fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Complete all steps except gate
	steps := []string{
		"distill_intent",
		"identify_constraints",
		"define_scope",
		"surface_unknowns",
	}

	for _, stepID := range steps {
		if err := nav.MarkComplete(ctx, stepID); err != nil {
			t.Fatalf("MarkComplete(%s) error = %v", stepID, err)
		}
	}

	// Should be at gate step
	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("GetNext() error = %v", err)
	}

	if step == nil {
		t.Fatal("GetNext() returned nil at gate")
	}

	if step.StepType != guidance.StepTypeApproval {
		t.Errorf("StepType = %v, want StepTypeApproval", step.StepType)
	}

	// Approve
	if err := nav.Approve(ctx); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	// Should be complete
	step, _ = nav.GetNext(ctx)
	if step != nil {
		t.Error("GetNext() should return nil after approval")
	}
}

func TestNavigator_LoopWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	festDir := filepath.Join(tmpDir, ".fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Complete all steps
	steps := []string{
		"distill_intent",
		"identify_constraints",
		"define_scope",
		"surface_unknowns",
	}

	for _, stepID := range steps {
		if err := nav.MarkComplete(ctx, stepID); err != nil {
			t.Fatalf("MarkComplete(%s) error = %v", stepID, err)
		}
	}

	// Get approval step
	step, _ := nav.GetNext(ctx)
	if step.StepType != guidance.StepTypeApproval {
		t.Fatal("Should be at approval step")
	}

	// Reject to start new iteration
	if err := nav.Reject(ctx, "needs improvement"); err != nil {
		t.Fatalf("Reject() error = %v", err)
	}

	// Should be at iteration 2, step 0
	if nav.loopState.IterationCount != 2 {
		t.Errorf("IterationCount = %d, want 2", nav.loopState.IterationCount)
	}

	// Should be back at first step
	step, _ = nav.GetNext(ctx)
	if step.ID != "distill_intent" {
		t.Errorf("After reject, step ID = %q, want distill_intent", step.ID)
	}
}

func TestNavigator_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeIngest,
		Config:       guidance.DefaultConfig(),
	}

	nav, _ := NewNavigator(gctx)
	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should fail with cancelled context
	_, err := nav.GetNext(ctx)
	if err == nil {
		t.Error("GetNext() should fail with cancelled context")
	}

	_, err = nav.GetProgress(ctx)
	if err == nil {
		t.Error("GetProgress() should fail with cancelled context")
	}

	_, err = nav.FormatInstructions(ctx)
	if err == nil {
		t.Error("FormatInstructions() should fail with cancelled context")
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
