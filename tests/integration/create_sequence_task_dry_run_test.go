//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type createDryRunPreview struct {
	OK           bool             `json:"ok"`
	Action       string           `json:"action"`
	DryRun       bool             `json:"dry_run"`
	PlannedPaths []string         `json:"planned_paths"`
	Count        int              `json:"count"`
	Markers      []map[string]any `json:"markers"`
}

func TestCreateSequenceAndTaskDryRunWritesNothing(t *testing.T) {
	tc := GetSharedContainer(t)
	festivalsPath := setupWorkspace(t, tc, "/seq-task-dry-run-campaign")
	installPreviewTemplates(t, tc, festivalsPath)
	installSequenceTaskMarkerTemplates(t, tc, festivalsPath)

	festivalPath := createScratchFestival(t, tc, festivalsPath, "dry-run-seq")
	phasePath := resolveOrCreatePhase(t, tc, festivalPath)

	before := snapshotFestivalWorkspace(t, tc, festivalsPath)
	jsonOutput, err := tc.RunFestInDir(
		phasePath,
		"create", "sequence",
		"--name", "hello",
		"--dry-run",
		"--json",
		"--no-prompt",
	)
	require.NoError(t, err, "sequence dry-run should succeed: %s", jsonOutput)
	preview := parseCreateDryRunPreview(t, jsonOutput)
	require.True(t, preview.DryRun)
	require.Equal(t, "dry_run", preview.Action)
	require.Len(t, preview.PlannedPaths, 1)
	seqRel := strings.TrimSuffix(preview.PlannedPaths[0], "/SEQUENCE_GOAL.md")
	require.Contains(t, seqRel, "hello")
	require.Greater(t, preview.Count, 0, "dry-run should report template markers")
	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
		"sequence dry-run must leave every workspace path and file byte unchanged")

	humanOutput, err := tc.RunFestInDir(
		phasePath,
		"create", "sequence",
		"--name", "hello",
		"--dry-run",
		"--no-color",
		"--no-prompt",
	)
	require.NoError(t, err, "human sequence dry-run should succeed: %s", humanOutput)
	require.Contains(t, humanOutput, "Dry Run — No Files Created")
	require.Contains(t, humanOutput, seqRel+"/SEQUENCE_GOAL.md")
	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
		"human sequence dry-run must not create the previewed directory")

	createOutput, err := tc.RunFestInDir(
		phasePath,
		"create", "sequence",
		"--name", "hello",
		"--json",
		"--skip-markers",
		"--no-prompt",
	)
	require.NoError(t, err, "create after dry-run should succeed: %s", createOutput)
	exists, err := tc.CheckDirExists(filepath.Join(phasePath, seqRel))
	require.NoError(t, err)
	require.True(t, exists, "real create should reuse the number previewed by dry-run: %s", seqRel)

	sequencePath := filepath.Join(phasePath, seqRel)
	before = snapshotFestivalWorkspace(t, tc, festivalsPath)
	jsonOutput, err = tc.RunFestInDir(
		sequencePath,
		"create", "task",
		"--name", "setup",
		"--dry-run",
		"--json",
	)
	require.NoError(t, err, "task dry-run should succeed: %s", jsonOutput)
	preview = parseCreateDryRunPreview(t, jsonOutput)
	require.True(t, preview.DryRun)
	require.Len(t, preview.PlannedPaths, 1)
	taskRel := preview.PlannedPaths[0]
	require.Contains(t, taskRel, "setup")
	require.Greater(t, preview.Count, 0, "task dry-run should report template markers")
	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
		"task dry-run must leave every workspace path and file byte unchanged")

	humanOutput, err = tc.RunFestInDir(
		sequencePath,
		"create", "task",
		"--name", "setup",
		"--dry-run",
		"--no-color",
	)
	require.NoError(t, err, "human task dry-run should succeed: %s", humanOutput)
	require.Contains(t, humanOutput, "Dry Run — No Files Created")
	require.Contains(t, humanOutput, taskRel)
	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
		"human task dry-run must not create the previewed file")

	createOutput, err = tc.RunFestInDir(
		sequencePath,
		"create", "task",
		"--name", "setup",
		"--json",
		"--skip-markers",
	)
	require.NoError(t, err, "create task after dry-run should succeed: %s", createOutput)
	exists, err = tc.CheckFileExists(filepath.Join(sequencePath, taskRel))
	require.NoError(t, err)
	require.True(t, exists, "real create should reuse the number previewed by dry-run: %s", taskRel)
}

func TestCreateTaskDryRunDoesNotRenameExistingTasks(t *testing.T) {
	tc := GetSharedContainer(t)
	festivalsPath := setupWorkspace(t, tc, "/seq-task-dry-run-rename")
	installPreviewTemplates(t, tc, festivalsPath)
	installSequenceTaskMarkerTemplates(t, tc, festivalsPath)

	festivalPath := createScratchFestival(t, tc, festivalsPath, "dry-run-rename")
	phasePath := resolveOrCreatePhase(t, tc, festivalPath)
	seqOutput, err := tc.RunFestInDir(phasePath, "create", "sequence", "--name", "work", "--json", "--skip-markers", "--no-prompt")
	require.NoError(t, err, "create sequence should succeed: %s", seqOutput)
	sequencePath := filepath.Dir(parseCreateSequenceOutput(t, seqOutput).Created[0])
	taskOutput, err := tc.RunFestInDir(sequencePath, "create", "task", "--name", "existing", "--json", "--skip-markers")
	require.NoError(t, err, "create existing task should succeed: %s", taskOutput)
	existingTask := filepath.Base(parseCreateTaskOutput(t, taskOutput).Created[0])

	before := snapshotFestivalWorkspace(t, tc, festivalsPath)
	output, err := tc.RunFestInDir(
		sequencePath,
		"create", "task",
		"--after", "0",
		"--name", "intro",
		"--dry-run",
		"--json",
	)
	require.NoError(t, err, "insert dry-run should succeed: %s", output)
	preview := parseCreateDryRunPreview(t, output)
	require.Contains(t, preview.PlannedPaths, "01_intro.md")
	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
		"dry-run insert must not rename existing tasks")
	exists, err := tc.CheckFileExists(filepath.Join(sequencePath, existingTask))
	require.NoError(t, err)
	require.True(t, exists, "existing task must keep its original number after dry-run")
}

func parseCreateDryRunPreview(t *testing.T, output string) createDryRunPreview {
	t.Helper()
	var preview createDryRunPreview
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(output)), &preview), "dry-run should emit JSON: %s", output)
	require.True(t, preview.OK, "dry-run returned an error payload: %s", output)
	return preview
}

func resolveOrCreatePhase(t *testing.T, tc *TestContainer, festivalPath string) string {
	t.Helper()
	dirs, err := tc.ListDirectories(festivalPath)
	require.NoError(t, err)
	for _, name := range dirs {
		if len(name) >= 5 && name[0] >= '0' && name[0] <= '9' {
			return filepath.Join(festivalPath, name)
		}
	}
	_, err = tc.RunFestInDir(festivalPath, "create", "phase", "--name", "PLAN", "--json", "--skip-markers")
	require.NoError(t, err, "create phase should succeed")
	return filepath.Join(festivalPath, "001_PLAN")
}

func createScratchFestival(t *testing.T, tc *TestContainer, festivalsPath, name string) string {
	t.Helper()
	output, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", name,
		"--json",
		"--skip-markers",
	)
	require.NoError(t, err, "create festival should succeed: %s", output)
	result := parseCreateFestivalOutput(t, output)
	return filepath.Join(festivalsPath, result.Festival.Dest, result.Festival.Directory)
}

func installSequenceTaskMarkerTemplates(t *testing.T, tc *TestContainer, festivalsPath string) {
	t.Helper()
	templatesDir := filepath.Join(festivalsPath, ".festival", "templates")
	require.NoError(t, writeFileInContainer(tc, filepath.Join(templatesDir, "SEQUENCE_GOAL_TEMPLATE.md"), `---
template_id: SEQUENCE_GOAL
required_variables:
  - sequence_id
---
# Sequence {{.sequence_id}}

[REPLACE: Describe the sequence objective]
`))
	require.NoError(t, writeFileInContainer(tc, filepath.Join(templatesDir, "TASK_TEMPLATE.md"), `---
template_id: TASK
required_variables:
  - task_id
---
# Task {{.task_id}}

[REPLACE: Describe the task objective]
`))
}
