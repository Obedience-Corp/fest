//go:build integration
// +build integration

package integration

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSystemRepair_DryRunReportsIssues verifies that --dry-run detects and reports
// all known issues (planned/ dir, top-level completed/ dir, missing directories,
// missing schema) without making any changes.
func TestSystemRepair_DryRunReportsIssues(t *testing.T) {
	tc := GetSharedContainer(t)

	base := "/repair-dryrun"
	festivalsDir := setupOldLayoutWorkspace(t, tc, base)

	// Run repair --dry-run
	output, err := tc.RunFestInDir(festivalsDir, "system", "repair", "--dry-run")
	t.Logf("Dry-run output: %s", output)
	require.NoError(t, err, "dry-run should not fail")

	// Verify it reports the issues
	assert.Contains(t, output, "planned", "should report planned/ rename needed")
	assert.Contains(t, output, "planning", "should mention planning/ as target")
	assert.Contains(t, output, "completed", "should report completed/ move needed")
	assert.Contains(t, output, "dungeon", "should mention dungeon/ as target")

	// Verify nothing was changed
	plannedExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "planned"))
	require.NoError(t, err)
	assert.True(t, plannedExists, "planned/ should still exist after dry-run")

	completedExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "completed"))
	require.NoError(t, err)
	assert.True(t, completedExists, "completed/ should still exist after dry-run")

	// Festivals inside completed/ should be untouched
	festivalExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "completed", "my-old-festival"))
	require.NoError(t, err)
	assert.True(t, festivalExists, "festival inside completed/ should still exist after dry-run")
}

// TestSystemRepair_FixPlannedToPlanning verifies that repair renames planned/ → planning/
// and preserves all content inside.
func TestSystemRepair_FixPlannedToPlanning(t *testing.T) {
	tc := GetSharedContainer(t)

	base := "/repair-planned"
	festivalsDir := base + "/festivals"

	// Create minimal workspace with old planned/ directory containing a festival
	_, err := tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`
		mkdir -p %s/.festival/.state %s/planned/some-festival %s/active
		echo '{"workspace": "test", "registered": "2024-01-01T00:00:00Z"}' > %s/.festival/.state/.workspace
		echo "# Some Festival" > %s/planned/some-festival/FESTIVAL_GOAL.md
	`, festivalsDir, festivalsDir, festivalsDir, festivalsDir, festivalsDir)})
	require.NoError(t, err, "should create test workspace")

	// Run repair --force
	output, err := tc.RunFestInDir(festivalsDir, "system", "repair", "--force")
	t.Logf("Repair output: %s", output)
	require.NoError(t, err, "repair should succeed")

	// Verify planned/ was renamed to planning/
	plannedExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "planned"))
	require.NoError(t, err)
	assert.False(t, plannedExists, "planned/ should no longer exist")

	planningExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "planning"))
	require.NoError(t, err)
	assert.True(t, planningExists, "planning/ should exist")

	// Content should be preserved
	goalExists, err := tc.CheckFileExists(filepath.Join(festivalsDir, "planning", "some-festival", "FESTIVAL_GOAL.md"))
	require.NoError(t, err)
	assert.True(t, goalExists, "FESTIVAL_GOAL.md should be preserved under planning/")

	content, err := tc.ReadFile(filepath.Join(festivalsDir, "planning", "some-festival", "FESTIVAL_GOAL.md"))
	require.NoError(t, err)
	assert.Contains(t, content, "Some Festival", "file content should be preserved")
}

// TestSystemRepair_FixCompletedToDungeon verifies that repair moves top-level
// completed/ → dungeon/completed/ and preserves all festival content inside.
func TestSystemRepair_FixCompletedToDungeon(t *testing.T) {
	tc := GetSharedContainer(t)

	base := "/repair-completed"
	festivalsDir := base + "/festivals"

	// Create workspace with old top-level completed/ containing festivals
	_, err := tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`
		mkdir -p %s/.festival/.state %s/active %s/planning
		mkdir -p %s/completed/finished-project-AB0001
		mkdir -p %s/completed/another-done-CD0002/001_implementation
		echo '{"workspace": "test", "registered": "2024-01-01T00:00:00Z"}' > %s/.festival/.state/.workspace
		echo "# Finished Project" > %s/completed/finished-project-AB0001/FESTIVAL_GOAL.md
		echo "# Phase 1" > %s/completed/another-done-CD0002/001_implementation/PHASE_GOAL.md
		echo "task content" > %s/completed/another-done-CD0002/001_implementation/01_task.md
	`, festivalsDir, festivalsDir, festivalsDir,
		festivalsDir, festivalsDir,
		festivalsDir,
		festivalsDir, festivalsDir, festivalsDir)})
	require.NoError(t, err, "should create test workspace")

	// Run repair --force
	output, err := tc.RunFestInDir(festivalsDir, "system", "repair", "--force")
	t.Logf("Repair output: %s", output)
	require.NoError(t, err, "repair should succeed")

	// Verify completed/ was moved to dungeon/completed/
	completedExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "completed"))
	require.NoError(t, err)
	assert.False(t, completedExists, "top-level completed/ should no longer exist")

	dungeonCompletedExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "dungeon", "completed"))
	require.NoError(t, err)
	assert.True(t, dungeonCompletedExists, "dungeon/completed/ should exist")

	// Festival content should be preserved
	fest1Exists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "dungeon", "completed", "finished-project-AB0001"))
	require.NoError(t, err)
	assert.True(t, fest1Exists, "finished-project-AB0001 should be preserved under dungeon/completed/")

	goalContent, err := tc.ReadFile(filepath.Join(festivalsDir, "dungeon", "completed", "finished-project-AB0001", "FESTIVAL_GOAL.md"))
	require.NoError(t, err)
	assert.Contains(t, goalContent, "Finished Project", "goal content should be preserved")

	// Nested festival with phases should be preserved
	phaseExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "dungeon", "completed", "another-done-CD0002", "001_implementation"))
	require.NoError(t, err)
	assert.True(t, phaseExists, "phase directory should be preserved")

	taskContent, err := tc.ReadFile(filepath.Join(festivalsDir, "dungeon", "completed", "another-done-CD0002", "001_implementation", "01_task.md"))
	require.NoError(t, err)
	assert.Contains(t, taskContent, "task content", "task content should be preserved")
}

// TestSystemRepair_FullRepairWithAllIssues verifies the full repair process:
// old planned/ + old completed/ + missing directories + missing schema.
// Then verifies the resulting directory is fully correct.
func TestSystemRepair_FullRepairWithAllIssues(t *testing.T) {
	tc := GetSharedContainer(t)

	base := "/repair-full"
	festivalsDir := setupOldLayoutWorkspace(t, tc, base)

	// Run repair --force
	output, err := tc.RunFestInDir(festivalsDir, "system", "repair", "--force")
	t.Logf("Repair output: %s", output)
	require.NoError(t, err, "repair should succeed")

	// Verify output mentions the fixes
	assert.Contains(t, output, "Renamed", "should report rename")
	assert.Contains(t, output, "Moved", "should report move")
	assert.Contains(t, output, "Created", "should report created dirs")

	// Verify old directories are gone
	plannedExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "planned"))
	require.NoError(t, err)
	assert.False(t, plannedExists, "planned/ should be gone")

	completedExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "completed"))
	require.NoError(t, err)
	assert.False(t, completedExists, "top-level completed/ should be gone")

	// Verify correct directory structure exists
	expectedDirs := []string{
		"planning",
		"ready",
		"active",
		"dungeon/completed",
		"dungeon/archived",
		"dungeon/someday",
	}
	for _, dir := range expectedDirs {
		exists, err := tc.CheckDirExists(filepath.Join(festivalsDir, dir))
		require.NoError(t, err)
		assert.True(t, exists, "%s/ should exist after repair", dir)
	}

	// Verify .workflow.yaml was created
	schemaExists, err := tc.CheckFileExists(filepath.Join(festivalsDir, ".workflow.yaml"))
	require.NoError(t, err)
	assert.True(t, schemaExists, ".workflow.yaml should exist after repair")

	schemaContent, err := tc.ReadFile(filepath.Join(festivalsDir, ".workflow.yaml"))
	require.NoError(t, err)
	assert.Contains(t, schemaContent, "planning", "schema should reference planning status")
	assert.Contains(t, schemaContent, "dungeon", "schema should reference dungeon")

	// Verify festival content was preserved through the moves
	goalExists, err := tc.CheckFileExists(filepath.Join(festivalsDir, "planning", "my-planned-festival", "FESTIVAL_GOAL.md"))
	require.NoError(t, err)
	assert.True(t, goalExists, "planned festival content should be preserved under planning/")

	oldFestExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "dungeon", "completed", "my-old-festival"))
	require.NoError(t, err)
	assert.True(t, oldFestExists, "completed festival should be preserved under dungeon/completed/")
}

// TestSystemRepair_Idempotent verifies that running repair on an already-repaired
// workspace detects no issues and makes no changes.
func TestSystemRepair_Idempotent(t *testing.T) {
	tc := GetSharedContainer(t)

	base := "/repair-idempotent"
	festivalsDir := setupOldLayoutWorkspace(t, tc, base)

	// First repair
	output1, err := tc.RunFestInDir(festivalsDir, "system", "repair", "--force")
	t.Logf("First repair output: %s", output1)
	require.NoError(t, err, "first repair should succeed")

	// Second repair - should detect no issues
	output2, err := tc.RunFestInDir(festivalsDir, "system", "repair", "--dry-run")
	t.Logf("Second repair output: %s", output2)
	require.NoError(t, err, "second repair should succeed")

	assert.Contains(t, output2, "No issues detected", "second run should find no issues")
}

// TestSystemRepair_CreatesOnlyMissingDirs verifies that repair creates only
// the directories that are actually missing, without touching existing ones.
func TestSystemRepair_CreatesOnlyMissingDirs(t *testing.T) {
	tc := GetSharedContainer(t)

	base := "/repair-missing"
	festivalsDir := base + "/festivals"

	// Create a workspace with correct names but missing some directories
	_, err := tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`
		mkdir -p %s/.festival/.state %s/planning %s/active
		echo '{"workspace": "test", "registered": "2024-01-01T00:00:00Z"}' > %s/.festival/.state/.workspace
		echo "existing content" > %s/active/test-file.md
	`, festivalsDir, festivalsDir, festivalsDir, festivalsDir, festivalsDir)})
	require.NoError(t, err, "should create test workspace")

	// Run repair --force
	output, err := tc.RunFestInDir(festivalsDir, "system", "repair", "--force")
	t.Logf("Repair output: %s", output)
	require.NoError(t, err, "repair should succeed")

	// Verify missing directories were created
	readyExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "ready"))
	require.NoError(t, err)
	assert.True(t, readyExists, "ready/ should be created")

	dungeonCompletedExists, err := tc.CheckDirExists(filepath.Join(festivalsDir, "dungeon", "completed"))
	require.NoError(t, err)
	assert.True(t, dungeonCompletedExists, "dungeon/completed/ should be created")

	// Verify existing content was NOT touched
	existingContent, err := tc.ReadFile(filepath.Join(festivalsDir, "active", "test-file.md"))
	require.NoError(t, err)
	assert.Contains(t, existingContent, "existing content", "existing files should be untouched")
}

// setupOldLayoutWorkspace creates a workspace with the old directory layout:
// - planned/ instead of planning/ (with a festival inside)
// - top-level completed/ instead of dungeon/completed/ (with a festival inside)
// - active/ exists
// - no ready/, no dungeon/ subdirs, no .workflow.yaml
// Returns the path to the festivals directory.
func setupOldLayoutWorkspace(t *testing.T, tc *TestContainer, basePath string) string {
	t.Helper()

	festivalsDir := basePath + "/festivals"

	_, err := tc.runCommand([]string{"sh", "-c", fmt.Sprintf(`
		mkdir -p %s/.festival/.state
		mkdir -p %s/planned/my-planned-festival
		mkdir -p %s/completed/my-old-festival
		mkdir -p %s/active

		echo '{"workspace": "test", "registered": "2024-01-01T00:00:00Z"}' > %s/.festival/.state/.workspace
		echo "# My Planned Festival" > %s/planned/my-planned-festival/FESTIVAL_GOAL.md
		echo "# My Old Festival" > %s/completed/my-old-festival/FESTIVAL_GOAL.md
	`, festivalsDir, festivalsDir, festivalsDir, festivalsDir,
		festivalsDir, festivalsDir, festivalsDir)})
	require.NoError(t, err, "should create old-layout workspace")

	return festivalsDir
}
