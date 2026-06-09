package chain

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunList_JSONShape(t *testing.T) {
	addTestEnv(t, noWaveChainYAML, "demo-chain.yaml", "A0001", "alpha")

	out := captureStdout(t, func() error {
		return runList(context.Background(), "", true)
	})

	var result chainListResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result.Chains, 1)

	c := result.Chains[0]
	assert.Equal(t, "CH0001", c.ID)
	assert.Equal(t, "demo-chain", c.Name)
	assert.Equal(t, "planning", c.Status)
	assert.Equal(t, 2, c.FestivalCount)
	assert.Equal(t, []string{"alpha", "beta"}, c.Refs)
}

func TestRunList_JSONEmptyWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "festivals")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".festival"), 0o755))
	t.Chdir(root)

	out := captureStdout(t, func() error {
		return runList(context.Background(), "", true)
	})

	var result chainListResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Empty(t, result.Chains)
}

func TestRunList_JSONStatusFilter(t *testing.T) {
	addTestEnv(t, noWaveChainYAML, "demo-chain.yaml", "A0001", "alpha")

	out := captureStdout(t, func() error {
		return runList(context.Background(), "active", true)
	})

	var result chainListResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Empty(t, result.Chains)
}

func TestRunList_HumanStatusFilterEmptyPrintsNoMessage(t *testing.T) {
	addTestEnv(t, noWaveChainYAML, "demo-chain.yaml", "A0001", "alpha")

	out := captureStdout(t, func() error {
		return runList(context.Background(), "active", false)
	})

	assert.NotContains(t, out, "No chains found")
}

func TestRunList_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.Error(t, runList(ctx, "", true))
}
