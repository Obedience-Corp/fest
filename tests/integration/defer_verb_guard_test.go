//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// deferGuardState is everything a deferral would have to change. A refusal that
// writes nothing leaves all three byte identical.
type deferGuardState struct {
	events      string
	taskFile    string
	nextJSON    string
	festivalDir []string
}

func captureDeferGuardState(t *testing.T, tc *TestContainer, festPath, taskPath string) deferGuardState {
	t.Helper()

	events, err := tc.ReadFile(festPath + "/.fest/progress_events.jsonl")
	require.NoError(t, err, "read progress events")

	taskFile, err := tc.ReadFile(festPath + "/" + taskPath)
	require.NoError(t, err, "read task file")

	nextJSON, err := tc.RunFestInDir(festPath, "next", "--json")
	require.NoError(t, err, "fest next --json: %s", nextJSON)

	entries, err := tc.ListDirectory(festPath + "/.fest")
	require.NoError(t, err, "list .fest")

	return deferGuardState{events: events, taskFile: taskFile, nextJSON: nextJSON, festivalDir: entries}
}

func TestDeferVerbRefusesOffATerminalAndWritesNothing(t *testing.T) {
	tc := GetSharedContainer(t)
	festPath := setupImplementationFestival(t, tc, "defer-guard")

	const blockedTask = "001_IMPLEMENTATION/01_core_work/01_first_task.md"

	output, err := tc.RunFestInDir(festPath, "task", "blocked", blockedTask,
		"--reason", "'upstream provider removed the endpoint'",
		"--tried", "'checked the changelog'", "--yes")
	require.NoError(t, err, "fest task blocked: %s", output)

	before := captureDeferGuardState(t, tc, festPath, blockedTask)

	output, err = tc.RunFestInDir(festPath, "task", "defer", blockedTask,
		"--reason", "'the vendor replies next week'")
	require.Error(t, err, "fest task defer off a terminal must fail: %s", output)
	require.Contains(t, output, "run this from your terminal",
		"the refusal must be the operator-decision message from design doc 04")

	after := captureDeferGuardState(t, tc, festPath, blockedTask)
	require.Equal(t, before.events, after.events,
		"a refused deferral must append no event")
	require.Equal(t, before.taskFile, after.taskFile,
		"a refused deferral must not touch the task file")
	require.Equal(t, before.nextJSON, after.nextJSON,
		"a refused deferral must not change what fest next offers")
	require.Equal(t, before.festivalDir, after.festivalDir,
		"a refused deferral must create no file in .fest")
	require.NotContains(t, after.events, "blocker_deferred",
		"nothing may write a deferral into the store from a refused verb")
}

func TestDeferVerbHasNoAgentFlags(t *testing.T) {
	tc := GetSharedContainer(t)
	festPath := setupImplementationFestival(t, tc, "defer-flags")

	const blockedTask = "001_IMPLEMENTATION/01_core_work/01_first_task.md"

	output, err := tc.RunFestInDir(festPath, "task", "blocked", blockedTask,
		"--reason", "'upstream provider removed the endpoint'", "--yes")
	require.NoError(t, err, "fest task blocked: %s", output)

	before := captureDeferGuardState(t, tc, festPath, blockedTask)

	for _, flag := range []string{"--yes", "--json"} {
		output, err := tc.RunFestInDir(festPath, "task", "defer", blockedTask,
			"--reason", "'the vendor replies next week'", flag)
		require.Error(t, err, "fest task defer %s must fail: %s", flag, output)
		require.Contains(t, output, "unknown flag",
			"cobra must reject %s before any code runs", flag)
	}

	help, err := tc.RunFestInDir(festPath, "task", "defer", "--help")
	require.NoError(t, err, "fest task defer --help: %s", help)
	require.Contains(t, help, "no --yes and no --json",
		"the help text must say why the agent flags are absent")

	after := captureDeferGuardState(t, tc, festPath, blockedTask)
	require.Equal(t, before.events, after.events,
		"an unknown flag must append no event")
	require.Equal(t, before.taskFile, after.taskFile,
		"an unknown flag must not touch the task file")
}
