//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type sweepNextResult struct {
	Task *struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"task"`
	Reason           string `json:"reason"`
	FestivalComplete bool   `json:"festival_complete"`
	Sweep            *struct {
		Number         int      `json:"number"`
		DeferredTotal  int      `json:"deferred_total"`
		BlockerMessage string   `json:"blocker_message"`
		Attempts       []string `json:"attempts"`
		DeferralReason string   `json:"deferral_reason"`
		CompletedSince []string `json:"completed_since"`
		Remaining      []struct {
			Task           string `json:"task"`
			BlockerMessage string `json:"blocker_message"`
			DeferralReason string `json:"deferral_reason"`
		} `json:"remaining"`
	} `json:"sweep"`
}

func nextResult(t *testing.T, tc *TestContainer, festPath string) (sweepNextResult, string) {
	t.Helper()
	output, err := tc.RunFestInDir(festPath, "next", "--json")
	require.NoError(t, err, "fest next --json: %s", output)

	var result sweepNextResult
	require.NoError(t, json.Unmarshal([]byte(output), &result), "decode fest next --json: %s", output)
	return result, output
}

// drainToSweep completes whatever fest next offers until the sweep starts.
// The bound is the safety net the sweep's own termination argument depends on:
// if this loop ever runs out, the sweep is not bounded and the test fails.
func drainToSweep(t *testing.T, tc *TestContainer, festPath string) (sweepNextResult, string) {
	t.Helper()
	for range 20 {
		result, raw := nextResult(t, tc, festPath)
		if result.Sweep != nil {
			return result, raw
		}
		require.False(t, result.FestivalComplete,
			"the festival must not read as complete with a deferred blocker open: %s", raw)
		require.NotNil(t, result.Task, "fest next offered nothing before the sweep: %s", raw)

		output, err := tc.RunFestInDir(festPath, "task", "completed", result.Task.Path, "--yes")
		require.NoError(t, err, "fest task completed %s: %s", result.Task.Path, output)
	}
	t.Fatal("the sweep never started after twenty hand-offs")
	return sweepNextResult{}, ""
}

func TestSweepHandsBackTheDeferredBlockerAtTheEnd(t *testing.T) {
	tc := GetSharedContainer(t)
	festPath := setupImplementationFestival(t, tc, "sweep-end")

	const blockedTask = "001_IMPLEMENTATION/01_core_work/01_first_task.md"

	output, err := tc.RunFestInDir(festPath, "task", "blocked", blockedTask,
		"--reason", "'upstream provider removed the endpoint'",
		"--tried", "'checked the changelog'", "--yes")
	require.NoError(t, err, "fest task blocked: %s", output)

	seedDeferral(t, tc, festPath, blockedTask, "the vendor replies next week")

	result, raw := drainToSweep(t, tc, festPath)
	t.Logf("fest next --json at the sweep:\n%s", raw)

	require.Equal(t, 1, result.Sweep.Number, "the first sweep is sweep 1")
	require.Equal(t, 1, result.Sweep.DeferredTotal, "one blocker is deferred")
	require.Equal(t, "upstream provider removed the endpoint", result.Sweep.BlockerMessage,
		"the sweep must show the executor's original blocker")
	require.Equal(t, []string{"checked the changelog"}, result.Sweep.Attempts,
		"the sweep must show what was already tried")
	require.Equal(t, "the vendor replies next week", result.Sweep.DeferralReason,
		"the sweep must show why the operator set it aside")
	require.NotEmpty(t, result.Sweep.CompletedSince,
		"the sweep must show what completed while the blocker was deferred")
	require.False(t, result.FestivalComplete,
		"a sweep is not a completed festival")
	require.NotNil(t, result.Task, "the sweep must hand back a task")
	require.True(t, strings.HasSuffix(result.Task.Path, "01_first_task.md"),
		"the sweep must hand back the deferred task, got %s", result.Task.Path)

	events, err := tc.ReadFile(festPath + "/.fest/progress_events.jsonl")
	require.NoError(t, err, "read progress events")
	require.Equal(t, 1, strings.Count(events, `"event":"sweep_started"`),
		"one sweep started, once")
	require.Contains(t, events, `"event":"blocker_revisited"`,
		"the hand-off must be recorded")

	output, err = tc.RunFestInDir(festPath, "task", "blocked", blockedTask,
		"--reason", "'still gone on the second attempt'",
		"--tried", "'tried the v2 endpoint'", "--yes")
	require.NoError(t, err, "fest task blocked again: %s", output)

	after, afterRaw := nextResult(t, tc, festPath)
	require.Nil(t, after.Task,
		"the sweep dead end must offer nothing an executor can run: %s", afterRaw)
	require.Equal(t,
		"1 deferred blockers remain after sweep 1. Unblock them, or promote with --force.",
		after.Reason, "the terminal message must match the design exactly")
	require.False(t, after.FestivalComplete,
		"a festival with an open deferred blocker is never complete: %s", afterRaw)
	require.NotNil(t, after.Sweep, "the terminal result must carry the sweep data")
	require.Len(t, after.Sweep.Remaining, 1, "the remaining deferred blocker must be listed")
	require.Equal(t, "still gone on the second attempt", after.Sweep.Remaining[0].BlockerMessage,
		"the re-block must have replaced the blocker while the deferral survived")
	require.Equal(t, "the vendor replies next week", after.Sweep.Remaining[0].DeferralReason,
		"a re-block must not undo the operator's deferral")

	rendered, err := tc.RunFestInDir(festPath, "next")
	require.NoError(t, err, "fest next: %s", rendered)
	t.Logf("fest next at the sweep dead end:\n%s", rendered)

	events, err = tc.ReadFile(festPath + "/.fest/progress_events.jsonl")
	require.NoError(t, err, "read progress events")
	require.Equal(t, 1, strings.Count(events, `"event":"sweep_started"`),
		"a sweep with no completion must not start another")

	// Design doc 05 H2: the half-tried sweep ends, and the executor still
	// cannot call the festival complete from its dead end.
	refused, err := tc.RunFestInDir(festPath, "status", "set", "completed")
	require.NoError(t, err, "fest status set completed prints its refusal rather than failing: %s", refused)
	require.Contains(t, refused, "1 deferred blockers are still open. fest next will revisit them.",
		"the sweep dead end must not let the festival be completed")

	stillActive, err := tc.CheckDirExists(festPath)
	require.NoError(t, err, "stat festival path")
	require.True(t, stillActive, "the refused completion must leave the festival where it was")
}

// TestPromotionRefusesWithDeferredBlockers is the executor's dead end proved
// through the real CLI: with a blocker deferred, neither promotion path
// completes the festival, and --force is refused off a terminal.
func TestPromotionRefusesWithDeferredBlockers(t *testing.T) {
	tc := GetSharedContainer(t)
	festPath := setupImplementationFestival(t, tc, "force-guard")

	const blockedTask = "001_IMPLEMENTATION/01_core_work/01_first_task.md"

	output, err := tc.RunFestInDir(festPath, "task", "blocked", blockedTask,
		"--reason", "'upstream provider removed the endpoint'",
		"--tried", "'checked the changelog'", "--yes")
	require.NoError(t, err, "fest task blocked: %s", output)

	seedDeferral(t, tc, festPath, blockedTask, "the vendor replies next week")

	output, err = tc.RunFestInDir(festPath, "promote")
	require.NoError(t, err, "fest promote prints its refusal rather than failing: %s", output)
	require.Contains(t, output, "1 deferred blockers are still open. fest next will revisit them.",
		"the refusal must be the design's message")
	require.Contains(t, output, "Promote anyway with --force to record them as dropped.")
	require.Contains(t, output, "upstream provider removed the endpoint",
		"the refusal must list the deferred blocker")
	require.Contains(t, output, "--force drops every deferred blocker. There is no per-task drop yet.",
		"the refusal must name the per-task limit")

	forced, err := tc.RunFestInDir(festPath, "promote", "--force")
	require.Error(t, err, "fest promote --force off a terminal must fail: %s", forced)
	require.Contains(t, forced, "run this from your terminal",
		"--force must pass through the operator guard while blockers are deferred")

	setForced, err := tc.RunFestInDir(festPath, "status", "set", "completed", "--force")
	require.Error(t, err, "fest status set completed --force off a terminal must fail: %s", setForced)
	require.Contains(t, setForced, "run this from your terminal",
		"--force must pass through the operator guard on the status path too")

	stillActive, err := tc.CheckDirExists(festPath)
	require.NoError(t, err, "stat festival path")
	require.True(t, stillActive, "no refusal may move the festival out of active")

	exists, err := tc.CheckFileExists(festPath + "/DROPPED_BLOCKERS.md")
	require.NoError(t, err, "stat dropped blocker record")
	require.False(t, exists, "a refused forced completion must write no dropped blocker record")

	events, err := tc.ReadFile(festPath + "/.fest/progress_events.jsonl")
	require.NoError(t, err, "read progress events")
	require.NotContains(t, events, `"event":"forced_complete"`,
		"a refused forced completion must append no event")
}
