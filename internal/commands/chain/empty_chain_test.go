package chain

import (
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const emptyChainYAML = `chain_version: "1.0"
metadata:
  id: CH0003
  name: empty-chain
  status: planning
festivals: []
edges: []
waves: []
`

const emptyChainFile = "empty-chain-CH0003.yaml"

func addFestivalDir(t *testing.T, chainsDir, festivalID, festivalName string) {
	t.Helper()
	festDir := filepath.Join(filepath.Dir(chainsDir), "planning", festivalName+"-"+festivalID)
	require.NoError(t, os.MkdirAll(festDir, 0o755))
	cfg := "version: \"1.0\"\nmetadata:\n  id: " + festivalID + "\n  name: " + festivalName + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(cfg), 0o644))
}

func TestEmptyChain_ReadCommands(t *testing.T) {
	const hint = "Chain CH0003 has no festivals yet. Add one with 'fest chain add --chain CH0003 --festival <id>'."
	tests := []struct {
		name    string
		run     func(t *testing.T) error
		want    []string
		notWant []string
	}{
		{
			name: "validate is valid with an S10 warning",
			run:  func(t *testing.T) error { return runValidate(t.Context(), "CH0003") },
			want: []string{
				"All structural checks passed",
				"! S10: chain has no festivals yet",
				"add one with 'fest chain add --chain CH0003 --festival <id>'",
				"Result: VALID (score: 95/100)",
			},
		},
		{
			name:    "status points at chain add",
			run:     func(t *testing.T) error { return runStatus(t.Context(), "CH0003") },
			want:    []string{"Chain: empty-chain (CH0003)", hint},
			notWant: []string{"Progress:"},
		},
		{
			name:    "graph points at chain add",
			run:     func(t *testing.T) error { return runGraph(t.Context(), "CH0003", false, false) },
			want:    []string{hint},
			notWant: []string{"topological order"},
		},
		{
			name: "complete refuses without force",
			run:  func(t *testing.T) error { return runComplete(t.Context(), "CH0003", false, "") },
			want: []string{"Cannot complete chain CH0003: it has no festivals.", "Use --force to complete anyway."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chainsDir := addTestEnv(t, emptyChainYAML, emptyChainFile, "A0001", "alpha")

			out := captureStdout(t, func() error { return tt.run(t) })

			for _, want := range tt.want {
				assert.Contains(t, out, want)
			}
			for _, notWant := range tt.notWant {
				assert.NotContains(t, out, notWant)
			}
			assert.FileExists(t, filepath.Join(chainsDir, emptyChainFile))
		})
	}
}

func TestEmptyChain_ListReportsZeroFestivals(t *testing.T) {
	addTestEnv(t, emptyChainYAML, emptyChainFile, "A0001", "alpha")

	out := captureStdout(t, func() error { return runList(t.Context(), "", true) })

	var result chainListResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result.Chains, 1)
	assert.Equal(t, "CH0003", result.Chains[0].ID)
	assert.Equal(t, 0, result.Chains[0].FestivalCount)
	assert.Equal(t, []string{}, result.Chains[0].Refs)
	assert.Contains(t, out, `"refs": []`)
}

func TestEmptyChain_CheckNamesTheEmptyChain(t *testing.T) {
	addTestEnv(t, emptyChainYAML, emptyChainFile, "A0001", "alpha")

	err := runCheck(t.Context(), "A0001", "CH0003")

	require.Error(t, err)
	var festErr *festerrors.Error
	require.True(t, stderrors.As(err, &festErr))
	assert.Equal(t, "Chain CH0003 has no festivals yet. Add one with 'fest chain add --chain CH0003 --festival <id>'.", festErr.Hint)
}

func TestEmptyChain_CheckNamesTheChainWhenFestivalIsMissing(t *testing.T) {
	addTestEnv(t, noWaveChainYAML, "demo-CH0001.yaml", "G0001", "gamma")

	err := runCheck(t.Context(), "G0001", "CH0001")

	require.Error(t, err)
	var festErr *festerrors.Error
	require.True(t, stderrors.As(err, &festErr))
	assert.Equal(t, "G0001 is not in chain CH0001; run 'fest chain status CH0001' to see its festivals", festErr.Hint)
}

func TestEmptyChain_AddFirstMemberOpensWaveOne(t *testing.T) {
	chainsDir := addTestEnv(t, emptyChainYAML, emptyChainFile, "A0001", "alpha")
	addFestivalDir(t, chainsDir, "B0001", "beta")
	path := filepath.Join(chainsDir, emptyChainFile)

	require.NoError(t, runAdd(t.Context(), &addOptions{chain: "CH0003", festival: "A0001", edgeType: "hard"}))

	c, err := chainpkg.Parse(t.Context(), path)
	require.NoError(t, err)
	require.Len(t, c.Festivals, 1)
	assert.Equal(t, "alpha", c.Festivals[0].Ref)
	assert.Empty(t, c.Edges)
	assert.Equal(t, []chainpkg.Wave{
		{ID: 1, Name: "Wave 1", Unlock: "none", Festivals: []string{"alpha"}},
	}, c.Waves)
	assert.True(t, chainpkg.Validate(t.Context(), c).Valid)

	require.NoError(t, runAdd(t.Context(), &addOptions{
		chain:    "CH0003",
		festival: "B0001",
		after:    []string{"alpha"},
		edgeType: "hard",
	}))

	c, err = chainpkg.Parse(t.Context(), path)
	require.NoError(t, err)
	require.Len(t, c.Festivals, 2)
	assert.Equal(t, []chainpkg.Edge{{From: "alpha", To: "beta", Type: chainpkg.EdgeHard}}, c.Edges)
	require.Len(t, c.Waves, 2)
	assert.Equal(t, chainpkg.Wave{ID: 2, Name: "Wave 2", Unlock: "alpha:completed", Festivals: []string{"beta"}}, c.Waves[1])
	result := chainpkg.Validate(t.Context(), c)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Warnings)
}
