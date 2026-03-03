package shared

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveFestivalSelector_ExactID(t *testing.T) {
	campaignRoot := createSelectorWorkspace(t)
	festivalPath := createSelectorFestival(t, campaignRoot, "active", "launch-readiness-LR0001")

	resolved, err := ResolveFestivalSelector(context.Background(), campaignRoot, "LR0001")
	if err != nil {
		t.Fatalf("ResolveFestivalSelector() unexpected error: %v", err)
	}
	if resolved != festivalPath {
		t.Fatalf("ResolveFestivalSelector() = %q, want %q", resolved, festivalPath)
	}
}

func TestResolveFestivalSelector_ExactName(t *testing.T) {
	campaignRoot := createSelectorWorkspace(t)
	festivalPath := createSelectorFestival(t, campaignRoot, "planning", "public-launch-PL0007")

	resolved, err := ResolveFestivalSelector(context.Background(), campaignRoot, "public-launch-PL0007")
	if err != nil {
		t.Fatalf("ResolveFestivalSelector() unexpected error: %v", err)
	}
	if resolved != festivalPath {
		t.Fatalf("ResolveFestivalSelector() = %q, want %q", resolved, festivalPath)
	}
}

func TestResolveFestivalSelector_FuzzySingleMatch(t *testing.T) {
	campaignRoot := createSelectorWorkspace(t)
	expected := createSelectorFestival(t, campaignRoot, "active", "launch-readiness-LR0001")
	createSelectorFestival(t, campaignRoot, "ready", "docs-refresh-DR0002")

	resolved, err := ResolveFestivalSelector(context.Background(), campaignRoot, "readi")
	if err != nil {
		t.Fatalf("ResolveFestivalSelector() unexpected error: %v", err)
	}
	if resolved != expected {
		t.Fatalf("ResolveFestivalSelector() = %q, want %q", resolved, expected)
	}
}

func TestResolveFestivalSelector_Ambiguous(t *testing.T) {
	campaignRoot := createSelectorWorkspace(t)
	createSelectorFestival(t, campaignRoot, "active", "launch-alpha-LA0001")
	createSelectorFestival(t, campaignRoot, "planning", "launch-beta-LB0002")

	_, err := ResolveFestivalSelector(context.Background(), campaignRoot, "launch")
	if err == nil {
		t.Fatal("expected ambiguous selector error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Fatalf("expected ambiguous error, got: %v", err)
	}
}

func TestResolveFestivalSelector_NotFound(t *testing.T) {
	campaignRoot := createSelectorWorkspace(t)
	createSelectorFestival(t, campaignRoot, "active", "launch-alpha-LA0001")

	_, err := ResolveFestivalSelector(context.Background(), campaignRoot, "does-not-exist")
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestResolveFestivalSelector_RequiresCampaignWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "festivals"), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveFestivalSelector(context.Background(), tmpDir, "LR0001")
	if err == nil {
		t.Fatal("expected campaign workspace error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "campaign workspace") {
		t.Fatalf("expected campaign workspace error, got: %v", err)
	}
}

func TestCompleteFestivalSelector(t *testing.T) {
	campaignRoot := createSelectorWorkspace(t)
	createSelectorFestival(t, campaignRoot, "active", "launch-readiness-LR0001")
	createSelectorFestival(t, campaignRoot, "planning", "docs-refresh-DR0002")

	all, err := CompleteFestivalSelector(context.Background(), campaignRoot, "")
	if err != nil {
		t.Fatalf("CompleteFestivalSelector() unexpected error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("CompleteFestivalSelector() len = %d, want 2 (%v)", len(all), all)
	}

	filtered, err := CompleteFestivalSelector(context.Background(), campaignRoot, "doc")
	if err != nil {
		t.Fatalf("CompleteFestivalSelector() unexpected error: %v", err)
	}
	if len(filtered) == 0 {
		t.Fatal("CompleteFestivalSelector() expected at least one fuzzy match")
	}
	if filtered[0] != "docs-refresh-DR0002" {
		t.Fatalf("CompleteFestivalSelector() first match = %q, want %q", filtered[0], "docs-refresh-DR0002")
	}
}

func createSelectorWorkspace(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "campaign")
	if err := os.MkdirAll(filepath.Join(root, ".campaign"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "festivals"), 0755); err != nil {
		t.Fatal(err)
	}
	return root
}

func createSelectorFestival(t *testing.T, campaignRoot, status, name string) string {
	t.Helper()
	festivalPath := filepath.Join(campaignRoot, "festivals", status, name)
	if err := os.MkdirAll(festivalPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalPath, "FESTIVAL_GOAL.md"), []byte("# Goal\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return festivalPath
}
