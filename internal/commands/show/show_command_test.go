package show

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShowCommand_RejectsPositionalWithFestivalFlag(t *testing.T) {
	cmd := NewShowCommand()
	cmd.SetArgs([]string{"launch-readiness-LR0001", "--festival", "LR0001"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot use positional target with --festival") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunShowBySelector_FromProjectDirInCampaign(t *testing.T) {
	campaignRoot := filepath.Join(t.TempDir(), "campaign")
	if err := os.MkdirAll(filepath.Join(campaignRoot, ".campaign"), 0755); err != nil {
		t.Fatal(err)
	}

	festivalDir := filepath.Join(campaignRoot, "festivals", "active", "launch-readiness-LR0001")
	if err := os.MkdirAll(festivalDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalDir, FestivalGoalFile), []byte("# Goal\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(campaignRoot, "projects", "app", "src")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	origCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origCWD) }()

	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}

	err = runShowBySelector(context.Background(), "LR0001", &showOptions{json: true})
	if err != nil {
		t.Fatalf("runShowBySelector() unexpected error: %v", err)
	}
}

func TestRunShowBySelector_RequiresCampaignWorkspace(t *testing.T) {
	origCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origCWD) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	err = runShowBySelector(context.Background(), "LR0001", &showOptions{})
	if err == nil {
		t.Fatal("expected campaign workspace error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "campaign workspace") {
		t.Fatalf("unexpected error: %v", err)
	}
}
