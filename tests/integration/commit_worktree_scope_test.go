//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fest commit from a linked gitignored worktree must not record unrelated
// campaign-root files. Staging the festival path list and then committing the
// whole index swept in concurrent campaign work that was already staged
// (OR0001: a 735-file campaign-root commit tagged as that festival, including
// none of its own files).
func TestCommitFromLinkedWorktreeKeepsCampaignRootScoped(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)

	const (
		src      = "/commit-wt-src"
		campaign = "/commit-wt-scope"
		festival = campaign + "/festivals/active/linked-commit-LC0001"
		other    = campaign + "/festivals/active/other-fest-OT0001"
		project  = campaign + "/projects/app"
		worktree = campaign + "/projects/worktrees/app/wt"
	)

	_, err := tc.Exec("sh", "-c", strings.Join([]string{
		"mkdir -p " + src + " && cd " + src + " && git init -q",
		"echo initial > app.txt && git add -A && git commit -q -m initial",
		"mkdir -p " + campaign + "/festivals/.festival " + campaign + "/.campaign",
		"cd " + campaign + " && git init -q",
		"printf 'worktrees/\\n' > .gitignore",
		"git submodule add -q " + src + " projects/app",
		"git add -A && git commit -q -m 'campaign baseline'",
	}, " && "))
	require.NoError(t, err)

	require.NoError(t, tc.WriteFile(festival+"/fest.yaml", `version: "1.0"
metadata:
  id: LC0001
  name: linked-commit
`))
	require.NoError(t, tc.WriteFile(festival+"/NOTES.md", "linked notes\n"))
	require.NoError(t, tc.WriteFile(other+"/fest.yaml", `version: "1.0"
metadata:
  id: OT0001
  name: other-fest
`))
	require.NoError(t, tc.WriteFile(other+"/NOTES.md", "other notes\n"))
	_, err = tc.Exec("sh", "-c", "cd "+campaign+" && git add festivals && git commit -q -m 'festivals baseline'")
	require.NoError(t, err)

	_, err = tc.Exec("sh", "-c", strings.Join([]string{
		"mkdir -p " + campaign + "/projects/worktrees/app",
		"git -C " + project + " worktree add -b wt-scope " + worktree,
	}, " && "))
	require.NoError(t, err, "project worktree must be created")

	out, err := tc.RunFestInDir(festival, "link", worktree)
	require.NoError(t, err, "festival link must be created: %s", out)

	require.NoError(t, tc.WriteFile(festival+"/NOTES.md", "linked festival evidence\n"))
	require.NoError(t, tc.WriteFile(other+"/NOTES.md", "unrelated festival dirty\n"))
	require.NoError(t, tc.WriteFile(campaign+"/workflow/explore/notes.md", "untracked explore\n"))
	require.NoError(t, tc.WriteFile(campaign+"/docs/marketing/Parallel festivals.png", "untracked png\n"))
	require.NoError(t, tc.WriteFile(campaign+"/.campaign/workitems/links.yaml", "untracked workitems\n"))
	require.NoError(t, tc.WriteFile(worktree+"/app.txt", "changed in worktree\n"))

	_, err = tc.Exec("git", "-C", campaign, "add", "--",
		"festivals/active/other-fest-OT0001/NOTES.md")
	require.NoError(t, err, "unrelated festival file must be staged before fest commit")

	out, err = tc.Exec("sh", "-c", "cd "+worktree+" && /fest commit --no-tag -m 'fix: change linked app'")
	require.NoError(t, err, "fest commit from the linked worktree must succeed: %s", out)

	projectLog, err := tc.Exec("git", "-C", worktree, "log", "-1", "--format=%s")
	require.NoError(t, err)
	assert.Contains(t, projectLog, "fix: change linked app")
	projectFile, err := tc.Exec("git", "-C", worktree, "show", "HEAD:app.txt")
	require.NoError(t, err)
	assert.Contains(t, projectFile, "changed in worktree")

	rootLog, err := tc.Exec("git", "-C", campaign, "log", "-1", "--format=%s")
	require.NoError(t, err)
	assert.Contains(t, rootLog, "fest: fix: change linked app")

	rootFiles, err := tc.Exec("git", "-C", campaign, "show", "--name-only", "--format=", "HEAD")
	require.NoError(t, err)
	assert.Contains(t, rootFiles, "festivals/active/linked-commit-LC0001/NOTES.md")
	assert.NotContains(t, rootFiles, "festivals/active/other-fest-OT0001")
	assert.NotContains(t, rootFiles, "workflow/explore")
	assert.NotContains(t, rootFiles, "docs/marketing")
	assert.NotContains(t, rootFiles, ".campaign/workitems")

	status, err := tc.Exec("git", "-C", campaign, "status", "--porcelain")
	require.NoError(t, err)
	assert.Contains(t, status, "M  festivals/active/other-fest-OT0001/NOTES.md",
		"unrelated staged festival files must remain staged: %s", status)
	assert.Contains(t, status, "?? workflow/",
		"unrelated untracked files must stay untracked: %s", status)
	assert.Contains(t, status, "?? docs/",
		"unrelated untracked files must stay untracked: %s", status)
	assert.Contains(t, status, "?? .campaign/workitems/",
		"unrelated untracked files must stay untracked: %s", status)
}
