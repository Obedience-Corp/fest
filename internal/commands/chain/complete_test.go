package chain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunComplete_ForceArchivesAnEmptyPlanningChain(t *testing.T) {
	root := filepath.Join(t.TempDir(), "festivals")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".festival"), 0o755))
	t.Chdir(root)

	cmd := newCreateCmd()
	cmd.SetArgs([]string{"--name", "release-train", "--json"})
	out := captureStdout(t, func() error { return cmd.ExecuteContext(t.Context()) })
	var created createResult
	require.NoError(t, json.Unmarshal([]byte(out), &created))
	require.Equal(t, "CH0001", created.ID)

	refused := captureStdout(t, func() error { return runComplete(t.Context(), "CH0001", false, "") })
	assert.Contains(t, refused, "Cannot complete chain CH0001: it has no festivals.")
	assert.Contains(t, refused, "Use --force to complete anyway.")
	require.FileExists(t, created.Path)

	forced := captureStdout(t, func() error { return runComplete(t.Context(), "CH0001", true, "") })
	assert.Contains(t, forced, "CHAIN COMPLETED")

	archived := workspace.JoinDungeon(root, "completed", "chains", filepath.Base(created.Path))
	assert.NoFileExists(t, created.Path)
	require.FileExists(t, archived)
	assert.Contains(t, forced, archived)

	c, err := chainpkg.Parse(t.Context(), archived)
	require.NoError(t, err)
	assert.Equal(t, chainpkg.StatusCompleted, c.Metadata.Status)
	require.Len(t, c.Metadata.StatusHistory, 2)
	assert.Equal(t, chainpkg.StatusPlanning, c.Metadata.StatusHistory[0].Status)
	last := c.Metadata.StatusHistory[1]
	assert.Equal(t, chainpkg.StatusCompleted, last.Status)
	assert.Equal(t, "Chain completed (forced from planning)", last.Notes)
	assert.False(t, last.Timestamp.IsZero())

	listed := captureStdout(t, func() error { return runList(t.Context(), "completed", true) })
	var list chainListResult
	require.NoError(t, json.Unmarshal([]byte(listed), &list))
	require.Len(t, list.Chains, 1)
	assert.Equal(t, "CH0001", list.Chains[0].ID)
}

func TestRunComplete_ForceKeepsCallerNotes(t *testing.T) {
	chainsDir := addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "G0001", "gamma")
	root := filepath.Dir(chainsDir)

	captureStdout(t, func() error { return runComplete(t.Context(), "CH0001", true, "abandoned") })

	c, err := chainpkg.Parse(t.Context(), workspace.JoinDungeon(root, "completed", "chains", "demo-CH0001.yaml"))
	require.NoError(t, err)
	assert.Equal(t, chainpkg.StatusCompleted, c.Metadata.Status)
	require.NotEmpty(t, c.Metadata.StatusHistory)
	assert.Equal(t, "abandoned", c.Metadata.StatusHistory[len(c.Metadata.StatusHistory)-1].Notes)
}

func TestRunComplete_ForceCompletesAnActiveChainOnce(t *testing.T) {
	activeYAML := strings.Replace(noWaveChainYAML, "status: planning", "status: active", 1)
	chainsDir := addTestEnv(t, activeYAML, "demo-CH0001.yaml", "G0001", "gamma")
	root := filepath.Dir(chainsDir)

	captureStdout(t, func() error { return runComplete(t.Context(), "CH0001", true, "") })

	c, err := chainpkg.Parse(t.Context(), workspace.JoinDungeon(root, "completed", "chains", "demo-CH0001.yaml"))
	require.NoError(t, err)
	assert.Equal(t, chainpkg.StatusCompleted, c.Metadata.Status)
	require.Len(t, c.Metadata.StatusHistory, 1)
	assert.Equal(t, "Chain completed", c.Metadata.StatusHistory[0].Notes)
}
