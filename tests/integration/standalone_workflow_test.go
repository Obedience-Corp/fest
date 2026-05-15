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
}
