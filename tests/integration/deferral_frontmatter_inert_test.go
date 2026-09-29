//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type inertNextResult struct {
	Task *struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Status string `json:"status"`
	} `json:"task"`
	Reason           string `json:"reason"`
	FestivalComplete bool   `json:"festival_complete"`
}

// rewriteFrontmatterWithDeferral replaces a task file's whole frontmatter block
// with one carrying blocker_deferred: true, leaving the body untouched. A whole
// block rewrite is used instead of a string replace so a miss cannot silently
// no-op and read as a pass.
func rewriteFrontmatterWithDeferral(t *testing.T, tc *TestContainer, taskPath string) {
	t.Helper()

	original, err := tc.ReadFile(taskPath)
	require.NoError(t, err, "read task file %s", taskPath)
	require.True(t, strings.HasPrefix(original, "---\n"), "task file must open with frontmatter: %s", original)

	parts := strings.SplitN(original, "\n---\n", 2)
	require.Len(t, parts, 2, "task file must have a closing frontmatter fence: %s", original)

	block := strings.TrimPrefix(parts[0], "---\n")
	rewritten := "---\n" + block + "\nblocker_deferred: true\n---\n" + parts[1]
	require.NoError(t, tc.WriteFile(taskPath, rewritten), "write task file %s", taskPath)

	readBack, err := tc.ReadFile(taskPath)
	require.NoError(t, err, "read back task file %s", taskPath)
	require.Contains(t, readBack, "blocker_deferred: true",
		"the fixture key must actually be on disk before the inertness claim is tested")
}

func decodeNext(t *testing.T, tc *TestContainer, festPath string) (inertNextResult, map[string]any, string) {
	t.Helper()

	output, err := tc.RunFestInDir(festPath, "next", "--json")
	require.NoError(t, err, "fest next --json: %s", output)

	var typed inertNextResult
	require.NoError(t, json.Unmarshal([]byte(output), &typed), "decode fest next --json: %s", output)

	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &raw), "decode fest next --json into a map: %s", output)

	return typed, raw, output
}

func TestFrontmatterDeferralKeyIsInert(t *testing.T) {
	tc := GetSharedContainer(t)
	festPath := setupImplementationFestival(t, tc, "deferral-inert")

	const (
		blockedTask  = "001_IMPLEMENTATION/01_core_work/01_first_task.md"
		dependentTsk = "02_second_task.md"
	)

	output, err := tc.RunFestInDir(festPath, "task", "blocked", blockedTask,
		"--reason", "'upstream provider removed the endpoint'", "--yes")
	require.NoError(t, err, "fest task blocked: %s", output)

	_, beforeRaw, _ := decodeNext(t, tc, festPath)
	require.Empty(t, beforeRaw["deferred_blockers"],
		"nothing is deferred in the store, so the result must carry no deferred blockers")

	rewriteFrontmatterWithDeferral(t, tc, festPath+"/"+blockedTask)

	after, afterRaw, afterJSON := decodeNext(t, tc, festPath)

	require.Equal(t, beforeRaw, afterRaw,
		"a blocker_deferred key in frontmatter must not change one field of fest next --json")
	require.Empty(t, afterRaw["deferred_blockers"],
		"a frontmatter key must not make fest next report a deferred blocker")

	require.Nil(t, after.Task,
		"fest next must still stall on the blocked task rather than hand out work")
	require.Contains(t, after.Reason, "dependencies not satisfied",
		"the stall must still be the dependency stall, not a deferral path")
	require.NotContains(t, afterJSON, dependentTsk,
		"the dependent task must not be named as selectable work while 01 is blocked")
	require.False(t, after.FestivalComplete,
		"a blocked task must not let the festival read as complete")

	events, err := tc.ReadFile(festPath + "/.fest/progress_events.jsonl")
	require.NoError(t, err, "read progress events")
	require.NotContains(t, events, "blocker_deferred",
		"nothing may write deferral into the store from a frontmatter edit")
	require.Contains(t, events, `"event":"blocked"`,
		"the store must still hold the real blocked event")
}
