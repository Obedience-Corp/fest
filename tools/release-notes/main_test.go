package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/tools/release-notes/internal/notes"
)

func TestResolvePreviousTagSkipsSameCommitTags(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
		}
	}

	run("init", "--initial-branch=main")
	run("config", "user.name", "Test User")
	run("config", "user.email", "test@example.com")

	readme := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readme, []byte("v1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "feat: first change")
	run("tag", "v0.2.0")

	if err := os.WriteFile(readme, []byte("v2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	run("commit", "-am", "fix: second change")
	run("tag", "v0.2.1")
	run("tag", "v0.2.2")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(cwd); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	target, err := notes.ParseTag("v0.2.2")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}
	targetCommit, err := tagCommit("v0.2.2")
	if err != nil {
		t.Fatalf("tagCommit() error = %v", err)
	}
	tags, err := gitLines("tag", "-l", "v*", "--sort=-version:refname")
	if err != nil {
		t.Fatalf("gitLines() error = %v", err)
	}

	previous, err := resolvePreviousTag(target, targetCommit, tags)
	if err != nil {
		t.Fatalf("resolvePreviousTag() error = %v", err)
	}
	if previous != "v0.2.0" {
		t.Fatalf("resolvePreviousTag() = %q, want %q", previous, "v0.2.0")
	}
}
