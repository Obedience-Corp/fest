package chain

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_Valid(t *testing.T) {
	root := t.TempDir()
	// Create festival directories matching the naming convention
	require.NoError(t, os.MkdirAll(filepath.Join(root, "hedera-foundation-HF0001"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "chain-agents-CA0001"), 0o755))

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "hf", ID: "HF0001", Name: "hedera-foundation"},
			{Ref: "ca", ID: "CA0001", Name: "chain-agents"},
		},
	}

	resolved, err := Resolve(context.Background(), c, []string{root})
	require.NoError(t, err)
	assert.Len(t, resolved, 2)

	hf := resolved["hf"]
	assert.Equal(t, "HF0001", hf.Node.ID)
	assert.Equal(t, filepath.Join(root, "hedera-foundation-HF0001"), hf.Path)

	ca := resolved["ca"]
	assert.Equal(t, "CA0001", ca.Node.ID)
	assert.Equal(t, filepath.Join(root, "chain-agents-CA0001"), ca.Path)
}

func TestResolve_MultipleSearchDirs(t *testing.T) {
	active := t.TempDir()
	planning := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(active, "alpha-A0001"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(planning, "beta-B0001"), 0o755))

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "b", ID: "B0001", Name: "beta"},
		},
	}

	resolved, err := Resolve(context.Background(), c, []string{active, planning})
	require.NoError(t, err)
	assert.Len(t, resolved, 2)
	assert.Contains(t, resolved["a"].Path, "alpha-A0001")
	assert.Contains(t, resolved["b"].Path, "beta-B0001")
}

func TestResolve_FestivalNotFound(t *testing.T) {
	root := t.TempDir()

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "x", ID: "X0001", Name: "missing"},
		},
	}

	_, err := Resolve(context.Background(), c, []string{root})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in search directories")
}

func TestResolve_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
		},
	}

	_, err := Resolve(ctx, c, []string{"/tmp"})
	assert.Error(t, err)
}

func TestResolve_ExactIDMatch(t *testing.T) {
	root := t.TempDir()
	// Directory named exactly as the ID (no name prefix)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "A0001"), 0o755))

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
		},
	}

	resolved, err := Resolve(context.Background(), c, []string{root})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "A0001"), resolved["a"].Path)
}

func TestResolve_SkipsUnreadableDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "alpha-A0001"), 0o755))

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
		},
	}

	// Include a nonexistent dir as first search path — should skip it
	resolved, err := Resolve(context.Background(), c, []string{"/nonexistent", root})
	require.NoError(t, err)
	assert.Contains(t, resolved["a"].Path, "alpha-A0001")
}

func TestMatchesFestivalID(t *testing.T) {
	tests := []struct {
		dirName    string
		festivalID string
		want       bool
	}{
		{"hedera-foundation-HF0001", "HF0001", true},
		{"chain-agents-CA0001", "CA0001", true},
		{"HF0001", "HF0001", true},
		{"some-other-dir", "HF0001", false},
		{"HF0001-extra", "HF0001", false},
		{"", "HF0001", false},
	}

	for _, tt := range tests {
		t.Run(tt.dirName, func(t *testing.T) {
			assert.Equal(t, tt.want, matchesFestivalID(tt.dirName, tt.festivalID))
		})
	}
}
