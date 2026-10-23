package commands

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/obey-shared/updatecheck"
	"github.com/spf13/cobra"
)

const probeNotice = "Festival suite update available (0.3.20 → 0.3.24)"

type suiteHarness struct {
	env       map[string]string
	obeyDir   string
	calls     int
	version   string
	opts      updatecheck.Options
	checks    int
	probeErr  error
	probeHelp bool
}

func (h *suiteHarness) cachePath() string {
	return filepath.Join(h.obeyDir, "update-check", "suite.json")
}

// stubSuiteNotice isolates the hook from the host: a fake bundle, a terminal
// stderr, an empty environment (the test runner may itself be an agent), and
// a temp ~/.obey. The notify and check functions are recorders.
func stubSuiteNotice(t *testing.T, bundle string) *suiteHarness {
	t.Helper()
	h := &suiteHarness{env: map[string]string{}, obeyDir: filepath.Join(t.TempDir(), ".obey")}
	saved := struct {
		bundle  func() string
		stdout  func() bool
		tty     func() bool
		getenv  func(string) string
		obeyDir func() (string, error)
		exe     func() string
		fetcher *updatecheck.Fetcher
		notify  func(context.Context, string, updatecheck.Options) *updatecheck.Notice
		check   func(context.Context, string, updatecheck.Options) (updatecheck.Result, error)
	}{suiteBundle, suiteStdoutIsTTY, suiteStderrIsTTY, suiteGetenv, suiteObeyDir, suiteExecutable, suiteFetcher, suiteNotify, suiteCheck}
	t.Cleanup(func() {
		suiteBundle, suiteStdoutIsTTY, suiteStderrIsTTY, suiteGetenv = saved.bundle, saved.stdout, saved.tty, saved.getenv
		suiteHelpShown = false
		suiteObeyDir, suiteExecutable, suiteFetcher = saved.obeyDir, saved.exe, saved.fetcher
		suiteNotify, suiteCheck = saved.notify, saved.check
	})
	suiteBundle = func() string { return bundle }
	suiteStdoutIsTTY = func() bool { return true }
	suiteStderrIsTTY = func() bool { return true }
	suiteHelpShown = false
	suiteGetenv = func(k string) string { return h.env[k] }
	suiteObeyDir = func() (string, error) { return h.obeyDir, nil }
	exe := filepath.Join(t.TempDir(), "bin", "fest")
	suiteExecutable = func() string { return exe }
	suiteFetcher = nil
	suiteNotify = func(_ context.Context, v string, o updatecheck.Options) *updatecheck.Notice {
		h.calls++
		h.version, h.opts = v, o
		return &updatecheck.Notice{Message: probeNotice, Command: "festival update", Kind: "run"}
	}
	suiteCheck = func(context.Context, string, updatecheck.Options) (updatecheck.Result, error) {
		h.checks++
		return updatecheck.Result{}, nil
	}
	return h
}

// addProbe registers a visible, global-scope command whose RunE returns
// h.probeErr, standing in for any human command.
func addProbe(t *testing.T, h *suiteHarness) string {
	t.Helper()
	cmd := &cobra.Command{
		Use:         "suite-notice-probe",
		Short:       "test helper",
		Annotations: map[string]string{"scope": string(scope.Global)},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if h.probeHelp {
				return cmd.Help()
			}
			_, _ = cmd.OutOrStdout().Write([]byte("probe output\n"))
			return h.probeErr
		},
	}
	rootCmd.AddCommand(cmd)
	t.Cleanup(func() { rootCmd.RemoveCommand(cmd) })
	return cmd.Use
}

func runFest(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)
	oldArgs := os.Args
	os.Args = append([]string{"fest"}, args...)
	t.Cleanup(func() {
		os.Args = oldArgs
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		if cmd, _, err := rootCmd.Find(args); err == nil {
			resetChangedFlags(cmd)
		}
	})
	err := ExecuteContext(context.Background())
	return stdout.String(), stderr.String(), err
}

func TestAmbientNoticeRunsAfterSuccessfulCommand(t *testing.T) {
	h := stubSuiteNotice(t, "v0.3.20")
	probe := addProbe(t, h)

	stdout, stderr, err := runFest(t, probe)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if h.calls != 1 {
		t.Fatalf("notify calls = %d, want 1", h.calls)
	}
	if h.version != "v0.3.20" {
		t.Fatalf("notify got version %q, want the suite bundle v0.3.20", h.version)
	}
	if h.opts.CachePath != h.cachePath() {
		t.Fatalf("CachePath = %q, want %q", h.opts.CachePath, h.cachePath())
	}
	if h.opts.ExecutablePath == "" || h.opts.ExecutablePath != suiteExecutable() {
		t.Fatalf("ExecutablePath = %q, want the resolved executable", h.opts.ExecutablePath)
	}
	if !strings.Contains(stderr, probeNotice) || !strings.Contains(stderr, "run:") || !strings.Contains(stderr, "festival update") {
		t.Fatalf("stderr missing notice:\n%s", stderr)
	}
	if lines := strings.Count(strings.TrimRight(stderr, "\n"), "\n") + 1; lines != 2 {
		t.Fatalf("notice is %d lines, want 2:\n%s", lines, stderr)
	}
	if strings.Contains(stdout, "notice") || stdout != "probe output\n" {
		t.Fatalf("stdout changed by the notice: %q", stdout)
	}
}

func TestAmbientNoticeSkippedWhenCommandFails(t *testing.T) {
	h := stubSuiteNotice(t, "v0.3.20")
	h.probeErr = errors.New("probe failed")
	probe := addProbe(t, h)

	if _, _, err := runFest(t, probe); err == nil {
		t.Fatal("probe error was swallowed")
	}
	if h.calls != 0 {
		t.Fatalf("notify ran after a failed command (%d calls)", h.calls)
	}
}

// A command that answers by printing its help, as group commands do, still
// returns nil, so cobra runs the post-run hook after it.
func TestAmbientNoticeSkipsHelpPrintedByRunE(t *testing.T) {
	h := stubSuiteNotice(t, "v0.3.20")
	h.probeHelp = true
	probe := addProbe(t, h)

	stdout, stderr, err := runFest(t, probe)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !strings.Contains(stdout, "suite-notice-probe") {
		t.Fatalf("probe did not print its help; stdout:\n%s", stdout)
	}
	if h.calls != 0 || stderr != "" {
		t.Fatalf("help output got a notice: calls = %d, stderr = %q", h.calls, stderr)
	}
}

func TestAmbientNoticeEmptyBundleDoesNothing(t *testing.T) {
	h := stubSuiteNotice(t, "")
	probe := addProbe(t, h)

	_, stderr, err := runFest(t, probe)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if h.calls != 0 || stderr != "" {
		t.Fatalf("empty bundle: notify calls = %d, stderr = %q", h.calls, stderr)
	}
	if _, err := os.Stat(h.obeyDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty bundle touched %s (stat err %v)", h.obeyDir, err)
	}
}

func TestAmbientNoticeSuppressedNeverCallsNotify(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		tty  bool
	}{
		{"piped stderr", nil, false},
		{"captured stdout", nil, true},
		{"opt out", map[string]string{"FESTIVAL_NO_UPDATE_CHECK": "1"}, true},
		{"ci", map[string]string{"CI": "1"}, true},
		{"github actions", map[string]string{"GITHUB_ACTIONS": "true"}, true},
		{"agent session", map[string]string{"CLAUDECODE": "1"}, true},
		{"obey session", map[string]string{"OBEY_SESSION_ID": "s-1"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := stubSuiteNotice(t, "v0.3.20")
			for k, v := range tt.env {
				h.env[k] = v
			}
			tty := tt.tty
			suiteStderrIsTTY = func() bool { return tty }
			if tt.name == "captured stdout" {
				suiteStdoutIsTTY = func() bool { return false }
			}
			probe := addProbe(t, h)

			_, stderr, err := runFest(t, probe)
			if err != nil {
				t.Fatalf("probe: %v", err)
			}
			if h.calls != 0 || stderr != "" {
				t.Fatalf("notify calls = %d, stderr = %q", h.calls, stderr)
			}
		})
	}
}

func TestAmbientNoticeSkipsBuiltinPaths(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
	}{
		{"help command", []string{"help", "version"}, "Show fest version"},
		{"cobra complete", []string{"__complete", "vers"}, "version"},
		{"version", []string{"version"}, "profile:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := stubSuiteNotice(t, "v0.3.20")
			stdout, stderr, err := runFest(t, tt.args...)
			if err != nil {
				t.Fatalf("%v: %v", tt.args, err)
			}
			if !strings.Contains(stdout, tt.wantStdout) {
				t.Fatalf("%v did not run to completion; stdout:\n%s", tt.args, stdout)
			}
			if h.calls != 0 || strings.Contains(stderr, "notice:") {
				t.Fatalf("%v: notify calls = %d, stderr = %q", tt.args, h.calls, stderr)
			}
		})
	}
}
