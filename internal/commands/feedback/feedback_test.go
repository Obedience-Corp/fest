package feedback

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	feedbackstore "github.com/Obedience-Corp/fest/internal/feedback"
)

func TestFeedbackInitCriteriaPreservesComma(t *testing.T) {
	festDir := setupFeedbackCommandFestival(t)

	cmd := NewFeedbackCommand()
	cmd.SetArgs([]string{
		"init",
		"--criteria", "Onboarding friction, especially copied commands",
		"--criteria", "Release blockers",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	config, err := feedbackstore.NewStore(festDir).LoadConfig(cmd.Context())
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if len(config.Criteria) != 2 {
		t.Fatalf("criteria count = %d, want 2", len(config.Criteria))
	}
	if got := config.Criteria[0].Name; got != "Onboarding friction, especially copied commands" {
		t.Fatalf("criteria[0] = %q", got)
	}
}

func TestFeedbackInitIsIdempotentAndForceReplacesCriteria(t *testing.T) {
	festDir := setupFeedbackCommandFestival(t)

	store := feedbackstore.NewStore(festDir)
	if _, err := store.Init(context.Background(), []string{"Original"}); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	cmd := NewFeedbackCommand()
	cmd.SetArgs([]string{"init", "--criteria", "Ignored"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("idempotent init Execute() error: %v", err)
	}

	config, err := store.LoadConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if got := config.Criteria[0].Name; got != "Original" {
		t.Fatalf("criteria after idempotent init = %q, want Original", got)
	}

	cmd = NewFeedbackCommand()
	cmd.SetArgs([]string{"init", "--force", "--criteria", "Replacement"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("force init Execute() error: %v", err)
	}

	config, err = store.LoadConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadConfig() after force error: %v", err)
	}
	if len(config.Criteria) != 1 || config.Criteria[0].Name != "Replacement" {
		t.Fatalf("criteria after force = %#v, want Replacement", config.Criteria)
	}
}

func TestFeedbackCriteriaAddCommand(t *testing.T) {
	festDir := setupFeedbackCommandFestival(t)

	store := feedbackstore.NewStore(festDir)
	if _, err := store.Init(context.Background(), []string{"Usability"}); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	cmd := NewFeedbackCommand()
	cmd.SetArgs([]string{
		"criteria", "add",
		"--criteria", "usability",
		"--criteria", "Docs, examples, and copied commands",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("criteria add Execute() error: %v", err)
	}

	config, err := store.LoadConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if len(config.Criteria) != 2 {
		t.Fatalf("criteria count = %d, want 2", len(config.Criteria))
	}
	if got := config.Criteria[1].Name; got != "Docs, examples, and copied commands" {
		t.Fatalf("criteria[1] = %q", got)
	}
}

func setupFeedbackCommandFestival(t *testing.T) string {
	t.Helper()

	festDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte("name: test\n"), 0644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(festDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	return festDir
}
