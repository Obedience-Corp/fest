//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A campaign fest has never navigated in has neither .campaign/fest nor
// festivals/.festival/.state. fest's scoped staging listed both regardless,
// and git refuses an entire `git add` over one pathspec it cannot match, so
// the very first `fest commit` in a new campaign died on
// "pathspec '.campaign/fest' did not match any files". That is the shipped
// bundle's first-run path: the failure met new users before anything else did.
//
// The campaign here is built by the released camp binary fest links against,
// so the fixture is the first run a real user gets, not an approximation of it.
func TestCommitFreshCampaign_FirstCommitSucceedsWithoutFestStateDirs(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)
	ensureCampInContainer(t, tc)

	out, err := tc.Exec("sh", "-c",
		"export HOME=/root && cd / && camp init fresh-campaign -d 'fresh campaign fixture' -m 'prove the first commit'")
	require.NoError(t, err, "camp init should succeed: %s", out)
	camp := "/fresh-campaign"

	// The whole test rests on camp init really leaving both fest-owned state
	// directories unborn, so assert that rather than assume it.
	for _, absent := range []string{camp + "/.campaign/fest", camp + "/festivals/.festival/.state"} {
		exists, existErr := tc.CheckDirExists(absent)
		require.NoError(t, existErr)
		require.False(t, exists,
			"a campaign fest has never navigated in must not have %s; the fixture is not a fresh campaign", absent)
	}

	// A baseline commit first: camp init leaves the branch unborn, and a
	// deferred bookkeeping job cannot execute against no HEAD (intent filed:
	// deferred-bookkeeping-commits-fail-on-an-unborn-branch). That camp-side
	// bug is separate from this one; the festival arrives after the baseline
	// so the fest commit has work of its own to do.
	out, err = tc.Exec("sh", "-c",
		"export HOME=/root && cd "+camp+" && camp commit -m 'campaign baseline'")
	require.NoError(t, err, "baseline camp commit should succeed: %s", out)

	// festivals/.festival marks the directory as fest's; the festival itself
	// lives under a status directory, as fest lays them out. Neither creates
	// the .state directory, which is what fest navigation would add later.
	_, err = tc.Exec("mkdir", "-p", camp+"/festivals/.festival")
	require.NoError(t, err)
	require.NoError(t, tc.WriteFile(camp+"/festivals/active/fresh-festival/fest.yaml", guardFestYAML))
	require.NoError(t, tc.WriteFile(camp+"/festivals/active/fresh-festival/NOTES.md", "# First run\n"))

	// An unrelated dirty file at the campaign root: scoped staging must still
	// be scoped after the fix. Dropping every path would leave commitkit an
	// empty file list, which it reads as "stage the whole tree" — the failure
	// mode a fix for this bug can easily introduce.
	require.NoError(t, tc.WriteFile(camp+"/unrelated.txt", "not part of the festival\n"))

	// Invoked through a shell, not RunFestInDir, so the quoted message survives
	// as one argument and the commit subject can be asserted.
	out, err = tc.Exec("sh", "-c",
		"export HOME=/root && cd "+camp+"/festivals/active/fresh-festival"+
			" && /fest commit --no-tag -m 'first festival commit'")
	require.NoError(t, err, "the first fest commit in a fresh campaign must succeed: %s", out)
	assert.NotContains(t, out, "did not match any files",
		"the unmatched-pathspec failure must be gone, not merely tolerated: %s", out)

	// Not a vacuous success: the festival content is in the commit.
	committed, err := tc.Exec("git", "-C", camp, "log", "-1", "--name-only", "--pretty=format:%s")
	require.NoError(t, err)
	assert.Contains(t, committed, "first festival commit", "the festival commit must be HEAD")
	assert.Contains(t, committed, "festivals/active/fresh-festival/NOTES.md",
		"the festival files must be in the commit: %s", committed)
	assert.NotContains(t, committed, "unrelated.txt",
		"staging must stay scoped to festival paths: %s", committed)

	status, err := tc.Exec("git", "-C", camp, "status", "--porcelain")
	require.NoError(t, err)
	assert.Contains(t, status, "?? unrelated.txt",
		"the unrelated file must be left untracked, not swept in: %s", status)

	// The commit is real: HEAD carries it and the tree holds the festival.
	tree, err := tc.Exec("git", "-C", camp, "ls-tree", "-r", "--name-only", "HEAD")
	require.NoError(t, err)
	assert.Contains(t, tree, "festivals/active/fresh-festival/fest.yaml",
		"the festival must be in history after the first commit")
}
