//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHarnessGitIdentity_IsContainerLocalAndDeterministic(t *testing.T) {
	tc := GetSharedContainer(t)

	out, err := tc.Exec("sh", "-c", strings.Join([]string{
		"mkdir -p /identity-check && cd /identity-check",
		"git init -q",
		"echo fixture > fixture.txt",
		"git add fixture.txt",
		"git commit -q -m fixture",
		"git log -1 --format='%an <%ae>|%cn <%ce>'",
	}, " && "))
	require.NoError(t, err, "the harness identity must support fixture commits: %s", out)
	assert.Equal(t,
		integrationGitName+" <"+integrationGitEmail+">|"+
			integrationGitName+" <"+integrationGitEmail+">",
		strings.TrimSpace(out))

	_, err = tc.Exec("sh", "-c", "test ! -e /root/.gitconfig")
	require.NoError(t, err, "the harness must not persist identity in global Git config")

	out, err = tc.Exec("git", "config", "--get", "protocol.file.allow")
	require.NoError(t, err)
	assert.Equal(t, "always", strings.TrimSpace(out),
		"fixture submodules must be allowed without persistent Git config")
}
