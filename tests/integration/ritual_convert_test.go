//go:build integration
// +build integration

package integration

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRitualConvert verifies that `fest ritual convert` copies a source
// festival from planning/ into ritual/ with the correct RI-XX0001 naming,
// mutates fest.yaml to set type=ritual and add ritual_config, and then
// produces a runnable active copy via `fest ritual run`.
//
// This test must live in the container harness because it exercises real
// filesystem mutation. CLAUDE.md §11 forbids running this class of test on
// the host against t.TempDir().
func TestRitualConvert(t *testing.T) {
	container := GetSharedContainer(t)

	const (
		workspaceRoot = "/workspace"
		festivalsRoot = workspaceRoot + "/festivals"
		sourceName    = "quarterly-security-review-QS0001"
		sourcePath    = festivalsRoot + "/planning/" + sourceName
		// GenerateRitualID scans all status dirs for the highest existing
		// counter per prefix. Since the source festival already uses QS0001,
		// the new ritual gets QS0002.
		ritualDirName = "quarterly-security-review-RI-QS0002"
	)

	// Set up the workspace marker and a source festival in planning/.
	_, err := container.runCommand([]string{
		"sh", "-c",
		fmt.Sprintf(
			"mkdir -p %s/.festival/.state %s/planning/%s %s/active %s/ritual",
			festivalsRoot, festivalsRoot, sourceName, festivalsRoot, festivalsRoot,
		),
	})
	require.NoError(t, err, "should create workspace dirs")

	writeContainerFile(t, container,
		festivalsRoot+"/.festival/.state/.workspace",
		`{"workspace": "workspace", "registered": "2024-01-01T00:00:00Z"}`)

	writeContainerFile(t, container,
		sourcePath+"/FESTIVAL_OVERVIEW.md",
		"# Quarterly Security Review\n")

	writeContainerFile(t, container,
		sourcePath+"/fest.yaml",
		`version: "1.0"
metadata:
  id: QS0001
  name: quarterly-security-review
  festival_type: standard
  status_history:
    - status: planning
      timestamp: 2026-01-01T00:00:00Z
`)

	writeContainerFile(t, container,
		sourcePath+"/.fest/progress_events.jsonl",
		`{"ts":"2026-01-15T00:00:00Z","event":"completed","task":"001_REVIEW/01_audit.md"}
`)
	writeContainerFile(t, container,
		sourcePath+"/.fest/status_history.json",
		`[{"status":"completed","timestamp":"2026-01-15T00:00:00Z"}]
`)
	writeContainerFile(t, container,
		sourcePath+"/.workflow/runs/r1/progress_events.jsonl",
		`{"ts":"2026-01-15T00:00:00Z","event":"wf_step_done"}
`)
	writeContainerFile(t, container,
		sourcePath+"/001_REVIEW/01_audit.md",
		`---
fest_type: task
fest_id: 01_audit
fest_status: completed
---

# Audit

- [x] Review access logs
- [X] Check firewall
See [x] in prose
`)

	t.Run("convert copies to ritual with correct config", func(t *testing.T) {
		output, err := container.RunFestInDir(workspaceRoot, "ritual", "convert", "quarterly-security-review", "--frequency", "quarterly")
		require.NoError(t, err, "fest ritual convert should succeed: %s", output)

		ritualPath := festivalsRoot + "/ritual/" + ritualDirName
		exists, err := container.CheckFileExists(ritualPath + "/FESTIVAL_OVERVIEW.md")
		require.NoError(t, err)
		require.True(t, exists, "expected converted ritual file at %s", ritualPath+"/FESTIVAL_OVERVIEW.md")

		// Verify fest.yaml was mutated correctly.
		config, err := container.ReadFile(ritualPath + "/fest.yaml")
		require.NoError(t, err)
		require.Contains(t, config, "festival_type: ritual",
			"expected festival_type=ritual in converted config, got:\n%s", config)
		require.Contains(t, config, "ritual_config:",
			"expected ritual_config block in converted config, got:\n%s", config)
		require.Contains(t, config, "schedule: quarterly",
			"expected schedule: quarterly in ritual_config, got:\n%s", config)
		// run_count: 0 is omitted by yaml omitempty; absence means zero runs.
		require.NotContains(t, config, "run_count: 1",
			"unexpected run_count: 1 in freshly converted ritual, got:\n%s", config)
		require.NotContains(t, config, "status_history",
			"expected status_history to be stripped from converted config, got:\n%s", config)

		assertConvertedProgressReset(t, container, ritualPath)

		// Source festival should still be in planning/ (preserve by default)
		// and should still carry its original progress artifacts.
		sourceExists, err := container.CheckFileExists(sourcePath + "/fest.yaml")
		require.NoError(t, err)
		require.True(t, sourceExists, "source festival should be preserved in planning/")
		sourceProgress, err := container.CheckFileExists(sourcePath + "/.fest/progress_events.jsonl")
		require.NoError(t, err)
		require.True(t, sourceProgress, "source progress artifacts should be left intact")
	})

	t.Run("fest ritual run on converted ritual produces active copy", func(t *testing.T) {
		// The ritual was created in the previous subtest.
		output, err := container.RunFestInDir(workspaceRoot, "ritual", "run", "quarterly-security-review")
		require.NoError(t, err, "fest ritual run should succeed: %s", output)

		// Verify a run copy was created in active/.
		runDirName := ritualDirName + "-0001"
		runPath := festivalsRoot + "/active/" + runDirName
		exists, err := container.CheckFileExists(runPath + "/FESTIVAL_OVERVIEW.md")
		require.NoError(t, err)
		require.True(t, exists, "expected ritual run file at %s", runPath+"/FESTIVAL_OVERVIEW.md")

		assertConvertedProgressReset(t, container, runPath)
	})

	t.Run("dry-run does not write", func(t *testing.T) {
		// Use a different source name to avoid collision.
		drySourceName := "dry-run-test-DR0001"
		drySourcePath := festivalsRoot + "/planning/" + drySourceName
		_, err := container.runCommand([]string{
			"sh", "-c",
			fmt.Sprintf("mkdir -p %s", drySourcePath),
		})
		require.NoError(t, err)

		writeContainerFile(t, container,
			drySourcePath+"/fest.yaml",
			`version: "1.0"
metadata:
  id: DR0001
  name: dry-run-test
  festival_type: standard
`)

		output, err := container.RunFestInDir(workspaceRoot, "ritual", "convert", "dry-run-test", "--dry-run")
		require.NoError(t, err, "dry-run should succeed: %s", output)
		require.Contains(t, output, "Dry run")

		// Verify nothing was written to ritual/.
		dryRitualDir := "dry-run-test-RI-DR0002"
		exists, err := container.CheckFileExists(festivalsRoot + "/ritual/" + dryRitualDir + "/fest.yaml")
		require.NoError(t, err)
		require.False(t, exists, "dry-run should not have written to ritual/")
	})

	t.Run("refuses to convert an already-ritual festival", func(t *testing.T) {
		// Try converting the ritual we already created.
		output, err := container.RunFestInDir(workspaceRoot, "ritual", "convert", ritualDirName)
		require.Error(t, err, "should refuse to convert an already-ritual festival")
		require.Contains(t, output, "already a ritual")
	})

	t.Run("ambiguous substring selector errors", func(t *testing.T) {
		alpha := festivalsRoot + "/planning/zzconvambig-alpha-ZA0001"
		beta := festivalsRoot + "/planning/zzconvambig-beta-ZB0001"
		writeContainerFile(t, container, alpha+"/fest.yaml",
			"version: \"1.0\"\nmetadata:\n  id: ZA0001\n  name: zzconvambig-alpha\n")
		writeContainerFile(t, container, beta+"/fest.yaml",
			"version: \"1.0\"\nmetadata:\n  id: ZB0001\n  name: zzconvambig-beta\n")

		output, err := container.RunFestInDir(workspaceRoot, "ritual", "convert", "zzconvambig")
		require.Error(t, err, "ambiguous substring should fail")
		require.Contains(t, strings.ToLower(output), "ambiguous")
	})

	t.Run("reset-progress false keeps progress artifacts", func(t *testing.T) {
		keepName := "keep-progress-KP0001"
		keepPath := festivalsRoot + "/planning/" + keepName
		writeContainerFile(t, container, keepPath+"/fest.yaml",
			`version: "1.0"
metadata:
  id: KP0001
  name: keep-progress
  festival_type: standard
`)
		writeContainerFile(t, container, keepPath+"/.fest/progress_events.jsonl",
			`{"ts":"2026-01-15T00:00:00Z","event":"completed","task":"01_keep.md"}
`)
		writeContainerFile(t, container, keepPath+"/01_keep.md",
			"---\nfest_type: task\nfest_status: completed\n---\n\n- [x] Kept\n")

		output, err := container.RunFestInDir(workspaceRoot, "ritual", "convert", "keep-progress", "--reset-progress=false")
		require.NoError(t, err, "convert --reset-progress=false should succeed: %s", output)

		ritualKeep := festivalsRoot + "/ritual/keep-progress-RI-KP0002"
		progressExists, err := container.CheckFileExists(ritualKeep + "/.fest/progress_events.jsonl")
		require.NoError(t, err)
		require.True(t, progressExists, "progress JSONL should be preserved when reset is off")

		task, err := container.ReadFile(ritualKeep + "/01_keep.md")
		require.NoError(t, err)
		require.Contains(t, task, "- [x] Kept")
		require.Contains(t, task, "fest_status: completed")
	})
}

func assertConvertedProgressReset(t *testing.T, container *TestContainer, festivalPath string) {
	t.Helper()

	progressExists, err := container.CheckFileExists(festivalPath + "/.fest/progress_events.jsonl")
	require.NoError(t, err)
	require.False(t, progressExists, "progress_events.jsonl should be stripped from %s", festivalPath)

	historyExists, err := container.CheckFileExists(festivalPath + "/.fest/status_history.json")
	require.NoError(t, err)
	require.False(t, historyExists, "status_history.json should be stripped from %s", festivalPath)

	workflowExists, err := container.CheckDirExists(festivalPath + "/.workflow")
	require.NoError(t, err)
	require.False(t, workflowExists, ".workflow should be stripped from %s", festivalPath)

	task, err := container.ReadFile(festivalPath + "/001_REVIEW/01_audit.md")
	require.NoError(t, err)
	require.Contains(t, task, "fest_status: pending", "task completion status should reset in %s", festivalPath)
	require.Contains(t, task, "- [ ] Review access logs")
	require.Contains(t, task, "- [ ] Check firewall")
	require.Contains(t, task, "See [x] in prose", "prose [x] must not be rewritten")
	require.NotContains(t, task, "fest_status: completed")
}
