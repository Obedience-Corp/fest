package validator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// setupFestival creates a temporary festival directory structure for testing.
// The structure map keys are relative paths; values are file contents.
// Directories are created by including paths with "/" separators.
func setupFestival(t *testing.T, structure map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for relPath, content := range structure {
		fullPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func countIssuesByCode(issues []Issue, code string) int {
	count := 0
	for _, iss := range issues {
		if iss.Code == code {
			count++
		}
	}
	return count
}

func TestCompletenessValidator_ImplementationPhaseRequiresSequences(t *testing.T) {
	tests := []struct {
		name             string
		structure        map[string]string
		wantSequenceErrs int
		wantTotalErrors  int
	}{
		{
			name: "implementation phase with sequences passes",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":                  "overview",
				"001_IMPLEMENT/PHASE_GOAL.md":           "goal",
				"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "goal",
			},
			wantSequenceErrs: 0,
		},
		{
			name: "implementation phase without sequences fails",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":        "overview",
				"001_IMPLEMENT/PHASE_GOAL.md": "goal",
			},
			wantSequenceErrs: 1,
		},
		{
			name: "planning phase without sequences passes",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":       "overview",
				"001_PLANNING/PHASE_GOAL.md": "goal",
			},
			wantSequenceErrs: 0,
		},
		{
			name: "plan phase without sequences passes",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":   "overview",
				"001_PLAN/PHASE_GOAL.md": "goal",
			},
			wantSequenceErrs: 0,
		},
		{
			name: "research phase without sequences passes",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":       "overview",
				"001_RESEARCH/PHASE_GOAL.md": "goal",
			},
			wantSequenceErrs: 0,
		},
		{
			name: "ingest phase without sequences passes",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":     "overview",
				"001_INGEST/PHASE_GOAL.md": "goal",
			},
			wantSequenceErrs: 0,
		},
		{
			name: "review phase without sequences passes",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":     "overview",
				"001_REVIEW/PHASE_GOAL.md": "goal",
			},
			wantSequenceErrs: 0,
		},
		{
			name: "non_coding_action phase without sequences passes",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":                "overview",
				"001_NON_CODING_ACTION/PHASE_GOAL.md": "goal",
			},
			wantSequenceErrs: 0,
		},
		{
			name: "design phase without sequences passes",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":     "overview",
				"001_DESIGN/PHASE_GOAL.md": "goal",
			},
			wantSequenceErrs: 0,
		},
		{
			name: "multiple implementation phases some missing sequences",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":                  "overview",
				"001_IMPLEMENT/PHASE_GOAL.md":           "goal",
				"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "goal",
				"002_BUILD/PHASE_GOAL.md":               "goal",
			},
			wantSequenceErrs: 1, // 002_BUILD has no sequences
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupFestival(t, tt.structure)
			ctx := context.Background()
			issues, err := ValidateCompleteness(ctx, dir)
			if err != nil {
				t.Fatalf("ValidateCompleteness() error = %v", err)
			}
			seqErrs := countIssuesByCode(issues, CodeMissingSequence)
			if seqErrs != tt.wantSequenceErrs {
				t.Errorf("got %d missing_sequence errors, want %d; issues: %+v", seqErrs, tt.wantSequenceErrs, issues)
			}
		})
	}
}

func TestCompletenessValidator_RequiredFiles(t *testing.T) {
	tests := []struct {
		name      string
		structure map[string]string
		wantCode  string
		wantCount int
	}{
		{
			name: "missing FESTIVAL_OVERVIEW.md is error",
			structure: map[string]string{
				"FESTIVAL_RULES.md": "rules", // provide rules so only overview is missing
			},
			wantCode:  CodeMissingFile,
			wantCount: 1,
		},
		{
			name: "missing PHASE_GOAL.md is error",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":      "overview",
				"001_IMPLEMENT/placeholder": "", // dir exists but no PHASE_GOAL.md
			},
			wantCode:  CodeMissingGoal,
			wantCount: 1,
		},
		{
			name: "missing SEQUENCE_GOAL.md is error",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":            "overview",
				"001_IMPLEMENT/PHASE_GOAL.md":     "goal",
				"001_IMPLEMENT/01_seq/01_task.md": "task",
			},
			wantCode:  CodeMissingGoal,
			wantCount: 1,
		},
		{
			name: "all required files present gives no errors for those codes",
			structure: map[string]string{
				"FESTIVAL_OVERVIEW.md":                  "overview",
				"FESTIVAL_RULES.md":                     "rules",
				"001_IMPLEMENT/PHASE_GOAL.md":           "goal",
				"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "goal",
			},
			wantCode:  CodeMissingFile,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupFestival(t, tt.structure)
			ctx := context.Background()
			issues, err := ValidateCompleteness(ctx, dir)
			if err != nil {
				t.Fatalf("ValidateCompleteness() error = %v", err)
			}
			count := countIssuesByCode(issues, tt.wantCode)
			if count != tt.wantCount {
				t.Errorf("got %d %s issues, want %d; issues: %+v", count, tt.wantCode, tt.wantCount, issues)
			}
		})
	}
}

func TestCompletenessValidator_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ValidateCompleteness(ctx, t.TempDir())
	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}

func TestIsImplementationPhase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantImpl bool
	}{
		{"IMPLEMENT is implementation", "IMPLEMENT", true},
		{"IMPLEMENTATION is implementation", "IMPLEMENTATION", true},
		{"BUILD is implementation", "BUILD", true},
		{"EXECUTE is implementation", "EXECUTE", true},
		{"PLANNING is not", "PLANNING", false},
		{"PLAN is not", "PLAN", false},
		{"RESEARCH is not", "RESEARCH", false},
		{"INGEST is not", "INGEST", false},
		{"REVIEW is not", "REVIEW", false},
		{"DESIGN is not", "DESIGN", false},
		{"NON_CODING_ACTION is not", "NON_CODING_ACTION", false},
		{"NON_CODING is not", "NON_CODING", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isImplementationPhase(tt.input)
			if got != tt.wantImpl {
				t.Errorf("isImplementationPhase(%q) = %v, want %v", tt.input, got, tt.wantImpl)
			}
		})
	}
}
