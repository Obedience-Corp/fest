package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolvedVersion(t *testing.T) {
	t.Run("uses VERSION env when set", func(t *testing.T) {
		t.Setenv("VERSION", "v9.9.9")
		if got := resolvedVersion(); got != "v9.9.9" {
			t.Fatalf("resolvedVersion() = %q, want %q", got, "v9.9.9")
		}
	})

	t.Run("uses exact git tag when VERSION is unset", func(t *testing.T) {
		t.Setenv("VERSION", "")
		repo := makeTaggedRepo(t, "v0.1.0")
		withWorkingDir(t, repo, func() {
			if got := resolvedVersion(); got != "v0.1.0" {
				t.Fatalf("resolvedVersion() = %q, want %q", got, "v0.1.0")
			}
		})
	})

	t.Run("falls back to dev when HEAD is untagged", func(t *testing.T) {
		t.Setenv("VERSION", "")
		repo := makeTaggedRepo(t, "")
		withWorkingDir(t, repo, func() {
			if got := resolvedVersion(); got != "dev" {
				t.Fatalf("resolvedVersion() = %q, want %q", got, "dev")
			}
		})
	})
}

func makeTaggedRepo(t *testing.T, tag string) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Fest Test")
	runGit(t, dir, "config", "user.email", "fest-test@example.com")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")

	if tag != "" {
		runGit(t, dir, "tag", "-a", tag, "-m", tag)
	}

	return dir
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir(%s): %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()

	fn()
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
