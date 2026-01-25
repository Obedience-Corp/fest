package research

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
	phasePath := filepath.Join(festivalPath, "001_RESEARCH_PHASE")

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

// createTopicDirectory creates a topic directory with a marker file.
func createTopicDirectory(t *testing.T, phasePath, topicName string) string {
	t.Helper()

	topicPath := filepath.Join(phasePath, topicName)
	if err := os.MkdirAll(topicPath, 0755); err != nil {
		t.Fatalf("failed to create topic directory: %v", err)
	}

	// Create topic marker
	marker := "# Topic: " + topicName + "\n\nResearch topic description."
	if err := os.WriteFile(filepath.Join(topicPath, "TOPIC.md"), []byte(marker), 0644); err != nil {
		t.Fatalf("failed to create topic marker: %v", err)
	}

	return topicPath
}

func TestNavigator_Registration(t *testing.T) {
	// Verify navigator is registered by creating via factory
	gctx := &guidance.GuidanceContext{
		FestivalPath: t.TempDir(),
		Mode:         guidance.ModeResearch,
		Config:       guidance.DefaultConfig(),
	}

	nav, err := guidance.NewNavigator(context.Background(), gctx)
	if err != nil {
		t.Fatalf("expected research navigator to be registered: %v", err)
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

	// Create some topics
	createTopicDirectory(t, gctx.PhasePath, "topic_a")
	createTopicDirectory(t, gctx.PhasePath, "topic_b")

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error creating navigator: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	topics := nav.GetTopics()
	if len(topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(topics))
	}
}

func TestNavigator_Initialize_NoTopics(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	topics := nav.GetTopics()
	if len(topics) != 0 {
		t.Errorf("expected 0 topics, got %d", len(topics))
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

func TestNavigator_GetNext(t *testing.T) {
	gctx := createTestContext(t)

	createTopicDirectory(t, gctx.PhasePath, "api_research")
	createTopicDirectory(t, gctx.PhasePath, "database_research")

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	step, err := nav.GetNext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if step == nil {
		t.Fatal("expected non-nil step")
	}

	if step.Mode != guidance.ModeResearch {
		t.Errorf("expected mode %s, got %s", guidance.ModeResearch, step.Mode)
	}

	if step.StepType != guidance.StepTypeResearchStep {
		t.Errorf("expected step type %s, got %s", guidance.StepTypeResearchStep, step.StepType)
	}

	if step.AutonomyLevel != guidance.AutonomyMedium {
		t.Errorf("expected autonomy %s, got %s", guidance.AutonomyMedium, step.AutonomyLevel)
	}
}

func TestNavigator_GetNext_NoTopics(t *testing.T) {
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
		t.Fatal("expected nil step when no topics")
	}
}

func TestNavigator_GetNext_ContextCancellation(t *testing.T) {
	gctx := createTestContext(t)
	createTopicDirectory(t, gctx.PhasePath, "topic")

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = nav.GetNext(cancelledCtx)
	if err == nil {
		t.Fatal("expected error due to cancelled context")
	}
}

func TestNavigator_MarkComplete(t *testing.T) {
	gctx := createTestContext(t)
	createTopicDirectory(t, gctx.PhasePath, "topic_one")

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	step, _ := nav.GetNext(ctx)
	if step == nil {
		t.Fatal("expected step")
	}

	err = nav.MarkComplete(ctx, step.ID)
	if err != nil {
		t.Fatalf("unexpected error marking complete: %v", err)
	}

	// Get next should return nil now
	nextStep, _ := nav.GetNext(ctx)
	if nextStep != nil {
		t.Error("expected nil step after marking complete")
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
		t.Fatalf("unexpected error initializing: %v", err)
	}

	err = nav.MarkComplete(ctx, "nonexistent_topic")
	if err == nil {
		t.Fatal("expected error for nonexistent topic")
	}
}

func TestNavigator_MarkSkipped(t *testing.T) {
	gctx := createTestContext(t)
	createTopicDirectory(t, gctx.PhasePath, "topic_skip")

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	step, _ := nav.GetNext(ctx)
	if step == nil {
		t.Fatal("expected step")
	}

	err = nav.MarkSkipped(ctx, step.ID)
	if err != nil {
		t.Fatalf("unexpected error marking skipped: %v", err)
	}

	topics := nav.GetTopics()
	if topics[0].Status != "skipped" {
		t.Errorf("expected status skipped, got %s", topics[0].Status)
	}
}

func TestNavigator_MarkFailed(t *testing.T) {
	gctx := createTestContext(t)
	createTopicDirectory(t, gctx.PhasePath, "topic_fail")

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	step, _ := nav.GetNext(ctx)
	if step == nil {
		t.Fatal("expected step")
	}

	err = nav.MarkFailed(ctx, step.ID)
	if err != nil {
		t.Fatalf("unexpected error marking failed: %v", err)
	}

	topics := nav.GetTopics()
	if topics[0].Status != "failed" {
		t.Errorf("expected status failed, got %s", topics[0].Status)
	}
}

func TestNavigator_Advance(t *testing.T) {
	gctx := createTestContext(t)
	createTopicDirectory(t, gctx.PhasePath, "topic_advance")

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	err = nav.Advance(ctx)
	if err != nil {
		t.Fatalf("unexpected error advancing: %v", err)
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
		t.Fatalf("unexpected error initializing: %v", err)
	}

	err = nav.Advance(ctx)
	if err != guidance.ErrAlreadyComplete {
		t.Errorf("expected ErrAlreadyComplete, got %v", err)
	}
}

func TestNavigator_GetProgress(t *testing.T) {
	gctx := createTestContext(t)
	createTopicDirectory(t, gctx.PhasePath, "topic_1")
	createTopicDirectory(t, gctx.PhasePath, "topic_2")
	createTopicDirectory(t, gctx.PhasePath, "topic_3")

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	// Mark first as complete
	step, _ := nav.GetNext(ctx)
	_ = nav.MarkComplete(ctx, step.ID)

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
	createTopicDirectory(t, gctx.PhasePath, "topic_a")
	createTopicDirectory(t, gctx.PhasePath, "topic_b")
	createTopicDirectory(t, gctx.PhasePath, "topic_c")

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	topics := nav.GetTopics()
	topics[0].Status = "documented"
	topics[1].Status = "skipped"
	topics[2].Status = "failed"

	progress, _ := nav.GetProgress(ctx)

	if progress.Completed != 1 {
		t.Errorf("expected completed 1, got %d", progress.Completed)
	}
	if progress.Skipped != 1 {
		t.Errorf("expected skipped 1, got %d", progress.Skipped)
	}
	if progress.Failed != 1 {
		t.Errorf("expected failed 1, got %d", progress.Failed)
	}
}

func TestNavigator_FormatInstructions(t *testing.T) {
	gctx := createTestContext(t)
	createTopicDirectory(t, gctx.PhasePath, "test_topic")

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
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
		"Research Phase Guidance",
		"Progress",
		"Current Topic",
	}

	for _, substr := range expectedSubstrings {
		if !containsString(instructions, substr) {
			t.Errorf("expected instructions to contain %q", substr)
		}
	}
}

func TestNavigator_FormatInstructions_NoTopics(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsString(instructions, "No research topics discovered") {
		t.Error("expected no topics message")
	}
}

func TestNavigator_FormatInstructions_Complete(t *testing.T) {
	gctx := createTestContext(t)
	createTopicDirectory(t, gctx.PhasePath, "done_topic")

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	// Mark all complete
	_ = nav.Advance(ctx)

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsString(instructions, "Research Complete") {
		t.Error("expected completion message")
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
		t.Fatalf("unexpected error initializing: %v", err)
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

func TestNavigator_AddTopic(t *testing.T) {
	gctx := createTestContext(t)

	nav, err := NewNavigator(gctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("unexpected error initializing: %v", err)
	}

	topic := &Topic{
		ID:     "custom_topic",
		Name:   "Custom Topic",
		Path:   "/path/to/topic",
		Status: "pending",
	}

	nav.AddTopic(topic)

	topics := nav.GetTopics()
	if len(topics) != 1 {
		t.Errorf("expected 1 topic, got %d", len(topics))
	}
}

func TestFormatTopicName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"api_research", "Api Research"},
		{"database-design", "Database Design"},
		{"simple", "Simple"},
		{"UPPERCASE", "UPPERCASE"},
		{"multiple_words_here", "Multiple Words Here"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatTopicName(tt.input)
			if result != tt.expected {
				t.Errorf("formatTopicName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHasTopicMarker(t *testing.T) {
	tempDir := t.TempDir()

	// Test with TOPIC.md
	topicDir := filepath.Join(tempDir, "with_topic")
	_ = os.MkdirAll(topicDir, 0755)
	_ = os.WriteFile(filepath.Join(topicDir, "TOPIC.md"), []byte("# Topic"), 0644)

	if !hasTopicMarker(topicDir) {
		t.Error("expected hasTopicMarker to return true for TOPIC.md")
	}

	// Test with README.md
	readmeDir := filepath.Join(tempDir, "with_readme")
	_ = os.MkdirAll(readmeDir, 0755)
	_ = os.WriteFile(filepath.Join(readmeDir, "README.md"), []byte("# README"), 0644)

	if !hasTopicMarker(readmeDir) {
		t.Error("expected hasTopicMarker to return true for README.md")
	}

	// Test without marker
	emptyDir := filepath.Join(tempDir, "empty")
	_ = os.MkdirAll(emptyDir, 0755)

	if hasTopicMarker(emptyDir) {
		t.Error("expected hasTopicMarker to return false for empty directory")
	}
}

// Helper function for string contains check
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
