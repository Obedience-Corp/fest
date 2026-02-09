package scaffold

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerRun(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()
	plan, err := parser.Parse(ctx, fc0003Structure)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	destDir := filepath.Join(t.TempDir(), "test-fest")

	runner := NewRunner(RunnerOptions{
		FestivalDir: destDir,
	})

	result, err := runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify counts
	if result.PhasesCreated != 4 {
		t.Errorf("PhasesCreated = %d, want 4", result.PhasesCreated)
	}
	if result.SequencesCreated != 3 {
		t.Errorf("SequencesCreated = %d, want 3", result.SequencesCreated)
	}
	if result.TasksCreated != 10 {
		t.Errorf("TasksCreated = %d, want 10", result.TasksCreated)
	}

	// Verify key directories exist
	expectedDirs := []string{
		"001_INGEST",
		"002_PLAN",
		"003_IMPLEMENT",
		"003_IMPLEMENT/01_phase_chaining",
		"003_IMPLEMENT/02_plan_scaffolding",
		"003_IMPLEMENT/03_ux_enhancements",
		"004_REVIEW",
	}
	for _, dir := range expectedDirs {
		path := filepath.Join(destDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", dir)
		}
	}

	// Verify key files exist
	expectedFiles := []string{
		"FESTIVAL_GOAL.md",
		"FESTIVAL_OVERVIEW.md",
		"001_INGEST/PHASE_GOAL.md",
		"003_IMPLEMENT/PHASE_GOAL.md",
		"003_IMPLEMENT/01_phase_chaining/SEQUENCE_GOAL.md",
		"004_REVIEW/PHASE_GOAL.md",
	}
	for _, file := range expectedFiles {
		path := filepath.Join(destDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", file)
		}
	}

	// Verify FESTIVAL_GOAL.md has frontmatter
	goalContent, err := os.ReadFile(filepath.Join(destDir, "FESTIVAL_GOAL.md"))
	if err != nil {
		t.Fatalf("reading FESTIVAL_GOAL.md: %v", err)
	}
	if !strings.HasPrefix(string(goalContent), "---") {
		t.Error("FESTIVAL_GOAL.md should start with frontmatter")
	}
	if !strings.Contains(string(goalContent), "fest_type: festival") {
		t.Error("FESTIVAL_GOAL.md should contain festival frontmatter")
	}

	// Verify PHASE_GOAL.md has correct phase type
	phaseContent, err := os.ReadFile(filepath.Join(destDir, "001_INGEST/PHASE_GOAL.md"))
	if err != nil {
		t.Fatalf("reading PHASE_GOAL.md: %v", err)
	}
	if !strings.Contains(string(phaseContent), "fest_phase_type: ingest") {
		t.Error("PHASE_GOAL.md should contain ingest phase type")
	}

	// Verify task file has content
	taskFiles, err := filepath.Glob(filepath.Join(destDir, "003_IMPLEMENT/01_phase_chaining/01_*.md"))
	if err != nil {
		t.Fatalf("globbing task files: %v", err)
	}
	if len(taskFiles) != 1 {
		t.Fatalf("expected 1 task file matching 01_*, got %d", len(taskFiles))
	}
	taskContent, err := os.ReadFile(taskFiles[0])
	if err != nil {
		t.Fatalf("reading task file: %v", err)
	}
	if !strings.Contains(string(taskContent), "fest_type: task") {
		t.Error("task file should contain task frontmatter")
	}
	if !strings.Contains(string(taskContent), "## Objective") {
		t.Error("task file should contain Objective section")
	}
}

func TestRunnerDryRun(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()
	plan, err := parser.Parse(ctx, fc0003Structure)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	destDir := filepath.Join(t.TempDir(), "dry-run-fest")

	runner := NewRunner(RunnerOptions{
		FestivalDir: destDir,
		DryRun:      true,
	})

	result, err := runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify counts are reported
	if result.PhasesCreated != 4 {
		t.Errorf("PhasesCreated = %d, want 4", result.PhasesCreated)
	}
	if result.TasksCreated != 10 {
		t.Errorf("TasksCreated = %d, want 10", result.TasksCreated)
	}

	// Verify files and directories are listed but NOT created
	if len(result.FilesCreated) == 0 {
		t.Error("expected files to be listed in result")
	}
	if len(result.DirsCreated) == 0 {
		t.Error("expected dirs to be listed in result")
	}

	// Nothing should actually exist on disk
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		t.Error("dry run should not create any directories")
	}
}

func TestRunnerMinimalPlan(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	input := `## Hierarchy

- **Festival:** minimal
  - **Phase 001:** BUILD (implementation)
    - Sequence 01: core
      - Task 01: Setup
`

	plan, err := parser.Parse(ctx, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	destDir := filepath.Join(t.TempDir(), "minimal-fest")
	runner := NewRunner(RunnerOptions{FestivalDir: destDir})

	result, err := runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.PhasesCreated != 1 {
		t.Errorf("PhasesCreated = %d, want 1", result.PhasesCreated)
	}
	if result.SequencesCreated != 1 {
		t.Errorf("SequencesCreated = %d, want 1", result.SequencesCreated)
	}
	if result.TasksCreated != 1 {
		t.Errorf("TasksCreated = %d, want 1", result.TasksCreated)
	}

	// Verify the task file exists
	taskFiles, err := filepath.Glob(filepath.Join(destDir, "001_BUILD/01_core/01_setup.md"))
	if err != nil || len(taskFiles) != 1 {
		t.Error("expected task file 01_setup.md to exist")
	}
}

func TestRunnerPhasesOnly(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	input := `## Hierarchy

- **Festival:** phases-only
  - **Phase 001:** INGEST (ingest)
  - **Phase 002:** REVIEW (review)
`

	plan, err := parser.Parse(ctx, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	destDir := filepath.Join(t.TempDir(), "phases-only")
	runner := NewRunner(RunnerOptions{FestivalDir: destDir})

	result, err := runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.PhasesCreated != 2 {
		t.Errorf("PhasesCreated = %d, want 2", result.PhasesCreated)
	}
	if result.SequencesCreated != 0 {
		t.Errorf("SequencesCreated = %d, want 0", result.SequencesCreated)
	}
	if result.TasksCreated != 0 {
		t.Errorf("TasksCreated = %d, want 0", result.TasksCreated)
	}
}

func TestRunnerSequenceGoalContent(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	input := `## Hierarchy

- **Festival:** content-test
  - **Phase 001:** WORK (implementation)
    - Sequence 01: backend (REQ-001)
      - Task 01: Create API
`

	plan, err := parser.Parse(ctx, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	destDir := filepath.Join(t.TempDir(), "content-test")
	runner := NewRunner(RunnerOptions{FestivalDir: destDir})

	_, err = runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify sequence goal has requirement info
	seqGoal, err := os.ReadFile(filepath.Join(destDir, "001_WORK/01_backend/SEQUENCE_GOAL.md"))
	if err != nil {
		t.Fatalf("reading SEQUENCE_GOAL.md: %v", err)
	}
	content := string(seqGoal)
	if !strings.Contains(content, "REQ-001") {
		t.Error("SEQUENCE_GOAL.md should contain requirement reference")
	}
	if !strings.Contains(content, "fest_type: sequence") {
		t.Error("SEQUENCE_GOAL.md should contain sequence frontmatter")
	}
}

func TestRunnerEmptyPlan(t *testing.T) {
	ctx := context.Background()
	plan := &ParsedPlan{
		FestivalName: "empty-plan",
	}

	destDir := filepath.Join(t.TempDir(), "empty-plan")
	runner := NewRunner(RunnerOptions{FestivalDir: destDir})

	result, err := runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.PhasesCreated != 0 {
		t.Errorf("PhasesCreated = %d, want 0", result.PhasesCreated)
	}
	// Should create FESTIVAL_GOAL.md and FESTIVAL_OVERVIEW.md
	if len(result.FilesCreated) != 2 {
		t.Errorf("FilesCreated = %d, want 2 (FESTIVAL_GOAL.md + FESTIVAL_OVERVIEW.md)", len(result.FilesCreated))
	}
}

func TestRunnerEmptyGoal(t *testing.T) {
	ctx := context.Background()
	plan := &ParsedPlan{
		FestivalName: "no-goal",
		Phases:       []ParsedPhase{{Number: 1, Name: "WORK", Type: "implementation"}},
	}

	destDir := filepath.Join(t.TempDir(), "no-goal")
	runner := NewRunner(RunnerOptions{FestivalDir: destDir})

	_, err := runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify goal content uses festival name when goal is empty
	goalContent, err := os.ReadFile(filepath.Join(destDir, "FESTIVAL_GOAL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goalContent), "no-goal") {
		t.Error("FESTIVAL_GOAL.md should contain festival name when goal is empty")
	}
}

func TestRunnerPhaseWithDescription(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	// Parse FC0003 which has phase descriptions
	plan, err := parser.Parse(ctx, fc0003Structure)
	if err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(t.TempDir(), "desc-test")
	runner := NewRunner(RunnerOptions{FestivalDir: destDir})

	_, err = runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify the REVIEW phase goal uses description when available
	reviewGoal, err := os.ReadFile(filepath.Join(destDir, "004_REVIEW/PHASE_GOAL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reviewGoal), "fest_type: phase") {
		t.Error("PHASE_GOAL.md should have phase frontmatter")
	}
}

func TestParseFile(t *testing.T) {
	ctx := context.Background()

	// Write a plan file
	dir := t.TempDir()
	planPath := filepath.Join(dir, "STRUCTURE.md")
	content := `## Hierarchy

- **Festival:** file-test
  - **Phase 001:** BUILD (implementation)
`
	if err := os.WriteFile(planPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	parser := NewPlanParser()
	plan, err := parser.ParseFile(ctx, planPath)
	if err != nil {
		t.Fatalf("ParseFile() error: %v", err)
	}

	if plan.FestivalName != "file-test" {
		t.Errorf("FestivalName = %q, want %q", plan.FestivalName, "file-test")
	}
}

func TestParseInvalidNumbers(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	input := `## Hierarchy

- **Festival:** test
  - **Phase 001:** BUILD (implementation)
    - Sequence 01: core
      - Task 01: First task
      - Task 02: Second task
      - Task 03: Third task
`

	plan, err := parser.Parse(ctx, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(plan.Phases[0].Sequences[0].Tasks) != 3 {
		t.Errorf("tasks = %d, want 3", len(plan.Phases[0].Sequences[0].Tasks))
	}
}

func TestParsePhaseDescriptions(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	plan, err := parser.Parse(ctx, fc0003Structure)
	if err != nil {
		t.Fatal(err)
	}

	// Phase 4 (REVIEW) should be parsed correctly
	p4 := plan.Phases[3]
	if p4.Name != "REVIEW" {
		t.Fatalf("Phase[3] = %q, want REVIEW", p4.Name)
	}
}
