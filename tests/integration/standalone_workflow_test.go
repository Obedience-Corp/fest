//go:build integration
// +build integration

package integration

import (
	"strconv"
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

func TestStandaloneWorkflow_TopLevelShowAndWatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)

	t.Run("ShowAnonymousWorkflow", func(t *testing.T) {
		dir := "/tmp/standalone-top-show-anonymous"
		_, _ = container.Exec("rm", "-rf", dir)
		require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", `# Top Level Workflow

## Step 1: First

**Goal:** show the first step.

## Step 2: Second

**Goal:** show the second step.
`))

		out, err := container.RunFestInDir(dir, "show")
		require.NoError(t, err, "fest show should support anonymous WORKFLOW.md: %s", out)
		require.Contains(t, out, "Workflow Progress")
		require.Contains(t, out, "Mode: anonymous")
		require.Contains(t, out, "Progress: 0/2")
		require.Contains(t, out, "Step 1: First")
		require.Contains(t, out, "Step 2: Second")
		require.Contains(t, out, "fest workflow advance")

		jsonOut, err := container.RunFestInDir(dir, "show", "--json")
		require.NoError(t, err, "fest show --json should support anonymous WORKFLOW.md: %s", jsonOut)
		require.Contains(t, jsonOut, `"mode": "standalone-anonymous"`)
		require.Contains(t, jsonOut, `"total_steps": 2`)
	})

	t.Run("ShowTrackedWorkflowProgress", func(t *testing.T) {
		dir := "/tmp/standalone-top-show-tracked"
		_, _ = container.Exec("rm", "-rf", dir)
		require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", `# Tracked Workflow

## Step 1: First

**Goal:** complete the first step.

## Step 2: Second

**Goal:** continue to the second step.
`))

		out, err := container.RunFestInDir(dir, "workflow", "init")
		require.NoError(t, err, "workflow init: %s", out)
		out, err = container.RunFestInDir(dir, "workflow", "start")
		require.NoError(t, err, "workflow start: %s", out)
		out, err = container.RunFestInDir(dir, "workflow", "advance")
		require.NoError(t, err, "workflow advance: %s", out)

		out, err = container.RunFestInDir(dir, "show")
		require.NoError(t, err, "fest show should support tracked WORKFLOW.md: %s", out)
		require.Contains(t, out, "Mode: tracked")
		require.Contains(t, out, "Progress: 1/2")
		require.Contains(t, out, "Step 1: First")
		require.Contains(t, out, "Step 2: Second")
		require.Contains(t, out, "Current: Step 2 - Second")
	})

	t.Run("WatchAnonymousWorkflowInitialRender", func(t *testing.T) {
		dir := "/tmp/standalone-top-watch-anonymous"
		_, _ = container.Exec("rm", "-rf", dir)
		require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", `# Watch Workflow

## Step 1: First

**Goal:** render first.

## Step 2: Second

**Goal:** render second.
`))

		// `timeout` stops the watch with SIGTERM, and since fest#367 a signal
		// is a clean stop rather than death by signal, so 0 joins 124 and 143
		// as a valid way for a watch that stayed up to end. Exit code alone can
		// no longer tell "ran until the bound" from "exited immediately", so the
		// elapsed seconds carry that half of the assertion.
		output, exitCode, elapsed := runStandaloneFestWatchBounded(t, container, dir)
		require.Contains(t, []int{0, 124, 143}, exitCode, "watch should end cleanly at the bounded timeout after initial render")
		require.GreaterOrEqual(t, elapsed, 1, "watch should stay running until the bounded timeout, not exit immediately")
		require.Contains(t, output, "Workflow Progress")
		require.Contains(t, output, "Step 1: First")
		require.Contains(t, output, "Step 2: Second")
		require.True(t,
			strings.Contains(output, "Watching for changes") || strings.Contains(output, "Polling for changes"),
			"watch output should include watch footer, got:\n%s",
			output,
		)
		require.NotContains(t, output, "festival could not be resolved")
	})

	t.Run("WatchAnonymousWorkflowRefreshesAfterBootstrap", func(t *testing.T) {
		dir := "/tmp/standalone-top-watch-bootstrap"
		_, _ = container.Exec("rm", "-rf", dir)
		require.NoError(t, container.WriteFile(dir+"/WORKFLOW.md", `# Watch Bootstrap Workflow

## Step 1: First

**Goal:** render first.

## Step 2: Second

**Goal:** render second.
`))

		output, exitCode, elapsed := runStandaloneFestWatchWithAdvance(t, container, dir)
		require.Contains(t, []int{0, 124, 143}, exitCode, "watch should end cleanly at the bounded timeout after refresh")
		require.GreaterOrEqual(t, elapsed, 2, "watch should stay running past the advance until the bounded timeout")
		require.Contains(t, output, "Workflow Progress")
		require.Contains(t, output, "Mode: tracked")
		require.Contains(t, output, "Progress: 1/2")
		require.Contains(t, output, "Current: Step 2 - Second")
		require.Contains(t, output, "Watching for changes")
		require.NotContains(t, output, "Polling for changes")
	})
}

func runStandaloneFestWatchBounded(t *testing.T, tc *TestContainer, cwd string) (string, int, int) {
	t.Helper()

	outputPath := "/tmp/fest-standalone-watch-output"
	cmd := "cd " + shellQuote(cwd) + " && rm -f " + outputPath + " && set +e; start=$(date +%s); timeout 2s /fest watch"
	cmd += " > " + outputPath + " 2>&1; code=$?; elapsed=$(( $(date +%s) - start )); cat " + outputPath + "; printf '\\n__FEST_STANDALONE_WATCH_EXIT_CODE:%d %d\\n' \"$code\" \"$elapsed\""

	output, err := tc.runCommand([]string{"sh", "-c", cmd})
	require.NoError(t, err, "fest watch should start")

	marker := "\n__FEST_STANDALONE_WATCH_EXIT_CODE:"
	idx := strings.LastIndex(output, marker)
	require.NotEqual(t, -1, idx, "bounded watch output should include exit marker: %s", output)

	fields := strings.Fields(strings.TrimSpace(output[idx+len(marker):]))
	require.Len(t, fields, 2, "bounded watch marker should carry an exit code and an elapsed second count")
	exitCode, err := strconv.Atoi(fields[0])
	require.NoError(t, err, "bounded watch exit code should parse")
	elapsed, err := strconv.Atoi(fields[1])
	require.NoError(t, err, "bounded watch elapsed seconds should parse")

	return output[:idx], exitCode, elapsed
}

func runStandaloneFestWatchWithAdvance(t *testing.T, tc *TestContainer, cwd string) (string, int, int) {
	t.Helper()

	outputPath := "/tmp/fest-standalone-watch-bootstrap-output"
	advancePath := "/tmp/fest-standalone-watch-bootstrap-advance"
	cmd := "rm -f " + outputPath + " " + advancePath
	cmd += "; (cd " + shellQuote(cwd) + " && sleep 0.5 && /fest workflow advance > " + advancePath + " 2>&1) &"
	cmd += " start=$(date +%s); (cd " + shellQuote(cwd) + " && timeout 3s /fest watch > " + outputPath + " 2>&1); code=$?; elapsed=$(( $(date +%s) - start )); wait || true; cat " + outputPath + "; printf '\\n__FEST_STANDALONE_WATCH_EXIT_CODE:%d %d\\n' \"$code\" \"$elapsed\""

	output, err := tc.runCommand([]string{"sh", "-c", cmd})
	require.NoError(t, err, "fest watch should start and background advance should finish")

	marker := "\n__FEST_STANDALONE_WATCH_EXIT_CODE:"
	idx := strings.LastIndex(output, marker)
	require.NotEqual(t, -1, idx, "bounded watch output should include exit marker: %s", output)

	fields := strings.Fields(strings.TrimSpace(output[idx+len(marker):]))
	require.Len(t, fields, 2, "bounded watch marker should carry an exit code and an elapsed second count")
	exitCode, err := strconv.Atoi(fields[0])
	require.NoError(t, err, "bounded watch exit code should parse")
	elapsed, err := strconv.Atoi(fields[1])
	require.NoError(t, err, "bounded watch elapsed seconds should parse")

	return output[:idx], exitCode, elapsed
}
