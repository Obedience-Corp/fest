package chain

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const malformedChainYAML = `
chain_version: "1.0"
metadata:
  id: BAD0001
  name: bad
  created_at: 2026-01-01T00:00:00Z
  status: planning
festivals:
  - ref: a
    id: A0001
    name: alpha
edges:
  - from: a
    to: a
    type: hard
  oops: [
`

func TestDiscoverAllStrictReturnsParseErrors(t *testing.T) {
	root := t.TempDir()
	chainsDir := filepath.Join(root, "chains")
	require.NoError(t, os.MkdirAll(chainsDir, 0o755))

	validPath := filepath.Join(chainsDir, "valid-EC0001.yaml")
	badPath := filepath.Join(chainsDir, "bad-BAD0001.yaml")
	require.NoError(t, os.WriteFile(validPath, []byte(validChainYAML), 0o644))
	require.NoError(t, os.WriteFile(badPath, []byte(malformedChainYAML), 0o644))

	chains, parseErrors, err := DiscoverAllStrict(context.Background(), root)
	require.NoError(t, err)
	require.Len(t, chains, 1)
	require.Len(t, parseErrors, 1)

	assert.Equal(t, badPath, parseErrors[0].File)
	assert.Contains(t, parseErrors[0].Error(), filepath.Base(badPath))
	assert.Contains(t, parseErrors[0].Err.Error(), "parsing chain YAML")
}

func TestDiscoverAllSilentlySkipsParseErrors(t *testing.T) {
	root := t.TempDir()
	chainsDir := filepath.Join(root, "chains")
	require.NoError(t, os.MkdirAll(chainsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(chainsDir, "valid-EC0001.yaml"), []byte(validChainYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chainsDir, "bad-BAD0001.yaml"), []byte(malformedChainYAML), 0o644))

	chains, err := DiscoverAll(context.Background(), root)
	require.NoError(t, err)
	require.Len(t, chains, 1)
}
