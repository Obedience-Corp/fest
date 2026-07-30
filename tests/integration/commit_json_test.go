//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Criterion 15a: `fest commit --json` emits exactly one JSON document on
// stdout and nothing else. Before the commitkit swap, executeGitCommit wired
// git's own stdout to the process stdout while --json encoded to the same
// stream, so scripts parsing the output got git's summary interleaved with
// the document (intent fest-commit-json-interleaves-git-20260726-001838).
//
// The streams are separated by shell redirection inside the container,
// because the harness's combined capture cannot distinguish them.
func TestCommitJSON_StdoutIsExactlyOneJSONDocument(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)

	dir := "/guard-json"
	_, err := tc.Exec("sh", "-c",
		"mkdir -p "+dir+"/festivals/.festival && cd "+dir+" && git init -q")
	require.NoError(t, err)
	require.NoError(t, tc.WriteFile(dir+"/festivals/active/guard-festival/fest.yaml", guardFestYAML))
	_, err = tc.Exec("sh", "-c", "echo change > "+dir+"/notes.md")
	require.NoError(t, err)

	_, err = tc.Exec("sh", "-c",
		"cd "+dir+"/festivals/active/guard-festival && /fest commit --json --no-tag -m 'json purity' > /tmp/json-stdout 2> /tmp/json-stderr")
	require.NoError(t, err, "fest commit --json must exit zero")

	stdout, err := tc.ReadFile("/tmp/json-stdout")
	require.NoError(t, err)

	// Exactly one document: a decoder consumes it, and nothing but whitespace
	// may follow. Git's "1 file changed..." summary before or after the JSON
	// fails both checks.
	dec := json.NewDecoder(strings.NewReader(stdout))
	var doc map[string]any
	require.NoError(t, dec.Decode(&doc),
		"stdout must begin with a JSON document, got:\n%s", stdout)
	rest := new(strings.Builder)
	if dec.More() {
		t.Fatalf("stdout carries more than one JSON value:\n%s", stdout)
	}
	_, _ = rest.WriteString(stdout[int(dec.InputOffset()):])
	assert.Empty(t, strings.TrimSpace(rest.String()),
		"nothing but the JSON document may reach stdout")

	assert.Equal(t, true, doc["success"], "the commit must report success: %v", doc)
	hash, _ := doc["hash"].(string)
	assert.NotEmpty(t, hash, "the document must carry the commit hash")

	// The hash is real: the commit exists at return.
	_, err = tc.Exec("git", "-C", dir, "cat-file", "-e", hash+"^{commit}")
	assert.NoError(t, err, "the reported hash must name a commit that exists")
}
