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
