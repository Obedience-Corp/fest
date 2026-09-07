package validation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeStatusFestival creates a festival directory whose root documents carry
// template markers, with a fest.yaml recording the given lifecycle status.
// An empty status writes no fest.yaml at all.
func writeStatusFestival(t *testing.T, status string) string {
	t.Helper()
	dir := t.TempDir()

	docs := map[string]string{
		"FESTIVAL_OVERVIEW.md": "# Overview\n\n[REPLACE: what this delivers]\n",
		"FESTIVAL_GOAL.md":     "# Goal\n\n[REPLACE: the outcome]\n",
	}
	for name, body := range docs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if status != "" {
		festYAML := fmt.Sprintf(`version: "1.0"
metadata:
  id: TF0001
  name: test-fest
  status_history:
    - status: %s
      timestamp: 2026-01-01T00:00:00Z
`, status)
		if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestValidateTemplateChecks_RootMarkerLevelByStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		wantLevel string
	}{
		{name: "ready festival reports root markers as errors", status: "ready", wantLevel: LevelError},
		{name: "active festival reports root markers as errors", status: "active", wantLevel: LevelError},
		{name: "planning festival reports root markers as warnings", status: "planning", wantLevel: LevelWarning},
		{name: "unreadable status reports root markers as warnings", status: "", wantLevel: LevelWarning},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeStatusFestival(t, tc.status)
			result := &ValidationResult{OK: true, Valid: true, Issues: []ValidationIssue{}}
			validateTemplateChecks(context.Background(), dir, result)

			found := 0
			for _, issue := range result.Issues {
				if issue.Code != CodeUnfilledTemplate {
					continue
				}
				found++
				if issue.Level != tc.wantLevel {
					t.Errorf("%s level = %q, want %q", issue.Path, issue.Level, tc.wantLevel)
				}
			}
			if found != 2 {
				t.Fatalf("marker issues = %d, want 2", found)
			}
		})
	}
}

func TestValidationLifecycleRule_MarkersPendingByStatus(t *testing.T) {
	tests := []struct {
		name            string
		markersBlocking bool
		issues          []ValidationIssue
		wantValid       bool
		wantBlocking    bool
	}{
		{
			name:            "structural error fails in either status",
			markersBlocking: false,
			issues: []ValidationIssue{
				{Level: LevelError, Code: CodeMissingFile, Path: "FESTIVAL_OVERVIEW.md"},
			},
			wantValid:    false,
			wantBlocking: true,
		},
		{
			name:            "markers pending before promotion still pass",
			markersBlocking: false,
			issues: []ValidationIssue{
				{Level: LevelWarning, Code: CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
			},
			wantValid:    true,
			wantBlocking: false,
		},
		{
			name:            "markers pending after promotion fail",
			markersBlocking: true,
			issues: []ValidationIssue{
				{Level: LevelError, Code: CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
			},
			wantValid:    false,
			wantBlocking: true,
		},
		{
			name:            "phase warning markers after promotion also fail",
			markersBlocking: true,
			issues: []ValidationIssue{
				{Level: LevelWarning, Code: CodeUnfilledTemplate, Path: "001_PLAN/PHASE_GOAL.md"},
			},
			wantValid:    false,
			wantBlocking: true,
		},
		{
			name:            "clean festival passes after promotion",
			markersBlocking: true,
			issues:          []ValidationIssue{},
			wantValid:       true,
			wantBlocking:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &ValidationResult{Issues: tc.issues, markersBlocking: tc.markersBlocking}
			finalizeValidationResult(result)

			if result.Valid != tc.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tc.wantValid)
			}
			if got := validationHasBlockingFailures(result); got != tc.wantBlocking {
				t.Errorf("validationHasBlockingFailures() = %v, want %v", got, tc.wantBlocking)
			}
			if result.Valid && validationHasBlockingFailures(result) {
				t.Error("valid and blocking must never both be true")
			}
		})
	}
}

func TestValidationResult_NoErrorIssueSurvivesAValidVerdict(t *testing.T) {
	// The payload that started this: a fresh planning festival reported
	// valid:true while carrying error-level marker issues.
	dir := writeStatusFestival(t, "planning")
	result := &ValidationResult{
		OK:              true,
		Action:          "validate",
		Valid:           true,
		Issues:          []ValidationIssue{},
		markersBlocking: false,
	}
	validateTemplateChecks(context.Background(), dir, result)
	finalizeValidationResult(result)

	if !result.Valid {
		t.Fatal("Valid = false, want true for a planning festival with markers pending")
	}
	if !result.MarkersPending {
		t.Fatal("MarkersPending = false, want true")
	}
	for _, issue := range result.Issues {
		if issue.Level == LevelError {
			t.Errorf("valid result must carry no error-level issue, got %+v", issue)
		}
	}
}

func TestApplyMarkerLifecycleLevels_LeavesPhaseFilesAlone(t *testing.T) {
	dir := writeStatusFestival(t, "planning")
	issues := []ValidationIssue{
		{Level: LevelError, Code: CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
		{Level: LevelError, Code: CodeUnfilledTemplate, Path: filepath.Join("001_IMPL", "01_seq", "01_task.md")},
		{Level: LevelError, Code: CodeMissingFile, Path: "TODO.md"},
	}
	applyMarkerLifecycleLevels(context.Background(), dir, issues)

	if issues[0].Level != LevelWarning {
		t.Errorf("root marker level = %q, want %q", issues[0].Level, LevelWarning)
	}
	if issues[1].Level != LevelError {
		t.Errorf("phase marker level = %q, want it left at %q", issues[1].Level, LevelError)
	}
	if issues[2].Level != LevelError {
		t.Errorf("non-marker level = %q, want it left at %q", issues[2].Level, LevelError)
	}
}
