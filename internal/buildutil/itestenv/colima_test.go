package itestenv

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type fakeRunner struct {
	output   []byte
	err      error
	commands [][]string
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	f.commands = append(f.commands, append([]string{name}, args...))
	return f.output, f.err
}

func (f *fakeRunner) Run(_ context.Context, _ io.Writer, name string, args ...string) error {
	f.commands = append(f.commands, append([]string{name}, args...))
	return f.err
}

func TestParseColimaList(t *testing.T) {
	t.Parallel()

	const listing = `{"name":"default","status":"Running","arch":"aarch64","cpus":4,"memory":17179869184,"disk":214748364800,"runtime":"docker"}
{"name":"fest-itest","status":"Stopped","arch":"aarch64","cpus":6,"memory":8589934592,"disk":107374182400,"runtime":"docker"}`

	tests := []struct {
		name        string
		data        string
		profile     string
		wantExists  bool
		wantRunning bool
		wantCPUs    int
		wantState   string
	}{
		{
			name: "running profile", data: listing, profile: "default",
			wantExists: true, wantRunning: true, wantCPUs: 4, wantState: "running",
		},
		{
			name: "stopped profile", data: listing, profile: ProfileName,
			wantExists: true, wantCPUs: 6, wantState: "stopped",
		},
		{name: "profile that was never created", data: listing, profile: "nope", wantState: "absent"},
		{name: "empty output", profile: ProfileName, wantState: "absent"},
		{
			name: "an unreadable line does not hide a later profile",
			data: "not json at all\n" + listing, profile: ProfileName,
			wantExists: true, wantCPUs: 6, wantState: "stopped",
		},
		{
			name: "status casing does not matter",
			data: `{"name":"fest-itest","status":"RUNNING","cpus":6}`, profile: ProfileName,
			wantExists: true, wantRunning: true, wantCPUs: 6, wantState: "running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseColimaList([]byte(tt.data), tt.profile)
			if got.Exists != tt.wantExists {
				t.Errorf("Exists = %v, want %v", got.Exists, tt.wantExists)
			}
			if got.Running != tt.wantRunning {
				t.Errorf("Running = %v, want %v", got.Running, tt.wantRunning)
			}
			if got.CPUs != tt.wantCPUs {
				t.Errorf("CPUs = %d, want %d", got.CPUs, tt.wantCPUs)
			}
			if got.State() != tt.wantState {
				t.Errorf("State() = %q, want %q", got.State(), tt.wantState)
			}
		})
	}
}

func TestColimaStartArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		spec     StartSpec
		wantArgs []string
		notArgs  []string
	}{
		{
			name:     "new profile is sized",
			spec:     StartSpec{Profile: ProfileName, CPUs: ProfileCPUs, MemoryGiB: ProfileMemoryGiB},
			wantArgs: []string{"start", ProfileName, activateFalse, cpusFlag, "6", memoryFlag, "8"},
		},
		{
			name:     "existing profile keeps its configuration",
			spec:     StartSpec{Profile: ProfileName},
			wantArgs: []string{"start", ProfileName, activateFalse},
			notArgs:  []string{cpusFlag, memoryFlag},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runner := &fakeRunner{}
			if err := newColimaWith(runner).Start(context.Background(), tt.spec, io.Discard); err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			if len(runner.commands) != 1 {
				t.Fatalf("commands = %d, want 1", len(runner.commands))
			}
			got := runner.commands[0]
			if got[0] != colimaBinary {
				t.Errorf("binary = %q, want %q", got[0], colimaBinary)
			}
			joined := strings.Join(got, " ")
			for _, want := range tt.wantArgs {
				if !strings.Contains(joined, want) {
					t.Errorf("command %q is missing %q", joined, want)
				}
			}
			for _, unwanted := range tt.notArgs {
				if strings.Contains(joined, unwanted) {
					t.Errorf("command %q must not carry %q", joined, unwanted)
				}
			}
		})
	}
}

func TestColimaStartReportsFailure(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{err: io.ErrClosedPipe}
	err := newColimaWith(runner).Start(context.Background(), StartSpec{Profile: ProfileName}, io.Discard)
	if err == nil {
		t.Fatal("Start() reported success for a failing command")
	}
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("Start() error = %v, want it to wrap the underlying failure", err)
	}
	if !strings.Contains(err.Error(), ProfileName) {
		t.Errorf("Start() error = %v, want it to name the profile", err)
	}
}

func TestColimaStartHonoursCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &fakeRunner{}
	if err := newColimaWith(runner).Start(ctx, StartSpec{Profile: ProfileName}, io.Discard); err == nil {
		t.Fatal("Start() with a cancelled context returned no error")
	}
	if len(runner.commands) != 0 {
		t.Errorf("commands = %d, want 0 on a cancelled context", len(runner.commands))
	}
}
