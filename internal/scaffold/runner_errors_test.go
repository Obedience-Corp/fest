package scaffold

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	// Cancel before running to ensure context error is caught
	cancel()

	_, err := runner.Run(ctx, plan)
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
	t.Cleanup(func() { _ = os.Chmod(destDir, 0755) })

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

	destDir := filepath.Join(t.TempDir(), "phase-err2")
	// Create the festival dir but make the phase dir read-only
	os.MkdirAll(filepath.Join(destDir, "001_BUILD"), 0555)
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(destDir, "001_BUILD"), 0755) })

	runner := NewRunner(RunnerOptions{FestivalDir: destDir})
	_, err := runner.Run(ctx, plan)
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
	_ = os.Chmod(seqParent, 0555)
	t.Cleanup(func() { _ = os.Chmod(seqParent, 0755) })

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
	_ = os.Chmod(seqDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(seqDir, 0755) })

	runner := NewRunner(RunnerOptions{FestivalDir: destDir})
	_, err := runner.Run(ctx, plan)
	if err == nil {
		t.Error("expected error creating task in read-only dir")
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

func TestReadFileContentTooLarge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.md")

	// Create a file that exceeds the size limit
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
