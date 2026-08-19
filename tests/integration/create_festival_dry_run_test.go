//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type createFestivalPreviewResult struct {
	OK              bool                          `json:"ok"`
	Action          string                        `json:"action"`
	DryRun          bool                          `json:"dry_run"`
	Festival        map[string]string             `json:"festival"`
	TargetPath      string                        `json:"target_path"`
	PlannedPaths    []string                      `json:"planned_paths"`
	Tree            string                        `json:"tree"`
	Markers         []createFestivalPreviewMarker `json:"markers"`
	MarkersTotal    int                           `json:"markers_total"`
	MarkersFilled   int                           `json:"markers_filled"`
	MarkersUnfilled int                           `json:"markers_unfilled"`
}

type createFestivalPreviewMarker struct {
	File string `json:"file"`
	Hint string `json:"hint"`
	Line int    `json:"line"`
}

func TestCreateFestivalDryRunPreviewsTreeWithoutWorkspaceWrites(t *testing.T) {
	tc := GetSharedContainer(t)
	festivalsPath := setupWorkspace(t, tc, "/preview-campaign")
	installPreviewTemplates(t, tc, festivalsPath)

	before := snapshotFestivalWorkspace(t, tc, festivalsPath)
	humanOutput, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "preview-only",
		"--type", "standard",
		"--seed", "Starting context",
		"--dry-run",
		"--no-color",
	)
	require.NoError(t, err, "dry-run should succeed: %s", humanOutput)
	require.Contains(t, humanOutput, "Dry Run — No Files Created")
	require.Contains(t, humanOutput, "preview-only-PO0001/")
	require.Contains(t, humanOutput, "├── 001_INGEST/")
	require.Contains(t, humanOutput, "WORKFLOW.md")
	require.Contains(t, humanOutput, "seed.md")
	require.Contains(t, humanOutput, "002_PLAN/")
	require.Contains(t, humanOutput, "FESTIVAL_GOAL.md")
	require.Contains(t, humanOutput, "gates/")
	require.Contains(t, humanOutput, "Replace Markers in Template",
		"human dry-run must list the markers a real create would leave")
	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
		"human dry-run must leave every workspace path and file byte unchanged")

	jsonOutput, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "preview-only",
		"--type", "standard",
		"--seed", "Starting context",
		"--dry-run",
		"--agent",
	)
	require.NoError(t, err, "agent dry-run should succeed: %s", jsonOutput)

	var preview createFestivalPreviewResult
	require.NoError(t, json.Unmarshal([]byte(jsonOutput), &preview), "dry-run should emit JSON: %s", jsonOutput)
	require.True(t, preview.OK)
	require.True(t, preview.DryRun)
	require.Equal(t, "create_festival_preview", preview.Action)
	require.Equal(t, "PO0001", preview.Festival["id"])
	require.NotEmpty(t, preview.TargetPath)
	require.NotEmpty(t, preview.PlannedPaths)
	require.Contains(t, preview.Tree, "001_INGEST/")
	require.Contains(t, preview.Tree, "input_specs/")
	require.NotEmpty(t, preview.Markers, "agent dry-run must report template markers")
	require.Equal(t, len(preview.Markers), preview.MarkersUnfilled)
	require.Equal(t, preview.MarkersTotal, preview.MarkersUnfilled)
	require.Zero(t, preview.MarkersFilled)
	for _, marker := range preview.Markers {
		require.NotEmpty(t, marker.Hint, "every reported marker needs a hint to fill")
		require.Positive(t, marker.Line)
		require.Contains(t, preview.PlannedPaths, marker.File,
			"every marker file must also be a planned path")
	}
	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
		"agent dry-run must leave every workspace path and file byte unchanged")

	createOutput, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "preview-only",
		"--type", "standard",
		"--seed", "Starting context",
		"--skip-markers",
	)
	require.NoError(t, err, "create after dry-run should succeed: %s", createOutput)
	dirs, err := tc.ListDirectories(filepath.Join(festivalsPath, "planning"))
	require.NoError(t, err)
	require.Equal(t, []string{"preview-only-PO0001"}, dirs,
		"real create should reuse the ID previewed by dry-run")
}

func TestCreateFestivalDryRunWithZeroMarkersDoesNotRegisterFestival(t *testing.T) {
	tc := GetSharedContainer(t)
	festivalsPath := setupWorkspace(t, tc, "/zero-marker-preview")
	writeZeroMarkerFestivalTemplates(t, tc, festivalsPath)

	before := snapshotFestivalWorkspace(t, tc, festivalsPath)
	output, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "marker-free",
		"--dry-run",
		"--json",
	)
	require.NoError(t, err, "zero-marker dry-run should succeed: %s", output)

	var preview createFestivalPreviewResult
	require.NoError(t, json.Unmarshal([]byte(output), &preview), "zero-marker dry-run should emit JSON: %s", output)
	require.True(t, preview.OK, "zero-marker dry-run returned an error payload: %s", output)
	require.Equal(t, "MF0001", preview.Festival["id"])
	require.Contains(t, preview.Tree, "fest.yaml")
	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
		"zero-marker dry-run must not create a directory or registry event")

	planning, err := tc.ListDirectories(filepath.Join(festivalsPath, "planning"))
	require.NoError(t, err)
	require.Empty(t, planning)
	eventsExist, err := tc.CheckFileExists(filepath.Join(festivalsPath, ".festival", ".state", "festival_events.jsonl"))
	require.NoError(t, err)
	require.False(t, eventsExist, "dry-run must not register the previewed festival")
}

func TestCreateFestivalDryRunAppliesMarkersFileWithoutWorkspaceWrites(t *testing.T) {
	tc := GetSharedContainer(t)
	festivalsPath := setupWorkspace(t, tc, "/marker-preview-campaign")
	installPreviewTemplates(t, tc, festivalsPath)

	before := snapshotFestivalWorkspace(t, tc, festivalsPath)
	discovery, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "marker-preview",
		"--type", "standard",
		"--dry-run",
		"--json",
	)
	require.NoError(t, err, "marker discovery dry-run should succeed: %s", discovery)

	var discovered createFestivalPreviewResult
	require.NoError(t, json.Unmarshal([]byte(discovery), &discovered), "dry-run should emit JSON: %s", discovery)
	require.NotEmpty(t, discovered.Markers, "dry-run is the documented way to discover markers")

	values := make(map[string]string, len(discovered.Markers))
	for _, marker := range discovered.Markers {
		values[marker.Hint] = "resolved: " + marker.Hint
	}
	encoded, err := json.Marshal(values)
	require.NoError(t, err)
	markersFile := "/tmp/marker-preview-markers.json"
	require.NoError(t, writeFileInContainer(tc, markersFile, string(encoded)))

	filled, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "marker-preview",
		"--type", "standard",
		"--dry-run",
		"--json",
		"--markers-file", markersFile,
	)
	require.NoError(t, err, "markers-file dry-run should succeed: %s", filled)

	var filledPreview createFestivalPreviewResult
	require.NoError(t, json.Unmarshal([]byte(filled), &filledPreview), "dry-run should emit JSON: %s", filled)
	require.Empty(t, filledPreview.Markers, "a complete markers file leaves nothing unfilled")
	require.Zero(t, filledPreview.MarkersUnfilled)
	require.Equal(t, discovered.MarkersTotal, filledPreview.MarkersTotal)
	require.Equal(t, discovered.MarkersTotal, filledPreview.MarkersFilled)

	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
		"marker discovery and marker verification must both leave the workspace unchanged")

	createOutput, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "marker-preview",
		"--type", "standard",
		"--markers-file", markersFile,
	)
	require.NoError(t, err, "create with the previewed markers should succeed: %s", createOutput)

	created := filepath.Join(festivalsPath, "planning", "marker-preview-"+discovered.Festival["id"])
	remaining, err := tc.runCommand([]string{
		"sh", "-c", "grep -rl '\\[REPLACE:' " + created + "/*.md " + created + "/*/PHASE_GOAL.md || true",
	})
	require.NoError(t, err)
	require.Empty(t, strings.TrimSpace(remaining),
		"the markers a dry-run reported must be exactly the markers a create fills")
}

func installPreviewTemplates(t *testing.T, tc *TestContainer, festivalsPath string) {
	t.Helper()
	_, err := tc.runCommand([]string{
		"cp", "-R", "/root/.obey/fest/festivals/.festival/templates", festivalsPath + "/.festival/",
	})
	require.NoError(t, err, "copy methodology templates into test workspace")
}

func writeZeroMarkerFestivalTemplates(t *testing.T, tc *TestContainer, festivalsPath string) {
	t.Helper()
	templatesDir := filepath.Join(festivalsPath, ".festival", "templates", "festival")
	_, err := tc.runCommand([]string{"mkdir", "-p", templatesDir})
	require.NoError(t, err)
	for _, name := range []string{"GOAL.md", "OVERVIEW.md", "RULES.md", "TODO.md"} {
		require.NoError(t, writeFileInContainer(tc, filepath.Join(templatesDir, name), "# Complete template\n"))
	}
}

func snapshotFestivalWorkspace(t *testing.T, tc *TestContainer, festivalsPath string) string {
	t.Helper()
	command := fmt.Sprintf(
		"find %q -mindepth 1 -print | sort; find %q -type f -exec sha256sum {} \\; | sort",
		festivalsPath,
		festivalsPath,
	)
	snapshot, err := tc.runCommand([]string{"sh", "-c", command})
	require.NoError(t, err)
	return strings.TrimSpace(snapshot)
}
