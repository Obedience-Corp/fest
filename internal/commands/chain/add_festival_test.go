package chain

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFestival(t *testing.T, root, status, id, name string) string {
	t.Helper()
	dir := filepath.Join(root, status, name+"-"+id)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	cfg := "version: \"1.0\"\nproject_path: projects/" + name +
		"\nmetadata:\n  id: " + id + "\n  name: " + name + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(cfg), 0o644))
	return dir
}

func TestResolveFestivalToAdd_ByID(t *testing.T) {
	root := filepath.Join(t.TempDir(), "festivals")
	writeFestival(t, root, "active", "FA0010", "festival-app-onboarding")

	rf, err := resolveFestivalToAdd(context.Background(), root, "FA0010")
	require.NoError(t, err)
	assert.Equal(t, "FA0010", rf.ID)
	assert.Equal(t, "festival-app-onboarding", rf.Name)
	assert.Equal(t, []string{"projects/festival-app-onboarding"}, rf.Projects)
}

func TestResolveFestivalToAdd_ByName(t *testing.T) {
	root := filepath.Join(t.TempDir(), "festivals")
	writeFestival(t, root, "ready", "FA0010", "onboarding-parity")

	rf, err := resolveFestivalToAdd(context.Background(), root, "onboarding-parity")
	require.NoError(t, err)
	assert.Equal(t, "FA0010", rf.ID)
}

func TestResolveFestivalToAdd_ByPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "festivals")
	dir := writeFestival(t, root, "planning", "FA0010", "onboarding")

	rf, err := resolveFestivalToAdd(context.Background(), root, dir)
	require.NoError(t, err)
	assert.Equal(t, "FA0010", rf.ID)
	assert.Equal(t, dir, rf.Path)
}

func TestResolveFestivalToAdd_NotFound(t *testing.T) {
	root := filepath.Join(t.TempDir(), "festivals")
	require.NoError(t, os.MkdirAll(root, 0o755))
	_, err := resolveFestivalToAdd(context.Background(), root, "ZZ9999")
	assert.Error(t, err)
}

func TestResolveFestivalToAdd_Empty(t *testing.T) {
	_, err := resolveFestivalToAdd(context.Background(), t.TempDir(), "")
	assert.Error(t, err)
}

// dupIDChainYAML parses structurally (validateStructure does not check S2) but
// fails Validate with an S2 duplicate-id error, so any add leaves it invalid.
const dupIDChainYAML = `chain_version: "1.0"
metadata:
  id: CH0009
  name: broken-chain
  status: planning
festivals:
  - ref: alpha
    id: A0001
    name: alpha
  - ref: beta
    id: A0001
    name: beta
edges:
  - from: alpha
    to: beta
    type: hard
`

func TestRunAdd_InvalidMutationWritesNothing(t *testing.T) {
	chainsDir := addTestEnv(t, dupIDChainYAML, "broken-CH0009.yaml", "G0001", "gamma")
	chainPath := filepath.Join(chainsDir, "broken-CH0009.yaml")

	before, err := os.ReadFile(chainPath)
	require.NoError(t, err)

	opts := &addOptions{chain: "CH0009", festival: "G0001", after: []string{"beta"}, edgeType: "hard"}
	err = runAdd(context.Background(), opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")

	after, err := os.ReadFile(chainPath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "chain file must be unchanged on invalid mutation")
}

func TestRunAdd_ValidMutationWrites(t *testing.T) {
	chainsDir := addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "G0001", "gamma")
	chainPath := filepath.Join(chainsDir, "demo-CH0001.yaml")

	before, err := os.ReadFile(chainPath)
	require.NoError(t, err)

	opts := &addOptions{chain: "CH0001", festival: "G0001", after: []string{"beta"}, edgeType: "hard"}
	require.NoError(t, runAdd(context.Background(), opts))

	after, err := os.ReadFile(chainPath)
	require.NoError(t, err)
	assert.NotEqual(t, string(before), string(after))

	c, err := chainpkg.Parse(context.Background(), chainPath)
	require.NoError(t, err)
	assert.Len(t, c.Festivals, 3)
}
