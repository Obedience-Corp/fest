//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// seedDeferral appends a blocker_deferred event to the store. The verb that
// writes this event refuses to run without an operator at a terminal, which is
// the point of the feature, so the fixture writes the event the operator's
// deferral would have written.
func seedDeferral(t *testing.T, tc *TestContainer, festPath, taskID, reason string) {
	t.Helper()

	eventsPath := festPath + "/.fest/progress_events.jsonl"
	existing, err := tc.ReadFile(eventsPath)
	require.NoError(t, err, "read progress events")
	require.Contains(t, existing, `"event":"blocked"`,
		"the task must be blocked before a deferral can be seeded")

	event := `{"ts":"` + time.Now().UTC().Format(time.RFC3339Nano) +
		`","event":"blocker_deferred","task":"` + taskID +
		`","deferral_reason":"` + reason +
		`","deferred_by":"Ada Lovelace","actor":"operator","tty":true}`

	require.NoError(t, tc.WriteFile(eventsPath, strings.TrimRight(existing, "\n")+"\n"+event+"\n"),
		"append the deferral event")
}

func TestUnblockNoteKeepsDownstreamWorkCompleted(t *testing.T) {
	tc := GetSharedContainer(t)
	festPath := setupImplementationFestival(t, tc, "unblock-note")

	const (
		blockedTask   = "001_IMPLEMENTATION/01_core_work/01_first_task.md"
		dependentTask = "001_IMPLEMENTATION/01_core_work/02_second_task.md"
	)

	output, err := tc.RunFestInDir(festPath, "task", "blocked", blockedTask,
		"--reason", "'upstream provider removed the endpoint'",
		"--tried", "'checked the changelog'", "--yes")
	require.NoError(t, err, "fest task blocked: %s", output)

	output, err = tc.RunFestInDir(festPath, "next", "--json")
	require.NoError(t, err, "fest next --json: %s", output)
	require.NotContains(t, output, "02_second_task",
		"a blocked, undeferred task must not let its dependent be offered")

	seedDeferral(t, tc, festPath, blockedTask, "the vendor replies next week")

	output, err = tc.RunFestInDir(festPath, "next", "--json")
	require.NoError(t, err, "fest next --json after the deferral: %s", output)
	require.Contains(t, output, "02_second_task",
		"a deferred blocker must let the dependent become ready")

	output, err = tc.RunFestInDir(festPath, "task", "completed", dependentTask, "--yes")
	require.NoError(t, err, "fest task completed: %s", output)

	output, err = tc.RunFestInDir(festPath, "task", "unblock", blockedTask,
		"--note", "'the v2 endpoint does the same thing, try that'")
	require.NoError(t, err, "fest task unblock --note: %s", output)

	events, err := tc.ReadFile(festPath + "/.fest/progress_events.jsonl")
	require.NoError(t, err, "read progress events")
	require.Contains(t, events, `"event":"unblocked"`,
		"the unblock must be recorded")
	require.Contains(t, events, "the v2 endpoint does the same thing, try that",
		"the operator note must be carried on the unblocked event")

	output, err = tc.RunFestInDir(festPath, "task", "show", dependentTask)
	require.NoError(t, err, "fest task show: %s", output)
	require.Contains(t, output, "completed",
		"work completed while the blocker was deferred must stay completed")

	output, err = tc.RunFestInDir(festPath, "task", "show", blockedTask)
	require.NoError(t, err, "fest task show: %s", output)
	require.Contains(t, output, "in_progress",
		"the unblocked task must return to in_progress")
}
