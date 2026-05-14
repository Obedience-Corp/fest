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
