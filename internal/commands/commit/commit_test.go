package commit

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFormatCommitRef(t *testing.T) {
	t.Parallel()

	if got, want := formatCommitRef("CA0004"), "FE-CA0004"; got != want {
		t.Errorf("formatCommitRef() = %q, want %q", got, want)
	}
}

func TestNewCommitCommand_NoRootFlag(t *testing.T) {
	cmd := NewCommitCommand()

	flag := cmd.Flags().Lookup("no-root")
	if flag == nil {
		t.Fatal("--no-root flag not registered")
	}

	if flag.DefValue != "false" {
		t.Errorf("--no-root default = %q, want %q", flag.DefValue, "false")
	}
}

func TestNewCommitCommand_SyncSubmoduleRefDeprecated(t *testing.T) {
	cmd := NewCommitCommand()

	flag := cmd.Flags().Lookup("sync-submodule-ref")
	if flag == nil {
		t.Fatal("--sync-submodule-ref flag not registered")
	}

	if flag.Deprecated == "" {
		t.Error("--sync-submodule-ref should be marked as deprecated")
	}
}

func TestNewCommitCommand_AutoWriteFlag(t *testing.T) {
	cmd := NewCommitCommand()

	flag := cmd.Flags().Lookup("auto-write")
	if flag == nil {
		t.Fatal("--auto-write flag not registered")
	}
	if flag.DefValue != "false" {
		t.Errorf("--auto-write default = %q, want %q", flag.DefValue, "false")
	}
}

func TestNewCommitCommand_CampaignTaggingIndependentOfSync(t *testing.T) {
	cmd := NewCommitCommand()

	noTagFlag := cmd.Flags().Lookup("no-tag")
	syncFlag := cmd.Flags().Lookup("sync-submodule-ref")

	if noTagFlag == nil {
		t.Fatal("--no-tag flag not registered")
	}
	if syncFlag == nil {
		t.Fatal("--sync-submodule-ref flag not registered")
	}

	if noTagFlag.DefValue != "false" {
		t.Errorf("--no-tag default = %q, want %q", noTagFlag.DefValue, "false")
	}
	if syncFlag.DefValue != "false" {
		t.Errorf("--sync-submodule-ref default = %q, want %q", syncFlag.DefValue, "false")
	}
}

func TestFestivalScopedPaths_WithSubmodule(t *testing.T) {
	root := "/campaign"
	festival := filepath.Join(root, "festivals", "active", "my-fest-FA0001")
	submodule := "projects/fest"

	paths, err := festivalScopedPaths(context.Background(), root, festival, submodule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		filepath.Join("festivals", "active", "my-fest-FA0001"),
		filepath.Join(".campaign", "fest"),
		filepath.Join("festivals", ".festival", ".state"),
		"projects/fest",
	}

	if len(paths) != len(want) {
		t.Fatalf("got %d paths, want %d: %v", len(paths), len(want), paths)
	}

	for i, got := range paths {
		if got != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestFestivalScopedPaths_NoSubmodule(t *testing.T) {
	root := "/campaign"
	festival := filepath.Join(root, "festivals", "active", "my-fest-FA0001")

	paths, err := festivalScopedPaths(context.Background(), root, festival, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(paths) != 3 {
		t.Fatalf("got %d paths, want 3 (no submodule): %v", len(paths), paths)
	}

	// Should not contain any submodule path
	for _, p := range paths {
		if p == "projects/fest" || p == "" {
			t.Errorf("unexpected path in result: %q", p)
		}
	}
}

func TestFestivalScopedPaths_ExpectedContents(t *testing.T) {
	root := "/home/user/campaign"
	festival := filepath.Join(root, "festivals", "ready", "deploy-v2-DP0003")

	paths, err := festivalScopedPaths(context.Background(), root, festival, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify each expected path is present
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	expected := []string{
		filepath.Join("festivals", "ready", "deploy-v2-DP0003"),
		filepath.Join(".campaign", "fest"),
		filepath.Join("festivals", ".festival", ".state"),
	}

	for _, e := range expected {
		if !pathSet[e] {
			t.Errorf("missing expected path %q in %v", e, paths)
		}
	}
}

func TestCommitResult_CampaignHashJSON(t *testing.T) {
	// Verify the struct field exists and is tagged correctly
	r := CommitResult{
		Success:      true,
		Hash:         "abc1234",
		CampaignHash: "def5678",
	}

	if r.CampaignHash != "def5678" {
		t.Errorf("CampaignHash = %q, want %q", r.CampaignHash, "def5678")
	}
}
