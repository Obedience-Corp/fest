package validation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func setupTestFestival(t *testing.T, opts testFestivalOpts) string {
	t.Helper()
	dir := t.TempDir()

	// Create FESTIVAL_OVERVIEW.md
	os.WriteFile(filepath.Join(dir, "FESTIVAL_OVERVIEW.md"), []byte("# Test Festival\nGoal: Test validation behavior.\n"), 0644)

	if opts.phaseType != "" {
		phaseName := "001_IMPLEMENTATION"
		if opts.phaseDirName != "" {
			phaseName = opts.phaseDirName
		}
		phasePath := filepath.Join(dir, phaseName)
		os.MkdirAll(phasePath, 0755)

		phaseGoal := fmt.Sprintf(`---
fest_type: phase
fest_phase_type: %s
fest_status: pending
---
# Phase Goal
`, opts.phaseType)
		if opts.withMarkers {
			phaseGoal += "\n[REPLACE: Some marker]\n[FILL: Another marker]\n"
		} else {
			phaseGoal += "\nActual content here.\n"
		}
		os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(phaseGoal), 0644)

		if opts.withSequence {
			seqPath := filepath.Join(phasePath, "01_test_sequence")
			os.MkdirAll(seqPath, 0755)
			os.WriteFile(filepath.Join(seqPath, "SEQUENCE_GOAL.md"), []byte("# Sequence Goal\nDo the thing.\n"), 0644)

			if opts.withTask {
				os.WriteFile(filepath.Join(seqPath, "01_test_task.md"), []byte("# Task: Test\n## Objective\nTest task.\n"), 0644)
			}
		}
	}

	return dir
}

type testFestivalOpts struct {
	phaseType    string
	phaseDirName string
	withMarkers  bool
	withSequence bool
	withTask     bool
}

func TestValidateTemplateMarkers_ImplementationPhaseFailsOnMarkers(t *testing.T) {
	dir := setupTestFestival(t, testFestivalOpts{
		phaseType:    "implementation",
		withMarkers:  true,
		withSequence: true,
		withTask:     true,
	})
	result := &ValidationResult{
		OK:     true,
		Valid:  true,
		Issues: []ValidationIssue{},
	}
	validateTemplateMarkers(dir, result)

	hasError := false
	for _, issue := range result.Issues {
		if issue.Code == CodeUnfilledTemplate && issue.Level == LevelError {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected error-level issue for unfilled markers in implementation phase")
	}
}

func TestValidateTemplateMarkers_PlanningPhaseWarnsOnMarkers(t *testing.T) {
	dir := setupTestFestival(t, testFestivalOpts{
		phaseType:    "planning",
		phaseDirName: "001_PLANNING",
		withMarkers:  true,
		withSequence: true,
		withTask:     true,
	})
	result := &ValidationResult{
		OK:     true,
		Valid:  true,
		Issues: []ValidationIssue{},
	}
	validateTemplateMarkers(dir, result)

	for _, issue := range result.Issues {
		if issue.Code == CodeUnfilledTemplate && issue.Level == LevelError {
			t.Errorf("planning phase markers should be warnings, not errors; got: %+v", issue)
		}
	}

	// Verify at least one warning was generated
	hasWarning := false
	for _, issue := range result.Issues {
		if issue.Code == CodeUnfilledTemplate && issue.Level == LevelWarning {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Error("expected warning-level issue for unfilled markers in planning phase")
	}
}

func TestValidateTemplateMarkers_ResearchPhaseWarnsOnMarkers(t *testing.T) {
	dir := setupTestFestival(t, testFestivalOpts{
		phaseType:    "research",
		phaseDirName: "001_RESEARCH",
		withMarkers:  true,
	})
	result := &ValidationResult{
		OK:     true,
		Valid:  true,
		Issues: []ValidationIssue{},
	}
	validateTemplateMarkers(dir, result)

	for _, issue := range result.Issues {
		if issue.Code == CodeUnfilledTemplate && issue.Level == LevelError {
			t.Errorf("research phase markers should be warnings, not errors; got: %+v", issue)
		}
	}
}

func TestValidateAll_CleanFestivalPasses(t *testing.T) {
	dir := setupTestFestival(t, testFestivalOpts{
		phaseType:    "implementation",
		withSequence: true,
		withTask:     true,
	})

	result := &ValidationResult{
		OK:       true,
		Action:   "validate",
		Festival: filepath.Base(dir),
		Valid:    true,
		Issues:   []ValidationIssue{},
	}
	ctx := context.Background()
	validateStructureChecks(ctx, dir, result)
	validateCompletenessChecks(ctx, dir, result)
	validateTaskFilesChecks(ctx, dir, result)
	validateTemplateMarkers(dir, result)

	// Check for errors (warnings are acceptable)
	for _, issue := range result.Issues {
		if issue.Level == LevelError {
			t.Errorf("clean festival should not have errors; got: %+v", issue)
		}
	}
}

func TestValidateTaskFiles_SequenceWithoutTasksFails(t *testing.T) {
	dir := setupTestFestival(t, testFestivalOpts{
		phaseType:    "implementation",
		withSequence: true,
		withTask:     false, // sequence exists but no task files
	})

	result := &ValidationResult{
		OK:     true,
		Valid:  true,
		Issues: []ValidationIssue{},
	}
	ctx := context.Background()
	validateTaskFilesChecks(ctx, dir, result)

	hasError := false
	for _, issue := range result.Issues {
		if issue.Code == CodeMissingTaskFiles && issue.Level == LevelError {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected error for implementation sequence without task files")
	}
}
