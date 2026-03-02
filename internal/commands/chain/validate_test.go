package chain

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validCrossValidateChainYAML = `
chain_version: "1.0"
metadata:
  id: CH0001
  name: valid-chain
  created_at: 2026-01-01T00:00:00Z
  status: planning
  status_history: []
festivals:
  - ref: a
    id: A0001
    name: alpha
edges: []
`

const malformedCrossValidateChainYAML = `
chain_version: "1.0"
metadata:
  id: CH0002
  name: malformed-chain
  created_at: 2026-01-01T00:00:00Z
  status: planning
festivals:
  - ref: b
    id: B0001
    name: beta
edges:
  - from: b
    to: b
    type: hard
  broken: [
`

func TestRunCrossValidateReportsParseErrors(t *testing.T) {
	festivalsRoot := filepath.Join(t.TempDir(), "festivals")
	chainsDir := filepath.Join(festivalsRoot, "chains")
	require.NoError(t, os.MkdirAll(filepath.Join(festivalsRoot, ".festival"), 0o755))
	require.NoError(t, os.MkdirAll(chainsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(chainsDir, "valid-CH0001.yaml"), []byte(validCrossValidateChainYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chainsDir, "bad-CH0002.yaml"), []byte(malformedCrossValidateChainYAML), 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(festivalsRoot))
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	output, runErr := captureCrossValidateOutput(t)
	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "failed to parse")
	assert.Contains(t, output, "Parse errors:")
	assert.Contains(t, output, "bad-CH0002.yaml")
}

func captureCrossValidateOutput(t *testing.T) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	runErr := runCrossValidate(context.Background())

	require.NoError(t, w.Close())
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	return buf.String(), runErr
}
