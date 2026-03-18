package validator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestValidateQualityGates_ImplementationOnly tests that validation only checks
// implementation phases for quality gates. Non-implementation phases are skipped.
func TestValidateQualityGates_ImplementationOnly(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		wantIssues  int
		wantMessage string // If set, verify the error message contains this
	}{
		{
			// Review phases are skipped entirely - no gates required
			name: "review_phase_skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "003_APPROVAL")
				seqPath := filepath.Join(phasePath, "01_merge_preparation")
				_ = os.MkdirAll(seqPath, 0755)

				goalContent := `---
fest_phase_type: review
---
# Phase Goal
Review and approve changes.
`
				_ = os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)
				_ = os.WriteFile(filepath.Join(seqPath, "01_review_changes.md"), []byte("# Review Changes"), 0644)

				return dir
			},
			wantIssues: 0, // Review phases are skipped
		},
		{
			name: "implementation_phase_with_impl_gates_passes",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "002_IMPLEMENTATION")
				seqPath := filepath.Join(phasePath, "01_core_feature")
				_ = os.MkdirAll(seqPath, 0755)

				_ = os.WriteFile(filepath.Join(seqPath, "01_implement.md"), []byte("# Implement"), 0644)
				_ = os.WriteFile(filepath.Join(seqPath, "02_testing.md"), []byte("# Testing"), 0644)
				_ = os.WriteFile(filepath.Join(seqPath, "03_review.md"), []byte("# Code Review"), 0644)
				_ = os.WriteFile(filepath.Join(seqPath, "04_iterate.md"), []byte("# Review Results and Iterate"), 0644)
				_ = os.WriteFile(filepath.Join(seqPath, "05_commit.md"), []byte("# Commit"), 0644)

				return dir
			},
			wantIssues: 0,
		},
		{
			name: "implementation_phase_missing_gates_fails",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "002_IMPLEMENTATION")
				seqPath := filepath.Join(phasePath, "01_core_feature")
				_ = os.MkdirAll(seqPath, 0755)

				_ = os.WriteFile(filepath.Join(seqPath, "01_implement.md"), []byte("# Implement"), 0644)
				_ = os.WriteFile(filepath.Join(seqPath, "02_more_work.md"), []byte("# More Work"), 0644)

				return dir
			},
			wantIssues:  1,
			wantMessage: "missing quality gates",
		},
		{
			// Planning phases are skipped entirely
			name: "planning_phase_skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "001_PLANNING")
				seqPath := filepath.Join(phasePath, "01_requirements")
				_ = os.MkdirAll(seqPath, 0755)

				goalContent := `---
fest_phase_type: planning
---
# Phase Goal
`
				_ = os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)
				_ = os.WriteFile(filepath.Join(seqPath, "01_gather_requirements.md"), []byte("# Gather"), 0644)

				return dir
			},
			wantIssues: 0, // Planning phases skipped
		},
		{
			// Research phases are skipped entirely
			name: "research_phase_skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "001_RESEARCH")
				seqPath := filepath.Join(phasePath, "01_investigation")
				_ = os.MkdirAll(seqPath, 0755)

				goalContent := `---
fest_phase_type: research
---
# Phase Goal
`
				_ = os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)
				_ = os.WriteFile(filepath.Join(seqPath, "01_investigate.md"), []byte("# Investigate"), 0644)

				return dir
			},
			wantIssues: 0, // Research phases skipped
		},
		{
			// Non-coding action phases are skipped entirely
			name: "action_phase_skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "004_DEPLOYMENT")
				seqPath := filepath.Join(phasePath, "01_deploy_production")
				_ = os.MkdirAll(seqPath, 0755)

				goalContent := `---
fest_phase_type: non_coding_action
---
# Phase Goal
`
				_ = os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)
				_ = os.WriteFile(filepath.Join(seqPath, "01_prepare_deploy.md"), []byte("# Prepare"), 0644)

				return dir
			},
			wantIssues: 0, // Action phases skipped
		},
		{
			// Excluded sequences should not trigger validation errors
			name: "excluded_sequence_pattern_skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "002_IMPLEMENTATION")
				seqPath := filepath.Join(phasePath, "01_core_planning") // ends with _planning
				_ = os.MkdirAll(seqPath, 0755)

				_ = os.WriteFile(filepath.Join(seqPath, "01_plan.md"), []byte("# Plan"), 0644)

				return dir
			},
			wantIssues: 0, // Should be excluded by pattern
		},
		{
			// Empty sequence (no tasks) should not trigger error
			name: "empty_sequence_no_error",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "002_IMPLEMENTATION")
				seqPath := filepath.Join(phasePath, "01_core_feature")
				_ = os.MkdirAll(seqPath, 0755)
				return dir
			},
			wantIssues: 0, // Empty sequences are OK
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			issues, err := ValidateQualityGates(ctx, dir)
			if err != nil {
				t.Fatalf("ValidateQualityGates() error = %v", err)
			}

			if len(issues) != tt.wantIssues {
				t.Errorf("ValidateQualityGates() got %d issues, want %d", len(issues), tt.wantIssues)
				for _, issue := range issues {
					t.Logf("  Issue: [%s] %s - %s", issue.Level, issue.Code, issue.Message)
					t.Logf("    Path: %s", issue.Path)
				}
			}

			if tt.wantMessage != "" && len(issues) > 0 {
				found := false
				for _, issue := range issues {
					if contains(issue.Message, tt.wantMessage) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected issue message containing %q, got:", tt.wantMessage)
					for _, issue := range issues {
						t.Logf("  %s", issue.Message)
					}
				}
			}
		})
	}
}

// TestValidateQualityGates_ErrorMessage tests that error messages include the phase type
func TestValidateQualityGates_ErrorMessage(t *testing.T) {
	ctx := context.Background()

	// Only implementation phases generate error messages (others are skipped)
	dir := t.TempDir()
	phasePath := filepath.Join(dir, "002_IMPLEMENTATION")
	seqPath := filepath.Join(phasePath, "01_work")
	_ = os.MkdirAll(seqPath, 0755)

	goalContent := "---\nfest_phase_type: implementation\n---\n# Goal\n"
	_ = os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)

	_ = os.WriteFile(filepath.Join(seqPath, "01_task.md"), []byte("# Task"), 0644)
	_ = os.WriteFile(filepath.Join(seqPath, "02_another.md"), []byte("# Another"), 0644)

	issues, err := ValidateQualityGates(ctx, dir)
	if err != nil {
		t.Fatalf("ValidateQualityGates() error = %v", err)
	}

	if len(issues) == 0 {
		t.Fatal("Expected at least one issue for missing gates")
	}

	if !contains(issues[0].Message, "Implementation") {
		t.Errorf("Error message %q should contain %q", issues[0].Message, "Implementation")
	}
}

func TestValidateQualityGates_CustomGateFilesPass(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	phasePath := filepath.Join(dir, "002_IMPLEMENTATION")
	seqPath := filepath.Join(phasePath, "01_core_feature")
	if err := os.MkdirAll(seqPath, 0755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}

	files := map[string]string{
		filepath.Join(phasePath, "PHASE_GOAL.md"):     "---\nfest_phase_type: implementation\n---\n# Goal\n",
		filepath.Join(seqPath, "01_build_feature.md"): "# Build Feature\n",
		filepath.Join(seqPath, "02_quality_gate_testing.md"): `---
fest_type: gate
fest_status: pending
---
# Gate: Testing and Verification
`,
		filepath.Join(seqPath, "03_quality_gate_review.md"): `---
fest_type: gate
fest_status: pending
---
# Gate: Code Review
`,
		filepath.Join(seqPath, "04_quality_gate_iterate.md"): `---
fest_type: gate
fest_status: pending
---
# Gate: Review Results and Iterate
`,
		filepath.Join(seqPath, "05_quality_gate_commit.md"): `---
fest_type: gate
fest_status: pending
---
# Gate: Commit Sequence Changes
`,
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	issues, err := ValidateQualityGates(ctx, dir)
	if err != nil {
		t.Fatalf("ValidateQualityGates() error = %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("ValidateQualityGates() got issues = %+v, want none", issues)
	}
}

func TestValidateQualityGates_RequiresEachExpectedGate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	phasePath := filepath.Join(dir, "002_IMPLEMENTATION")
	seqPath := filepath.Join(phasePath, "01_core_feature")
	if err := os.MkdirAll(seqPath, 0755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}

	files := map[string]string{
		filepath.Join(phasePath, "PHASE_GOAL.md"):     "---\nfest_phase_type: implementation\n---\n# Goal\n",
		filepath.Join(seqPath, "01_build_feature.md"): "# Build Feature\n",
		filepath.Join(seqPath, "02_testing.md"):       "# Testing\n",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	issues, err := ValidateQualityGates(ctx, dir)
	if err != nil {
		t.Fatalf("ValidateQualityGates() error = %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("ValidateQualityGates() got %d issues, want 1", len(issues))
	}

	message := issues[0].Message
	for _, want := range []string{"review", "iterate", "fest-commit"} {
		if !contains(message, want) {
			t.Fatalf("ValidateQualityGates() message = %q, want to contain %q", message, want)
		}
	}
}

// contains checks if s contains substr (simple helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
