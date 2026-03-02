package next

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/feedback"
	"github.com/Obedience-Corp/fest/internal/validator"
)

func TestHasBlockingIssues(t *testing.T) {
	tests := []struct {
		name   string
		issues []validator.Issue
		want   bool
	}{
		{
			name:   "no issues",
			issues: nil,
			want:   false,
		},
		{
			name: "naming warning does not block",
			issues: []validator.Issue{
				{Level: validator.LevelWarning, Code: validator.CodeNamingConvention},
			},
			want: false,
		},
		{
			name: "info only does not block",
			issues: []validator.Issue{
				{Level: validator.LevelInfo, Code: validator.CodeNamingConvention},
			},
			want: false,
		},
		{
			name: "error level issue",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeMissingFile},
			},
			want: true,
		},
		{
			name: "unfilled template warning does not block",
			issues: []validator.Issue{
				{Level: validator.LevelWarning, Code: validator.CodeUnfilledTemplate},
			},
			want: false,
		},
		{
			name: "unfilled template error blocks",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate},
			},
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &validator.Result{Issues: tc.issues}
			got := hasBlockingIssues(result)
			if got != tc.want {
				t.Errorf("hasBlockingIssues() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEmitValidationBlock(t *testing.T) {
	festivalPath := "/tmp/test-fest"
	result := &validator.Result{
		Valid: false,
		Issues: []validator.Issue{
			{
				Level:   validator.LevelWarning,
				Code:    validator.CodeUnfilledTemplate,
				Path:    "/tmp/test-fest/001_PLAN/PHASE_GOAL.md",
				Message: "File contains unfilled template marker: [FILL:",
			},
			{
				Level:   validator.LevelWarning,
				Code:    validator.CodeNamingConvention,
				Path:    "/tmp/test-fest/002_IMPL/notes.txt",
				Message: "naming issue",
			},
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := emitValidationBlock(festivalPath, result)

	w.Close()
	os.Stdout = old

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	// Should return an error
	if err == nil {
		t.Fatal("expected error from emitValidationBlock")
	}

	// Should contain STOP header
	if !strings.Contains(output, "STOP") {
		t.Errorf("expected STOP header, got: %s", output)
	}

	// Should contain the template issue with relative path
	if !strings.Contains(output, "001_PLAN/PHASE_GOAL.md") {
		t.Errorf("expected relative path in output, got: %s", output)
	}

	// Should contain the naming warning in Warnings section
	if !strings.Contains(output, "naming issue") {
		t.Errorf("warnings should appear in output, got: %s", output)
	}
	if !strings.Contains(output, "Warnings:") {
		t.Errorf("expected Warnings: section header, got: %s", output)
	}

	// Should suggest running fest validate
	if !strings.Contains(output, "fest validate") {
		t.Errorf("expected hint to run fest validate, got: %s", output)
	}
}

func TestNextBlocksOnUnfilledMarkers(t *testing.T) {
	// Create a temp festival with unfilled template markers
	root := t.TempDir()
	festDir := filepath.Join(root, "festivals", "active", "test-fest")
	phaseDir := filepath.Join(festDir, "001_PLAN")
	seqDir := filepath.Join(phaseDir, "01_task_seq")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a PHASE_GOAL.md with unfilled markers (validator checks [FILL:, [GUIDANCE:, {{ )
	goalContent := "---\ntitle: Plan\n---\n# Goal\n\n[FILL: Describe the phase goal here]\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a task file with markers
	taskContent := "---\ntitle: Task\n---\n# Task\n\n[GUIDANCE: Describe what to do]\n"
	if err := os.WriteFile(filepath.Join(seqDir, "01_do_thing.md"), []byte(taskContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Run FullValidate and verify it finds template issues
	ctx := context.Background()
	result, err := validator.FullValidate(ctx, festDir)
	if err != nil {
		t.Fatalf("FullValidate error: %v", err)
	}

	// hasBlockingIssues should detect the unfilled markers
	if !hasBlockingIssues(result) {
		t.Fatal("expected blocking issues for festival with unfilled markers")
	}

	// Verify specific issue codes
	hasTemplateIssue := false
	for _, issue := range result.Issues {
		if issue.Code == validator.CodeUnfilledTemplate {
			hasTemplateIssue = true
			break
		}
	}
	if !hasTemplateIssue {
		t.Error("expected unfilled_template issue code")
	}
}

func TestNextBlocksOnReplaceMarkerInImplementation(t *testing.T) {
	// Parity test: [REPLACE:] in an implementation task should block fest next
	// because the validator now returns error-level issues for implementation phases.
	root := t.TempDir()
	festDir := filepath.Join(root, "test-fest")
	phaseDir := filepath.Join(festDir, "001_IMPL")
	seqDir := filepath.Join(phaseDir, "01_seq")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write FESTIVAL_OVERVIEW.md
	os.WriteFile(filepath.Join(festDir, "FESTIVAL_OVERVIEW.md"), []byte("# Test\n"), 0644)

	// Write PHASE_GOAL.md with implementation phase type
	goalContent := "---\nfest_type: phase\nfest_phase_type: implementation\nfest_status: pending\nfest_id: P001\nfest_parent: F001\nfest_order: 1\nfest_created: 2026-01-01T00:00:00Z\n---\n# Phase Goal\nImplement the thing.\n"
	os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644)

	// Write SEQUENCE_GOAL.md
	os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte("# Sequence Goal\n"), 0644)

	// Write a task file with [REPLACE:] marker
	taskContent := "# Task: Do Thing\n\n## Objective\n[REPLACE: Describe the objective]\n"
	os.WriteFile(filepath.Join(seqDir, "01_do_thing.md"), []byte(taskContent), 0644)

	ctx := context.Background()
	result, err := validator.FullValidate(ctx, festDir)
	if err != nil {
		t.Fatalf("FullValidate error: %v", err)
	}

	// Verify [REPLACE:] is detected as an error in implementation phase
	hasError := false
	for _, issue := range result.Issues {
		if issue.Code == validator.CodeUnfilledTemplate && issue.Level == validator.LevelError {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("expected error-level unfilled_template issue for [REPLACE:] in implementation phase")
	}

	// hasBlockingIssues should block
	if !hasBlockingIssues(result) {
		t.Error("expected hasBlockingIssues to return true for [REPLACE:] in implementation phase")
	}
}

func TestNextDoesNotBlockOnPlanningWarnings(t *testing.T) {
	// Planning-phase markers should produce warnings, which no longer block fest next.
	root := t.TempDir()
	festDir := filepath.Join(root, "test-fest")
	phaseDir := filepath.Join(festDir, "001_PLAN")
	seqDir := filepath.Join(phaseDir, "01_seq")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(festDir, "FESTIVAL_OVERVIEW.md"), []byte("# Test\n"), 0644)

	// Planning phase type → markers should be warnings
	goalContent := "---\nfest_type: phase\nfest_phase_type: planning\nfest_status: pending\nfest_id: P001\nfest_parent: F001\nfest_order: 1\nfest_created: 2026-01-01T00:00:00Z\n---\n# Phase Goal\nPlan the thing.\n"
	os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644)

	os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte("# Sequence Goal\n"), 0644)

	taskContent := "# Task\n\n[FILL: describe planning approach]\n"
	os.WriteFile(filepath.Join(seqDir, "01_plan_task.md"), []byte(taskContent), 0644)

	ctx := context.Background()
	result, err := validator.FullValidate(ctx, festDir)
	if err != nil {
		t.Fatalf("FullValidate error: %v", err)
	}

	// Verify markers are warnings, not errors
	for _, issue := range result.Issues {
		if issue.Code == validator.CodeUnfilledTemplate && issue.Level == validator.LevelError {
			t.Errorf("planning phase markers should be warnings, got error: %+v", issue)
		}
	}

	// hasBlockingIssues should NOT block (warnings don't block)
	if hasBlockingIssues(result) {
		t.Error("expected hasBlockingIssues to return false for planning-phase warnings")
	}
}

func TestLoadFeedbackCriteria(t *testing.T) {
	t.Run("no feedback configured", func(t *testing.T) {
		festDir := t.TempDir()
		ctx := context.Background()
		criteria := loadFeedbackCriteria(ctx, festDir)
		if criteria != nil {
			t.Errorf("expected nil criteria, got %v", criteria)
		}
	})

	t.Run("feedback configured", func(t *testing.T) {
		festDir := t.TempDir()
		ctx := context.Background()

		// Initialize feedback with criteria
		store := feedback.NewStore(festDir)
		_, err := store.Init(ctx, []string{"usability", "performance", "documentation"})
		if err != nil {
			t.Fatalf("Init() error: %v", err)
		}

		criteria := loadFeedbackCriteria(ctx, festDir)
		if len(criteria) != 3 {
			t.Fatalf("expected 3 criteria, got %d", len(criteria))
		}
		if criteria[0] != "usability" {
			t.Errorf("criteria[0] = %q, want %q", criteria[0], "usability")
		}
		if criteria[1] != "performance" {
			t.Errorf("criteria[1] = %q, want %q", criteria[1], "performance")
		}
		if criteria[2] != "documentation" {
			t.Errorf("criteria[2] = %q, want %q", criteria[2], "documentation")
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		festDir := t.TempDir()
		ctx, cancel := context.WithCancel(context.Background())

		// Initialize feedback first
		store := feedback.NewStore(festDir)
		_, err := store.Init(context.Background(), []string{"test"})
		if err != nil {
			t.Fatalf("Init() error: %v", err)
		}

		cancel()
		criteria := loadFeedbackCriteria(ctx, festDir)
		if criteria != nil {
			t.Errorf("expected nil on cancelled context, got %v", criteria)
		}
	})
}

func TestPrintFeedbackReminder(t *testing.T) {
	t.Run("no feedback configured", func(t *testing.T) {
		festDir := t.TempDir()
		ctx := context.Background()

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		printFeedbackReminder(ctx, festDir)

		w.Close()
		os.Stdout = old

		var buf [4096]byte
		n, _ := r.Read(buf[:])
		output := string(buf[:n])

		if output != "" {
			t.Errorf("expected no output when feedback not configured, got %q", output)
		}
	})

	t.Run("with feedback configured", func(t *testing.T) {
		festDir := t.TempDir()
		ctx := context.Background()

		// Initialize feedback with criteria
		store := feedback.NewStore(festDir)
		_, err := store.Init(ctx, []string{"usability", "performance"})
		if err != nil {
			t.Fatalf("Init() error: %v", err)
		}

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		printFeedbackReminder(ctx, festDir)

		w.Close()
		os.Stdout = old

		var buf [4096]byte
		n, _ := r.Read(buf[:])
		output := string(buf[:n])

		if !strings.Contains(output, "Feedback Collection") {
			t.Errorf("expected 'Feedback Collection' header in output, got %q", output)
		}
		if !strings.Contains(output, "1. usability") {
			t.Errorf("expected numbered 'usability' criterion in output, got %q", output)
		}
		if !strings.Contains(output, "2. performance") {
			t.Errorf("expected numbered 'performance' criterion in output, got %q", output)
		}
		if !strings.Contains(output, "fest feedback add --criteria") {
			t.Errorf("expected recording command with --criteria flag in output, got %q", output)
		}
		if !strings.Contains(output, "--severity") {
			t.Errorf("expected optional flags in output, got %q", output)
		}
	})
}

func TestFeedbackCriteriaInNextTaskResult(t *testing.T) {
	// Verify that FeedbackCriteria is included in JSON output
	result := &feedbackTestResult{
		FeedbackCriteria: []string{"usability", "clarity"},
	}

	if len(result.FeedbackCriteria) != 2 {
		t.Errorf("expected 2 criteria, got %d", len(result.FeedbackCriteria))
	}
}

// feedbackTestResult mirrors the relevant field for testing.
type feedbackTestResult struct {
	FeedbackCriteria []string `json:"feedback_criteria,omitempty"`
}

func TestCheckPreActiveStatus(t *testing.T) {
	tests := []struct {
		name           string
		status         string
		phaseType      string
		expectBlocked  bool
		expectContains string // substring to check in error message
	}{
		{
			name:           "planning+implementation is blocked",
			status:         "planning",
			phaseType:      "implementation",
			expectBlocked:  true,
			expectContains: "festival is in planning status",
		},
		{
			name:           "planning+review is blocked",
			status:         "planning",
			phaseType:      "review",
			expectBlocked:  true,
			expectContains: "festival is in planning status",
		},
		{
			name:          "planning+ingest is allowed",
			status:        "planning",
			phaseType:     "ingest",
			expectBlocked: false,
		},
		{
			name:          "planning+research is allowed",
			status:        "planning",
			phaseType:     "research",
			expectBlocked: false,
		},
		{
			name:          "planning+planning is allowed",
			status:        "planning",
			phaseType:     "planning",
			expectBlocked: false,
		},
		{
			name:          "active+implementation is allowed",
			status:        "active",
			phaseType:     "implementation",
			expectBlocked: false,
		},
		{
			name:          "active+review is allowed",
			status:        "active",
			phaseType:     "review",
			expectBlocked: false,
		},
		{
			name:           "ready+implementation is blocked",
			status:         "ready",
			phaseType:      "implementation",
			expectBlocked:  true,
			expectContains: "festival is in ready status",
		},
		{
			name:           "ready+review is blocked",
			status:         "ready",
			phaseType:      "review",
			expectBlocked:  true,
			expectContains: "festival is in ready status",
		},
		{
			name:          "ready+ingest is allowed",
			status:        "ready",
			phaseType:     "ingest",
			expectBlocked: false,
		},
		{
			name:          "ready+research is allowed",
			status:        "ready",
			phaseType:     "research",
			expectBlocked: false,
		},
		{
			name:          "ready+planning is allowed",
			status:        "ready",
			phaseType:     "planning",
			expectBlocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			festDir := t.TempDir()
			phaseDir := filepath.Join(festDir, "001_PHASE")
			if err := os.MkdirAll(phaseDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Write fest.yaml with status history
			festYAML := "version: \"1.0\"\nmetadata:\n  id: TS0001\n  status_history:\n    - status: " + tt.status + "\n      timestamp: 2026-02-10T00:00:00Z\n"
			if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
				t.Fatal(err)
			}

			// Write PHASE_GOAL.md with phase type
			goalContent := "---\nfest_phase_type: " + tt.phaseType + "\n---\n# Phase Goal\n"
			if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644); err != nil {
				t.Fatal(err)
			}

			err := checkPreActiveStatus(festDir, phaseDir)

			if tt.expectBlocked {
				if err == nil {
					t.Error("expected blocked error, got nil")
				} else if tt.expectContains != "" && !strings.Contains(err.Error(), tt.expectContains) {
					t.Errorf("expected error containing %q, got: %v", tt.expectContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestCheckPreActiveStatus_NoConfig(t *testing.T) {
	festDir := t.TempDir()
	// No fest.yaml — should not block
	err := checkPreActiveStatus(festDir, "")
	if err != nil {
		t.Errorf("expected no error without config, got: %v", err)
	}
}

func TestCheckPreActiveStatus_NoPhasePath(t *testing.T) {
	festDir := t.TempDir()
	phaseDir := filepath.Join(festDir, "001_IMPL")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write fest.yaml as planning
	festYAML := "version: \"1.0\"\nmetadata:\n  id: TS0001\n  status_history:\n    - status: planning\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Write PHASE_GOAL.md with implementation type
	goalContent := "---\nfest_phase_type: implementation\n---\n# Phase Goal\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Pass empty phasePath — should auto-detect first phase
	err := checkPreActiveStatus(festDir, "")
	if err == nil {
		t.Error("expected blocked error when auto-detecting implementation phase, got nil")
	}
}

func TestCheckPreActiveStatus_ReadyMessage(t *testing.T) {
	festDir := t.TempDir()
	phaseDir := filepath.Join(festDir, "001_PHASE")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}

	festYAML := "version: \"1.0\"\nmetadata:\n  id: TS0001\n  status_history:\n    - status: ready\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}

	goalContent := "---\nfest_phase_type: implementation\n---\n# Phase Goal\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout to verify prompt block content
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := checkPreActiveStatus(festDir, phaseDir)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old
	output := buf.String()

	if err == nil {
		t.Fatal("expected blocked error for ready+implementation, got nil")
	}

	// Error should be short
	if !strings.Contains(err.Error(), "festival is in ready status") {
		t.Errorf("expected short error about ready status, got: %v", err)
	}

	// Prompt block on stdout should contain the detailed guidance
	if !strings.Contains(output, "STOP") {
		t.Errorf("expected prompt block header, got: %s", output)
	}
	if !strings.Contains(output, "fest promote") {
		t.Errorf("expected promote instruction in prompt block, got: %s", output)
	}
	if !strings.Contains(output, "Did the user approve") {
		t.Errorf("expected user approval check in prompt block, got: %s", output)
	}
	if !strings.Contains(output, "planning -> ready -> [active] -> completed") {
		t.Errorf("expected lifecycle in prompt block, got: %s", output)
	}
}
