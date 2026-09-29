//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// acceptanceNextResult decodes the fields this scenario asserts on. A struct
// rather than substring matching, so a renamed field fails loudly.
type acceptanceNextResult struct {
	Task *struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"task"`
	ParallelTasks []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"parallel_tasks"`
	Reason           string `json:"reason"`
	FestivalComplete bool   `json:"festival_complete"`
	DeferredBlockers []struct {
		Task           string `json:"task"`
		BlockerMessage string `json:"blocker_message"`
		DeferralReason string `json:"deferral_reason"`
		DeferredAt     string `json:"deferred_at"`
	} `json:"deferred_blockers"`
	BlockedTasks []struct {
		Task           string `json:"task"`
		BlockerMessage string `json:"blocker_message"`
	} `json:"blocked_tasks"`
	Sweep *struct {
		Number         int      `json:"number"`
		DeferredTotal  int      `json:"deferred_total"`
		BlockerMessage string   `json:"blocker_message"`
		DeferralReason string   `json:"deferral_reason"`
		CompletedSince []string `json:"completed_since"`
	} `json:"sweep"`
}

func acceptanceNext(t *testing.T, tc *TestContainer, festPath string) (acceptanceNextResult, string) {
	t.Helper()
	output, err := tc.RunFestInDir(festPath, "next", "--json")
	require.NoError(t, err, "fest next --json: %s", output)

	var result acceptanceNextResult
	require.NoError(t, json.Unmarshal([]byte(output), &result), "decode fest next --json: %s", output)
	return result, output
}

// requireNoAgentMarkers proves the container did not inherit the outer
// environment. Without this, a CI change that starts passing the environment
// through would turn the deferral step into a confusing refusal rather than a
// clear failure.
func requireNoAgentMarkers(t *testing.T, tc *TestContainer) {
	t.Helper()
	out, err := tc.Exec("env")
	require.NoError(t, err, "read the container environment")

	for _, marker := range []string{
		"OBEY_AGENT", "CLAUDE_CODE", "CLAUDECODE", "CLAUDE_CODE_SESSION_ID",
		"CODEX_TASK", "OBEY_SESSION_ID",
	} {
		require.NotContains(t, out, marker+"=",
			"the container inherited the agent marker %s; the operator guard would refuse", marker)
	}
}

// drainToSweepAcceptance completes whatever fest next offers until the sweep
// starts, so the scenario reaches its end state through the real CLI. The bound
// is the safety net for the sweep's own termination argument.
func drainToSweepAcceptance(t *testing.T, tc *TestContainer, festPath string) (acceptanceNextResult, string) {
	t.Helper()
	for range 20 {
		result, raw := acceptanceNext(t, tc, festPath)
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
	return acceptanceNextResult{}, ""
}

// TestDeferralAcceptanceScenario is design doc 03's acceptance scenario run end
// to end through the real binary. Tasks 01, 02 and 03 run in order, so 03
// depends on 02 through the sequence's implicit ordering, which is the same
// edge an explicit fest_dependencies entry would create and is asserted
// behaviourally below rather than declared.
func TestDeferralAcceptanceScenario(t *testing.T) {
	tc := GetSharedContainer(t)
	requireNoAgentMarkers(t, tc)

	festPath := setupImplementationFestival(t, tc, "deferral-acceptance")

	const (
		taskOne   = "001_IMPLEMENTATION/01_core_work/01_first_task.md"
		taskTwo   = "001_IMPLEMENTATION/01_core_work/02_second_task.md"
		taskThree = "001_IMPLEMENTATION/01_core_work/03_third_task.md"

		blockerMessage = "the required API no longer exists"
		deferralReason = "API removed upstream; revisit after the rest lands"
	)

	// Step 1: 01 completes.
	output, err := tc.RunFestInDir(festPath, "task", "completed", taskOne, "--yes")
	require.NoError(t, err, "fest task completed 01: %s", output)

	// Step 2: fest next selects 02, which is also the dependency proof: 03 is
	// never offered before 02.
	result, raw := acceptanceNext(t, tc, festPath)
	require.NotNil(t, result.Task, "fest next offered nothing: %s", raw)
	require.Contains(t, result.Task.Path, "02_second_task",
		"fest next must select 02 before 03: %s", raw)

	// Step 3: the executor reports 02 blocked.
	output, err = tc.RunFestInDir(festPath, "task", "blocked", taskTwo,
		"--reason", "'"+blockerMessage+"'",
		"--tried", "'checked the vendor changelog'", "--yes")
	require.NoError(t, err, "fest task blocked 02: %s", output)

	// Step 4: the negative assertion. Reporting blocked and running fest next
	// moves nothing. This is design doc 05 scenario A1 and it is the property
	// the whole festival is built not to break.
	result, raw = acceptanceNext(t, tc, festPath)
	require.Nil(t, result.Task,
		"a blocked task with no deferral must leave fest next with nothing to offer: %s", raw)
	require.NotContains(t, raw, "03_third_task",
		"the dependent of a blocked task must never be offered: %s", raw)
	require.Len(t, result.ParallelTasks, 0,
		"no parallel task may be offered around an open blocker: %s", raw)
	require.Len(t, result.BlockedTasks, 1,
		"the stalled result must name the open blocker: %s", raw)
	require.Equal(t, blockerMessage, result.BlockedTasks[0].BlockerMessage,
		"the stalled result must carry the executor's message: %s", raw)
	require.Nil(t, result.Sweep, "nothing is deferred, so no sweep: %s", raw)

	// Step 5: the operator defers, on a real terminal, typing the task number.
	// No guard is weakened: the command sees a pseudo-terminal on stdin, the
	// container carries no agent marker, and the process ancestry is sh only.
	output, err = tc.RunFestInDirTTYWithInput(festPath, "02\n",
		"task", "defer", taskTwo, "--reason", "'"+deferralReason+"'")
	require.NoError(t, err, "fest task defer 02 through a TTY: %s", output)
	require.Contains(t, output, "Blocker deferred",
		"the deferral must report success: %s", output)
	require.Contains(t, output, "Type the task number (02) to defer",
		"the typed confirmation must have been asked for: %s", output)

	events, err := tc.ReadFile(festPath + "/.fest/progress_events.jsonl")
	require.NoError(t, err, "read progress events")
	require.Contains(t, events, `"event":"blocker_deferred"`,
		"the deferral must be durable in the event log")
	require.Contains(t, events, `"tty":true`,
		"the audit record must show the guard saw a terminal")

	// Step 6: fest next now selects 03 and says what it is building on.
	result, raw = acceptanceNext(t, tc, festPath)
	require.NotNil(t, result.Task, "fest next offered nothing after the deferral: %s", raw)
	require.Contains(t, result.Task.Path, "03_third_task",
		"the deferral must release the dependent: %s", raw)
	require.Len(t, result.DeferredBlockers, 1,
		"the dependent must carry its deferred blocker: %s", raw)
	require.Equal(t, taskTwo, result.DeferredBlockers[0].Task)
	require.Equal(t, blockerMessage, result.DeferredBlockers[0].BlockerMessage)
	require.Equal(t, deferralReason, result.DeferredBlockers[0].DeferralReason)
	require.NotEmpty(t, result.DeferredBlockers[0].DeferredAt,
		"the deferral timestamp must travel with the reference: %s", raw)
	require.Len(t, result.BlockedTasks, 0,
		"a deferred blocker is no longer what the executor is waiting on: %s", raw)

	// Step 7: 03 completes.
	output, err = tc.RunFestInDir(festPath, "task", "completed", taskThree, "--yes")
	require.NoError(t, err, "fest task completed 03: %s", output)

	// Step 8: the sweep returns 02 rather than reporting the festival complete.
	result, raw = drainToSweepAcceptance(t, tc, festPath)
	t.Logf("fest next --json at the sweep:\n%s", raw)
	require.False(t, result.FestivalComplete,
		"a festival with a deferred blocker is not complete: %s", raw)
	require.NotNil(t, result.Task, "the sweep must hand a task back: %s", raw)
	require.Contains(t, result.Task.Path, "02_second_task",
		"the sweep must return the deferred task: %s", raw)
	require.Equal(t, 1, result.Sweep.Number)
	require.Equal(t, 1, result.Sweep.DeferredTotal)
	require.Equal(t, blockerMessage, result.Sweep.BlockerMessage)
	require.Equal(t, deferralReason, result.Sweep.DeferralReason)
	require.Contains(t, strings.Join(result.Sweep.CompletedSince, "\n"), "03_third_task",
		"the sweep must show what completed since the deferral: %s", raw)

	// Step 9: completion is refused while 02 is deferred. The refusal is
	// printed and the festival is left alone rather than the command exiting
	// non-zero, which is the behaviour 004/03/03 shipped on both completion
	// paths and what TestPromotionRefusesWithDeferredBlockers already records.
	refusal, err := tc.RunFestInDir(festPath, "status", "set", "completed")
	require.NoError(t, err, "fest status set completed prints its refusal: %s", refusal)
	require.Contains(t, refusal, "1 deferred blockers are still open. fest next will revisit them.",
		"the refusal must be the design's message: %s", refusal)
	require.Contains(t, refusal, taskTwo,
		"the refusal must name the deferred task: %s", refusal)
	require.Contains(t, refusal, "Promote anyway with --force to record them as dropped.",
		"the refusal must name the only way past it: %s", refusal)

	stillActive, err := tc.CheckDirExists(festPath)
	require.NoError(t, err, "stat festival path")
	require.True(t, stillActive, "a refused completion must leave the festival where it is")

	events, err = tc.ReadFile(festPath + "/.fest/progress_events.jsonl")
	require.NoError(t, err, "read progress events")
	require.NotContains(t, events, `"event":"forced_complete"`,
		"a refused completion must append no forced completion event")

	dropped, err := tc.CheckFileExists(festPath + "/DROPPED_BLOCKERS.md")
	require.NoError(t, err, "stat dropped blocker record")
	require.False(t, dropped, "a refused completion must write no dropped blocker record")

	forced, err := tc.RunFestInDir(festPath, "status", "set", "completed", "--force")
	require.Error(t, err, "--force off a terminal must fail: %s", forced)
	require.Contains(t, forced, "run this from your terminal",
		"--force must pass through the operator guard while a blocker is deferred: %s", forced)

	// Step 10: the audit surface shows the blocker message and the reason.
	listed, err := tc.RunFestInDir(festPath, "task", "blocked", "--list", "--deferred")
	require.NoError(t, err, "fest task blocked --list --deferred: %s", listed)
	require.Contains(t, listed, taskTwo, "the list must name the deferred task: %s", listed)
	require.Contains(t, listed, blockerMessage,
		"the list must show the executor's blocker message: %s", listed)
	require.Contains(t, listed, "reason: "+deferralReason,
		"the list must show the operator's deferral reason: %s", listed)
	require.Contains(t, listed, "Deferred", "the deferred section heading must be present: %s", listed)
	require.NotContains(t, listed, "Open",
		"--deferred must print no empty Open heading: %s", listed)

	open, err := tc.RunFestInDir(festPath, "task", "blocked", "--list", "--open")
	require.NoError(t, err, "fest task blocked --list --open: %s", open)
	require.Equal(t, "No blocked tasks.\n", open,
		"with the only blocker deferred, the open list is empty: %q", open)
}
