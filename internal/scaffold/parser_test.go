package scaffold

import (
	"context"
	"strings"
	"testing"
)

// fc0003Structure is the reference STRUCTURE.md from FC0003.
const fc0003Structure = `# Festival Structure

## Festival Goal
Implement 4 validated fest CLI improvements identified from agent feedback across 5 prior festivals, improving phase chaining, plan scaffolding, and UX.

REQ-001 (workspace detection) removed — verified as already working correctly.

## Hierarchy

- **Festival:** fest-cli-agent-feedback-FC0003
  - **Phase 001:** INGEST (ingest) - COMPLETED
    - Gathered and validated agent feedback from 5 source festivals
    - Reduced 16 items to 5 validated requirements (13 already addressed)
    - REQ-001 later verified as also already addressed (total: 14 of 16 fixed)
  - **Phase 002:** PLAN (planning) - IN PROGRESS
    - Create implementation plan and validate structure
  - **Phase 003:** IMPLEMENT (implementation)
    - Sequence 01: phase_chaining (REQ-003)
      - Task 01: Design phase chaining mechanism in workflow engine
      - Task 02: Implement auto-create planning phase after ingest completes
      - Task 03: Link ingest output as planning input
      - Task 04: Tests for phase chaining
    - Sequence 02: plan_scaffolding (REQ-004)
      - Task 01: Design plan document parser for structure extraction
      - Task 02: Implement fest scaffold from-plan command
      - Task 03: Tests for plan scaffolding
    - Sequence 03: ux_enhancements (REQ-005, REQ-006)
      - Task 01: Add feedback criteria display to fest next output
      - Task 02: Add marker fill reminder after batch task creation
      - Task 03: Tests for UX enhancements
  - **Phase 004:** REVIEW (review)
    - Validate all changes, run full test suite, code review

## Dependencies

- Sequences 01-03 are independent and can be worked in parallel
- Phase 004 (REVIEW) depends on all sequences in Phase 003 completing
- Quality gates (testing, review, iterate, commit) are applied to each sequence
`

func TestParseFC0003Structure(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	plan, err := parser.Parse(ctx, fc0003Structure)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Verify festival name
	if plan.FestivalName != "fest-cli-agent-feedback-FC0003" {
		t.Errorf("FestivalName = %q, want %q", plan.FestivalName, "fest-cli-agent-feedback-FC0003")
	}

	// Verify goal was extracted
	if plan.Goal == "" {
		t.Error("Goal is empty, expected content")
	}

	// Verify 4 phases
	if len(plan.Phases) != 4 {
		t.Fatalf("len(Phases) = %d, want 4", len(plan.Phases))
	}

	// Phase 1: INGEST
	p1 := plan.Phases[0]
	if p1.Number != 1 {
		t.Errorf("Phase[0].Number = %d, want 1", p1.Number)
	}
	if p1.Name != "INGEST" {
		t.Errorf("Phase[0].Name = %q, want %q", p1.Name, "INGEST")
	}
	if p1.Type != "ingest" {
		t.Errorf("Phase[0].Type = %q, want %q", p1.Type, "ingest")
	}
	if p1.Status != "COMPLETED" {
		t.Errorf("Phase[0].Status = %q, want %q", p1.Status, "COMPLETED")
	}
	if len(p1.Sequences) != 0 {
		t.Errorf("Phase[0] has %d sequences, want 0", len(p1.Sequences))
	}

	// Phase 2: PLAN
	p2 := plan.Phases[1]
	if p2.Name != "PLAN" {
		t.Errorf("Phase[1].Name = %q, want %q", p2.Name, "PLAN")
	}
	if p2.Type != "planning" {
		t.Errorf("Phase[1].Type = %q, want %q", p2.Type, "planning")
	}
	if p2.Status != "IN PROGRESS" {
		t.Errorf("Phase[1].Status = %q, want %q", p2.Status, "IN PROGRESS")
	}

	// Phase 3: IMPLEMENT
	p3 := plan.Phases[2]
	if p3.Name != "IMPLEMENT" {
		t.Errorf("Phase[2].Name = %q, want %q", p3.Name, "IMPLEMENT")
	}
	if p3.Type != "implementation" {
		t.Errorf("Phase[2].Type = %q, want %q", p3.Type, "implementation")
	}
	if len(p3.Sequences) != 3 {
		t.Fatalf("Phase[2] has %d sequences, want 3", len(p3.Sequences))
	}

	// Sequence 1: phase_chaining
	s1 := p3.Sequences[0]
	if s1.Number != 1 {
		t.Errorf("Sequence[0].Number = %d, want 1", s1.Number)
	}
	if s1.Name != "phase_chaining" {
		t.Errorf("Sequence[0].Name = %q, want %q", s1.Name, "phase_chaining")
	}
	if s1.Requirement != "REQ-003" {
		t.Errorf("Sequence[0].Requirement = %q, want %q", s1.Requirement, "REQ-003")
	}
	if len(s1.Tasks) != 4 {
		t.Fatalf("Sequence[0] has %d tasks, want 4", len(s1.Tasks))
	}

	// Verify task details
	if s1.Tasks[0].Number != 1 {
		t.Errorf("Task[0].Number = %d, want 1", s1.Tasks[0].Number)
	}
	if s1.Tasks[0].Name != "Design phase chaining mechanism in workflow engine" {
		t.Errorf("Task[0].Name = %q", s1.Tasks[0].Name)
	}
	if s1.Tasks[3].Number != 4 {
		t.Errorf("Task[3].Number = %d, want 4", s1.Tasks[3].Number)
	}
	if s1.Tasks[3].Name != "Tests for phase chaining" {
		t.Errorf("Task[3].Name = %q", s1.Tasks[3].Name)
	}

	// Sequence 2: plan_scaffolding
	s2 := p3.Sequences[1]
	if s2.Name != "plan_scaffolding" {
		t.Errorf("Sequence[1].Name = %q, want %q", s2.Name, "plan_scaffolding")
	}
	if s2.Requirement != "REQ-004" {
		t.Errorf("Sequence[1].Requirement = %q, want %q", s2.Requirement, "REQ-004")
	}
	if len(s2.Tasks) != 3 {
		t.Errorf("Sequence[1] has %d tasks, want 3", len(s2.Tasks))
	}

	// Sequence 3: ux_enhancements
	s3 := p3.Sequences[2]
	if s3.Name != "ux_enhancements" {
		t.Errorf("Sequence[2].Name = %q, want %q", s3.Name, "ux_enhancements")
	}
	if s3.Requirement != "REQ-005, REQ-006" {
		t.Errorf("Sequence[2].Requirement = %q, want %q", s3.Requirement, "REQ-005, REQ-006")
	}
	if len(s3.Tasks) != 3 {
		t.Errorf("Sequence[2] has %d tasks, want 3", len(s3.Tasks))
	}

	// Phase 4: REVIEW
	p4 := plan.Phases[3]
	if p4.Name != "REVIEW" {
		t.Errorf("Phase[3].Name = %q, want %q", p4.Name, "REVIEW")
	}
	if p4.Type != "review" {
		t.Errorf("Phase[3].Type = %q, want %q", p4.Type, "review")
	}
}

func TestParseMinimalPlan(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	input := `## Hierarchy

- **Festival:** my-project
  - **Phase 001:** BUILD (implementation)
    - Sequence 01: core
      - Task 01: Setup project
`

	plan, err := parser.Parse(ctx, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if plan.FestivalName != "my-project" {
		t.Errorf("FestivalName = %q, want %q", plan.FestivalName, "my-project")
	}
	if len(plan.Phases) != 1 {
		t.Fatalf("len(Phases) = %d, want 1", len(plan.Phases))
	}
	if len(plan.Phases[0].Sequences) != 1 {
		t.Fatalf("len(Sequences) = %d, want 1", len(plan.Phases[0].Sequences))
	}
	if len(plan.Phases[0].Sequences[0].Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(plan.Phases[0].Sequences[0].Tasks))
	}
}

func TestParsePhaseWithoutSequences(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	input := `## Hierarchy

- **Festival:** review-project
  - **Phase 001:** INGEST (ingest)
  - **Phase 002:** REVIEW (review)
    - Final validation and sign-off
`

	plan, err := parser.Parse(ctx, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(plan.Phases) != 2 {
		t.Fatalf("len(Phases) = %d, want 2", len(plan.Phases))
	}
	if len(plan.Phases[0].Sequences) != 0 {
		t.Errorf("Phase[0] sequences = %d, want 0", len(plan.Phases[0].Sequences))
	}
	if len(plan.Phases[1].Sequences) != 0 {
		t.Errorf("Phase[1] sequences = %d, want 0", len(plan.Phases[1].Sequences))
	}
}

func TestParseEmptyInput(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	_, err := parser.Parse(ctx, "")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseNoStructure(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	_, err := parser.Parse(ctx, "# Just a heading\n\nSome text without structure.")
	if err == nil {
		t.Error("expected error for input without festival structure")
	}
}

func TestParseContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	parser := NewPlanParser()
	_, err := parser.Parse(ctx, fc0003Structure)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestParseGoal(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	plan, err := parser.Parse(ctx, fc0003Structure)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if plan.Goal == "" {
		t.Fatal("Goal should not be empty")
	}

	// Goal should contain key phrases from the document
	if !contains(plan.Goal, "4 validated") {
		t.Errorf("Goal missing expected content, got: %q", plan.Goal)
	}
}

func TestParseMultipleSequencesWithTasks(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	input := `## Hierarchy

- **Festival:** test-fest
  - **Phase 001:** WORK (implementation)
    - Sequence 01: backend (REQ-001)
      - Task 01: Create API
      - Task 02: Add auth
      - Task 03: Write tests
    - Sequence 02: frontend (REQ-002)
      - Task 01: Build UI
      - Task 02: Add routing
`

	plan, err := parser.Parse(ctx, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	phase := plan.Phases[0]
	if len(phase.Sequences) != 2 {
		t.Fatalf("sequences = %d, want 2", len(phase.Sequences))
	}

	if len(phase.Sequences[0].Tasks) != 3 {
		t.Errorf("seq[0] tasks = %d, want 3", len(phase.Sequences[0].Tasks))
	}
	if len(phase.Sequences[1].Tasks) != 2 {
		t.Errorf("seq[1] tasks = %d, want 2", len(phase.Sequences[1].Tasks))
	}
}

func TestParsePhaseDescriptionText(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	input := `## Hierarchy

- **Festival:** desc-test
  - **Phase 001:** WORK (implementation)
    This phase implements the core functionality
    across multiple components.
    - Sequence 01: api
      - Task 01: Setup
`

	plan, err := parser.Parse(ctx, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(plan.Phases) != 1 {
		t.Fatalf("phases = %d, want 1", len(plan.Phases))
	}

	// Description should be collected from indented non-list text
	if plan.Phases[0].Description == "" {
		t.Error("expected phase description from indented text")
	}
	if !contains(plan.Phases[0].Description, "core functionality") {
		t.Errorf("Description = %q, should contain 'core functionality'", plan.Phases[0].Description)
	}
}

func TestParseTaskWithBackticks(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	input := `## Hierarchy

- **Festival:** backtick-test
  - **Phase 001:** BUILD (implementation)
    - Sequence 01: core
      - Task 01: Implement ` + "`fest scaffold`" + ` command
`

	plan, err := parser.Parse(ctx, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	task := plan.Phases[0].Sequences[0].Tasks[0]
	if strings.Contains(task.Name, "`") {
		t.Errorf("task name should not contain backticks, got %q", task.Name)
	}
	if task.Name != "Implement fest scaffold command" {
		t.Errorf("task name = %q, want %q", task.Name, "Implement fest scaffold command")
	}
}

func TestParsePhaseStatus(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantStatus string
	}{
		{
			name:       "completed",
			input:      "  - **Phase 001:** INGEST (ingest) - COMPLETED",
			wantStatus: "COMPLETED",
		},
		{
			name:       "in progress",
			input:      "  - **Phase 002:** PLAN (planning) - IN PROGRESS",
			wantStatus: "IN PROGRESS",
		},
		{
			name:       "no status",
			input:      "  - **Phase 003:** IMPLEMENT (implementation)",
			wantStatus: "",
		},
	}

	ctx := context.Background()
	parser := NewPlanParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullInput := "## Hierarchy\n\n- **Festival:** test\n" + tt.input + "\n"
			plan, err := parser.Parse(ctx, fullInput)
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			if len(plan.Phases) != 1 {
				t.Fatalf("phases = %d, want 1", len(plan.Phases))
			}
			if plan.Phases[0].Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", plan.Phases[0].Status, tt.wantStatus)
			}
		})
	}
}

func contains(s, substr string) bool {
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
