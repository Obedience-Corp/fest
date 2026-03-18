package validation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestFestival(t *testing.T, opts testFestivalOpts) string {
	t.Helper()
	dir := t.TempDir()

	// Create FESTIVAL_OVERVIEW.md
	_ = os.WriteFile(filepath.Join(dir, "FESTIVAL_OVERVIEW.md"), []byte("# Test Festival\nGoal: Test validation behavior.\n"), 0644)

	if opts.phaseType != "" {
		phaseName := "001_IMPLEMENTATION"
		if opts.phaseDirName != "" {
			phaseName = opts.phaseDirName
		}
		phasePath := filepath.Join(dir, phaseName)
		_ = os.MkdirAll(phasePath, 0755)

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
		_ = os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(phaseGoal), 0644)

		if opts.withSequence {
			seqPath := filepath.Join(phasePath, "01_test_sequence")
			_ = os.MkdirAll(seqPath, 0755)
			_ = os.WriteFile(filepath.Join(seqPath, "SEQUENCE_GOAL.md"), []byte("# Sequence Goal\nDo the thing.\n"), 0644)

			if opts.withTask {
				_ = os.WriteFile(filepath.Join(seqPath, "01_test_task.md"), []byte("# Task: Test\n## Objective\nTest task.\n"), 0644)
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
	validateTemplateChecks(dir, result)

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
	validateTemplateChecks(dir, result)

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
	validateTemplateChecks(dir, result)

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
	validateTemplateChecks(dir, result)

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

func TestValidateQualityGatesChecks_AutoFixAddsMissingGates(t *testing.T) {
	dir := setupTestFestival(t, testFestivalOpts{
		phaseType:    "implementation",
		withSequence: true,
		withTask:     true,
	})

	gateDir := filepath.Join(dir, "gates", "implementation")
	if err := os.MkdirAll(gateDir, 0755); err != nil {
		t.Fatalf("mkdir gate dir: %v", err)
	}

	templates := map[string]string{
		"QUALITY_GATE_TESTING.md": `---
fest_type: gate
fest_gate_type: testing
fest_status: pending
---
# Gate: Testing and Verification
`,
		"QUALITY_GATE_REVIEW.md": `---
fest_type: gate
fest_gate_type: review
fest_status: pending
---
# Gate: Code Review
`,
		"QUALITY_GATE_ITERATE.md": `---
fest_type: gate
fest_gate_type: iterate
fest_status: pending
---
# Gate: Review Results and Iterate
`,
		"QUALITY_GATE_FEST_COMMIT.md": `---
fest_type: gate
fest_gate_type: commit
fest_status: pending
---
# Gate: Commit Sequence Changes
`,
	}

	for name, content := range templates {
		if err := os.WriteFile(filepath.Join(gateDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write template %s: %v", name, err)
		}
	}

	result := &ValidationResult{
		OK:     true,
		Valid:  true,
		Issues: []ValidationIssue{},
	}

	validateQualityGatesChecks(context.Background(), dir, result, true)

	if len(result.Issues) != 0 {
		t.Fatalf("validateQualityGatesChecks() issues = %+v, want none after autofix", result.Issues)
	}
	if len(result.FixesApplied) != 4 {
		t.Fatalf("validateQualityGatesChecks() applied %d fixes, want 4", len(result.FixesApplied))
	}

	seqPath := filepath.Join(dir, "001_IMPLEMENTATION", "01_test_sequence")
	for _, gate := range []struct {
		name   string
		gateID string
	}{
		{name: "02_testing.md", gateID: "testing"},
		{name: "03_review.md", gateID: "review"},
		{name: "04_iterate.md", gateID: "iterate"},
		{name: "05_fest_commit.md", gateID: "fest-commit"},
	} {
		path := filepath.Join(seqPath, gate.name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected autofixed gate %s: %v", gate.name, err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read autofixed gate %s: %v", gate.name, err)
		}
		if !strings.Contains(string(content), "fest_gate_id: "+gate.gateID) {
			t.Fatalf("autofixed gate %s missing fest_gate_id %q", gate.name, gate.gateID)
		}
		if !strings.Contains(string(content), "fest_managed: true") {
			t.Fatalf("autofixed gate %s missing fest_managed marker", gate.name)
		}
	}
}

func TestValidateQualityGatesChecks_AutoFixBackfillsLegacyGateID(t *testing.T) {
	dir := setupTestFestival(t, testFestivalOpts{
		phaseType:    "implementation",
		withSequence: true,
		withTask:     true,
	})

	gateDir := filepath.Join(dir, "gates", "implementation")
	if err := os.MkdirAll(gateDir, 0755); err != nil {
		t.Fatalf("mkdir gate dir: %v", err)
	}

	templates := map[string]string{
		"QUALITY_GATE_TESTING.md": `---
fest_type: gate
fest_status: pending
---
# Gate: Testing and Verification
`,
		"QUALITY_GATE_REVIEW.md": `---
fest_type: gate
fest_status: pending
---
# Gate: Code Review
`,
		"QUALITY_GATE_ITERATE.md": `---
fest_type: gate
fest_status: pending
---
# Gate: Review Results and Iterate
`,
		"QUALITY_GATE_FEST_COMMIT.md": `---
fest_type: gate
fest_status: pending
---
# Gate: Commit Sequence Changes
`,
	}

	for name, content := range templates {
		if err := os.WriteFile(filepath.Join(gateDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write template %s: %v", name, err)
		}
	}

	seqPath := filepath.Join(dir, "001_IMPLEMENTATION", "01_test_sequence")
	legacyCommitPath := filepath.Join(seqPath, "02_fest_commit.md")
	legacyCommitContent := `---
fest_type: gate
fest_status: pending
---
# Gate: Commit Sequence Changes
`
	if err := os.WriteFile(legacyCommitPath, []byte(legacyCommitContent), 0644); err != nil {
		t.Fatalf("write legacy commit gate: %v", err)
	}

	result := &ValidationResult{
		OK:     true,
		Valid:  true,
		Issues: []ValidationIssue{},
	}

	validateQualityGatesChecks(context.Background(), dir, result, true)

	if len(result.Issues) != 0 {
		t.Fatalf("validateQualityGatesChecks() issues = %+v, want none after autofix", result.Issues)
	}

	content, err := os.ReadFile(legacyCommitPath)
	if err != nil {
		t.Fatalf("read legacy commit gate: %v", err)
	}
	if !strings.Contains(string(content), "fest_gate_id: fest-commit") {
		t.Fatalf("legacy commit gate missing backfilled fest_gate_id")
	}

	entries, err := os.ReadDir(seqPath)
	if err != nil {
		t.Fatalf("read sequence dir: %v", err)
	}

	commitCount := 0
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "fest_commit") {
			commitCount++
		}
	}
	if commitCount != 1 {
		t.Fatalf("expected exactly 1 fest_commit gate after autofix, found %d", commitCount)
	}
}

func TestFinalizeValidationResult(t *testing.T) {
	falseValue := false

	tests := []struct {
		name         string
		result       *ValidationResult
		wantValid    bool
		wantBlocking bool
	}{
		{
			name: "clean result is valid",
			result: &ValidationResult{
				Issues:   []ValidationIssue{},
				Warnings: []string{},
			},
			wantValid:    true,
			wantBlocking: false,
		},
		{
			name: "warning issue is invalid but not blocking",
			result: &ValidationResult{
				Issues: []ValidationIssue{{
					Level:   LevelWarning,
					Code:    CodeNamingConvention,
					Message: "warning only",
				}},
			},
			wantValid:    false,
			wantBlocking: false,
		},
		{
			name: "warning string is invalid but not blocking",
			result: &ValidationResult{
				Warnings: []string{"warning only"},
			},
			wantValid:    false,
			wantBlocking: false,
		},
		{
			name: "info issue stays valid and non-blocking",
			result: &ValidationResult{
				Issues: []ValidationIssue{{
					Level:   LevelInfo,
					Code:    "info_only",
					Message: "informational note",
				}},
			},
			wantValid:    true,
			wantBlocking: false,
		},
		{
			name: "error issue is invalid and blocking",
			result: &ValidationResult{
				Issues: []ValidationIssue{{
					Level:   LevelError,
					Code:    CodeMissingTaskFiles,
					Message: "blocking issue",
				}},
			},
			wantValid:    false,
			wantBlocking: true,
		},
		{
			name: "failed checklist is invalid and blocking",
			result: &ValidationResult{
				Checklist: &Checklist{
					TaskFilesExist: &falseValue,
				},
			},
			wantValid:    false,
			wantBlocking: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finalizeValidationResult(tt.result)

			if tt.result.Valid != tt.wantValid {
				t.Fatalf("Valid = %v, want %v", tt.result.Valid, tt.wantValid)
			}
			if got := validationHasBlockingFailures(tt.result); got != tt.wantBlocking {
				t.Fatalf("validationHasBlockingFailures() = %v, want %v", got, tt.wantBlocking)
			}
		})
	}
}
