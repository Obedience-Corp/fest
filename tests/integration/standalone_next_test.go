//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStandaloneNext_AnonymousReadOnly verifies fest next on a WORKFLOW.md
// without .workflow/ is a pure read: it renders step 1 but creates no files
// (D013 cwd-only detection + anonymous render contract from 008.02).
func TestStandaloneNext_AnonymousReadOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)
	const dir = "/tmp/standalone-anon"
	_, err := container.Exec("rm", "-rf", dir)
	require.NoError(t, err)

	workflowMD := `# Test Workflow

## Step 1: First

Do the first thing.

## Step 2: Second

Do the second thing.
`
	require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", workflowMD))

	out, err := container.RunFestInDir(dir, "next")
	require.NoError(t, err, "fest next should succeed in anonymous mode: %s", out)
	require.Contains(t, out, "Step 1")

	exists, _ := container.CheckDirExists(dir + "/.workflow")
	require.False(t, exists, "anonymous fest next must not create .workflow/")
	exists, _ = container.CheckFileExists(dir + "/.workitem")
	require.False(t, exists, "anonymous fest next must not create .workitem")
}

// TestStandaloneNext_BootstrapAndAdvance covers the full PR 170 flow:
// anonymous -> bootstrap via fest workflow advance -> tracked -> fest next
// shows the next step (not the just-completed one, D030 #6) -> advance again.
// Asserts .workitem is never created (D007).
func TestStandaloneNext_BootstrapAndAdvance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)
	const dir = "/tmp/standalone-bootstrap"
	_, err := container.Exec("rm", "-rf", dir)
	require.NoError(t, err)

	workflowMD := `# Bootstrap Workflow

## Step 1: Plan

Plan the work.

## Step 2: Execute

Do the work.

## Step 3: Verify

Verify the work.
`
	require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", workflowMD))

	t.Run("AdvanceBootstrapsAndCompletesStep1", func(t *testing.T) {
		out, err := container.RunFestInDir(dir, "workflow", "advance")
		require.NoError(t, err, "advance should bootstrap anonymous workflow: %s", out)
		require.Contains(t, out, "Bootstrapped")

		exists, _ := container.CheckDirExists(dir + "/.workflow")
		require.True(t, exists, "advance should create .workflow/")
		exists, _ = container.CheckFileExists(dir + "/.workitem")
		require.False(t, exists, "fest must not write .workitem (D007)")
	})

	t.Run("NextShowsStep2NotJustCompleted", func(t *testing.T) {
		out, err := container.RunFestInDir(dir, "next")
		require.NoError(t, err, "fest next after bootstrap: %s", out)
		// D030 #6: post-bootstrap should show step 2, not the just-completed step 1.
		require.Contains(t, out, "Step 2", "should advance render past completed step 1")
		require.Contains(t, out, "Execute")
	})

	t.Run("AdvanceTrackedMovesToNextStep", func(t *testing.T) {
		// D030 #5: after bootstrap, advance must handle tracked mode without
		// falling through to the festival navigator.
		out, err := container.RunFestInDir(dir, "workflow", "advance")
		require.NoError(t, err, "tracked advance should succeed: %s", out)
		require.Contains(t, out, "Step 2")
		require.Contains(t, out, "completed")
	})
}

// TestStandaloneAdvance_RejectsEmptyWorkflow locks in D030 #7: bootstrap
// must validate WORKFLOW.md has parseable steps before creating .workflow/.
func TestStandaloneAdvance_RejectsEmptyWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)
	const dir = "/tmp/standalone-empty"
	_, err := container.Exec("rm", "-rf", dir)
	require.NoError(t, err)

	require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", "# No steps here\n"))

	out, err := container.RunFestInDir(dir, "workflow", "advance")
	require.Error(t, err, "advance must reject WORKFLOW.md with no steps")
	require.True(t,
		strings.Contains(out, "no parseable steps") || strings.Contains(out, "no steps"),
		"error should mention missing steps, got: %s", out,
	)

	exists, _ := container.CheckDirExists(dir + "/.workflow")
	require.False(t, exists, ".workflow/ must not be created on validation failure (D030 #7)")
}

// TestStandaloneAdvance_RejectsBlockingCheckpoint pins #173 review feedback:
// standalone runs do not yet implement approve/reject/reset, so a blocking
// checkpoint reached by tracked-mode advance must fail closed instead of
// silently appending wf_step_start and pointing the user toward
// festival-only commands.
func TestStandaloneAdvance_RejectsBlockingCheckpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)
	const dir = "/tmp/standalone-checkpoint"
	_, err := container.Exec("rm", "-rf", dir)
	require.NoError(t, err)

	const workflowMD = "## Step 1: First\n\n**Goal:** Do.\n\n**Output:** Done.\n\n## Step 2: Second\n\n**Goal:** Approve.\n\n**Output:** Approved.\n\n**Checkpoint:** USER APPROVAL REQUIRED\n\n## Step 3: Third\n\n**Goal:** Finish.\n"
	require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", workflowMD))

	bootstrapOut, err := container.RunFestInDir(dir, "workflow", "advance")
	require.NoError(t, err, "bootstrap (step 1, no checkpoint) should succeed: %s", bootstrapOut)

	checkpointOut, checkpointErr := container.RunFestInDir(dir, "workflow", "advance")
	require.Error(t, checkpointErr,
		"advance into step 2's blocking checkpoint must refuse cleanly: %s", checkpointOut)
	require.Contains(t, checkpointOut, "blocking checkpoints",
		"error should explain standalone limitation, got: %s", checkpointOut)

	retryOut, retryErr := container.RunFestInDir(dir, "workflow", "advance")
	require.Error(t, retryErr,
		"second advance must remain at the checkpoint, not silently progress: %s", retryOut)
	require.Contains(t, retryOut, "blocking checkpoints",
		"second attempt should hit the same checkpoint, got: %s", retryOut)
}

// TestStandaloneAdvance_BlockedRunHintStaysStandalone pins the tracked
// blocked-state branch: if an existing .workflow/ event stream is already
// blocked, standalone mode must not point the user at festival-only
// approve/reset commands.
func TestStandaloneAdvance_BlockedRunHintStaysStandalone(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)
	const dir = "/tmp/standalone-blocked-run"
	_, err := container.Exec("rm", "-rf", dir)
	require.NoError(t, err)

	const workflowMD = "## Step 1: First\n\n**Goal:** Do.\n\n## Step 2: Second\n\n**Goal:** Continue.\n"
	require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", workflowMD))

	out, err := container.RunFestInDir(dir, "workflow", "init")
	require.NoError(t, err, "init: %s", out)
	out, err = container.RunFestInDir(dir, "workflow", "start")
	require.NoError(t, err, "start: %s", out)

	_, err = container.Exec("sh", "-c",
		`f=$(ls `+dir+`/.workflow/runs/*/progress_events.jsonl); `+
			`printf '%s\n' '{"version":1,"event_id":"evt_manual_start","event_type":"wf_step_start","run_id":"manual","timestamp":"2026-05-18T00:00:00Z"}' >> "$f"; `+
			`printf '%s\n' '{"version":1,"event_id":"evt_manual_block","event_type":"wf_step_block","run_id":"manual","timestamp":"2026-05-18T00:00:01Z"}' >> "$f"`)
	require.NoError(t, err)

	out, err = container.RunFestInDir(dir, "workflow", "advance")
	require.Error(t, err, "advance must reject already-blocked standalone run: %s", out)
	require.Contains(t, out, "standalone workflow step is blocked")
	require.Contains(t, out, "standalone workflows do not support approve/reset yet")
	require.NotContains(t, out, "fest workflow approve")
	require.NotContains(t, out, "fest workflow reset")
}

// TestStandaloneAdvance_BootstrapRejectsStep1Checkpoint pins the second
// re-review finding on #173: anonymous bootstrap previously appended
// wf_step_start + wf_step_done unconditionally, silently auto-approving a
// blocking checkpoint on step 1. The bootstrap path must apply the same
// fail-closed guard as runAdvanceTracked, and refuse before creating
// .workflow/ at all.
func TestStandaloneAdvance_BootstrapRejectsStep1Checkpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)
	const dir = "/tmp/standalone-bootstrap-checkpoint"
	_, err := container.Exec("rm", "-rf", dir)
	require.NoError(t, err)

	const workflowMD = "## Step 1: Approve\n\n**Goal:** Approve.\n\n**Output:** Approved.\n\n**Checkpoint:** USER APPROVAL REQUIRED\n\n## Step 2: Second\n\n**Goal:** Then.\n"
	require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", workflowMD))

	out, advanceErr := container.RunFestInDir(dir, "workflow", "advance")
	require.Error(t, advanceErr,
		"bootstrap into step 1 with blocking checkpoint must refuse: %s", out)
	require.Contains(t, out, "blocking checkpoints",
		"error should explain standalone limitation, got: %s", out)

	manifestExists, _ := container.CheckFileExists(dir + "/.workflow/workflow.yaml")
	require.False(t, manifestExists,
		"refused bootstrap must not create .workflow/ — refusal is pre-mutation")
}
