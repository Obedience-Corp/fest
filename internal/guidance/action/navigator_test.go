package action

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
)

// createTestContext creates a test GuidanceContext with temp directories.
func createTestContext(t *testing.T) *guidance.GuidanceContext {
	t.Helper()

	tempDir := t.TempDir()
	festivalPath := filepath.Join(tempDir, "test-festival")
	phasePath := filepath.Join(festivalPath, "001_ACTION_PHASE")

	// Create directories
	if err := os.MkdirAll(phasePath, 0755); err != nil {
		t.Fatalf("failed to create phase directory: %v", err)
	}

	// Create fest.yaml
	festYAML := `name: test-festival
status: active
`
	if err := os.WriteFile(filepath.Join(festivalPath, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatalf("failed to create fest.yaml: %v", err)
	}

	return &guidance.GuidanceContext{
		FestivalPath: festivalPath,
		PhasePath:    phasePath,
	}
}

func TestNavigator_Registration(t *testing.T) {
	// Verify navigator is registered by creating via factory
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Mode:         guidance.ModeAction,
		Config:       guidance.DefaultConfig(),
	}

	nav, err := guidance.NewNavigator(context.Background(), gctx)
	if err != nil {
		t.Fatalf("expected action navigator to be registered: %v", err)
	}
	if nav == nil {
		t.Fatal("expected non-nil navigator from factory")
	}
}

func TestNewNavigator(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if nav == nil {
		t.Fatal("expected non-nil navigator")
	}

	if nav.BaseNavigator == nil {
		t.Fatal("expected BaseNavigator to be set")
	}
}

func TestNavigator_Initialize(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error creating navigator: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	// Initially no actions (loadActions returns nil)
	actions := nav.GetActions()
	if len(actions) != 0 {
		t.Errorf("expected 0 actions initially, got %d", len(actions))
	}
}

func TestNavigator_Initialize_ContextCancellation(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = nav.Initialize(ctx)
	if err == nil {
		t.Fatal("expected error due to cancelled context")
	}
}

func TestNavigator_GetNext_NoActions(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if step != nil {
		t.Error("expected nil step when no actions")
	}
}

func TestNavigator_GetNext_WithActions(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add an action manually
	nav.AddAction(&ActionItem{
		ID:           "deploy_app",
		Title:        "Deploy Application",
		Description:  "Deploy the application to production",
		Commands:     []string{"docker push app:latest", "kubectl apply -f deploy.yaml"},
		Verification: []string{"curl http://localhost:8080/health"},
		Rollback:     []string{"kubectl rollout undo deployment/app"},
	})

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if step == nil {
		t.Fatal("expected non-nil step")
	}

	if step.Mode != guidance.ModeAction {
		t.Errorf("expected mode %s, got %s", guidance.ModeAction, step.Mode)
	}

	if step.StepType != guidance.StepTypeActionStep {
		t.Errorf("expected step type %s, got %s", guidance.StepTypeActionStep, step.StepType)
	}

	if step.ID != "deploy_app" {
		t.Errorf("expected ID deploy_app, got %s", step.ID)
	}
}

func TestNavigator_GetNext_RequiresHuman(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add action that requires human
	nav.AddAction(&ActionItem{
		ID:            "manual_approval",
		Title:         "Manual Approval Required",
		Description:   "Requires human sign-off",
		RequiresHuman: true,
	})

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if step.AutonomyLevel != guidance.AutonomyLow {
		t.Errorf("expected autonomy %s for RequiresHuman action, got %s", guidance.AutonomyLow, step.AutonomyLevel)
	}
}

func TestNavigator_GetNext_ContextCancellation(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{ID: "test_action", Title: "Test"})

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = nav.GetNext(cancelledCtx)
	if err == nil {
		t.Fatal("expected error due to cancelled context")
	}
}

func TestNavigator_MarkComplete(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{
		ID:    "action_1",
		Title: "Action 1",
	})

	err = nav.MarkComplete(ctx, "action_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions := nav.GetActions()
	if actions[0].Status != guidance.StatusCompleted {
		t.Errorf("expected status %s, got %s", guidance.StatusCompleted, actions[0].Status)
	}
}

func TestNavigator_MarkComplete_NotFound(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = nav.MarkComplete(ctx, "nonexistent_action")
	if err == nil {
		t.Fatal("expected error for nonexistent action")
	}
}

func TestNavigator_MarkSkipped(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{
		ID:    "skip_action",
		Title: "Skip This",
	})

	err = nav.MarkSkipped(ctx, "skip_action")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions := nav.GetActions()
	if actions[0].Status != guidance.StatusSkipped {
		t.Errorf("expected status %s, got %s", guidance.StatusSkipped, actions[0].Status)
	}
}

func TestNavigator_MarkFailed(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{
		ID:       "fail_action",
		Title:    "Failing Action",
		Rollback: []string{"rollback step 1", "rollback step 2"},
	})

	err = nav.MarkFailed(ctx, "fail_action")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions := nav.GetActions()
	if actions[0].Status != guidance.StatusFailed {
		t.Errorf("expected status %s, got %s", guidance.StatusFailed, actions[0].Status)
	}
}

func TestNavigator_Advance(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{ID: "action_advance", Title: "Advance Action"})

	err = nav.Advance(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be complete now
	step, _ := nav.GetNext(ctx)
	if step != nil {
		t.Error("expected nil step after advance")
	}
}

func TestNavigator_Advance_WhenComplete(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = nav.Advance(ctx)
	if err != guidance.ErrAlreadyComplete {
		t.Errorf("expected ErrAlreadyComplete, got %v", err)
	}
}

func TestNavigator_GetProgress(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{ID: "a1", Title: "A1"})
	nav.AddAction(&ActionItem{ID: "a2", Title: "A2"})
	nav.AddAction(&ActionItem{ID: "a3", Title: "A3"})

	_ = nav.MarkComplete(ctx, "a1")

	progress, err := nav.GetProgress(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if progress.Total != 3 {
		t.Errorf("expected total 3, got %d", progress.Total)
	}

	if progress.Completed != 1 {
		t.Errorf("expected completed 1, got %d", progress.Completed)
	}

	if progress.Pending != 2 {
		t.Errorf("expected pending 2, got %d", progress.Pending)
	}
}

func TestNavigator_GetProgress_MixedStatuses(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{ID: "a1", Title: "A1", Status: guidance.StatusCompleted})
	nav.AddAction(&ActionItem{ID: "a2", Title: "A2", Status: guidance.StatusFailed})
	nav.AddAction(&ActionItem{ID: "a3", Title: "A3", Status: guidance.StatusSkipped})
	nav.AddAction(&ActionItem{ID: "a4", Title: "A4"}) // pending

	progress, _ := nav.GetProgress(ctx)

	if progress.Completed != 1 {
		t.Errorf("expected completed 1, got %d", progress.Completed)
	}
	if progress.Failed != 1 {
		t.Errorf("expected failed 1, got %d", progress.Failed)
	}
	if progress.Skipped != 1 {
		t.Errorf("expected skipped 1, got %d", progress.Skipped)
	}
	if progress.Pending != 1 {
		t.Errorf("expected pending 1, got %d", progress.Pending)
	}
}

func TestNavigator_FormatInstructions(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{
		ID:           "test_action",
		Title:        "Test Action",
		Description:  "Testing action format",
		Commands:     []string{"echo hello", "echo world"},
		Verification: []string{"check output"},
		Rollback:     []string{"revert changes"},
	})

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if instructions == "" {
		t.Error("expected non-empty instructions")
	}

	// Check for expected content
	expectedSubstrings := []string{
		"Action Phase Guidance",
		"Progress",
		"Current Action",
		"Commands",
		"Verification",
		"Rollback",
	}

	for _, substr := range expectedSubstrings {
		if !containsString(instructions, substr) {
			t.Errorf("expected instructions to contain %q", substr)
		}
	}
}

func TestNavigator_FormatInstructions_NoActions(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsString(instructions, "No action items discovered") {
		t.Error("expected no actions message")
	}
}

func TestNavigator_FormatInstructions_Complete(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{ID: "done_action", Title: "Done"})
	_ = nav.Advance(ctx)

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsString(instructions, "Actions Complete") {
		t.Error("expected completion message")
	}
}

func TestNavigator_FormatInstructions_WithFailures(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{
		ID:       "failed_action",
		Title:    "Failed Action",
		Status:   guidance.StatusFailed,
		Rollback: []string{"rollback step"},
	})

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsString(instructions, "WARNING") || !containsString(instructions, "failed") {
		t.Error("expected failure warning in output")
	}
}

func TestNavigator_GetContextFiles(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files, err := nav.GetContextFiles(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) == 0 {
		t.Error("expected at least one context file")
	}

	// Should include FESTIVAL_GOAL.md
	foundFestivalGoal := false
	for _, f := range files {
		if containsString(f, "FESTIVAL_GOAL.md") {
			foundFestivalGoal = true
			break
		}
	}

	if !foundFestivalGoal {
		t.Error("expected FESTIVAL_GOAL.md in context files")
	}
}

func TestNavigator_GetRollbackSteps(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{
		ID:       "deploy_action",
		Title:    "Deploy",
		Rollback: []string{"kubectl rollout undo", "docker pull previous"},
	})

	rollback := nav.GetRollbackSteps("deploy_action")
	if len(rollback) != 2 {
		t.Errorf("expected 2 rollback steps, got %d", len(rollback))
	}

	if rollback[0] != "kubectl rollout undo" {
		t.Errorf("unexpected rollback step: %s", rollback[0])
	}
}

func TestNavigator_GetRollbackSteps_NotFound(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rollback := nav.GetRollbackSteps("nonexistent")
	if rollback != nil {
		t.Error("expected nil rollback for nonexistent action")
	}
}

func TestNavigator_HasFailedActions(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Initially no failed actions
	if nav.HasFailedActions() {
		t.Error("expected no failed actions initially")
	}

	nav.AddAction(&ActionItem{ID: "a1", Title: "A1"})
	_ = nav.MarkFailed(ctx, "a1")

	if !nav.HasFailedActions() {
		t.Error("expected HasFailedActions to return true")
	}
}

func TestNavigator_AddAction(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := &ActionItem{
		ID:    "custom_action",
		Title: "Custom Action",
	}

	nav.AddAction(action)

	actions := nav.GetActions()
	if len(actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(actions))
	}
}

func TestNavigator_StepMetadata(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nav.AddAction(&ActionItem{
		ID:           "meta_action",
		Title:        "Meta Action",
		Commands:     []string{"cmd1", "cmd2"},
		Verification: []string{"verify1"},
		Rollback:     []string{"rollback1"},
	})

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check metadata
	if step.Metadata["action_number"] != 1 {
		t.Errorf("expected action_number 1, got %v", step.Metadata["action_number"])
	}

	if step.Metadata["total_actions"] != 1 {
		t.Errorf("expected total_actions 1, got %v", step.Metadata["total_actions"])
	}

	commands, ok := step.Metadata["commands"].([]string)
	if !ok || len(commands) != 2 {
		t.Error("expected commands in metadata")
	}

	verification, ok := step.Metadata["verification"].([]string)
	if !ok || len(verification) != 1 {
		t.Error("expected verification in metadata")
	}

	rollback, ok := step.Metadata["rollback"].([]string)
	if !ok || len(rollback) != 1 {
		t.Error("expected rollback in metadata")
	}
}

// Helper function for string contains check
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
