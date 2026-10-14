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

	output, err := tc.RunFestInDir(festPath, "task", "defer", blockedTask,
		"--reason", "'the vendor replies next week'", "--yes", "--json")
	require.Error(t, err, "--yes must not defer a task that is not blocked: %s", output)
	require.Contains(t, output, "only a blocked task can be deferred")

	output, err = tc.RunFestInDir(festPath, "task", "blocked", blockedTask,
		"--reason", "'upstream provider removed the endpoint'",
		"--tried", "'checked the changelog'", "--yes")
	require.NoError(t, err, "fest task blocked: %s", output)

	before := captureDeferGuardState(t, tc, festPath, blockedTask)

	output, err = tc.RunFestInDir(festPath, "task", "defer", blockedTask,
		"--reason", "'the vendor replies next week'")
	require.Error(t, err, "fest task defer off a terminal must fail: %s", output)
	require.Contains(t, output, "pass --yes for non-interactive use")

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

func TestDeferVerbRequiresYesForJSONAndLetsAnAgentDefer(t *testing.T) {
	tc := GetSharedContainer(t)
	festPath := setupImplementationFestival(t, tc, "defer-flags")

	const blockedTask = "001_IMPLEMENTATION/01_core_work/01_first_task.md"

	output, err := tc.RunFestInDir(festPath, "task", "blocked", blockedTask,
		"--reason", "'upstream provider removed the endpoint'", "--yes")
	require.NoError(t, err, "fest task blocked: %s", output)

	before := captureDeferGuardState(t, tc, festPath, blockedTask)

	output, err = tc.RunFestInDir(festPath, "task", "defer", blockedTask,
		"--reason", "'the vendor replies next week'", "--json")
	require.Error(t, err, "--json without --yes must fail: %s", output)
	require.Contains(t, output, "confirmation required")

	afterRefusal := captureDeferGuardState(t, tc, festPath, blockedTask)
	require.Equal(t, before.events, afterRefusal.events)
	require.Equal(t, before.taskFile, afterRefusal.taskFile)
	require.Equal(t, before.nextJSON, afterRefusal.nextJSON)

	output, err = tc.Exec("sh", "-c", "cd "+festPath+
		" && CODEX_TASK=integration /fest task defer "+blockedTask+
		" --reason 'the vendor replies next week' --yes --json")
	require.NoError(t, err, "agent deferral off a terminal: %s", output)
	require.Contains(t, output, `"deferred": true`)
	require.Contains(t, output, `"actor": "automation"`)

	help, err := tc.RunFestInDir(festPath, "task", "defer", "--help")
	require.NoError(t, err, "fest task defer --help: %s", help)
	require.Contains(t, help, "--yes")

	after := captureDeferGuardState(t, tc, festPath, blockedTask)
	require.Contains(t, after.events, `"event":"blocker_deferred"`)
	require.Contains(t, after.events, `"actor":"automation"`)
	require.Contains(t, after.events, `"deferred_by":"automation"`)
	require.Equal(t, before.taskFile, after.taskFile,
		"deferral must not touch the task file")
	require.NotEqual(t, before.nextJSON, after.nextJSON,
		"the dependent task must become ready")
}
