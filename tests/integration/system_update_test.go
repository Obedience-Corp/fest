//go:build integration
// +build integration

package integration

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSystemUpdate_DeletesOrphanedFiles verifies that `fest system update --force`
// deletes files that exist in the workspace but not in the source templates.
// This is a regression test for the bug where old template files were not cleaned up.
func TestSystemUpdate_DeletesOrphanedFiles(t *testing.T) {
	tc := GetSharedContainer(t)

	// Use unique paths to avoid conflicts with other tests
	// Note: fest init appends "festivals" to the path, so we use a parent dir
	parentDir := "/sysupdate-delete-test"
	sourceDir := "/root/.obey/fest/festivals"
	workspaceDir := parentDir + "/festivals" // init creates this

	// Step 1: Create a minimal source template structure (simulating ~/.config/fest/festivals/)
	// This represents the "new" template structure from upstream
	_, err := tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`
		mkdir -p %s/.festival/templates/festival
		mkdir -p %s/.festival/templates/phases/implementation
		mkdir -p %s/.festival/templates/tasks

		echo "# Festival Goal Template" > %s/.festival/templates/festival/GOAL.md
		echo "# Implementation Phase" > %s/.festival/templates/phases/implementation/GOAL.md
		echo "# Task Template" > %s/.festival/templates/tasks/TASK.md
	`, sourceDir, sourceDir, sourceDir, sourceDir, sourceDir, sourceDir)})
	require.NoError(t, err, "failed to create source templates")

	// Step 2: Create parent directory
	_, err = tc.runCommand([]string{"mkdir", "-p", parentDir})
	require.NoError(t, err, "failed to create parent directory")

	// Step 3: Initialize workspace using fest init (this creates parentDir/festivals with .festival)
	output, err := tc.RunFest("init", parentDir)
	require.NoError(t, err, "fest init should succeed: %s", output)
	t.Logf("Init output: %s", output)

	// Step 4: Create orphaned files (files that shouldn't be in source)
	_, err = tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`
		mkdir -p %s/.festival/templates/gates

		# Create orphaned files (don't exist in source)
		echo "OLD TEMPLATE - SHOULD BE DELETED" > %s/.festival/templates/FESTIVAL_GOAL_TEMPLATE.md
		echo "OLD TEMPLATE - SHOULD BE DELETED" > %s/.festival/templates/PHASE_GOAL_TEMPLATE.md
		echo "OLD TEMPLATE - SHOULD BE DELETED" > %s/.festival/templates/gates/QUALITY_GATE.md
	`, workspaceDir, workspaceDir, workspaceDir, workspaceDir)})
	require.NoError(t, err, "failed to create orphaned files")

	// Verify orphaned files exist before update
	orphan1Exists, err := tc.CheckFileExists(workspaceDir + "/.festival/templates/FESTIVAL_GOAL_TEMPLATE.md")
	require.NoError(t, err)
	assert.True(t, orphan1Exists, "orphaned file should exist before update")

	orphan2Exists, err := tc.CheckFileExists(workspaceDir + "/.festival/templates/PHASE_GOAL_TEMPLATE.md")
	require.NoError(t, err)
	assert.True(t, orphan2Exists, "orphaned file should exist before update")

	gatesOrphanExists, err := tc.CheckFileExists(workspaceDir + "/.festival/templates/gates/QUALITY_GATE.md")
	require.NoError(t, err)
	assert.True(t, gatesOrphanExists, "orphaned gates file should exist before update")

	// Step 5: Run fest system update --force
	output, err = tc.RunFestInDir(workspaceDir, "system", "update", "--force")
	t.Logf("Update output: %s", output)
	// Note: We don't require NoError because the update might succeed but return non-zero

	// Step 6: Verify orphaned files were deleted
	orphan1Exists, err = tc.CheckFileExists(workspaceDir + "/.festival/templates/FESTIVAL_GOAL_TEMPLATE.md")
	require.NoError(t, err)
	assert.False(t, orphan1Exists, "orphaned FESTIVAL_GOAL_TEMPLATE.md should be deleted after update")

	orphan2Exists, err = tc.CheckFileExists(workspaceDir + "/.festival/templates/PHASE_GOAL_TEMPLATE.md")
	require.NoError(t, err)
	assert.False(t, orphan2Exists, "orphaned PHASE_GOAL_TEMPLATE.md should be deleted after update")

	gatesOrphanExists, err = tc.CheckFileExists(workspaceDir + "/.festival/templates/gates/QUALITY_GATE.md")
	require.NoError(t, err)
	assert.False(t, gatesOrphanExists, "orphaned gates/QUALITY_GATE.md should be deleted after update")

	// Verify the empty gates directory was cleaned up
	gatesDirExists, err := tc.CheckDirExists(workspaceDir + "/.festival/templates/gates")
	require.NoError(t, err)
	assert.False(t, gatesDirExists, "empty gates directory should be removed after update")

	// Step 7: Verify good files still exist
	goodFile1Exists, err := tc.CheckFileExists(workspaceDir + "/.festival/templates/festival/GOAL.md")
	require.NoError(t, err)
	assert.True(t, goodFile1Exists, "festival/GOAL.md should still exist after update")

	goodFile2Exists, err := tc.CheckFileExists(workspaceDir + "/.festival/templates/phases/implementation/GOAL.md")
	require.NoError(t, err)
	assert.True(t, goodFile2Exists, "phases/implementation/GOAL.md should still exist after update")

	goodFile3Exists, err := tc.CheckFileExists(workspaceDir + "/.festival/templates/tasks/TASK.md")
	require.NoError(t, err)
	assert.True(t, goodFile3Exists, "tasks/TASK.md should still exist after update")
}

// TestSystemUpdate_ShowsOrphanedInDryRun verifies that `fest system update --dry-run`
// correctly reports orphaned files that would be deleted.
func TestSystemUpdate_ShowsOrphanedInDryRun(t *testing.T) {
	tc := GetSharedContainer(t)

	// Use unique paths to avoid conflicts with other tests
	// Note: fest init appends "festivals" to the path, so we use a parent dir
	parentDir := "/sysupdate-dryrun-test"
	sourceDir := "/root/.obey/fest/festivals"
	workspaceDir := parentDir + "/festivals" // init creates this

	// Setup source templates
	_, err := tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`
		mkdir -p %s/.festival/templates/festival
		echo "# Festival Goal" > %s/.festival/templates/festival/GOAL.md
	`, sourceDir, sourceDir)})
	require.NoError(t, err, "failed to create source templates")

	// Create parent directory
	_, err = tc.runCommand([]string{"mkdir", "-p", parentDir})
	require.NoError(t, err, "failed to create parent directory")

	// Initialize workspace using fest init (creates parentDir/festivals with .festival)
	output, err := tc.RunFest("init", parentDir)
	require.NoError(t, err, "fest init should succeed: %s", output)
	t.Logf("Init output: %s", output)

	// Create orphaned file (doesn't exist in source)
	_, err = tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`
		echo "OLD FILE" > %s/.festival/templates/OLD_ORPHAN.md
	`, workspaceDir)})
	require.NoError(t, err, "failed to create orphaned file")

	// Run dry-run
	output, err = tc.RunFestInDir(workspaceDir, "system", "update", "--dry-run")
	t.Logf("Dry-run output: %s", output)

	// Verify output mentions orphaned files
	assert.Contains(t, output, "Orphaned", "dry-run should report orphaned files")
	assert.Contains(t, output, "1 files", "should show 1 orphaned file")

	// Verify orphaned file still exists (dry-run shouldn't delete)
	orphanExists, err := tc.CheckFileExists(workspaceDir + "/.festival/templates/OLD_ORPHAN.md")
	require.NoError(t, err)
	assert.True(t, orphanExists, "orphaned file should still exist after dry-run")
}
