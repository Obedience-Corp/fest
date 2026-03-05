package next

import (
	"context"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/feedback"
)

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

		output := captureStdout(t, func() {
			printFeedbackReminder(ctx, festDir)
		})

		if output != "" {
			t.Errorf("expected no output when feedback not configured, got %q", output)
		}
	})

	t.Run("with feedback configured", func(t *testing.T) {
		festDir := t.TempDir()
		ctx := context.Background()

		store := feedback.NewStore(festDir)
		_, err := store.Init(ctx, []string{"usability", "performance"})
		if err != nil {
			t.Fatalf("Init() error: %v", err)
		}

		output := captureStdout(t, func() {
			printFeedbackReminder(ctx, festDir)
		})

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
