package chain

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validChainYAML = `
chain_version: "1.0"

metadata:
  id: EC0001
  name: ethdenver-agents
  goal: "Build 3-agent autonomous economy"
  created_at: 2026-02-18T20:00:00Z
  status: active
  status_history:
    - status: planning
      timestamp: 2026-02-18T20:00:00Z
      notes: "Chain created"
    - status: active
      timestamp: 2026-02-20T10:00:00Z

festivals:
  - ref: hf
    id: HF0001
    name: hedera-foundation
    projects: [agent-coordinator]
  - ref: ca
    id: CA0001
    name: chain-agents
    projects: [agent-inference, agent-defi]
  - ref: sp
    id: SP0001
    name: submission-and-polish

edges:
  - from: hf
    to: ca
    type: hard
    note: "Agents need HCS topics"
  - from: ca
    to: sp
    type: hard
  - from: hf
    to: sp
    type: soft

waves:
  - id: 1
    name: "Foundation"
    unlock: "none"
    festivals: [hf]
  - id: 2
    name: "Agents"
    unlock: "hf:completed"
    festivals: [ca]
  - id: 3
    name: "Final"
    unlock: "ca:completed"
    festivals: [sp]
`

func TestParse_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-chain.yaml")
	require.NoError(t, os.WriteFile(path, []byte(validChainYAML), 0o644))

	c, err := Parse(context.Background(), path)
	require.NoError(t, err)

	assert.Equal(t, "1.0", c.ChainVersion)
	assert.Equal(t, "EC0001", c.Metadata.ID)
	assert.Equal(t, "ethdenver-agents", c.Metadata.Name)
	assert.Equal(t, StatusActive, c.Metadata.Status)
	assert.Len(t, c.Metadata.StatusHistory, 2)
	assert.Len(t, c.Festivals, 3)
	assert.Len(t, c.Edges, 3)
	assert.Len(t, c.Waves, 3)

	// Verify festival nodes
	hf := c.FestivalByRef("hf")
	require.NotNil(t, hf)
	assert.Equal(t, "HF0001", hf.ID)
	assert.Equal(t, []string{"agent-coordinator"}, hf.Projects)

	// Verify edge types
	assert.Equal(t, EdgeHard, c.Edges[0].Type)
	assert.Equal(t, EdgeSoft, c.Edges[2].Type)
}

func TestParseBytes_Valid(t *testing.T) {
	c, err := ParseBytes(context.Background(), []byte(validChainYAML))
	require.NoError(t, err)
	assert.Equal(t, "EC0001", c.Metadata.ID)
}

func TestParse_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{{invalid"), 0o644))

	_, err := Parse(context.Background(), path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing chain YAML")
}

func TestParse_MissingFile(t *testing.T) {
	_, err := Parse(context.Background(), "/nonexistent/chain.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading chain file")
}

func TestParse_MissingChainVersion(t *testing.T) {
	yaml := `
metadata:
  id: X0001
  name: test
festivals:
  - ref: a
    id: A0001
    name: alpha
edges: []
`
	_, err := ParseBytes(context.Background(), []byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chain_version is required")
}

func TestParse_MissingMetadataID(t *testing.T) {
	yaml := `
chain_version: "1.0"
metadata:
  name: test
festivals:
  - ref: a
    id: A0001
    name: alpha
edges: []
`
	_, err := ParseBytes(context.Background(), []byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metadata.id is required")
}

func TestParse_MissingMetadataName(t *testing.T) {
	yaml := `
chain_version: "1.0"
metadata:
  id: X0001
festivals:
  - ref: a
    id: A0001
    name: alpha
edges: []
`
	_, err := ParseBytes(context.Background(), []byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metadata.name is required")
}

func TestParse_NoFestivals(t *testing.T) {
	yaml := `
chain_version: "1.0"
metadata:
  id: X0001
  name: test
festivals: []
edges: []
`
	_, err := ParseBytes(context.Background(), []byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one festival is required")
}

func TestParse_EdgeUnknownRef(t *testing.T) {
	yaml := `
chain_version: "1.0"
metadata:
  id: X0001
  name: test
festivals:
  - ref: a
    id: A0001
    name: alpha
edges:
  - from: a
    to: b
    type: hard
`
	_, err := ParseBytes(context.Background(), []byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown festival ref")
}

func TestParse_EdgeInvalidType(t *testing.T) {
	yaml := `
chain_version: "1.0"
metadata:
  id: X0001
  name: test
festivals:
  - ref: a
    id: A0001
    name: alpha
  - ref: b
    id: B0001
    name: beta
edges:
  - from: a
    to: b
    type: unknown
`
	_, err := ParseBytes(context.Background(), []byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be")
}

func TestParse_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ParseBytes(ctx, []byte(validChainYAML))
	assert.Error(t, err)
}

func TestParse_FestivalMissingRef(t *testing.T) {
	yaml := `
chain_version: "1.0"
metadata:
  id: X0001
  name: test
festivals:
  - id: A0001
    name: alpha
edges: []
`
	_, err := ParseBytes(context.Background(), []byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "festival ref is required")
}

func TestParse_FestivalMissingID(t *testing.T) {
	yaml := `
chain_version: "1.0"
metadata:
  id: X0001
  name: test
festivals:
  - ref: a
    name: alpha
edges: []
`
	_, err := ParseBytes(context.Background(), []byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "festival id is required")
}

func TestChain_RefSet(t *testing.T) {
	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "a"}, {Ref: "b"}, {Ref: "c"},
		},
	}
	refs := c.RefSet()
	assert.True(t, refs["a"])
	assert.True(t, refs["b"])
	assert.True(t, refs["c"])
	assert.False(t, refs["d"])
}

func TestChain_FestivalByRef(t *testing.T) {
	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001"},
			{Ref: "b", ID: "B0001"},
		},
	}
	assert.Equal(t, "A0001", c.FestivalByRef("a").ID)
	assert.Nil(t, c.FestivalByRef("missing"))
}

func TestParse_SequenceHints(t *testing.T) {
	yaml := `
chain_version: "1.0"
metadata:
  id: X0001
  name: test
festivals:
  - ref: a
    id: A0001
    name: alpha
edges: []
sequence_hints:
  - festival: a
    early_start:
      sequences: [01_data, 02_ui]
      condition: "Can start with mocks"
    needs_upstream:
      sequences: [03_integration]
    internal_parallelism:
      parallel: [01_data, 02_ui]
      then: [03_integration]
`
	c, err := ParseBytes(context.Background(), []byte(yaml))
	require.NoError(t, err)
	require.Len(t, c.SequenceHints, 1)

	hint := c.SequenceHints[0]
	assert.Equal(t, "a", hint.Festival)
	require.NotNil(t, hint.EarlyStart)
	assert.Equal(t, []string{"01_data", "02_ui"}, hint.EarlyStart.Sequences)
	require.NotNil(t, hint.NeedsUpstream)
	assert.Equal(t, []string{"03_integration"}, hint.NeedsUpstream.Sequences)
	require.NotNil(t, hint.InternalParallelism)
	assert.Equal(t, []string{"01_data", "02_ui"}, hint.InternalParallelism.Parallel)
}
