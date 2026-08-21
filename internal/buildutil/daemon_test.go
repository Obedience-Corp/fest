package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/buildutil/itestenv"
)

type fakeColima struct {
	status itestenv.ProfileStatus
}

func (f fakeColima) Status(context.Context, string) (itestenv.ProfileStatus, error) {
	return f.status, nil
}

func (f fakeColima) Start(context.Context, itestenv.StartSpec, io.Writer) error {
	return nil
}

func TestPrepareIntegrationDaemonPublishesOverride(t *testing.T) {
	t.Setenv(itestenv.DockerHostVar, "")
	t.Setenv(itestenv.EnvDockerHost, "unix:///tmp/docker.sock")
	t.Setenv(ryukDisabledEnv, "")
	t.Setenv(socketOverrideEnv, "")

	if err := prepareIntegrationDaemon(context.Background(), itestenv.Options{
		Home: t.TempDir(),
		Out:  io.Discard,
	}); err != nil {
		t.Fatalf("prepareIntegrationDaemon() error = %v", err)
	}

	if got := os.Getenv(itestenv.DockerHostVar); got != "unix:///tmp/docker.sock" {
		t.Fatalf("DOCKER_HOST = %q, want the override", got)
	}
	if got := os.Getenv(ryukDisabledEnv); got != ryukDisabledValue {
		t.Fatalf("%s = %q, want %q", ryukDisabledEnv, got, ryukDisabledValue)
	}
	if got := os.Getenv(socketOverrideEnv); got != inVMDockerSocket {
		t.Fatalf("%s = %q, want %q", socketOverrideEnv, got, inVMDockerSocket)
	}
}

func TestPrepareIntegrationDaemonUsesDedicatedProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv(itestenv.DockerHostVar, "")
	t.Setenv(itestenv.EnvDockerHost, "")
	t.Setenv(itestenv.EnvProfile, "")

	running := itestenv.ProfileStatus{Name: itestenv.ProfileName, Exists: true, Running: true, Status: "Running"}
	if err := prepareIntegrationDaemon(context.Background(), itestenv.Options{
		Home:   home,
		Colima: fakeColima{status: running},
		Out:    io.Discard,
	}); err != nil {
		t.Fatalf("prepareIntegrationDaemon() error = %v", err)
	}

	want := itestenv.ProfileDockerHost(home, itestenv.ProfileName)
	if got := os.Getenv(itestenv.DockerHostVar); got != want {
		t.Fatalf("DOCKER_HOST = %q, want dedicated profile %q", got, want)
	}
}

func TestPrepareIntegrationDaemonHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := prepareIntegrationDaemon(ctx, itestenv.Options{Home: t.TempDir(), Out: io.Discard}); err == nil {
		t.Fatal("prepareIntegrationDaemon() with a cancelled context returned no error")
	}
}

func TestDoctorStartRequested(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "bare doctor", args: []string{"integration-doctor"}},
		{name: "start", args: []string{"integration-doctor", "start"}, want: true},
		{name: "flags then start", args: []string{"-v", "integration-doctor", "start"}, want: true},
		{name: "other extra arg", args: []string{"integration-doctor", "status"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := doctorStartRequested(tt.args); got != tt.want {
				t.Fatalf("doctorStartRequested(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestReportNonRun(t *testing.T) {
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	reportNonRun(io.ErrClosedPipe)
	_ = w.Close()
	os.Stderr = orig

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	got := buf.String()
	if !strings.Contains(got, "✗ NON-RUN") {
		t.Fatalf("reportNonRun() output = %q, want the NON-RUN verdict", got)
	}
	if !strings.Contains(got, itestenv.StartCommand) {
		t.Fatalf("reportNonRun() output = %q, want the recovery command", got)
	}
}

func TestHostRowMarksSharedDaemons(t *testing.T) {
	t.Parallel()

	shared := hostRow(itestenv.Resolution{
		DockerHost: "unix:///d.sock",
		Source:     itestenv.SourceFallback,
	})
	if !strings.HasPrefix(shared, "shared:") {
		t.Fatalf("hostRow(fallback) = %q, want a shared: prefix", shared)
	}

	dedicated := hostRow(itestenv.Resolution{
		DockerHost: "unix:///d.sock",
		Source:     itestenv.SourceProfile,
	})
	if strings.Contains(dedicated, "shared:") {
		t.Fatalf("hostRow(dedicated) = %q, want no shared: prefix", dedicated)
	}
}

func TestShortHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory")
	}

	if got := shortHome(home); got != "~" {
		t.Fatalf("shortHome(home) = %q, want ~", got)
	}
	if got := shortHome(home + "/.obey/locks/x.lock"); !strings.HasPrefix(got, "~/") {
		t.Fatalf("shortHome(nested) = %q, want a ~ prefix", got)
	}
	if got := shortHome("/elsewhere"); got != "/elsewhere" {
		t.Fatalf("shortHome(other) = %q, want unchanged", got)
	}
}
