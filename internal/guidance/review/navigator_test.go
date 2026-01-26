package review

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
	phasePath := filepath.Join(festivalPath, "001_REVIEW_PHASE")

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
		Mode:         guidance.ModeReview,
		Config:       guidance.DefaultConfig(),
	}

	nav, err := guidance.NewNavigator(context.Background(), gctx)
	if err != nil {
		t.Fatalf("expected review navigator to be registered: %v", err)
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

	checklist := nav.GetChecklist()
	if len(checklist) != 7 {
		t.Errorf("expected 7 checklist items, got %d", len(checklist))
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

func TestNavigator_DefaultChecklist(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checklist := nav.GetChecklist()

	// Verify expected items exist
	expectedItems := map[string]string{
		"context_propagation": "blocker",
		"error_handling":      "blocker",
		"file_size":           "major",
		"interfaces":          "major",
		"dependencies":        "major",
		"test_coverage":       "major",
		"magic_values":        "minor",
	}

	for _, item := range checklist {
		expectedSeverity, found := expectedItems[item.ID]
		if !found {
			t.Errorf("unexpected checklist item: %s", item.ID)
			continue
		}
		if item.Severity != expectedSeverity {
			t.Errorf("item %s: expected severity %s, got %s", item.ID, expectedSeverity, item.Severity)
		}
	}
}

func TestNavigator_GetNext(t *testing.T) {
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

	if step == nil {
		t.Fatal("expected non-nil step")
	}

	if step.Mode != guidance.ModeReview {
		t.Errorf("expected mode %s, got %s", guidance.ModeReview, step.Mode)
	}

	if step.StepType != guidance.StepTypeReviewStep {
		t.Errorf("expected step type %s, got %s", guidance.StepTypeReviewStep, step.StepType)
	}

	if step.AutonomyLevel != guidance.AutonomyLow {
		t.Errorf("expected autonomy %s, got %s (review requires human judgment)", guidance.AutonomyLow, step.AutonomyLevel)
	}

	// First item should be context_propagation
	if step.ID != "context_propagation" {
		t.Errorf("expected first item to be context_propagation, got %s", step.ID)
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

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = nav.GetNext(cancelledCtx)
	if err == nil {
		t.Fatal("expected error due to cancelled context")
	}
}

func TestNavigator_GetNext_AllReviewed(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mark all items as passed
	for _, item := range nav.GetChecklist() {
		_ = nav.MarkComplete(ctx, item.ID)
	}

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if step != nil {
		t.Error("expected nil step when all items reviewed")
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

	err = nav.MarkComplete(ctx, "context_propagation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checklist := nav.GetChecklist()
	if checklist[0].Status != "passed" {
		t.Errorf("expected status passed, got %s", checklist[0].Status)
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

	err = nav.MarkComplete(ctx, "nonexistent_item")
	if err == nil {
		t.Fatal("expected error for nonexistent item")
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

	err = nav.MarkSkipped(ctx, "magic_values")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the item and check status
	for _, item := range nav.GetChecklist() {
		if item.ID == "magic_values" {
			if item.Status != "skipped" {
				t.Errorf("expected status skipped, got %s", item.Status)
			}
			return
		}
	}
	t.Error("magic_values item not found")
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

	err = nav.MarkFailed(ctx, "error_handling")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the item and check status
	for _, item := range nav.GetChecklist() {
		if item.ID == "error_handling" {
			if item.Status != "failed" {
				t.Errorf("expected status failed, got %s", item.Status)
			}
			return
		}
	}
	t.Error("error_handling item not found")
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

	err = nav.Advance(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First item should be passed now
	checklist := nav.GetChecklist()
	if checklist[0].Status != "passed" {
		t.Errorf("expected first item to be passed, got %s", checklist[0].Status)
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

	// Mark all items as passed
	for _, item := range nav.GetChecklist() {
		_ = nav.MarkComplete(ctx, item.ID)
	}

	err = nav.Advance(ctx)
	if err != guidance.ErrAlreadyComplete {
		t.Errorf("expected ErrAlreadyComplete, got %v", err)
	}
}

func TestNavigator_HasBlockers(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Initially no blockers
	if nav.HasBlockers() {
		t.Error("expected no blockers initially")
	}

	// Fail a blocker item
	err = nav.MarkFailed(ctx, "context_propagation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !nav.HasBlockers() {
		t.Error("expected HasBlockers to return true after failing blocker item")
	}
}

func TestNavigator_HasBlockers_FailedNonBlocker(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fail a non-blocker item (minor severity)
	err = nav.MarkFailed(ctx, "magic_values")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if nav.HasBlockers() {
		t.Error("expected no blockers when only minor item failed")
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

	progress, err := nav.GetProgress(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if progress.Total != 7 {
		t.Errorf("expected total 7, got %d", progress.Total)
	}

	if progress.Pending != 7 {
		t.Errorf("expected pending 7, got %d", progress.Pending)
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

	// Mark some items with different statuses
	_ = nav.MarkComplete(ctx, "context_propagation")
	_ = nav.MarkComplete(ctx, "error_handling")
	_ = nav.MarkFailed(ctx, "file_size")
	_ = nav.MarkSkipped(ctx, "magic_values")

	progress, err := nav.GetProgress(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if progress.Completed != 3 { // 2 passed + 1 skipped
		t.Errorf("expected completed 3, got %d", progress.Completed)
	}

	if progress.Failed != 1 {
		t.Errorf("expected failed 1, got %d", progress.Failed)
	}

	if progress.Pending != 3 {
		t.Errorf("expected pending 3, got %d", progress.Pending)
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

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if instructions == "" {
		t.Error("expected non-empty instructions")
	}

	// Check for expected content
	expectedSubstrings := []string{
		"Review Phase Guidance",
		"Progress",
		"Current Checklist Item",
		"blocker", // severity
	}

	for _, substr := range expectedSubstrings {
		if !containsString(instructions, substr) {
			t.Errorf("expected instructions to contain %q", substr)
		}
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

	// Mark all as passed
	for _, item := range nav.GetChecklist() {
		_ = nav.MarkComplete(ctx, item.ID)
	}

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsString(instructions, "Review Complete") {
		t.Error("expected completion message")
	}
}

func TestNavigator_FormatInstructions_WithBlockers(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fail a blocker, pass the rest
	_ = nav.MarkFailed(ctx, "context_propagation")
	for _, item := range nav.GetChecklist() {
		if item.ID != "context_propagation" {
			_ = nav.MarkComplete(ctx, item.ID)
		}
	}

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsString(instructions, "BLOCKERS DETECTED") {
		t.Error("expected blockers warning in output")
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
}

func TestNavigator_AddNote(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = nav.AddNote("context_propagation", "Verified context is passed throughout")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checklist := nav.GetChecklist()
	if checklist[0].Notes != "Verified context is passed throughout" {
		t.Errorf("expected note to be set, got %s", checklist[0].Notes)
	}
}

func TestNavigator_AddNote_NotFound(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = nav.AddNote("nonexistent_item", "some note")
	if err == nil {
		t.Fatal("expected error for nonexistent item")
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

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check metadata
	if step.Metadata["severity"] != "blocker" {
		t.Errorf("expected severity blocker, got %v", step.Metadata["severity"])
	}

	if step.Metadata["item_number"] != 1 {
		t.Errorf("expected item_number 1, got %v", step.Metadata["item_number"])
	}

	if step.Metadata["total_items"] != 7 {
		t.Errorf("expected total_items 7, got %v", step.Metadata["total_items"])
	}

	// Check commands are present
	if _, ok := step.Metadata["fail_command"]; !ok {
		t.Error("expected fail_command in metadata")
	}

	if _, ok := step.Metadata["skip_command"]; !ok {
		t.Error("expected skip_command in metadata")
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
