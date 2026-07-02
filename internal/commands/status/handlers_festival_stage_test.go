package status

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/scope"
)

func TestAutoCommitStatusChange_ToleratesMissingPremovePath(t *testing.T) {
	root := t.TempDir()
	initTestRepo(t, root)

	newDir := filepath.Join(root, "festivals", "ready", "my-fest")
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "FESTIVAL_GOAL.md"), []byte("# Goal"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{
		Root:          root,
		FestivalsPath: filepath.Join(root, "festivals"),
		Type:          scope.WorkspaceTypeStandalone,
	})

	missingOldDir := filepath.Join(root, "festivals", "planning", "my-fest")
	hash, err := AutoCommitStatusChange(ctx, "my-fest", "MF001", "planning", "ready", []string{missingOldDir, newDir})
	if err != nil {
		t.Fatalf("AutoCommitStatusChange failed with a missing pre-move path: %v", err)
	}
	if hash == "" {
		t.Fatal("expected a commit hash, got empty string")
	}

	cmd := exec.Command("git", "log", "-1", "--name-only", "--pretty=format:")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "festivals/ready/my-fest/FESTIVAL_GOAL.md") {
		t.Errorf("moved festival file missing from commit, log:\n%s", out)
	}
}

func TestAutoCommitStatusChange_MissingOnlyPathsIsNoOp(t *testing.T) {
	root := t.TempDir()
	initTestRepo(t, root)

	ctx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{
		Root:          root,
		FestivalsPath: filepath.Join(root, "festivals"),
		Type:          scope.WorkspaceTypeStandalone,
	})

	missing1 := filepath.Join(root, "festivals", "planning", "ghost")
	missing2 := filepath.Join(root, "festivals", "ready", "ghost")
	hash, err := AutoCommitStatusChange(ctx, "ghost", "GH001", "planning", "ready", []string{missing1, missing2})
	if err != nil {
		t.Fatalf("missing-only paths are benign, expected no error, got: %v", err)
	}
	if hash != "" {
		t.Fatalf("expected no commit for missing-only paths, got hash %q", hash)
	}
}

func TestAutoCommitStatusChange_RealStageFailurePropagates(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based failure injection does not work as root")
	}
	root := t.TempDir()
	initTestRepo(t, root)

	okDir := filepath.Join(root, "festivals", "ready", "my-fest")
	if err := os.MkdirAll(okDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(okDir, "FESTIVAL_GOAL.md"), []byte("# Goal"), 0644); err != nil {
		t.Fatal(err)
	}

	brokenDir := filepath.Join(root, "festivals", "ready", "broken")
	if err := os.MkdirAll(brokenDir, 0755); err != nil {
		t.Fatal(err)
	}
	brokenFile := filepath.Join(brokenDir, "file.md")
	if err := os.WriteFile(brokenFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(brokenFile, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(brokenFile, 0644) })

	ctx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{
		Root:          root,
		FestivalsPath: filepath.Join(root, "festivals"),
		Type:          scope.WorkspaceTypeStandalone,
	})

	_, err := AutoCommitStatusChange(ctx, "my-fest", "MF001", "planning", "ready", []string{brokenDir, okDir})
	if err == nil {
		t.Fatal("expected a real staging failure on an existing path to propagate, not a partial commit")
	}
	if !strings.Contains(err.Error(), filepath.Join("festivals", "ready", "broken")) {
		t.Errorf("error should name the failing path, got: %v", err)
	}

	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = root
	out, _ := cmd.CombinedOutput()
	if strings.Count(strings.TrimSpace(string(out)), "\n")+1 > 1 {
		t.Errorf("no lifecycle commit should land on a staging failure, log:\n%s", out)
	}
}
