//go:build integration
// +build integration

package integration

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDungeonDualResolution exercises fest's status/list/promote-to-dungeon
// flows against campaigns scaffolded with the visible dungeon/ spelling, the
// hidden .dungeon/ spelling, and both at once, verifying every flow resolves
// to the correct on-disk directory per docs/tribal-knowledge dungeon
// resolution rules: whichever exists; visible wins (with a warning) if both
// exist; visible is created when neither exists yet.
func TestDungeonDualResolution(t *testing.T) {
	tc := GetSharedContainer(t)

	t.Run("HiddenFixture_PromoteAndList", func(t *testing.T) {
		base := "/dungeon-hidden"
		festivalsDir := setupDungeonWorkspace(t, tc, base, ".dungeon")

		festPath := createPlanningFestival(t, tc, festivalsDir, "hidden-dungeon-test")

		output, err := tc.RunFestInDir(festPath, "promote", "--dungeon", "completed", "--force", "--no-commit")
		require.NoError(t, err, "promote --dungeon completed should succeed: %s", output)

		// The festival must land under the hidden spelling's completed/ bucket.
		completedPath, found := findDungeonCompletedFestival(t, tc, festivalsDir+"/.dungeon/completed", "hidden-dungeon-test")
		assert.True(t, found, "festival should be moved under .dungeon/completed/<date>/")
		if found {
			goalExists, err := tc.CheckFileExists(completedPath + "/fest.yaml")
			require.NoError(t, err)
			assert.True(t, goalExists, "fest.yaml should be preserved through the move")
		}

		// A visible dungeon/ must never have been created for this campaign.
		visibleExists, err := tc.CheckDirExists(festivalsDir + "/dungeon")
		require.NoError(t, err)
		assert.False(t, visibleExists, "visible dungeon/ should not be created when .dungeon/ already exists")

		// The original planning/ directory should be gone.
		oldExists, err := tc.CheckDirExists(festPath)
		require.NoError(t, err)
		assert.False(t, oldExists, "festival should no longer exist at its old planning/ path")

		// fest list must discover the festival through the hidden spelling.
		listOutput, err := tc.RunFestInDir(festivalsDir, "list", "dungeon/completed")
		require.NoError(t, err, "fest list dungeon/completed should succeed: %s", listOutput)
		assert.Contains(t, listOutput, "hidden-dungeon-test", "list output should include the completed festival")
	})

	t.Run("HiddenFixture_StatusSetToArchived", func(t *testing.T) {
		// Covers the fest status set code path (separate from fest promote),
		// which resolves the dungeon independently.
		base := "/dungeon-hidden-status"
		festivalsDir := setupDungeonWorkspace(t, tc, base, ".dungeon")

		festPath := createPlanningFestival(t, tc, festivalsDir, "hidden-status-test")

		output, err := tc.RunFestInDir(festPath, "status", "set", "archived", "--force", "--no-commit")
		require.NoError(t, err, "status set archived should succeed: %s", output)

		archivedPath, found := findDungeonCompletedFestival(t, tc, festivalsDir+"/.dungeon/archived", "hidden-status-test")
		assert.True(t, found, "festival should be moved under .dungeon/archived/<date>/")
		_ = archivedPath

		visibleExists, err := tc.CheckDirExists(festivalsDir + "/dungeon")
		require.NoError(t, err)
		assert.False(t, visibleExists, "visible dungeon/ should not be created by fest status set either")
	})

	t.Run("LegacyFixture_UntouchedAndList", func(t *testing.T) {
		base := "/dungeon-legacy"
		festivalsDir := setupDungeonWorkspace(t, tc, base, "dungeon")

		festPath := createPlanningFestival(t, tc, festivalsDir, "legacy-dungeon-test")

		output, err := tc.RunFestInDir(festPath, "promote", "--dungeon", "completed", "--force", "--no-commit")
		require.NoError(t, err, "promote --dungeon completed should succeed on a legacy fixture: %s", output)

		completedPath, found := findDungeonCompletedFestival(t, tc, festivalsDir+"/dungeon/completed", "legacy-dungeon-test")
		assert.True(t, found, "festival should be moved under the legacy visible dungeon/completed/<date>/")
		if found {
			goalExists, err := tc.CheckFileExists(completedPath + "/fest.yaml")
			require.NoError(t, err)
			assert.True(t, goalExists, "fest.yaml should be preserved through the move")
		}

		// A hidden .dungeon/ must never be introduced for a legacy campaign.
		hiddenExists, err := tc.CheckDirExists(festivalsDir + "/.dungeon")
		require.NoError(t, err)
		assert.False(t, hiddenExists, "legacy campaigns must never gain a .dungeon/ directory")

		listOutput, err := tc.RunFestInDir(festivalsDir, "list", "dungeon/completed")
		require.NoError(t, err, "fest list dungeon/completed should succeed: %s", listOutput)
		assert.Contains(t, listOutput, "legacy-dungeon-test", "list output should include the completed festival")
	})

	t.Run("BothExist_PrefersVisibleAndWarns", func(t *testing.T) {
		base := "/dungeon-both"
		festivalsDir := setupDungeonWorkspace(t, tc, base, "dungeon", ".dungeon")

		festPath := createPlanningFestival(t, tc, festivalsDir, "both-dungeon-test")

		output, err := tc.RunFestInDir(festPath, "promote", "--dungeon", "completed", "--force", "--no-commit")
		require.NoError(t, err, "promote --dungeon completed should succeed when both spellings exist: %s", output)

		assert.True(t,
			strings.Contains(output, "dungeon/") && strings.Contains(output, ".dungeon/"),
			"promote output should warn that both dungeon/ and .dungeon/ exist: %s", output)

		visiblePath, foundVisible := findDungeonCompletedFestival(t, tc, festivalsDir+"/dungeon/completed", "both-dungeon-test")
		assert.True(t, foundVisible, "festival should be moved under the visible dungeon/completed/<date>/")
		_ = visiblePath

		_, foundHidden := findDungeonCompletedFestival(t, tc, festivalsDir+"/.dungeon/completed", "both-dungeon-test")
		assert.False(t, foundHidden, "festival must not land under the hidden .dungeon/completed/ when both exist")
	})
}

// setupDungeonWorkspace creates a minimal festivals workspace at basePath/festivals
// with active/ and planning/ present, plus completed/archived/someday
// pre-created under every dungeon directory name passed in dungeonDirNames
// (e.g. "dungeon", ".dungeon", or both). Returns the festivals directory path.
func setupDungeonWorkspace(t *testing.T, tc *TestContainer, basePath string, dungeonDirNames ...string) string {
	t.Helper()

	festivalsDir := basePath + "/festivals"

	dirs := []string{
		festivalsDir + "/.festival/.state",
		festivalsDir + "/active",
		festivalsDir + "/planning",
	}
	for _, name := range dungeonDirNames {
		dirs = append(dirs,
			festivalsDir+"/"+name+"/completed",
			festivalsDir+"/"+name+"/archived",
			festivalsDir+"/"+name+"/someday",
		)
	}

	_, err := tc.runCommand([]string{"sh", "-c", "mkdir -p " + strings.Join(dirs, " ")})
	require.NoError(t, err, "should create dungeon workspace directories")

	err = writeFileInContainer(tc, festivalsDir+"/.festival/.state/.workspace",
		`{"workspace": "test", "registered": "2024-01-01T00:00:00Z"}`)
	require.NoError(t, err, "should create workspace marker")

	return festivalsDir
}

// createPlanningFestival creates a festival under festivalsDir/planning and
// returns its full path (fest appends a generated ID suffix to festName).
func createPlanningFestival(t *testing.T, tc *TestContainer, festivalsDir, festName string) string {
	t.Helper()

	output, err := tc.RunFestInDir(festivalsDir, "create", "festival", "--name", festName, "--dest", "planning")
	require.NoError(t, err, "should create festival: %s", output)

	return findFestivalPath(t, tc, festivalsDir+"/planning", festName)
}

// findDungeonCompletedFestival looks for a festival directory whose name has
// the given prefix nested one date-bucket level under completedDir (the
// layout fest uses for every dungeon substatus: <substatus>/<date>/<name>).
func findDungeonCompletedFestival(t *testing.T, tc *TestContainer, completedDir, namePrefix string) (string, bool) {
	t.Helper()

	buckets, err := tc.ListDirectories(completedDir)
	require.NoError(t, err, "should list date buckets under %s", completedDir)

	for _, bucket := range buckets {
		bucketPath := fmt.Sprintf("%s/%s", completedDir, bucket)
		entries, err := tc.ListDirectories(bucketPath)
		require.NoError(t, err, "should list entries under %s", bucketPath)
		for _, entry := range entries {
			if strings.HasPrefix(entry, namePrefix) {
				return fmt.Sprintf("%s/%s", bucketPath, entry), true
			}
		}
	}
	return "", false
}
