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

// TestStandaloneAdvance_StopsAtBlockingCheckpoint pins the standalone
// checkpoint contract: advancing a step with a blocking checkpoint records the
// work as done and stops, repeated advances stay there, an agent (no terminal)
// cannot approve, and the run only moves on a recorded human decision.
func TestStandaloneAdvance_StopsAtBlockingCheckpoint(t *testing.T) {
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

	checkpointOut, err := container.RunFestInDir(dir, "workflow", "advance")
	require.NoError(t, err, "advance into step 2's checkpoint records the work and stops: %s", checkpointOut)
	require.Contains(t, checkpointOut, "Awaiting approval")
	require.Contains(t, checkpointOut, "fest workflow approve")

	retryOut, err := container.RunFestInDir(dir, "workflow", "advance")
	require.NoError(t, err, "a second advance must stay at the checkpoint: %s", retryOut)
	require.Contains(t, retryOut, "Awaiting approval",
		"second advance must report the same checkpoint, not progress: %s", retryOut)

	statusOut, err := container.RunFestInDir(dir, "workflow", "status", "--json")
	require.NoError(t, err, "status: %s", statusOut)
	require.Contains(t, statusOut, `"awaiting_approval": true`)
	require.Contains(t, statusOut, `"completed_steps": 1`)

	nextOut, err := container.RunFestInDir(dir, "next")
	require.NoError(t, err, "next: %s", nextOut)
	require.Contains(t, nextOut, "awaiting approval")

	// The container has no terminal, which is exactly an agent's situation.
	approveOut, approveErr := container.RunFestInDir(dir, "workflow", "approve")
	require.Error(t, approveErr, "approve without a terminal must be refused: %s", approveOut)
	require.Contains(t, approveOut, "interactive terminal")

	pinnedOut, pinnedErr := container.RunFestInDir(dir, "workflow", "reject", "--reason", "no", "--expect-step", "3")
	require.Error(t, pinnedErr, "reject pinned to another step must be refused: %s", pinnedOut)
	require.Contains(t, pinnedOut, "not at the expected step")

	rejectOut, err := container.RunFestInDir(dir, "workflow", "reject", "--reason", "missing-evidence", "--expect-step", "2")
	require.NoError(t, err, "reject: %s", rejectOut)
	require.Contains(t, rejectOut, "missing-evidence")

	statusOut, err = container.RunFestInDir(dir, "workflow", "status", "--json")
	require.NoError(t, err, "status after reject: %s", statusOut)
	require.NotContains(t, statusOut, `"awaiting_approval": true`)
	require.Contains(t, statusOut, `"feedback": "missing-evidence"`)

	resubmitOut, err := container.RunFestInDir(dir, "workflow", "advance")
	require.NoError(t, err, "advance after a rejection re-submits the step: %s", resubmitOut)
	require.Contains(t, resubmitOut, "re-submitted")

	statusOut, err = container.RunFestInDir(dir, "workflow", "status", "--json")
	require.NoError(t, err, "status after re-submit: %s", statusOut)
	require.Contains(t, statusOut, `"awaiting_approval": true`)
	require.Contains(t, statusOut, `"completed_steps": 1`)
}

// TestStandaloneAdvance_BlockedRunWithoutCheckpoint pins the tracked
// blocked-state branch for a block that no checkpoint recorded (a hand-edited
// stream): there is no decision to record, so the run must not point the user
// at approve.
func TestStandaloneAdvance_BlockedRunWithoutCheckpoint(t *testing.T) {
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
	require.Error(t, err, "advance must reject a run blocked outside a checkpoint: %s", out)
	require.Contains(t, out, "standalone workflow step is blocked")
	require.NotContains(t, out, "fest workflow approve")
	require.NotContains(t, out, "fest workflow reset")

	out, err = container.RunFestInDir(dir, "workflow", "reject", "--reason", "x")
	require.Error(t, err, "reject must refuse a step with no checkpoint: %s", out)
	require.Contains(t, out, "does not have a blocking checkpoint")
}

// TestStandaloneAdvance_BootstrapStopsAtStep1Checkpoint pins the second
// re-review finding on #173: anonymous bootstrap previously appended
// wf_step_start + wf_step_done unconditionally, silently auto-approving a
// blocking checkpoint on step 1. Bootstrap now creates the run and stops at
// that checkpoint; the step is not completed until it is approved.
func TestStandaloneAdvance_BootstrapStopsAtStep1Checkpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)
	const dir = "/tmp/standalone-bootstrap-checkpoint"
	_, err := container.Exec("rm", "-rf", dir)
	require.NoError(t, err)

	const workflowMD = "## Step 1: Approve\n\n**Goal:** Approve.\n\n**Output:** Approved.\n\n**Checkpoint:** USER APPROVAL REQUIRED\n\n## Step 2: Second\n\n**Goal:** Then.\n"
	require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", workflowMD))

	out, err := container.RunFestInDir(dir, "workflow", "advance")
	require.NoError(t, err, "bootstrap into a step 1 checkpoint stops there: %s", out)
	require.Contains(t, out, "Awaiting approval")

	statusOut, err := container.RunFestInDir(dir, "workflow", "status", "--json")
	require.NoError(t, err, "status: %s", statusOut)
	require.Contains(t, statusOut, `"awaiting_approval": true`)
	require.Contains(t, statusOut, `"completed_steps": 0`,
		"the checkpoint step must not be completed by bootstrap: %s", statusOut)
}
