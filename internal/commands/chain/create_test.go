package chain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextChainIDUsesMaxNumericSuffix(t *testing.T) {
	chainsDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(chainsDir, "alpha-beta-CH0001.yaml"), []byte("one"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chainsDir, "alpha-beta-CH0003.yaml"), []byte("three"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chainsDir, "other-XY0007.yaml"), []byte("other"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chainsDir, "README.md"), []byte("ignore"), 0o644))

	got := nextChainID(chainsDir, chainIDPrefix)
	assert.Equal(t, "CH0004", got)
}

func TestNextChainIDIgnoresMalformedPrefixMatches(t *testing.T) {
	chainsDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(chainsDir, "CH-not-a-number.yaml"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chainsDir, "alpha-beta-CH0002.yaml"), []byte("y"), 0o644))

	got := nextChainID(chainsDir, chainIDPrefix)
	assert.Equal(t, "CH0003", got)
}

func TestChainIDPrefixUsesChainNamespace(t *testing.T) {
	assert.Equal(t, "CH", chainIDPrefix)
}

func TestCreateChainFileDoesNotOverwriteExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alpha-beta-CH0001.yaml")
	original := "original chain contents"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o644))

	f, err := createChainFile(path)
	if f != nil {
		_ = f.Close()
	}
	require.Error(t, err)
	assert.True(t, os.IsExist(err))

	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(data))
}
