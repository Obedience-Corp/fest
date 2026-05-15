//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const standaloneStepsJSON = `{"title":"My Workflow","description":"Built by integration test","steps":[{"name":"PLAN","goal":"Plan the work","actions":["Think"],"output":"Plan"},{"name":"BUILD","goal":"Build it","actions":["Code"],"output":"Code"}]}`

// TestCreateWorkflowStandalone covers the D009 standalone dispatch surface
// of `fest create workflow`: writes WORKFLOW.md, initializes .workflow/ unless
// --no-init is set, refuses overwrite, rejects --festival outside a festival,
// errors on non-TTY without --steps.
func TestCreateWorkflowStandalone(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	container := GetSharedContainer(t)

	t.Run("CreatesWorkflowAndInitsRuntime", func(t *testing.T) {
		const dir = "/tmp/cw-standalone-init"
		_, _ = container.Exec("rm", "-rf", dir)
		_, err := container.Exec("mkdir", "-p", dir)
		require.NoError(t, err)

		require.NoError(t, container.WriteFile(dir+"/steps.json", standaloneStepsJSON))
		out, err := container.RunFestInDir(dir, "create", "workflow", "demo", "--steps-file", dir+"/steps.json")
		require.NoError(t, err, "fest create workflow: %s", out)
		require.Contains(t, out, "created")
		require.Contains(t, out, "initialized .workflow/workflow.yaml")
		require.Contains(t, out, "workflow_id=wf-demo")

		exists, _ := container.CheckFileExists(dir + "/WORKFLOW.md")
		require.True(t, exists, "WORKFLOW.md should exist")
		exists, _ = container.CheckFileExists(dir + "/.workflow/workflow.yaml")
		require.True(t, exists, ".workflow/workflow.yaml should exist")
		exists, _ = container.CheckFileExists(dir + "/.workitem")
		require.False(t, exists, "fest must not write .workitem (D007)")
	})

	t.Run("NoInitSkipsRuntime", func(t *testing.T) {
		const dir = "/tmp/cw-standalone-noinit"
		_, _ = container.Exec("rm", "-rf", dir)
		_, err := container.Exec("mkdir", "-p", dir)
		require.NoError(t, err)

		require.NoError(t, container.WriteFile(dir+"/steps.json", standaloneStepsJSON))
		out, err := container.RunFestInDir(dir, "create", "workflow", "demo", "--steps-file", dir+"/steps.json", "--no-init")
		require.NoError(t, err, "fest create workflow --no-init: %s", out)
		require.Contains(t, out, "skipped .workflow/ init")

		exists, _ := container.CheckFileExists(dir + "/WORKFLOW.md")
		require.True(t, exists, "WORKFLOW.md should exist")
		exists, _ = container.CheckDirExists(dir + "/.workflow")
		require.False(t, exists, ".workflow/ must NOT exist with --no-init")
	})

	t.Run("RefusesOverwrite", func(t *testing.T) {
		const dir = "/tmp/cw-standalone-overwrite"
		_, _ = container.Exec("rm", "-rf", dir)
		_, err := container.Exec("mkdir", "-p", dir)
		require.NoError(t, err)
		require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", "# pre-existing\n"))

		require.NoError(t, container.WriteFile(dir+"/steps.json", standaloneStepsJSON))
		out, err := container.RunFestInDir(dir, "create", "workflow", "demo", "--steps-file", dir+"/steps.json")
		require.Error(t, err, "expected error on existing WORKFLOW.md")
		require.True(t,
			strings.Contains(out, "already exists") || strings.Contains(out, "WORKFLOW.md"),
			"error should mention overwrite, got: %s", out)
	})

	t.Run("NonTTYRequiresStepsFlag", func(t *testing.T) {
		const dir = "/tmp/cw-standalone-notty"
		_, _ = container.Exec("rm", "-rf", dir)
		_, err := container.Exec("mkdir", "-p", dir)
		require.NoError(t, err)

		out, err := container.RunFestInDir(dir, "create", "workflow", "demo")
		require.Error(t, err, "expected error in non-TTY without --steps")
		require.True(t,
			strings.Contains(out, "non-interactive") || strings.Contains(out, "steps required"),
			"error should mention steps requirement, got: %s", out)
	})

	t.Run("RejectsFestivalFlagOutsideFestival", func(t *testing.T) {
		const dir = "/tmp/cw-standalone-festflag"
		_, _ = container.Exec("rm", "-rf", dir)
		_, err := container.Exec("mkdir", "-p", dir)
		require.NoError(t, err)

		require.NoError(t, container.WriteFile(dir+"/steps.json", standaloneStepsJSON))
		out, err := container.RunFestInDir(dir, "create", "workflow", "demo", "--steps-file", dir+"/steps.json", "--festival", "/some/path")
		require.Error(t, err, "--festival in standalone mode must error")
		require.Contains(t, out, "--festival is not valid")
	})
}
