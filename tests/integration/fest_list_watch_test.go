//go:build integration && dev
// +build integration,dev

package integration

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
)

// These tests drive the real `fest list --watch` refresh loop end to end
// (fixture creation, board rendering, live progress updates). They must stay
// in the containerized integration harness, not host t.TempDir tests, since
// they mutate a festivals/ tree and depend on real terminal semantics.

func TestFestListWatchInitialFrameAndCancel(t *testing.T) {
	container := GetSharedContainer(t)
	workspaceRoot, _, readyPath := setupListWatchFixture(t, container)
	readyName := filepath.Base(readyPath)

	output, exitCode := runFestListWatchBounded(t, container, workspaceRoot, 2, "--watch")

	require.Contains(t, []int{124, 143}, exitCode, "list --watch should stay running until the bounded timeout")
	require.Contains(t, output, "ACTIVE Festivals", "default board should render the active status section")
	require.Contains(t, output, "READY Festivals", "default board should render the ready status section")
	require.Contains(t, output, "list-watch-active-LW0001", "default board should include the active fixture festival")
	require.Contains(t, output, readyName, "default board should include the ready fixture festival")
	require.Contains(t, output, "watching", "watch footer should announce what is being watched")
	require.Contains(t, output, "Ctrl+C to quit", "watch footer should document the quit key")
	require.NotContains(t, output, "unknown command")
}

func TestFestListWatchFilterPreservesStatusMembership(t *testing.T) {
	container := GetSharedContainer(t)
	workspaceRoot, activePath, readyPath := setupListWatchFixture(t, container)
	activeName := filepath.Base(activePath)
	readyName := filepath.Base(readyPath)

	output, exitCode := runFestListWatchBounded(t, container, workspaceRoot, 2, "active", "--watch")

	require.Contains(t, []int{124, 143}, exitCode, "filtered list --watch should stay running until the bounded timeout")
	require.Contains(t, output, activeName, "active filter should include the active fixture festival")
	require.NotContains(t, output, readyName, "active filter should exclude the ready fixture festival")
	require.NotContains(t, output, "READY Festivals", "single-status watch should not render other status headers")
}

func TestFestListWatchRejectsJSON(t *testing.T) {
	container := GetSharedContainer(t)
	workspaceRoot, _, _ := setupListWatchFixture(t, container)

	output, err := container.RunFestInDir(workspaceRoot, "list", "--watch", "--json")

	require.Error(t, err, "--watch combined with --json should fail fast")
	require.Contains(t, output, "--watch cannot be combined with --json")
}

func TestFestListWatchNonTTYFailsFast(t *testing.T) {
	container := GetSharedContainer(t)
	workspaceRoot, _, _ := setupListWatchFixture(t, container)

	output, err := container.RunFestInDir(workspaceRoot, "list", "--watch")

	require.Error(t, err, "--watch on non-TTY stdout should fail fast")
	require.Contains(t, output, "interactive terminal")
}

func TestFestListWatchProgressUpdatesBetweenTicks(t *testing.T) {
	container := GetSharedContainer(t)
	workspaceRoot, activePath, _ := setupListWatchFixture(t, container)

	script := fmt.Sprintf(
		"cd %s && timeout 7s /fest list active --watch & watchpid=$!; sleep 3; (cd %s && /fest progress --task 001_IMPLEMENT/01_core/01_first_task.md --complete >/dev/null 2>&1); wait $watchpid",
		shellQuote(workspaceRoot), shellQuote(activePath),
	)

	output, exitCode := runContainerScriptTTY(t, container, script)

	require.Contains(t, []int{124, 143}, exitCode, "watch process should be stopped by the bounded timeout")
	require.Contains(t, output, "[0%]", "initial frame should render 0%% before the task is completed")
	require.Contains(t, output, "[100%]", "a later frame should render 100%% after fest progress --complete")
}

// setupListWatchFixture creates a workspace with one active and one ready
// festival, each with a single task, so watch tests can assert status
// membership and progress changes without depending on unrelated fixtures.
func setupListWatchFixture(t *testing.T, tc *TestContainer) (workspaceRoot, activeFestivalPath, readyFestivalPath string) {
	t.Helper()

	workspaceRoot = "/workspace"
	festivalsRoot := filepath.Join(workspaceRoot, "festivals")
	activeFestivalPath = filepath.Join(festivalsRoot, "active", "list-watch-active-LW0001")
	readyFestivalPath = filepath.Join(festivalsRoot, "ready", "list-watch-ready-LW0002")

	_, err := tc.runCommand([]string{
		"sh", "-c",
		fmt.Sprintf(
			"mkdir -p %[1]s/.campaign %[2]s/.festival/.state %[2]s/planning %[2]s/ritual %[2]s/dungeon/completed %[2]s/dungeon/archived %[2]s/dungeon/someday %[3]s/001_IMPLEMENT/01_core %[4]s/001_IMPLEMENT/01_core",
			workspaceRoot, festivalsRoot, activeFestivalPath, readyFestivalPath,
		),
	})
	require.NoError(t, err, "should create list watch fixture directories")

	err = writeFileInContainer(tc, filepath.Join(festivalsRoot, ".festival", ".state", ".workspace"), `{"workspace":"workspace","registered":"2024-01-01T00:00:00Z"}`)
	require.NoError(t, err, "should create workspace marker")

	writeListWatchFestivalFixture(t, tc, activeFestivalPath, "active", "Watch the active board render and refresh correctly.")
	writeListWatchFestivalFixture(t, tc, readyFestivalPath, "ready", "Confirm ready festivals stay out of the active filter.")

	return workspaceRoot, activeFestivalPath, readyFestivalPath
}

func writeListWatchFestivalFixture(t *testing.T, tc *TestContainer, festivalPath, status, goalLine string) {
	t.Helper()

	festivalName := filepath.Base(festivalPath)
	logicalID := festivalName
	name := festivalName
	if idx := strings.LastIndex(festivalName, "-"); idx >= 0 {
		logicalID = festivalName[idx+1:]
		name = festivalName[:idx]
	}

	// status_history is required by the pre-active lifecycle gate that
	// `fest progress --complete` enforces; without it, status is
	// undeterminable and the gate fails closed.
	err := writeFileInContainer(tc, filepath.Join(festivalPath, "fest.yaml"), fmt.Sprintf(`version: "1.0"
metadata:
  id: %s
  name: %s
  status_history:
    - status: %s
      timestamp: 2026-01-01T00:00:00Z
auto_link:
  enabled: false
`, logicalID, name, status))
	require.NoError(t, err, "should create fest.yaml")

	err = writeFileInContainer(tc, filepath.Join(festivalPath, "FESTIVAL_GOAL.md"), fmt.Sprintf("# %s\n\n## Goal\n\n%s\n", festivalName, goalLine))
	require.NoError(t, err, "should create festival goal")

	err = writeFileInContainer(tc, filepath.Join(festivalPath, "FESTIVAL_RULES.md"), "# Festival Rules\n\n- Keep list watch tests containerized.\n")
	require.NoError(t, err, "should create festival rules")

	phasePath := filepath.Join(festivalPath, "001_IMPLEMENT")
	sequencePath := filepath.Join(phasePath, "01_core")

	err = writeFileInContainer(tc, filepath.Join(phasePath, "PHASE_GOAL.md"), `---
fest_type: phase
fest_phase_type: implementation
---

# Implement Phase

## Objective

Build the list watch board.
`)
	require.NoError(t, err, "should create phase goal")

	err = writeFileInContainer(tc, filepath.Join(sequencePath, "SEQUENCE_GOAL.md"), "# Core Sequence\n\nImplement the core list watch flow.\n")
	require.NoError(t, err, "should create sequence goal")

	err = writeFileInContainer(tc, filepath.Join(sequencePath, "01_first_task.md"), "# First Task\n\n## Definition of Done\n\n- [ ] Board renders the festival.\n")
	require.NoError(t, err, "should create first task")
}

// runFestListWatchBounded runs `fest list <args>` from cwd under a real TTY,
// bounded by an OS-level `timeout` so the watch loop cannot hang the test
// suite. It mirrors runFestWatchBounded in fest_watch_test.go but allocates
// a TTY, since list --watch (unlike fest watch) requires an interactive
// stdout even when a direct context is already resolved.
func runFestListWatchBounded(t *testing.T, tc *TestContainer, cwd string, timeoutSeconds int, args ...string) (string, int) {
	t.Helper()

	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		quotedArgs = append(quotedArgs, shellQuote(arg))
	}

	script := fmt.Sprintf("cd %s && timeout %ds /fest list", shellQuote(cwd), timeoutSeconds)
	if len(quotedArgs) > 0 {
		script += " " + strings.Join(quotedArgs, " ")
	}

	return runContainerScriptTTY(t, tc, script)
}

// runContainerScriptTTY runs an arbitrary shell script inside the container
// with a pseudo-TTY attached to stdout, returning combined output and the
// real exit code of the script (unlike tc.runCommand, which discards it).
func runContainerScriptTTY(t *testing.T, tc *TestContainer, script string) (string, int) {
	t.Helper()

	options := []tcexec.ProcessOption{
		tcexec.ProcessOptionFunc(func(opts *tcexec.ProcessOptions) {
			opts.ExecConfig.TTY = true
			opts.ExecConfig.AttachStdin = true
		}),
	}

	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{"sh", "-c", script}, options...)
	require.NoError(t, err, "container script should start")

	outputBytes, err := io.ReadAll(reader)
	require.NoError(t, err, "should read TTY output")

	return string(outputBytes), exitCode
}
