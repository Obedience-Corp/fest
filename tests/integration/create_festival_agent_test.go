//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type createFestivalAgentResult struct {
	OK              bool              `json:"ok"`
	Festival        map[string]string `json:"festival"`
	CreatedPath     string            `json:"created_path"`
	MarkersUnfilled int               `json:"markers_unfilled"`
	UnfilledMarkers []struct {
		File    string `json:"file"`
		Count   int    `json:"count"`
		Markers []struct {
			Line       int    `json:"line"`
			MarkerType string `json:"marker_type"`
			Content    string `json:"content"`
		} `json:"markers"`
	} `json:"unfilled_markers"`
	Validation *struct {
		OK     bool `json:"ok"`
		Errors int  `json:"errors"`
		Issues []struct {
			Code string `json:"code"`
			Path string `json:"path"`
		} `json:"issues"`
	} `json:"validation"`
	RolledBack *bool `json:"rolled_back"`
	Errors     []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func TestCreateFestivalAgentValidationFailureRollsBackAndReportsMarkers(t *testing.T) {
	tc := GetSharedContainer(t)
	festivalsPath := setupWorkspace(t, tc, "/")
	writeAgentMarkerTemplates(t, tc, festivalsPath)

	output, err := tc.RunFestInDir(festivalsPath, "create", "festival", "--name", "agent-rollback", "--dest", "planning", "--agent")
	require.Error(t, err, "agent mode should exit non-zero when strict validation blocks")

	var failed createFestivalAgentResult
	require.NoError(t, json.Unmarshal([]byte(output), &failed), "agent failure should be JSON: %s", output)
	require.False(t, failed.OK)
	require.NotEmpty(t, failed.CreatedPath)
	require.NotNil(t, failed.RolledBack)
	require.True(t, *failed.RolledBack)
	require.Equal(t, 3, failed.MarkersUnfilled)
	require.Len(t, failed.UnfilledMarkers, 3)
	seenMarkers := make(map[string]int)
	for _, file := range failed.UnfilledMarkers {
		require.NotEmpty(t, file.File)
		require.Equal(t, 1, file.Count)
		require.NotEmpty(t, file.Markers)
		seenMarkers[file.File] = file.Count
	}
	require.Equal(t, map[string]int{
		"FESTIVAL_GOAL.md":     1,
		"FESTIVAL_OVERVIEW.md": 1,
		"TODO.md":              1,
	}, seenMarkers)
	require.NotNil(t, failed.Validation)
	require.False(t, failed.Validation.OK)
	require.NotEmpty(t, failed.Validation.Issues)
	require.NotEmpty(t, failed.Errors)
	require.Contains(t, failed.Errors[0].Message, "validation errors detected")

	exists, err := tc.CheckDirExists(failed.CreatedPath)
	require.NoError(t, err)
	require.False(t, exists, "strict agent failure should remove the created festival directory")

	dirs, err := tc.ListDirectories(festivalsPath + "/planning")
	require.NoError(t, err)
	require.Empty(t, dirs, "strict agent failure should leave no planning festival behind")

	markersArg := `'{"goal":"Goal","overview":"Overview","todo":"Todo"}'`
	output, err = tc.RunFestInDir(festivalsPath, "create", "festival", "--name", "agent-rollback", "--dest", "planning", "--agent", "--markers", markersArg)
	require.NoError(t, err, "retry with marker values should succeed: %s", output)

	var succeeded createFestivalAgentResult
	require.NoError(t, json.Unmarshal([]byte(output), &succeeded), "agent success should be JSON: %s", output)
	require.True(t, succeeded.OK)
	require.Equal(t, "AR0001", succeeded.Festival["id"], "retry should reuse the first ID after rollback")

	dirs, err = tc.ListDirectories(festivalsPath + "/planning")
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	require.True(t, strings.Contains(dirs[0], "AR0001"), "created directory should use first ID, got %v", dirs)
}

func writeAgentMarkerTemplates(t *testing.T, tc *TestContainer, festivalsPath string) {
	t.Helper()

	templatesDir := festivalsPath + "/.festival/templates/festival"
	_, err := tc.runCommand([]string{"mkdir", "-p", templatesDir})
	require.NoError(t, err)

	files := map[string]string{
		"GOAL.md":     "# Festival Goal\n\n[REPLACE: goal]\n",
		"OVERVIEW.md": "# Overview\n\n[REPLACE: overview]\n",
		"RULES.md":    "# Rules\n\nFollow the rules.\n",
		"TODO.md":     "# TODO\n\n- [REPLACE: todo]\n",
	}
	for name, content := range files {
		require.NoError(t, writeFileInContainer(tc, templatesDir+"/"+name, content), "write template %s", name)
	}
}
