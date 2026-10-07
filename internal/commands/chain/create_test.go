package chain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
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

func TestRenderChainTemplateProducesAnEmptyValidChain(t *testing.T) {
	rendered, err := renderChainTemplate(chainTemplateData{
		ID:        "CH0007",
		Name:      "release-train",
		Goal:      "ship v1",
		CreatedAt: "2026-10-07T00:00:00Z",
	})
	require.NoError(t, err)

	c, err := chainpkg.ParseBytes(t.Context(), rendered)
	require.NoError(t, err)
	assert.Equal(t, "CH0007", c.Metadata.ID)
	assert.Equal(t, "release-train", c.Metadata.Name)
	assert.Equal(t, "ship v1", c.Metadata.Goal)
	assert.Equal(t, chainpkg.StatusPlanning, c.Metadata.Status)
	assert.Empty(t, c.Festivals)
	assert.Empty(t, c.Edges)
	assert.Empty(t, c.Waves)

	result := chainpkg.Validate(t.Context(), c)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "S10", result.Warnings[0].Code)

	assert.Contains(t, string(rendered), "#   - ref: example1")
	for line := range strings.SplitSeq(string(rendered), "\n") {
		if strings.Contains(line, "EX000") || strings.Contains(line, "example1") || strings.Contains(line, "example2") {
			assert.True(t, strings.HasPrefix(strings.TrimSpace(line), "#"), "example must stay commented out: %q", line)
		}
	}
}

func TestRunCreate_JSONReportsTheNewEmptyChain(t *testing.T) {
	root := filepath.Join(t.TempDir(), "festivals")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".festival"), 0o755))
	t.Chdir(root)

	cmd := newCreateCmd()
	cmd.SetArgs([]string{"--name", "Release Train", "--goal", "ship v1", "--json"})
	out := captureStdout(t, func() error { return cmd.ExecuteContext(t.Context()) })

	var result createResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "CH0001", result.ID)
	assert.Equal(t, "release-train", result.Name)
	assert.Equal(t, "planning", result.Status)
	assert.Equal(t, "ship v1", result.Goal)
	assert.Equal(t, "release-train-CH0001.yaml", filepath.Base(result.Path))
	assert.NotContains(t, out, "CHAIN CREATED")

	c, err := chainpkg.Parse(t.Context(), result.Path)
	require.NoError(t, err)
	assert.Equal(t, "CH0001", c.Metadata.ID)
	assert.Empty(t, c.Festivals)
}

func TestRunCreate_JSONOmitsBlankGoal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "festivals")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".festival"), 0o755))
	t.Chdir(root)

	cmd := newCreateCmd()
	cmd.SetArgs([]string{"--name", "docs", "--json"})
	out := captureStdout(t, func() error { return cmd.ExecuteContext(t.Context()) })

	assert.NotContains(t, out, `"goal"`)
}

func TestRenderChainTemplate_EscapesNameAndGoal(t *testing.T) {
	tests := []struct {
		name string
		goal string
	}{
		{name: `quote"me`, goal: `say "hi" \ and: # x`},
		{name: `back\slash`, goal: "first line\nsecond line"},
		{name: "colon: #hash", goal: "- starts like a list item"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := renderChainTemplate(chainTemplateData{
				ID:        "CH0007",
				Name:      tt.name,
				Goal:      tt.goal,
				CreatedAt: "2026-10-07T00:00:00Z",
			})
			require.NoError(t, err)

			c, err := chainpkg.ParseBytes(t.Context(), rendered)
			require.NoError(t, err)
			assert.Equal(t, tt.name, c.Metadata.Name)
			assert.Equal(t, tt.goal, c.Metadata.Goal)
		})
	}
}

func TestRunCreate_EscapedNameAndGoalAreListedAndValid(t *testing.T) {
	root := filepath.Join(t.TempDir(), "festivals")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".festival"), 0o755))
	t.Chdir(root)
	const goal = `say "hi" \ and: # x`

	cmd := newCreateCmd()
	cmd.SetArgs([]string{"--name", `Quote"Me Train`, "--goal", goal, "--json"})
	out := captureStdout(t, func() error { return cmd.ExecuteContext(t.Context()) })

	var created createResult
	require.NoError(t, json.Unmarshal([]byte(out), &created))
	assert.Equal(t, `quote"me-train`, created.Name)
	assert.Equal(t, goal, created.Goal)

	c, err := chainpkg.Parse(t.Context(), created.Path)
	require.NoError(t, err)
	assert.Equal(t, `quote"me-train`, c.Metadata.Name)
	assert.Equal(t, goal, c.Metadata.Goal)

	listed := captureStdout(t, func() error { return runList(t.Context(), "", true) })
	var list chainListResult
	require.NoError(t, json.Unmarshal([]byte(listed), &list))
	require.Len(t, list.Chains, 1)
	assert.Equal(t, created.ID, list.Chains[0].ID)
	assert.Equal(t, `quote"me-train`, list.Chains[0].Name)

	validated := captureStdout(t, func() error { return runValidate(t.Context(), created.ID) })
	assert.Contains(t, validated, "Result: VALID")
}
