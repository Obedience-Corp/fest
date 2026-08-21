package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// shaPattern matches the abbreviated commit sha `git describe --always` falls
// back to when no tag is reachable from HEAD.
var shaPattern = regexp.MustCompile(`^[0-9a-f]{7,}$`)

func TestResolvedVersion(t *testing.T) {
	t.Run("falls back to dev outside a git repository", func(t *testing.T) {
		t.Setenv("VERSION", "")
		withWorkingDir(t, t.TempDir(), func() {
			if got := resolvedVersion(); got != "dev" {
				t.Fatalf("resolvedVersion() = %q, want %q", got, "dev")
			}
		})
	})

	t.Run("falls back to the commit sha when no tag is reachable", func(t *testing.T) {
		t.Setenv("VERSION", "")
		repo := makeTaggedRepo(t, "")
		withWorkingDir(t, repo, func() {
			got := resolvedVersion()
			if got == "dev" {
				t.Fatalf("resolvedVersion() = %q, want an abbreviated commit sha", got)
			}
			if !shaPattern.MatchString(got) {
				t.Fatalf("resolvedVersion() = %q, want an abbreviated commit sha", got)
			}
		})
	})

	t.Run("uses the exact git tag when HEAD is tagged", func(t *testing.T) {
		t.Setenv("VERSION", "")
		repo := makeTaggedRepo(t, "v0.1.0")
		withWorkingDir(t, repo, func() {
			if got := resolvedVersion(); got != "v0.1.0" {
				t.Fatalf("resolvedVersion() = %q, want %q", got, "v0.1.0")
			}
		})
	})

	t.Run("describes the distance from the last tag", func(t *testing.T) {
		t.Setenv("VERSION", "")
		repo := makeTaggedRepo(t, "v0.1.0")
		addCommit(t, repo, "second")
		withWorkingDir(t, repo, func() {
			got := resolvedVersion()
			if !strings.HasPrefix(got, "v0.1.0-1-g") {
				t.Fatalf("resolvedVersion() = %q, want prefix %q", got, "v0.1.0-1-g")
			}
		})
	})

	t.Run("marks an uncommitted worktree dirty", func(t *testing.T) {
		t.Setenv("VERSION", "")
		repo := makeTaggedRepo(t, "v0.1.0")
		dirtyWorktree(t, repo)
		withWorkingDir(t, repo, func() {
			if got := resolvedVersion(); got != "v0.1.0-dirty" {
				t.Fatalf("resolvedVersion() = %q, want %q", got, "v0.1.0-dirty")
			}
		})
	})

	t.Run("uses VERSION env when set", func(t *testing.T) {
		t.Setenv("VERSION", "v9.9.9")
		repo := makeTaggedRepo(t, "v0.1.0")
		withWorkingDir(t, repo, func() {
			if got := resolvedVersion(); got != "v9.9.9" {
				t.Fatalf("resolvedVersion() = %q, want %q", got, "v9.9.9")
			}
		})
	})
}

func TestConfigureIntegrationEnvironment(t *testing.T) {
	t.Run("sets Colima defaults for integration commands", func(t *testing.T) {
		home := t.TempDir()
		socket := filepath.Join(home, ".colima", "default", "docker.sock")
		if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
			t.Fatalf("mkdir socket dir: %v", err)
		}
		if err := os.WriteFile(socket, []byte{}, 0o644); err != nil {
			t.Fatalf("write socket placeholder: %v", err)
		}

		t.Setenv("HOME", home)
		t.Setenv("DOCKER_HOST", "")
		t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "")

		if err := configureIntegrationEnvironment([]string{"integration"}); err != nil {
			t.Fatalf("configureIntegrationEnvironment() error = %v", err)
		}

		if got := os.Getenv("DOCKER_HOST"); got != "unix://"+socket {
			t.Fatalf("DOCKER_HOST = %q, want %q", got, "unix://"+socket)
		}
		if got := os.Getenv("TESTCONTAINERS_RYUK_DISABLED"); got != "true" {
			t.Fatalf("TESTCONTAINERS_RYUK_DISABLED = %q, want %q", got, "true")
		}
	})

	t.Run("preserves existing docker host", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "unix:///tmp/docker.sock")
		t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "")

		if err := configureIntegrationEnvironment([]string{"all"}); err != nil {
			t.Fatalf("configureIntegrationEnvironment() error = %v", err)
		}

		if got := os.Getenv("DOCKER_HOST"); got != "unix:///tmp/docker.sock" {
			t.Fatalf("DOCKER_HOST = %q, want %q", got, "unix:///tmp/docker.sock")
		}
		if got := os.Getenv("TESTCONTAINERS_RYUK_DISABLED"); got != "true" {
			t.Fatalf("TESTCONTAINERS_RYUK_DISABLED = %q, want %q", got, "true")
		}
	})

	t.Run("skips non-integration commands", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "")
		t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "")

		if err := configureIntegrationEnvironment([]string{"build"}); err != nil {
			t.Fatalf("configureIntegrationEnvironment() error = %v", err)
		}

		if got := os.Getenv("DOCKER_HOST"); got != "" {
			t.Fatalf("DOCKER_HOST = %q, want empty", got)
		}
		if got := os.Getenv("TESTCONTAINERS_RYUK_DISABLED"); got != "" {
			t.Fatalf("TESTCONTAINERS_RYUK_DISABLED = %q, want empty", got)
		}
	})
}

func TestRequestedCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "plain command",
			args: []string{"integration"},
			want: "integration",
		},
		{
			name: "skips flags",
			args: []string{"-v", "--no-color", "all"},
			want: "all",
		},
		{
			name: "no command",
			args: []string{"-v"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := requestedCommand(tt.args); got != tt.want {
				t.Fatalf("requestedCommand(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
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

func addCommit(t *testing.T, dir, name string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", name)
}

func dirtyWorktree(t *testing.T, dir string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("dirty README: %v", err)
	}
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
