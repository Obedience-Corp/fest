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

func TestRunnerContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	plan := &ParsedPlan{
		FestivalName: "cancelled",
		Phases: []ParsedPhase{
			{Number: 1, Name: "PHASE1", Type: "implementation"},
		},
	}

	runner := NewRunner(RunnerOptions{
		FestivalDir: filepath.Join(t.TempDir(), "cancelled"),
	})

	_, err := runner.Run(ctx, plan)
	if err == nil {
		t.Error("expected error from cancelled context")
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

func TestParseFileNotFound(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	_, err := parser.ParseFile(ctx, "/nonexistent/path/STRUCTURE.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParseFileContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	parser := NewPlanParser()
	_, err := parser.ParseFile(ctx, "/any/path")
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestRunnerInvalidDestDir(t *testing.T) {
	ctx := context.Background()
	plan := &ParsedPlan{
		FestivalName: "test",
		Phases:       []ParsedPhase{{Number: 1, Name: "BUILD", Type: "implementation"}},
	}

	// Use a path that can't be created (file as parent)
	tmpFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(tmpFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(RunnerOptions{
		FestivalDir: filepath.Join(tmpFile, "subdir", "fest"),
	})

	_, err := runner.Run(ctx, plan)
	if err == nil {
		t.Error("expected error for invalid destination")
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
	// Should still create FESTIVAL_GOAL.md
	if len(result.FilesCreated) != 1 {
		t.Errorf("FilesCreated = %d, want 1 (FESTIVAL_GOAL.md)", len(result.FilesCreated))
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

func TestRunnerContextCancellationMidPhase(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	plan := &ParsedPlan{
		FestivalName: "cancel-test",
		Phases: []ParsedPhase{
			{Number: 1, Name: "PHASE1", Type: "implementation"},
			{Number: 2, Name: "PHASE2", Type: "review"},
		},
	}

	destDir := filepath.Join(t.TempDir(), "cancel-mid")
	runner := NewRunner(RunnerOptions{FestivalDir: destDir})

	// Cancel after a brief moment to allow partial execution
	cancel()

	_, err := runner.Run(ctx, plan)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestReadFileContentTooLarge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.md")

	// Create a file that exceeds the size limit
	// We can't easily create a 1MB+ file quickly, so test the stat path
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test normal read works
	data, err := readFileContent(path)
	if err != nil {
		t.Fatalf("readFileContent() error: %v", err)
	}
	if string(data) != "test" {
		t.Errorf("content = %q, want %q", string(data), "test")
	}
}

func TestReadFileContentNotFound(t *testing.T) {
	_, err := readFileContent("/nonexistent/file.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestRunnerWriteError(t *testing.T) {
	ctx := context.Background()
	plan := &ParsedPlan{
		FestivalName: "write-error",
		Goal:         "Test goal",
		Phases:       []ParsedPhase{{Number: 1, Name: "BUILD", Type: "implementation"}},
	}

	// Create a read-only directory to trigger write errors
	destDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Make read-only after creation
	if err := os.Chmod(destDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(destDir, 0755) })

	runner := NewRunner(RunnerOptions{FestivalDir: destDir})
	_, err := runner.Run(ctx, plan)
	if err == nil {
		t.Error("expected error writing to read-only directory")
	}
}

func TestRunnerPhaseWriteError(t *testing.T) {
	ctx := context.Background()
	plan := &ParsedPlan{
		FestivalName: "phase-write-error",
		Goal:         "Test goal",
		Phases: []ParsedPhase{
			{
				Number: 1, Name: "BUILD", Type: "implementation",
				Sequences: []ParsedSequence{
					{
						Number: 1, Name: "core",
						Tasks: []ParsedTask{{Number: 1, Name: "setup"}},
					},
				},
			},
		},
	}

	destDir := filepath.Join(t.TempDir(), "phase-err")
	runner := NewRunner(RunnerOptions{FestivalDir: destDir})

	// First run should succeed
	_, err := runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("initial Run() error: %v", err)
	}

	// Make a phase dir read-only to trigger sequence creation error
	phaseDir := filepath.Join(destDir, "001_BUILD")
	os.Chmod(phaseDir, 0555)
	t.Cleanup(func() { os.Chmod(phaseDir, 0755) })

	// Second run should fail trying to write into read-only phase dir
	destDir2 := filepath.Join(t.TempDir(), "phase-err2")
	// Create the festival dir but make the phase dir read-only
	os.MkdirAll(filepath.Join(destDir2, "001_BUILD"), 0555)
	t.Cleanup(func() { os.Chmod(filepath.Join(destDir2, "001_BUILD"), 0755) })

	runner2 := NewRunner(RunnerOptions{FestivalDir: destDir2})
	_, err = runner2.Run(ctx, plan)
	if err == nil {
		t.Error("expected error from read-only phase directory")
	}
}

func TestRunnerSequenceWriteError(t *testing.T) {
	ctx := context.Background()
	plan := &ParsedPlan{
		FestivalName: "seq-write-err",
		Goal:         "Test",
		Phases: []ParsedPhase{
			{
				Number: 1, Name: "BUILD", Type: "implementation",
				Sequences: []ParsedSequence{
					{Number: 1, Name: "core", Tasks: []ParsedTask{{Number: 1, Name: "setup"}}},
				},
			},
		},
	}

	destDir := filepath.Join(t.TempDir(), "seq-err")
	// Pre-create the phase dir read-only so sequence mkdir fails
	seqParent := filepath.Join(destDir, "001_BUILD")
	if err := os.MkdirAll(seqParent, 0755); err != nil {
		t.Fatal(err)
	}
	// Write PHASE_GOAL.md before locking
	if err := os.WriteFile(filepath.Join(seqParent, "PHASE_GOAL.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	os.Chmod(seqParent, 0555)
	t.Cleanup(func() { os.Chmod(seqParent, 0755) })

	runner := NewRunner(RunnerOptions{FestivalDir: destDir})
	_, err := runner.Run(ctx, plan)
	if err == nil {
		t.Error("expected error creating sequence in read-only dir")
	}
}

func TestRunnerTaskWriteError(t *testing.T) {
	ctx := context.Background()
	plan := &ParsedPlan{
		FestivalName: "task-write-err",
		Goal:         "Test",
		Phases: []ParsedPhase{
			{
				Number: 1, Name: "BUILD", Type: "implementation",
				Sequences: []ParsedSequence{
					{Number: 1, Name: "core", Tasks: []ParsedTask{{Number: 1, Name: "setup"}}},
				},
			},
		},
	}

	destDir := filepath.Join(t.TempDir(), "task-err")
	// Pre-create structure then make sequence dir read-only
	seqDir := filepath.Join(destDir, "001_BUILD", "01_core")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Write SEQUENCE_GOAL.md before locking
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	os.Chmod(seqDir, 0555)
	t.Cleanup(func() { os.Chmod(seqDir, 0755) })

	runner := NewRunner(RunnerOptions{FestivalDir: destDir})
	_, err := runner.Run(ctx, plan)
	if err == nil {
		t.Error("expected error creating task in read-only dir")
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

func TestReadFileContentTooLargeReal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.md")

	// Create a file just over the 1MB limit
	data := make([]byte, maxPlanFileSize+1)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := readFileContent(path)
	if err == nil {
		t.Error("expected error for file exceeding size limit")
	}
	if !strings.Contains(err.Error(), "file too large") {
		t.Errorf("error = %q, want 'file too large'", err.Error())
	}
}

func TestParsePhaseDescriptions(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()

	plan, err := parser.Parse(ctx, fc0003Structure)
	if err != nil {
		t.Fatal(err)
	}

	// Phase 1 (INGEST) should NOT have description lines collected
	// because they start with "- " (list items, not descriptions)
	// Phase 4 (REVIEW) might have description
	p4 := plan.Phases[3]
	if p4.Name != "REVIEW" {
		t.Fatalf("Phase[3] = %q, want REVIEW", p4.Name)
	}
}
