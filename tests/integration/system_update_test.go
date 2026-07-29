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

// writeOperatorConfig writes a .festival/config.yaml carrying an operator judge
// command marker, so tests can prove the exact content survives an update.
func writeOperatorConfig(t *testing.T, tc *TestContainer, configPath, command string) {
	t.Helper()
	_, err := tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`cat > %s <<'EOF'
version: "1.0"
hooks:
  definitions:
    approval_judge:
      command: %s
EOF`, configPath, command)})
	require.NoError(t, err, "failed to write operator config")
}

// assertConfigPreserved asserts config.yaml still exists and still carries the
// operator's judge command verbatim (i.e. it was neither deleted nor overwritten).
func assertConfigPreserved(t *testing.T, tc *TestContainer, configPath, command string) {
	t.Helper()
	exists, err := tc.CheckFileExists(configPath)
	require.NoError(t, err)
	require.True(t, exists, "user-owned config.yaml must survive system update --force")
	content, err := tc.runCommand([]string{"cat", configPath})
	require.NoError(t, err, "should read config.yaml after update")
	assert.Contains(t, content, "command: "+command, "operator judge command must be preserved verbatim")
}

// TestSystemUpdate_PreservesUserConfig verifies that `fest system update --force`
// never deletes the user-owned .festival/config.yaml. init.go scaffolds it after
// checksums so it is deliberately absent from source templates; it must not be
// treated as an orphan. Deleting it would wipe operator hooks such as
// hooks.approval_judge.command, the very config non-interactive judging relies on.
func TestSystemUpdate_PreservesUserConfig(t *testing.T) {
	tc := GetSharedContainer(t)

	parentDir := "/sysupdate-preserve-config-test"
	sourceDir := "/root/.obey/fest/festivals"
	workspaceDir := parentDir + "/festivals" // init creates this
	configPath := workspaceDir + "/.festival/config.yaml"
	orphanPath := workspaceDir + "/.festival/templates/OLD_ORPHAN.md"

	// Minimal source templates. config.yaml is user-owned and never shipped as a
	// methodology template, so remove any stray copy from the shared source to
	// deterministically reproduce a real install where source lacks config.yaml.
	_, err := tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`
		mkdir -p %s/.festival/templates/festival
		echo "# Festival Goal" > %s/.festival/templates/festival/GOAL.md
		rm -f %s/.festival/config.yaml
	`, sourceDir, sourceDir, sourceDir)})
	require.NoError(t, err, "failed to create source templates")

	_, err = tc.runCommand([]string{"mkdir", "-p", parentDir})
	require.NoError(t, err, "failed to create parent directory")

	// fest init scaffolds workspaceDir/.festival including a user-owned config.yaml.
	output, err := tc.RunFest("init", parentDir)
	require.NoError(t, err, "fest init should succeed: %s", output)

	writeOperatorConfig(t, tc, configPath, "/bin/false")

	// Plant a genuine orphan (present locally, absent from source) so we can prove
	// the destructive orphan-deletion phase actually executed during --force.
	_, err = tc.runCommand([]string{"sh", "-c", "echo OLD > " + orphanPath})
	require.NoError(t, err, "failed to create orphan file")

	// The fixture is deterministic, so the update must succeed. Discarding the
	// error would let an early failure (before orphan cleanup) satisfy the
	// preservation assertions without ever exercising the deletion path.
	output, err = tc.RunFestInDir(workspaceDir, "system", "update", "--force")
	require.NoError(t, err, "system update --force should succeed: %s", output)

	// Proof the deletion phase ran: the genuine orphan is gone...
	orphanExists, err := tc.CheckFileExists(orphanPath)
	require.NoError(t, err)
	assert.False(t, orphanExists, "genuine orphan must be deleted (proves the deletion phase ran)")

	// ...while the user-owned config survives untouched, and the update never
	// tried to update it from source.
	assertConfigPreserved(t, tc, configPath, "/bin/false")
	assert.NotContains(t, output, "config.yaml", "update must not report config.yaml in any category")
}

// TestSystemUpdate_PreservesUserConfigAcrossUpdates covers the checksum-persistence
// regression: protecting config.yaml only in the orphan loop is not enough. A
// successful update records every workspace file in .fest-checksums.json, so a
// later operator edit would be seen as "modified methodology" and a subsequent
// --force would overwrite it from source. Excluding config.yaml at the checksum
// layer means it is never tracked, so operator edits survive repeated updates
// even when a stale source later contains a config.yaml of its own.
func TestSystemUpdate_PreservesUserConfigAcrossUpdates(t *testing.T) {
	tc := GetSharedContainer(t)

	parentDir := "/sysupdate-preserve-config-multi-test"
	sourceDir := "/root/.obey/fest/festivals"
	workspaceDir := parentDir + "/festivals"
	configPath := workspaceDir + "/.festival/config.yaml"

	_, err := tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`
		mkdir -p %s/.festival/templates/festival
		echo "# Festival Goal" > %s/.festival/templates/festival/GOAL.md
		rm -f %s/.festival/config.yaml
	`, sourceDir, sourceDir, sourceDir)})
	require.NoError(t, err, "failed to create source templates")

	_, err = tc.runCommand([]string{"mkdir", "-p", parentDir})
	require.NoError(t, err, "failed to create parent directory")

	output, err := tc.RunFest("init", parentDir)
	require.NoError(t, err, "fest init should succeed: %s", output)

	// First update with the operator's original command.
	writeOperatorConfig(t, tc, configPath, "/bin/first-judge")
	output, err = tc.RunFestInDir(workspaceDir, "system", "update", "--force")
	require.NoError(t, err, "first system update --force should succeed: %s", output)
	assertConfigPreserved(t, tc, configPath, "/bin/first-judge")

	// A stale source now contains a config.yaml of its own. If config.yaml were
	// tracked, the operator's edit below would be classified as modified and the
	// second --force would overwrite it with this stale content. Guarantee cleanup
	// via t.Cleanup so a failing assertion cannot leak it into the shared source
	// dir and make other tests falsely pass.
	t.Cleanup(func() { _, _ = tc.runCommand([]string{"rm", "-f", sourceDir + "/.festival/config.yaml"}) })
	_, err = tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`cat > %s/.festival/config.yaml <<'EOF'
version: "1.0"
hooks:
  definitions:
    approval_judge:
      command: /bin/STALE-SOURCE
EOF`, sourceDir)})
	require.NoError(t, err, "failed to plant stale source config")

	// Operator edits their config after the first update, then runs update again.
	writeOperatorConfig(t, tc, configPath, "/bin/second-judge")
	output, err = tc.RunFestInDir(workspaceDir, "system", "update", "--force")
	require.NoError(t, err, "second system update --force should succeed: %s", output)

	// The operator's edit must survive, and must not be replaced by stale source.
	assertConfigPreserved(t, tc, configPath, "/bin/second-judge")

	content, err := tc.runCommand([]string{"cat", configPath})
	require.NoError(t, err, "should read config.yaml after second update")
	assert.NotContains(t, content, "STALE-SOURCE", "operator config must not be overwritten from source")
}
