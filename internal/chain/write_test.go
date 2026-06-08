package chain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.yaml")
	require.NoError(t, os.WriteFile(src, []byte(validChainYAML), 0o644))

	original, err := Parse(context.Background(), src)
	require.NoError(t, err)

	out := filepath.Join(dir, "out.yaml")
	require.NoError(t, Write(context.Background(), out, original))

	reparsed, err := Parse(context.Background(), out)
	require.NoError(t, err)

	assert.Equal(t, original.ChainVersion, reparsed.ChainVersion)
	assert.Equal(t, original.Metadata.ID, reparsed.Metadata.ID)
	assert.Equal(t, original.Metadata.Name, reparsed.Metadata.Name)
	assert.Equal(t, original.Metadata.Status, reparsed.Metadata.Status)
	assert.Len(t, reparsed.Metadata.StatusHistory, len(original.Metadata.StatusHistory))
	assert.Equal(t, original.Festivals, reparsed.Festivals)
	assert.Equal(t, original.Edges, reparsed.Edges)
	assert.Equal(t, original.Waves, reparsed.Waves)
}

func TestWrite_DropsTemplateComments(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.yaml")
	commented := "# explanatory header comment\n" + validChainYAML
	require.NoError(t, os.WriteFile(src, []byte(commented), 0o644))

	c, err := Parse(context.Background(), src)
	require.NoError(t, err)

	out := filepath.Join(dir, "out.yaml")
	require.NoError(t, Write(context.Background(), out, c))

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "explanatory header comment")
}

func TestWrite_AtomicNoTempLeftBehind(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.yaml")

	c, err := ParseBytes(context.Background(), []byte(validChainYAML))
	require.NoError(t, err)
	require.NoError(t, Write(context.Background(), out, c))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
		assert.False(t, strings.HasSuffix(e.Name(), ".tmp"),
			"temp file left behind: %s", e.Name())
	}
	assert.Equal(t, []string{"out.yaml"}, names)
}

func TestWrite_OverwriteDoesNotCorruptOnReParse(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.yaml")
	require.NoError(t, os.WriteFile(out, []byte("stale: contents\n"), 0o644))

	c, err := ParseBytes(context.Background(), []byte(validChainYAML))
	require.NoError(t, err)
	require.NoError(t, Write(context.Background(), out, c))

	reparsed, err := Parse(context.Background(), out)
	require.NoError(t, err)
	assert.Equal(t, "EC0001", reparsed.Metadata.ID)
}

func TestWrite_NilChain(t *testing.T) {
	err := Write(context.Background(), filepath.Join(t.TempDir(), "x.yaml"), nil)
	assert.Error(t, err)
}

func TestWrite_BadDir(t *testing.T) {
	c, err := ParseBytes(context.Background(), []byte(validChainYAML))
	require.NoError(t, err)
	err = Write(context.Background(), "/nonexistent-dir-xyz/out.yaml", c)
	assert.Error(t, err)
}
