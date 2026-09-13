//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/gif"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompletionReplay(t *testing.T) {
	for _, command := range []string{"promote", "status", "dungeon"} {
		t.Run(command, func(t *testing.T) {
			tc := GetSharedContainer(t)
			root, festival := setupCompletionReplay(t, tc)
			args := completionArgs(command)
			output, err := tc.RunFestInDir(festival, args...)
			require.NoError(t, err, output)
			var result struct {
				Success bool   `json:"success"`
				NewPath string `json:"new_path"`
				Commit  string `json:"commit"`
				Replay  struct {
					Path    string `json:"path"`
					Warning string `json:"warning"`
				} `json:"replay"`
			}
			require.NoError(t, json.Unmarshal([]byte(output), &result), output)
			require.True(t, result.Success)
			require.NotEmpty(t, result.Commit, output)
			require.Empty(t, result.Replay.Warning)
			require.Equal(t, filepath.Join(result.NewPath, "festival-replay.gif"), result.Replay.Path)
			overview, err := tc.ReadFile(result.NewPath + "/FESTIVAL_OVERVIEW.md")
			require.NoError(t, err)
			require.Contains(t, overview, "# Overview\n\nKeep my notes.\n")
			require.Contains(t, overview, "![Festival execution replay](festival-replay.gif)")
			data, err := tc.ReadFile(result.Replay.Path)
			require.NoError(t, err)
			animation, err := gif.DecodeAll(bytes.NewReader([]byte(data)))
			require.NoError(t, err)
			require.Greater(t, len(animation.Image), 1)
			// The completion commit must include the asset and the embed together.
			rel, err := filepath.Rel(root, result.NewPath)
			require.NoError(t, err)
			committed := runGit(t, tc, root, "show", "HEAD:"+rel+"/FESTIVAL_OVERVIEW.md")
			require.Equal(t, overview, committed)
			runGit(t, tc, root, "cat-file", "-e", "HEAD:"+rel+"/festival-replay.gif")
			// Manual refresh and reopening/completing preserve a single embed.
			output, err = tc.RunFestInDir(result.NewPath, "gif", "--embed", "--speed", "2")
			require.NoError(t, err, output)
			refreshed, err := tc.ReadFile(result.NewPath + "/FESTIVAL_OVERVIEW.md")
			require.NoError(t, err)
			require.Equal(t, overview, refreshed)
			output, err = tc.RunFestInDir(result.NewPath, "status", "set", "active", "--force", "--no-commit", "--json")
			require.NoError(t, err, output)
			output, err = tc.RunFestInDir(festival, "promote", "--no-commit", "--json")
			require.NoError(t, err, output)
			refreshed, err = tc.ReadFile(result.NewPath + "/FESTIVAL_OVERVIEW.md")
			require.NoError(t, err)
			require.Equal(t, overview, refreshed)
		})
	}
}

func TestCompletionReplayFailureKeepsCompletionAndJSON(t *testing.T) {
	for _, command := range []string{"promote", "status"} {
		t.Run(command, func(t *testing.T) {
			tc := GetSharedContainer(t)
			_, festival := setupCompletionReplay(t, tc)
			// An existing user asset is a reachable render failure, preserved by completion.
			require.NoError(t, tc.WriteFile(festival+"/festival-replay.gif", "user asset"))
			// Separate stderr so the stdout JSON contract is exercised on failure.
			output, err := tc.RunFestInDir(festival, append(completionArgs(command), "2>/tmp/replay-warning")...)
			require.NoError(t, err, output)
			var result struct {
				Success bool   `json:"success"`
				NewPath string `json:"new_path"`
				Replay  struct {
					Warning string `json:"warning"`
				} `json:"replay"`
			}
			require.NoError(t, json.Unmarshal([]byte(output), &result), output)
			require.True(t, result.Success)
			require.Contains(t, result.NewPath, "/completed/")
			require.Contains(t, result.Replay.Warning, "fest gif --embed")
			warning, err := tc.ReadFile("/tmp/replay-warning")
			require.NoError(t, err)
			require.Contains(t, warning, "replay filename is already in use")
			overview, err := tc.ReadFile(result.NewPath + "/FESTIVAL_OVERVIEW.md")
			require.NoError(t, err)
			require.Equal(t, "# Overview\n\nKeep my notes.\n", overview)
			asset, err := tc.ReadFile(result.NewPath + "/festival-replay.gif")
			require.NoError(t, err)
			require.Equal(t, "user asset", asset)
		})
	}
}

func TestNonCompletionDoesNotCreateReplay(t *testing.T) {
	tc := GetSharedContainer(t)
	_, festival := setupCompletionReplay(t, tc)
	output, err := tc.RunFestInDir(festival, "promote", "--dungeon", "archived", "--json")
	require.NoError(t, err, output)
	var result struct {
		NewPath string `json:"new_path"`
	}
	require.NoError(t, json.Unmarshal([]byte(output), &result))
	exists, err := tc.CheckFileExists(result.NewPath + "/festival-replay.gif")
	require.NoError(t, err)
	require.False(t, exists)
}

func completionArgs(command string) []string {
	switch command {
	case "status":
		return []string{"status", "set", "completed", "--force", "--json"}
	case "dungeon":
		return []string{"promote", "--dungeon", "completed", "--json"}
	default:
		return []string{"promote", "--json"}
	}
}

func setupCompletionReplay(t *testing.T, tc *TestContainer) (string, string) {
	t.Helper()
	ensureGit(t, tc)
	root := "/workspace/completion-" + strings.ReplaceAll(t.Name(), "/", "-")
	festival := root + "/festivals/active/replay-RP0001"
	files := map[string]string{
		root + "/festivals/.festival/.state/.workspace": fmt.Sprintf(`{"workspace":%q}`, root),
		festival + "/FESTIVAL_GOAL.md":                  "---\nfest_type: festival\nfest_id: RP0001\nfest_status: active\n---\n# Goal\n",
		festival + "/FESTIVAL_OVERVIEW.md":              "# Overview\n\nKeep my notes.\n",
		festival + "/fest.yaml":                         "version: \"1.0\"\nmetadata:\n  id: RP0001\n  name: Replay\n  status_history:\n    - status: active\n",
		festival + "/001_IMPLEMENT/01_build/01_task.md": "---\nfest_type: task\nfest_tracking: true\n---\n# Task\n",
		festival + "/.fest/progress_events.jsonl":       "{\"event\":\"started\",\"task\":\"001_IMPLEMENT/01_build/01_task.md\",\"ts\":\"2026-09-01T00:00:00Z\"}\n{\"event\":\"completed\",\"task\":\"001_IMPLEMENT/01_build/01_task.md\",\"ts\":\"2026-09-01T01:00:00Z\"}\n",
	}
	for path, content := range files {
		require.NoError(t, tc.WriteFile(path, content))
	}
	setupGitRepo(t, tc, root)
	return root, festival
}
