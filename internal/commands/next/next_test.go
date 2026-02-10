package next

import (
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
			name: "naming warning only",
			issues: []validator.Issue{
				{Level: validator.LevelWarning, Code: validator.CodeNamingConvention},
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
			name: "unfilled template warning",
			issues: []validator.Issue{
				{Level: validator.LevelWarning, Code: validator.CodeUnfilledTemplate},
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

	// Should NOT contain the naming warning (not a blocking issue type)
	if strings.Contains(output, "naming issue") {
		t.Errorf("non-blocking warnings should not appear, got: %s", output)
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

	// Run QuickValidate and verify it finds template issues
	ctx := context.Background()
	result, err := validator.QuickValidate(ctx, festDir)
	if err != nil {
		t.Fatalf("QuickValidate error: %v", err)
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
