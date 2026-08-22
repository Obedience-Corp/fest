//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// git commit --only re-reads the working tree at commit time, so a write
// after StageFilesWithOptions (the DrainJobs window, including camp jobs
// touching .campaign/fest/ and festivals/.festival/.state/) would replace
// the staged snapshot. The campaign-root commit must keep the staged bytes.
func TestCommitCampaignRootKeepsStagedContentWhenWorktreeChanges(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)

	const (
		src      = "/commit-staged-src"
		campaign = "/commit-staged-snapshot"
		festival = campaign + "/festivals/active/staged-snapshot-SS0001"
		sequence = festival + "/001_IMPLEMENT/01_change_app"
		project  = campaign + "/projects/app"
		notes    = festival + "/NOTES.md"
		wrapper  = campaign + "/bin/git"
		v1       = "staged-v1-content\n"
		v2       = "dirty-v2-content\n"
	)

	_, err := tc.Exec("sh", "-c", strings.Join([]string{
		"mkdir -p " + src + " && cd " + src + " && git init -q",
		"echo initial > app.txt && git add -A && git commit -q -m initial",
		"mkdir -p " + campaign + "/festivals/.festival " + campaign + "/.campaign " + campaign + "/bin",
		"cd " + campaign + " && git init -q",
		"git submodule add -q " + src + " projects/app",
		"git add -A && git commit -q -m 'campaign baseline'",
	}, " && "))
	require.NoError(t, err)

	require.NoError(t, tc.WriteFile(festival+"/fest.yaml", `version: "1.0"
metadata:
  id: SS0001
  name: staged-snapshot
`))
	require.NoError(t, tc.WriteFile(sequence+"/SEQUENCE_GOAL.md", `---
fest_type: sequence
fest_id: 01_change_app
fest_name: change app
fest_parent: 001_IMPLEMENT
fest_order: 1
fest_status: in_progress
fest_working_dir: projects/app
---

# Sequence Goal
`))
	require.NoError(t, tc.WriteFile(sequence+"/01_change.md", `---
fest_type: task
fest_id: 01_change.md
fest_name: change
fest_parent: 01_change_app
fest_order: 1
fest_status: in_progress
---

# Change app
`))
	require.NoError(t, tc.WriteFile(notes, "baseline notes\n"))
	_, err = tc.Exec("sh", "-c", "cd "+campaign+" && git add festivals && git commit -q -m 'festivals baseline'")
	require.NoError(t, err)

	require.NoError(t, tc.WriteFile(notes, v1))
	require.NoError(t, tc.WriteFile(project+"/app.txt", "changed in linked project\n"))

	require.NoError(t, tc.WriteFile(wrapper, strings.TrimSpace(`
#!/bin/sh
real_git=/usr/bin/git
is_add=0
saw_festival=0
for arg in "$@"; do
	if [ "$arg" = "add" ]; then
		is_add=1
	fi
	case "$arg" in
	*staged-snapshot-SS0001*)
		saw_festival=1
		;;
	esac
done
if [ "$is_add" = 1 ] && [ "$saw_festival" = 1 ]; then
	"$real_git" "$@"
	st=$?
	if [ "$st" -eq 0 ]; then
		printf 'dirty-v2-content\n' > '`+notes+`'
	fi
	exit "$st"
fi
exec "$real_git" "$@"
`)+"\n"))
	_, err = tc.Exec("chmod", "+x", wrapper)
	require.NoError(t, err, "git wrapper must be executable")

	out, err := tc.Exec("sh", "-c",
		"cd "+sequence+" && PATH="+campaign+"/bin:$PATH /fest commit --no-tag -m 'keep staged v1'")
	require.NoError(t, err, "fest commit must succeed with a post-stage worktree write: %s", out)

	committed, err := tc.Exec("git", "-C", campaign, "show", "HEAD:"+strings.TrimPrefix(notes, campaign+"/"))
	require.NoError(t, err)
	assert.Equal(t, v1, committed, "campaign-root commit must contain the staged snapshot, not the later worktree write")
	assert.NotContains(t, committed, strings.TrimSpace(v2))

	worktree, err := tc.ReadFile(notes)
	require.NoError(t, err)
	assert.Equal(t, v2, worktree, "the post-stage worktree write must still be on disk")
}
