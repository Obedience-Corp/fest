package itestenv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenSuiteHonoursCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	home := t.TempDir()
	suite, err := OpenSuite(ctx, Options{
		Getenv: env(map[string]string{EnvDockerHost: "tcp://remote:2375"}),
		Home:   home,
		Colima: &fakeColima{},
	})
	if err == nil {
		_ = suite.Close()
		t.Fatal("OpenSuite() with a cancelled context returned no error")
	}
	if _, statErr := os.Stat(filepath.Join(home, obeyDirName, locksDirName)); statErr == nil {
		t.Errorf("OpenSuite() created lock artifacts despite the cancelled context")
	}
}

func TestOpenSuitePublishesDockerHostAndLocks(t *testing.T) {
	home := t.TempDir()
	t.Setenv(DockerHostVar, "")
	t.Setenv(SocketOverrideEnv, "")
	t.Setenv(RyukDisabledEnv, "")

	running := ProfileStatus{Name: ProfileName, Exists: true, Running: true, Status: "Running"}
	suite, err := OpenSuite(context.Background(), Options{
		Getenv:    env(nil),
		Home:      home,
		Colima:    &fakeColima{status: running},
		AutoStart: true,
	})
	if err != nil {
		t.Fatalf("OpenSuite() error = %v", err)
	}
	t.Cleanup(func() { _ = suite.Close() })

	wantHost := ProfileDockerHost(home, ProfileName)
	if suite.Resolution.DockerHost != wantHost {
		t.Fatalf("DockerHost = %q, want %q", suite.Resolution.DockerHost, wantHost)
	}
	if suite.Resolution.Source != SourceProfile {
		t.Fatalf("Source = %q, want %q", suite.Resolution.Source, SourceProfile)
	}
	if got := os.Getenv(DockerHostVar); got != wantHost {
		t.Fatalf("published %s = %q, want %q", DockerHostVar, got, wantHost)
	}
	if got := os.Getenv(SocketOverrideEnv); got != InVMDockerSocket {
		t.Fatalf("published %s = %q, want %q whenever SourceProfile is Colima",
			SocketOverrideEnv, got, InVMDockerSocket)
	}
	if got := os.Getenv(RyukDisabledEnv); got != RyukDisabledValue {
		t.Fatalf("published %s = %q, want %q", RyukDisabledEnv, got, RyukDisabledValue)
	}
	if suite.lock == nil || suite.lock.Path() == "" {
		t.Fatal("OpenSuite() did not take the suite lock")
	}
}

func TestPublishRuntimeEnvSetsOverrideForSourceProfile(t *testing.T) {
	t.Setenv(SocketOverrideEnv, "")
	t.Setenv(RyukDisabledEnv, "")

	resolution := Resolution{
		DockerHost: "unix:///var/run/docker.sock",
		Profile:    ProfileName,
		Source:     SourceProfile,
	}
	if err := PublishRuntimeEnv(resolution); err != nil {
		t.Fatalf("PublishRuntimeEnv() error = %v", err)
	}
	if got := os.Getenv(SocketOverrideEnv); got != InVMDockerSocket {
		t.Fatalf("%s = %q, want %q whenever SourceProfile is Colima",
			SocketOverrideEnv, got, InVMDockerSocket)
	}
	if got := os.Getenv(RyukDisabledEnv); got != RyukDisabledValue {
		t.Fatalf("%s = %q, want %q", RyukDisabledEnv, got, RyukDisabledValue)
	}
}

func TestPublishRuntimeEnvSkipsOverrideForRemoteDaemon(t *testing.T) {
	t.Setenv(SocketOverrideEnv, "")
	t.Setenv(RyukDisabledEnv, "")

	if err := PublishRuntimeEnv(Resolution{DockerHost: "tcp://remote:2375", Source: SourceOverride}); err != nil {
		t.Fatalf("PublishRuntimeEnv() error = %v", err)
	}
	if got := os.Getenv(SocketOverrideEnv); got != "" {
		t.Fatalf("%s = %q, want unset for a remote daemon", SocketOverrideEnv, got)
	}
	if got := os.Getenv(RyukDisabledEnv); got != RyukDisabledValue {
		t.Fatalf("%s = %q, want %q", RyukDisabledEnv, got, RyukDisabledValue)
	}
}

func TestRefusalLines(t *testing.T) {
	t.Parallel()

	line := RefusalLine("daemon locked")
	if !strings.HasPrefix(line, "INFRASTRUCTURE FAILURE (not a test failure): ") {
		t.Fatalf("RefusalLine() = %q, want the infrastructure banner", line)
	}
	if !strings.Contains(line, "daemon locked") {
		t.Fatalf("RefusalLine() = %q, want the reason", line)
	}

	recovery := RefusalRecovery()
	for _, want := range []string{StartCommand, DoctorCommand, EnvDockerHost} {
		if !strings.Contains(recovery, want) {
			t.Errorf("RefusalRecovery() = %q, want it to contain %q", recovery, want)
		}
	}

	nonRun := NonRunLine("probe failed")
	if !strings.HasPrefix(nonRun, "✗ NON-RUN") {
		t.Fatalf("NonRunLine() = %q, want the NON-RUN verdict", nonRun)
	}
}
