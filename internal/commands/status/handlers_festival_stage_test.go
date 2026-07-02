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

func TestAutoCommitStatusChange_AllPathsFailReturnsError(t *testing.T) {
	root := t.TempDir()
	initTestRepo(t, root)

	ctx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{
		Root:          root,
		FestivalsPath: filepath.Join(root, "festivals"),
		Type:          scope.WorkspaceTypeStandalone,
	})

	missing1 := filepath.Join(root, "festivals", "planning", "ghost")
	missing2 := filepath.Join(root, "festivals", "ready", "ghost")
	_, err := AutoCommitStatusChange(ctx, "ghost", "GH001", "planning", "ready", []string{missing1, missing2})
	if err == nil {
		t.Fatal("expected an error when no path can be staged")
	}
	for _, p := range []string{missing1, missing2} {
		if !strings.Contains(err.Error(), p) {
			t.Errorf("error should name failed path %s, got: %v", p, err)
		}
	}
}
