//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStandaloneWorkflow_InitStart exercises the standalone WORKFLOW.md
// init/start lifecycle inside a container, validating real filesystem effects
// (atomic writes, directory creation, manifest YAML shape) without mutating
// the host. Closes a slice of WW0001 task 008.01.10's container-harness
// requirement; the in-package store_test.go tests remain as pure unit tests
// for internal APIs the CLI surface can't drive directly.
func TestStandaloneWorkflow_InitStart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)

	const dir = "/tmp/standalone-wf"
	_, err := container.Exec("rm", "-rf", dir)
	require.NoError(t, err)

	workflowMD := `# Test Workflow

## Step 1: First

Do the first thing.

## Step 2: Second

Do the second thing.
`
	require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", workflowMD))

	t.Run("Init", func(t *testing.T) {
		out, err := container.RunFestInDir(dir, "workflow", "init")
		require.NoError(t, err, "fest workflow init: %s", out)
		require.Contains(t, out, "Initialized standalone workflow runtime")
		require.Contains(t, out, "workflow_id=wf-standalone-wf")

		manifest, err := container.ReadFile(dir + "/.workflow/workflow.yaml")
		require.NoError(t, err)
		require.Contains(t, manifest, "workflow_id: wf-standalone-wf")
		require.NotContains(t, manifest, "workitem_id", "manifest must not contain workitem_id post-D024")

		exists, _ := container.CheckFileExists(dir + "/.workitem")
		require.False(t, exists, "fest workflow init must not write .workitem (D007)")
	})

	t.Run("Start", func(t *testing.T) {
		out, err := container.RunFestInDir(dir, "workflow", "start")
		require.NoError(t, err, "fest workflow start: %s", out)

		runs, err := container.ListDirectory(dir + "/.workflow/runs")
		require.NoError(t, err)
		require.NotEmpty(t, runs, "expected at least one run file under runs/")

		// Find a run.yaml among the listed files.
		var runYAMLPath string
		for _, f := range runs {
			if strings.HasSuffix(f, "/run.yaml") {
				runYAMLPath = f
				break
			}
		}
		require.NotEmpty(t, runYAMLPath, "run.yaml not found under runs/")

		runYAML, err := container.ReadFile(runYAMLPath)
		require.NoError(t, err)
		require.Contains(t, runYAML, "status: active")
		require.NotContains(t, runYAML, "workitem_id", "run.yaml must not contain workitem_id post-D024")
	})

	t.Run("InitRejectsInvalidWorkflowID", func(t *testing.T) {
		badDir := "/tmp/standalone-bad"
		_, _ = container.Exec("rm", "-rf", badDir)
		require.NoError(t, container.WriteFile(badDir+"/WORKFLOW.md", "## Step 1: X\n"))

		out, err := container.RunFestInDir(badDir, "workflow", "init", "--workflow-id", "Bad Name!")
		require.Error(t, err, "expected non-zero exit for invalid workflow_id")
		require.Contains(t, out, "invalid workflow_id")
	})

	t.Run("InitRefusesUnparseableWorkflowDoc", func(t *testing.T) {
		emptyDir := "/tmp/standalone-empty"
		_, _ = container.Exec("rm", "-rf", emptyDir)
		require.NoError(t, container.WriteFile(emptyDir+"/WORKFLOW.md", "# No steps here\n\nJust prose.\n"))

		out, err := container.RunFestInDir(emptyDir, "workflow", "init")
		require.Error(t, err, "expected non-zero exit for WORKFLOW.md with no parseable steps")
		require.Contains(t, out, "no parseable steps")

		exists, _ := container.CheckFileExists(emptyDir + "/.workflow/workflow.yaml")
		require.False(t, exists, ".workflow/ must not be created when WORKFLOW.md is invalid")
	})

	t.Run("StartRefusesMissingWorkflowDoc", func(t *testing.T) {
		startDir := "/tmp/standalone-start-missing"
		_, _ = container.Exec("rm", "-rf", startDir)
		require.NoError(t, container.WriteFile(startDir+"/WORKFLOW.md", "## Step 1: X\n"))

		out, err := container.RunFestInDir(startDir, "workflow", "init")
		require.NoError(t, err, "init: %s", out)

		_, err = container.Exec("rm", startDir+"/WORKFLOW.md")
		require.NoError(t, err)

		out, err = container.RunFestInDir(startDir, "workflow", "start")
		require.Error(t, err, "start must fail when WORKFLOW.md is missing: %s", out)

		entries, _ := container.ListDirectory(startDir + "/.workflow/runs")
		require.Empty(t, entries, "no run dir should be created on hash failure: %v", entries)
	})

	t.Run("StartRefusesUnparseableWorkflowDoc", func(t *testing.T) {
		startDir := "/tmp/standalone-start-empty"
		_, _ = container.Exec("rm", "-rf", startDir)
		require.NoError(t, container.WriteFile(startDir+"/WORKFLOW.md", "## Step 1: X\n"))

		out, err := container.RunFestInDir(startDir, "workflow", "init")
		require.NoError(t, err, "init: %s", out)

		require.NoError(t, container.WriteFile(startDir+"/WORKFLOW.md", "# No steps here\n"))

		out, err = container.RunFestInDir(startDir, "workflow", "start")
		require.Error(t, err, "start must fail when WORKFLOW.md has no parseable steps: %s", out)
		require.Contains(t, out, "no parseable steps")

		entries, _ := container.ListDirectory(startDir + "/.workflow/runs")
		require.Empty(t, entries, "no run dir should be created for invalid doc: %v", entries)
	})

	t.Run("ShowToleratesMissingEventStream", func(t *testing.T) {
		showDir := "/tmp/standalone-show-missing-events"
		_, _ = container.Exec("rm", "-rf", showDir)
		require.NoError(t, container.WriteFile(showDir+"/WORKFLOW.md", "## Step 1: X\n\n**Goal:** g\n\n## Step 2: Y\n"))

		out, err := container.RunFestInDir(showDir, "workflow", "init")
		require.NoError(t, err, "init: %s", out)
		out, err = container.RunFestInDir(showDir, "workflow", "start")
		require.NoError(t, err, "start: %s", out)

		_, err = container.Exec("sh", "-c", "rm "+showDir+"/.workflow/runs/*/progress_events.jsonl")
		require.NoError(t, err)

		out, err = container.RunFestInDir(showDir, "workflow", "show")
		require.NoError(t, err, "show must degrade to cached summary when events file missing: %s", out)
		require.Contains(t, out, "Step 1")
	})

	t.Run("StartRefusesSecondConcurrentActiveRun", func(t *testing.T) {
		// Single-active-run invariant: two consecutive `fest workflow start`
		// invocations must not leave the manifest with two active runs.
		// Either the second start fails closed, or the prior run terminates
		// implicitly. Current behavior: fail closed with a clear hint.
		const dir = "/tmp/standalone-double-start"
		_, _ = container.Exec("rm", "-rf", dir)
		require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", "## Step 1: X\n\n**Goal:** g\n\n## Step 2: Y\n"))

		out, err := container.RunFestInDir(dir, "workflow", "init")
		require.NoError(t, err, "init: %s", out)
		out, err = container.RunFestInDir(dir, "workflow", "start")
		require.NoError(t, err, "first start: %s", out)

		out, err = container.RunFestInDir(dir, "workflow", "start")
		require.Error(t, err, "second start must refuse while a run is active: %s", out)
		require.Contains(t, out, "already active", "error should mention active run: %s", out)
	})

	t.Run("ShowSurfacesCorruptEventStream", func(t *testing.T) {
		showDir := "/tmp/standalone-show-corrupt-events"
		_, _ = container.Exec("rm", "-rf", showDir)
		require.NoError(t, container.WriteFile(showDir+"/WORKFLOW.md", "## Step 1: X\n\n**Goal:** g\n\n## Step 2: Y\n"))

		out, err := container.RunFestInDir(showDir, "workflow", "init")
		require.NoError(t, err, "init: %s", out)
		out, err = container.RunFestInDir(showDir, "workflow", "start")
		require.NoError(t, err, "start: %s", out)

		// Write an oversize single line (no newline) to trip bufio.Scanner's
		// token-too-long limit; replayEvents must surface scanner.Err()
		// instead of silently returning empty state.
		_, err = container.Exec("sh", "-c",
			"f=$(ls "+showDir+"/.workflow/runs/*/progress_events.jsonl); "+
				"head -c 4194304 /dev/urandom | tr -d '\\n' > \"$f\"")
		require.NoError(t, err)

		out, err = container.RunFestInDir(showDir, "workflow", "show")
		require.Error(t, err, "show must surface scanner error from corrupt event stream: %s", out)
	})
}
