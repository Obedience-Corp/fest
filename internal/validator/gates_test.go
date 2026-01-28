package validator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestValidateQualityGates_PhaseTypeAwareness tests that validation checks for
// the correct gates based on phase type, not just implementation gates.
func TestValidateQualityGates_PhaseTypeAwareness(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		wantIssues  int
		wantMessage string // If set, verify the error message contains this
	}{
		{
			// BUG: This currently fails because APPROVAL is not in the skip list
			// and gets checked against implementation gates instead of review gates
			name: "review_phase_with_review_gates_should_pass",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				// Create a review/approval phase with review gates
				phasePath := filepath.Join(dir, "003_APPROVAL")
				seqPath := filepath.Join(phasePath, "01_merge_preparation")
				os.MkdirAll(seqPath, 0755)

				// Write PHASE_GOAL.md with review phase type
				goalContent := `---
fest_phase_type: review
---
# Phase Goal
Review and approve changes.
`
				os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)

				// Write review gate tasks (checklist and sign_off)
				os.WriteFile(filepath.Join(seqPath, "01_review_changes.md"), []byte("# Review Changes"), 0644)
				os.WriteFile(filepath.Join(seqPath, "02_checklist.md"), []byte("# Review Checklist"), 0644)
				os.WriteFile(filepath.Join(seqPath, "03_sign_off.md"), []byte("# Sign-off"), 0644)

				return dir
			},
			wantIssues: 0, // Should pass with review gates
		},
		{
			// BUG: This currently fails because APPROVAL is not in skip list
			name: "approval_phase_with_sign_off_gate_should_pass",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "003_APPROVAL")
				seqPath := filepath.Join(phasePath, "01_final_review")
				os.MkdirAll(seqPath, 0755)

				// No frontmatter - should infer from name containing "APPROVAL"
				// which maps to review type

				os.WriteFile(filepath.Join(seqPath, "01_verify.md"), []byte("# Verify"), 0644)
				os.WriteFile(filepath.Join(seqPath, "02_sign_off.md"), []byte("# Sign-off"), 0644)

				return dir
			},
			wantIssues: 0, // Should pass - sign_off is a review gate
		},
		{
			name: "implementation_phase_with_impl_gates_passes",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "002_IMPLEMENTATION")
				seqPath := filepath.Join(phasePath, "01_core_feature")
				os.MkdirAll(seqPath, 0755)

				os.WriteFile(filepath.Join(seqPath, "01_implement.md"), []byte("# Implement"), 0644)
				os.WriteFile(filepath.Join(seqPath, "02_testing.md"), []byte("# Testing"), 0644)
				os.WriteFile(filepath.Join(seqPath, "03_review.md"), []byte("# Code Review"), 0644)
				os.WriteFile(filepath.Join(seqPath, "04_commit.md"), []byte("# Commit"), 0644)

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
				os.MkdirAll(seqPath, 0755)

				// Only implementation tasks, no gates
				os.WriteFile(filepath.Join(seqPath, "01_implement.md"), []byte("# Implement"), 0644)
				os.WriteFile(filepath.Join(seqPath, "02_more_work.md"), []byte("# More Work"), 0644)

				return dir
			},
			wantIssues:  1,
			wantMessage: "missing quality gates",
		},
		{
			// Non-implementation phases should also be validated for their gates
			name: "review_phase_missing_review_gates_should_fail",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "003_REVIEW")
				seqPath := filepath.Join(phasePath, "01_qa_testing")
				os.MkdirAll(seqPath, 0755)

				// Set phase type explicitly
				goalContent := `---
fest_phase_type: review
---
# Phase Goal
`
				os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)

				// Tasks but no review gates (checklist, sign_off)
				os.WriteFile(filepath.Join(seqPath, "01_run_tests.md"), []byte("# Run Tests"), 0644)
				os.WriteFile(filepath.Join(seqPath, "02_check_results.md"), []byte("# Check Results"), 0644)

				return dir
			},
			wantIssues:  1,
			wantMessage: "missing quality gates",
		},
		{
			name: "planning_phase_with_planning_gates_passes",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "001_PLANNING")
				seqPath := filepath.Join(phasePath, "01_requirements")
				os.MkdirAll(seqPath, 0755)

				goalContent := `---
fest_phase_type: planning
---
# Phase Goal
`
				os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)

				// Planning gates: plan_review, approval
				os.WriteFile(filepath.Join(seqPath, "01_gather_requirements.md"), []byte("# Gather"), 0644)
				os.WriteFile(filepath.Join(seqPath, "02_plan_review.md"), []byte("# Plan Review"), 0644)
				os.WriteFile(filepath.Join(seqPath, "03_approval.md"), []byte("# Approval"), 0644)

				return dir
			},
			wantIssues: 0,
		},
		{
			name: "research_phase_with_research_gates_passes",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "001_RESEARCH")
				seqPath := filepath.Join(phasePath, "01_investigation")
				os.MkdirAll(seqPath, 0755)

				goalContent := `---
fest_phase_type: research
---
# Phase Goal
`
				os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)

				// Research gates: findings_review, documentation
				os.WriteFile(filepath.Join(seqPath, "01_investigate.md"), []byte("# Investigate"), 0644)
				os.WriteFile(filepath.Join(seqPath, "02_findings_review.md"), []byte("# Findings Review"), 0644)
				os.WriteFile(filepath.Join(seqPath, "03_documentation.md"), []byte("# Documentation"), 0644)

				return dir
			},
			wantIssues: 0,
		},
		{
			name: "action_phase_with_action_gates_passes",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "004_DEPLOYMENT")
				seqPath := filepath.Join(phasePath, "01_deploy_production")
				os.MkdirAll(seqPath, 0755)

				goalContent := `---
fest_phase_type: non_coding_action
---
# Phase Goal
`
				os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)

				// Action gates: action_verify, completion
				os.WriteFile(filepath.Join(seqPath, "01_prepare_deploy.md"), []byte("# Prepare"), 0644)
				os.WriteFile(filepath.Join(seqPath, "02_action_verify.md"), []byte("# Action Verify"), 0644)
				os.WriteFile(filepath.Join(seqPath, "03_completion.md"), []byte("# Completion"), 0644)

				return dir
			},
			wantIssues: 0,
		},
		{
			// Excluded sequences should not trigger validation errors
			name: "excluded_sequence_pattern_skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				phasePath := filepath.Join(dir, "002_IMPLEMENTATION")
				seqPath := filepath.Join(phasePath, "01_core_planning") // ends with _planning
				os.MkdirAll(seqPath, 0755)

				// No gates, but sequence matches exclude pattern *_planning
				os.WriteFile(filepath.Join(seqPath, "01_plan.md"), []byte("# Plan"), 0644)

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
				os.MkdirAll(seqPath, 0755)
				// No task files
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

	tests := []struct {
		name         string
		phaseType    string
		phaseName    string
		wantContains string
	}{
		{
			name:         "implementation_error_message",
			phaseType:    "implementation",
			phaseName:    "002_IMPLEMENTATION",
			wantContains: "Implementation", // Should say "Implementation sequence..."
		},
		{
			name:         "review_error_message",
			phaseType:    "review",
			phaseName:    "003_REVIEW",
			wantContains: "Review", // Should say "Review sequence..."
		},
		{
			name:         "planning_error_message",
			phaseType:    "planning",
			phaseName:    "001_PLANNING",
			wantContains: "Planning", // Should say "Planning sequence..."
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			phasePath := filepath.Join(dir, tt.phaseName)
			seqPath := filepath.Join(phasePath, "01_work")
			os.MkdirAll(seqPath, 0755)

			// Set phase type
			goalContent := "---\nfest_phase_type: " + tt.phaseType + "\n---\n# Goal\n"
			os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)

			// Add tasks but no gates - should trigger error
			os.WriteFile(filepath.Join(seqPath, "01_task.md"), []byte("# Task"), 0644)
			os.WriteFile(filepath.Join(seqPath, "02_another.md"), []byte("# Another"), 0644)

			issues, err := ValidateQualityGates(ctx, dir)
			if err != nil {
				t.Fatalf("ValidateQualityGates() error = %v", err)
			}

			if len(issues) == 0 {
				t.Fatal("Expected at least one issue for missing gates")
			}

			// Check that error message includes phase type
			if !contains(issues[0].Message, tt.wantContains) {
				t.Errorf("Error message %q should contain %q", issues[0].Message, tt.wantContains)
			}
		})
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
