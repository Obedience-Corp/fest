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
// of `fest create workflow`: writes WORKFLOW.md, initializes .workflow/ and
// starts a run unless --no-init is set, refuses overwrite, rejects --festival
// outside a festival, errors on non-TTY without --steps.
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
		require.Contains(t, out, "started workflow run")
		require.Contains(t, out, "workflow_id=wf-demo")

		exists, _ := container.CheckFileExists(dir + "/WORKFLOW.md")
		require.True(t, exists, "WORKFLOW.md should exist")
		exists, _ = container.CheckFileExists(dir + "/.workflow/workflow.yaml")
		require.True(t, exists, ".workflow/workflow.yaml should exist")
		exists, _ = container.CheckDirExists(dir + "/.workflow/runs")
		require.True(t, exists, ".workflow/runs should exist")
		exists, _ = container.CheckFileExists(dir + "/.workitem")
		require.False(t, exists, "fest must not write .workitem (D007)")

		out, err = container.RunFestInDir(dir, "next", "--json")
		require.NoError(t, err, "fest next should work immediately after create workflow: %s", out)
		require.Contains(t, out, `"mode": "standalone-tracked"`)
		require.Contains(t, out, `"current_step": 1`)

		out, err = container.RunFestInDir(dir, "workflow", "advance")
		require.NoError(t, err, "fest workflow advance should work after create workflow: %s", out)
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

	t.Run("RefusesPreexistingRuntimeDir", func(t *testing.T) {
		const dir = "/tmp/cw-standalone-existing-runtime"
		_, _ = container.Exec("rm", "-rf", dir)
		_, err := container.Exec("mkdir", "-p", dir+"/.workflow")
		require.NoError(t, err)
		require.NoError(t, container.WriteFile(dir+"/.workflow/notes.txt", "keep me\n"))

		require.NoError(t, container.WriteFile(dir+"/steps.json", standaloneStepsJSON))
		out, err := container.RunFestInDir(dir, "create", "workflow", "demo", "--steps-file", dir+"/steps.json")
		require.Error(t, err, "expected error when .workflow/ already exists")
		require.Contains(t, out, ".workflow/ already exists")

		exists, _ := container.CheckFileExists(dir + "/.workflow/notes.txt")
		require.True(t, exists, "pre-existing .workflow/ content must not be removed")
		exists, _ = container.CheckFileExists(dir + "/WORKFLOW.md")
		require.False(t, exists, "create should refuse before writing WORKFLOW.md")
		exists, _ = container.CheckFileExists(dir + "/.workflow/workflow.yaml")
		require.False(t, exists, "create should not add workflow.yaml to pre-existing .workflow/")
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

	t.Run("ExplicitFestivalFlagRoutesToFestivalHandler", func(t *testing.T) {
		// Explicit --festival / --path now wins over cwd-derived mode so users
		// can target a phase from outside it (#173 review). When the target
		// isn't a real phase, the error must come from the festival handler's
		// path-resolution, not "festival flag invalid in standalone mode".
		const dir = "/tmp/cw-standalone-festflag"
		_, _ = container.Exec("rm", "-rf", dir)
		_, err := container.Exec("mkdir", "-p", dir)
		require.NoError(t, err)

		require.NoError(t, container.WriteFile(dir+"/steps.json", standaloneStepsJSON))
		out, err := container.RunFestInDir(dir, "create", "workflow", "demo", "--steps-file", dir+"/steps.json", "--festival", "/some/missing/path")
		require.Error(t, err, "--festival pointing at a non-phase must error")
		require.Contains(t, out, "not inside a phase directory",
			"error should come from festival handler's path resolution, got: %s", out)
	})

	t.Run("StandaloneJSONEmitsStructuredResult", func(t *testing.T) {
		const dir = "/tmp/cw-standalone-json"
		_, _ = container.Exec("rm", "-rf", dir)
		_, err := container.Exec("mkdir", "-p", dir)
		require.NoError(t, err)

		require.NoError(t, container.WriteFile(dir+"/steps.json", standaloneStepsJSON))
		out, err := container.RunFestInDir(dir, "create", "workflow", "demo", "--steps-file", dir+"/steps.json", "--json")
		require.NoError(t, err, "fest create workflow --json: %s", out)
		require.Contains(t, out, `"ok": true`, "JSON success payload missing: %s", out)
		require.Contains(t, out, `"mode": "standalone"`, "JSON should declare standalone mode: %s", out)
		require.Contains(t, out, `"workflow_id": "wf-demo"`, "JSON missing workflow_id: %s", out)
		require.Contains(t, out, `"runtime_initialized": true`, "JSON should report runtime_initialized: %s", out)
		require.Contains(t, out, `"active_run_id":`, "JSON should report active run: %s", out)
		require.NotContains(t, out, "fest workflow start", "default create starts the run, so JSON should not suggest start: %s", out)
	})

	t.Run("StandaloneRejectsNonDefaultPosition", func(t *testing.T) {
		const dir = "/tmp/cw-standalone-pos"
		_, _ = container.Exec("rm", "-rf", dir)
		_, err := container.Exec("mkdir", "-p", dir)
		require.NoError(t, err)

		require.NoError(t, container.WriteFile(dir+"/steps.json", standaloneStepsJSON))
		out, err := container.RunFestInDir(dir, "create", "workflow", "demo", "--steps-file", dir+"/steps.json", "--position", "before", "--no-init")
		require.Error(t, err, "--position non-default in standalone mode must error")
		require.Contains(t, out, "--position is not valid", "error should mention --position: %s", out)
	})
}
