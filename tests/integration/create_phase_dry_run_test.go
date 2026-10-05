//go:build integration

package integration

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreatePhaseDryRunWritesNothing(t *testing.T) {
	tc := GetSharedContainer(t)
	festivalsPath := setupWorkspace(t, tc, "/phase-dry-run-campaign")
	installPreviewTemplates(t, tc, festivalsPath)
	festivalPath := createScratchFestival(t, tc, festivalsPath, "phase-preview")

	before := snapshotFestivalWorkspace(t, tc, festivalsPath)
	output, err := tc.RunFestInDir(festivalPath, "create", "phase",
		"--name", "PREVIEW", "--type", "implementation", "--dry-run", "--json")
	require.NoError(t, err, "phase dry-run should succeed: %s", output)
	preview := parseCreateDryRunPreview(t, output)
	require.True(t, preview.DryRun)
	require.Equal(t, "dry_run", preview.Action)
	require.Len(t, preview.PlannedPaths, 1)
	require.Greater(t, preview.Count, 0, "dry-run should report phase template markers")
	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
		"phase preview must preserve every directory, file byte, and activity log")

	output, err = tc.RunFestInDir(festivalPath, "create", "phase",
		"--name", "PREVIEW", "--type", "implementation", "--dry-run", "--no-color")
	require.NoError(t, err, "human phase dry-run should succeed: %s", output)
	require.Contains(t, output, "Dry Run: No Files Created")
	require.Contains(t, output, preview.PlannedPaths[0])
	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath))

	output, err = tc.RunFestInDir(festivalPath, "create", "phase",
		"--name", "PREVIEW", "--type", "implementation", "--json", "--skip-markers")
	require.NoError(t, err, "real create after preview should succeed: %s", output)
	goalPath := filepath.Join(festivalPath, preview.PlannedPaths[0])
	exists, err := tc.CheckFileExists(goalPath)
	require.NoError(t, err)
	require.True(t, exists, "real create must reuse the previewed number")
	exists, err = tc.CheckFileExists(filepath.Join(filepath.Dir(goalPath), "GATES.md"))
	require.NoError(t, err)
	require.True(t, exists, "real create must still copy the phase structure")
}

func TestCreatePhaseDryRunDoesNotRenumberExistingPhases(t *testing.T) {
	tc := GetSharedContainer(t)
	festivalsPath := setupWorkspace(t, tc, "/phase-dry-run-insert")
	installPreviewTemplates(t, tc, festivalsPath)
	festivalPath := createScratchFestival(t, tc, festivalsPath, "phase-insert-preview")
	resolveOrCreatePhase(t, tc, festivalPath)

	// No-marker templates and --skip-markers must still take the preview path.
	templatePath := filepath.Join(festivalsPath, ".festival", "templates", "phases", "implementation", "GOAL.md")
	require.NoError(t, writeFileInContainer(tc, templatePath, "# Complete phase goal\n"))
	before := snapshotFestivalWorkspace(t, tc, festivalsPath)
	for _, extra := range [][]string{{"--json"}, {"--agent", "--skip-markers"}} {
		args := []string{"create", "phase", "--name", "INTRO", "--type", "implementation", "--after", "0", "--dry-run"}
		output, err := tc.RunFestInDir(festivalPath, append(args, extra...)...)
		require.NoError(t, err, "insert preview should succeed: %s", output)
		preview := parseCreateDryRunPreview(t, output)
		require.True(t, preview.DryRun)
		require.Contains(t, preview.PlannedPaths, "001_INTRO/PHASE_GOAL.md")
		require.Zero(t, preview.Count, "zero-marker templates must still emit a dry-run payload")
		require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
			"insert preview must not renumber phases or rewrite frontmatter/activity")
	}

	output, err := tc.RunFestInDir(festivalPath, "create", "phase", "--name", "INVALID",
		"--type", "unknown", "--after", "0", "--dry-run", "--json")
	require.NoError(t, err, "JSON errors should be returned as a structured payload: %s", output)
	require.Contains(t, output, `"ok": false`)
	require.Contains(t, output, "unknown phase type")
	require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festivalsPath),
		"failed previews must not renumber phases or leave directories behind")
}
