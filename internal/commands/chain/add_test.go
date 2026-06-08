package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const noWaveChainYAML = `chain_version: "1.0"
metadata:
  id: CH0001
  name: demo-chain
  status: planning
festivals:
  - ref: alpha
    id: A0001
    name: alpha
  - ref: beta
    id: B0001
    name: beta
edges:
  - from: alpha
    to: beta
    type: hard
`

const wavedChainYAML = `chain_version: "1.0"
metadata:
  id: CH0002
  name: waved-chain
  status: planning
festivals:
  - ref: alpha
    id: A0001
    name: alpha
  - ref: beta
    id: B0001
    name: beta
edges:
  - from: alpha
    to: beta
    type: hard
waves:
  - id: 1
    name: Foundation
    unlock: none
    festivals: [alpha]
  - id: 2
    name: Wave 2
    unlock: alpha:completed
    festivals: [beta]
`

// addTestEnv sets up a festivals root with chains/ and a festival dir, chdirs
// into it, and returns the chains directory path.
func addTestEnv(t *testing.T, chainYAML, chainFile, festivalID, festivalName string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "festivals")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".festival"), 0o755))
	chainsDir := filepath.Join(root, "chains")
	require.NoError(t, os.MkdirAll(chainsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(chainsDir, chainFile), []byte(chainYAML), 0o644))

	festDir := filepath.Join(root, "planning", festivalName+"-"+festivalID)
	require.NoError(t, os.MkdirAll(festDir, 0o755))
	cfg := "version: \"1.0\"\nmetadata:\n  id: " + festivalID +
		"\n  name: " + festivalName + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(cfg), 0o644))

	t.Chdir(root)
	return chainsDir
}

func TestRunAdd_NoWavesWithEdge(t *testing.T) {
	chainsDir := addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "G0001", "gamma")

	opts := &addOptions{
		chain:    "CH0001",
		festival: "G0001",
		after:    []string{"beta"},
		edgeType: "hard",
	}
	require.NoError(t, runAdd(context.Background(), opts))

	c, err := chainpkg.Parse(context.Background(), filepath.Join(chainsDir, "demo-CH0001.yaml"))
	require.NoError(t, err)
	require.Len(t, c.Festivals, 3)

	node := c.FestivalByRef("gamma")
	require.NotNil(t, node)
	assert.Equal(t, "G0001", node.ID)

	var found bool
	for _, e := range c.Edges {
		if e.From == "beta" && e.To == "gamma" && e.Type == chainpkg.EdgeHard {
			found = true
		}
	}
	assert.True(t, found, "expected beta->gamma hard edge")
	assert.Empty(t, c.Waves)
}

func TestRunAdd_RepeatableAfterCreatesMultipleEdges(t *testing.T) {
	chainsDir := addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "G0001", "gamma")

	opts := &addOptions{
		chain:    "CH0001",
		festival: "G0001",
		after:    []string{"alpha", "beta"},
		edgeType: "soft",
	}
	require.NoError(t, runAdd(context.Background(), opts))

	c, err := chainpkg.Parse(context.Background(), filepath.Join(chainsDir, "demo-CH0001.yaml"))
	require.NoError(t, err)

	var toGamma int
	for _, e := range c.Edges {
		if e.To == "gamma" {
			toGamma++
			assert.Equal(t, chainpkg.EdgeSoft, e.Type)
		}
	}
	assert.Equal(t, 2, toGamma)
}

func TestRunAdd_RootNodeNoAfter(t *testing.T) {
	chainsDir := addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "G0001", "gamma")

	opts := &addOptions{chain: "CH0001", festival: "G0001", edgeType: "hard"}
	require.NoError(t, runAdd(context.Background(), opts))

	c, err := chainpkg.Parse(context.Background(), filepath.Join(chainsDir, "demo-CH0001.yaml"))
	require.NoError(t, err)
	require.Len(t, c.Festivals, 3)
	for _, e := range c.Edges {
		assert.NotEqual(t, "gamma", e.To)
	}
}

func TestRunAdd_HasWavesAppendsTrailingWave(t *testing.T) {
	chainsDir := addTestEnv(t, wavedChainYAML, "waved-CH0002.yaml", "G0001", "gamma")

	opts := &addOptions{
		chain:    "CH0002",
		festival: "G0001",
		after:    []string{"beta"},
		edgeType: "hard",
	}
	require.NoError(t, runAdd(context.Background(), opts))

	c, err := chainpkg.Parse(context.Background(), filepath.Join(chainsDir, "waved-CH0002.yaml"))
	require.NoError(t, err)
	require.Len(t, c.Waves, 3)
	last := c.Waves[2]
	assert.Equal(t, 3, last.ID)
	assert.Equal(t, []string{"gamma"}, last.Festivals)
	assert.Equal(t, "beta:completed", last.Unlock)

	result := chainpkg.Validate(context.Background(), c)
	assert.True(t, result.Valid, "mutated waved chain must validate: %+v", result.Errors)
}

func TestRunAdd_DuplicateFestivalRejected(t *testing.T) {
	chainsDir := addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "A0001", "alpha")

	opts := &addOptions{chain: "CH0001", festival: "A0001", edgeType: "hard"}
	err := runAdd(context.Background(), opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in chain")

	c, err := chainpkg.Parse(context.Background(), filepath.Join(chainsDir, "demo-CH0001.yaml"))
	require.NoError(t, err)
	assert.Len(t, c.Festivals, 2, "no node should be written on duplicate")
}

func TestRunAdd_SelfEdgeImpossibleButPrereqUnknownRejected(t *testing.T) {
	chainsDir := addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "G0001", "gamma")

	opts := &addOptions{chain: "CH0001", festival: "G0001", after: []string{"nonexistent"}, edgeType: "hard"}
	err := runAdd(context.Background(), opts)
	require.Error(t, err)

	c, err := chainpkg.Parse(context.Background(), filepath.Join(chainsDir, "demo-CH0001.yaml"))
	require.NoError(t, err)
	assert.Len(t, c.Festivals, 2, "no write on unknown prerequisite")
}

func TestRunAdd_InvalidTypeRejected(t *testing.T) {
	addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "G0001", "gamma")
	opts := &addOptions{chain: "CH0001", festival: "G0001", edgeType: "medium"}
	err := runAdd(context.Background(), opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "edge type")
}

func TestRunAdd_ContextCancelled(t *testing.T) {
	addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "G0001", "gamma")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts := &addOptions{chain: "CH0001", festival: "G0001", edgeType: "hard"}
	err := runAdd(ctx, opts)
	assert.Error(t, err)
}

func TestRunAdd_FestivalByName(t *testing.T) {
	chainsDir := addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "G0001", "gamma")

	opts := &addOptions{chain: "CH0001", festival: "gamma", after: []string{"beta"}, edgeType: "hard"}
	require.NoError(t, runAdd(context.Background(), opts))

	c, err := chainpkg.Parse(context.Background(), filepath.Join(chainsDir, "demo-CH0001.yaml"))
	require.NoError(t, err)
	assert.NotNil(t, c.FestivalByRef("gamma"))
}

func TestRunAdd_JSONOutput(t *testing.T) {
	addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "G0001", "gamma")

	opts := &addOptions{
		chain:    "CH0001",
		festival: "G0001",
		after:    []string{"beta"},
		edgeType: "hard",
		note:     "needs beta",
		json:     true,
	}

	out := captureStdout(t, func() error {
		return runAdd(context.Background(), opts)
	})

	var result addResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "CH0001", result.ChainID)
	assert.Equal(t, "demo-chain", result.ChainName)
	assert.Equal(t, "gamma", result.Node.Ref)
	assert.Equal(t, "G0001", result.Node.ID)
	require.Len(t, result.Edges, 1)
	assert.Equal(t, "beta", result.Edges[0].From)
	assert.Equal(t, "gamma", result.Edges[0].To)
	assert.Equal(t, "hard", result.Edges[0].Type)
	assert.Equal(t, "needs beta", result.Edges[0].Note)
	assert.True(t, result.Validation.Valid)
	assert.Equal(t, 100, result.Validation.Score)
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	old := os.Stdout
	t.Cleanup(func() { os.Stdout = old })
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	runErr := fn()

	require.NoError(t, w.Close())
	os.Stdout = old
	require.NoError(t, runErr)

	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)
	return buf.String()
}
