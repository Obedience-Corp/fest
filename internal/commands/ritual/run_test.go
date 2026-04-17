package ritual

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

func TestRunRitual_AutoCommitsScopedFilesystemChanges(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")
	ritualDir := filepath.Join(festivalsDir, "ritual", "daily-job-search-RI-DJ0001")

	if err := os.MkdirAll(ritualDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ritualDir, "FESTIVAL_OVERVIEW.md"), []byte("# Overview\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ritualDir, "fest.yaml"), []byte("version: \"1.0\"\nmetadata:\n  name: daily-job-search\nritual_config:\n  run_count: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := workspace.RegisterFestivals(festivalsDir); err != nil {
		t.Fatal(err)
	}

	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.name", "Test User")
	runGit(t, tmpDir, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial state")

	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("base\nunrelated change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ctx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{
		Root:          tmpDir,
		FestivalsPath: festivalsDir,
		Type:          scope.WorkspaceTypeStandalone,
	})

	if err := runRitual(ctx, "daily-job", &runOptions{}); err != nil {
		t.Fatalf("runRitual returned error: %v", err)
	}

	activeRun := filepath.Join(festivalsDir, "active", "daily-job-search-RI-DJ0001-0001")
	if _, err := os.Stat(filepath.Join(activeRun, "FESTIVAL_OVERVIEW.md")); err != nil {
		t.Fatalf("expected copied ritual run file, got error: %v", err)
	}

	ritualConfig := mustReadFile(t, filepath.Join(ritualDir, "fest.yaml"))
	if !strings.Contains(ritualConfig, "run_count: 1") {
		t.Fatalf("expected ritual config to increment run_count, got:\n%s", ritualConfig)
	}

	subject := strings.TrimSpace(runGit(t, tmpDir, "log", "-1", "--pretty=%s"))
	wantSubject := "chore(fest): ritual run: daily-job-search-RI-DJ0001 (DJ0001) -> daily-job-search-RI-DJ0001-0001"
	if subject != wantSubject {
		t.Fatalf("commit subject = %q, want %q", subject, wantSubject)
	}

	changedFiles := runGit(t, tmpDir, "show", "--name-only", "--pretty=", "HEAD")
	if !strings.Contains(changedFiles, "festivals/ritual/daily-job-search-RI-DJ0001/fest.yaml") {
		t.Fatalf("expected ritual config in commit, got:\n%s", changedFiles)
	}
	if !strings.Contains(changedFiles, "festivals/active/daily-job-search-RI-DJ0001-0001/FESTIVAL_OVERVIEW.md") {
		t.Fatalf("expected active ritual run files in commit, got:\n%s", changedFiles)
	}
	if strings.Contains(changedFiles, "README.md") {
		t.Fatalf("expected unrelated README change to remain uncommitted, got:\n%s", changedFiles)
	}

	status := runGit(t, tmpDir, "status", "--short")
	if !strings.Contains(status, " M README.md") && !strings.Contains(status, "M README.md") {
		t.Fatalf("expected unrelated README change to remain in worktree, got:\n%s", status)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}
