package commit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/scope"
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

func TestRunCommit_CleanProjectWithoutFestivalReturnsNoChanges(t *testing.T) {
	campaign := t.TempDir()
	project := filepath.Join(campaign, "projects", "app")
	if err := os.MkdirAll(filepath.Join(campaign, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, repo := range []string{campaign, project} {
		cmd := exec.Command("git", "init", "-q", repo)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init %s: %v\n%s", repo, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(project, "app.txt"), []byte("initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-C", project, "add", "app.txt"},
		{"-C", project, "-c", "user.name=Fest Test", "-c", "user.email=fest@example.com", "commit", "-q", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	t.Chdir(project)
	cmd := NewCommitCommand()
	cmd.SetContext(scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{
		Root: campaign,
		Type: scope.WorkspaceTypeCampaign,
	}))
	message = "chore: no changes"
	autoStage = true
	autoWrite = false
	noRoot = false
	noTag = true
	jsonOut = false
	festivalFlag = ""
	taskRef = ""

	err := runCommit(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "no changes to commit") {
		t.Fatalf("runCommit error = %v, want no changes to commit", err)
	}
}

// makeDirs creates each campaign-root-relative directory under root.
func makeDirs(t *testing.T, root string, rel ...string) {
	t.Helper()
	for _, r := range rel {
		if err := os.MkdirAll(filepath.Join(root, r), 0o755); err != nil {
			t.Fatalf("creating %s: %v", r, err)
		}
	}
}

// livedInCampaign is a campaign fest has already navigated in: the festival,
// both fest-owned state directories, and a project submodule all on disk.
func livedInCampaign(t *testing.T) (root, festival string) {
	t.Helper()
	root = t.TempDir()
	makeDirs(t, root,
		filepath.Join("festivals", "active", "my-fest-FA0001"),
		filepath.Join(".campaign", "fest"),
		filepath.Join("festivals", ".festival", ".state"),
		filepath.Join("projects", "fest"),
	)
	return root, filepath.Join(root, "festivals", "active", "my-fest-FA0001")
}

func TestFestivalScopedPaths_WithSubmodule(t *testing.T) {
	root, festival := livedInCampaign(t)
	submodule := filepath.Join("projects", "fest")

	paths, err := festivalScopedPaths(context.Background(), root, festival, submodule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		filepath.Join("festivals", "active", "my-fest-FA0001"),
		filepath.Join(".campaign", "fest"),
		filepath.Join("festivals", ".festival", ".state"),
		filepath.Join("projects", "fest"),
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
	root, festival := livedInCampaign(t)

	paths, err := festivalScopedPaths(context.Background(), root, festival, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(paths) != 3 {
		t.Fatalf("got %d paths, want 3 (no submodule): %v", len(paths), paths)
	}

	// Should not contain any submodule path
	for _, p := range paths {
		if p == filepath.Join("projects", "fest") || p == "" {
			t.Errorf("unexpected path in result: %q", p)
		}
	}
}

func TestFestivalScopedPaths_ExpectedContents(t *testing.T) {
	root := t.TempDir()
	makeDirs(t, root,
		filepath.Join("festivals", "ready", "deploy-v2-DP0003"),
		filepath.Join(".campaign", "fest"),
		filepath.Join("festivals", ".festival", ".state"),
	)
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

// A campaign fresh from `camp init` has neither fest-owned state directory.
// Listing them anyway aborts the whole `git add`, so the first fest commit in
// a new campaign never happened.
func TestFestivalScopedPaths_FreshCampaignOmitsAbsentStatePaths(t *testing.T) {
	root := t.TempDir()
	makeDirs(t, root, filepath.Join("festivals", "active", "my-fest-FA0001"))
	festival := filepath.Join(root, "festivals", "active", "my-fest-FA0001")

	paths, err := festivalScopedPaths(context.Background(), root, festival, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{filepath.Join("festivals", "active", "my-fest-FA0001")}
	if len(paths) != len(want) || paths[0] != want[0] {
		t.Fatalf("got %v, want %v", paths, want)
	}
}

// Only the absent paths are dropped: a campaign missing just one of the two
// state directories still stages the other.
func TestFestivalScopedPaths_PartialStatePaths(t *testing.T) {
	root := t.TempDir()
	makeDirs(t, root,
		filepath.Join("festivals", "active", "my-fest-FA0001"),
		filepath.Join(".campaign", "fest"),
	)
	festival := filepath.Join(root, "festivals", "active", "my-fest-FA0001")

	paths, err := festivalScopedPaths(context.Background(), root, festival, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		filepath.Join("festivals", "active", "my-fest-FA0001"),
		filepath.Join(".campaign", "fest"),
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

// Absence from the working tree is not absence from git: a tracked path the
// user deleted must still be staged, because staging it is what records the
// deletion.
func TestFestivalScopedPaths_KeepsTrackedButDeletedPath(t *testing.T) {
	root, festival := livedInCampaign(t)
	navFile := filepath.Join(root, ".campaign", "fest", "navigation.json")
	if err := os.WriteFile(navFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init"}, {"add", "--", ".campaign/fest"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	if err := os.RemoveAll(filepath.Join(root, ".campaign", "fest")); err != nil {
		t.Fatal(err)
	}

	paths, err := festivalScopedPaths(context.Background(), root, festival, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, p := range paths {
		if p == filepath.Join(".campaign", "fest") {
			found = true
		}
	}
	if !found {
		t.Errorf("tracked-but-deleted %q must stay staged so the deletion is recorded, got %v",
			filepath.Join(".campaign", "fest"), paths)
	}
}

func TestMatchablePaths_ContextCancelled(t *testing.T) {
	root := t.TempDir()
	makeDirs(t, root, "present")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// A path on disk needs no git call, so it survives cancellation; an absent
	// one cannot be confirmed tracked and is dropped rather than guessed at.
	paths := matchablePaths(ctx, root, []string{"present", "absent"})
	if len(paths) != 1 || paths[0] != "present" {
		t.Fatalf("got %v, want [present]", paths)
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
