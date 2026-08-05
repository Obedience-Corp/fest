//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Criterion 15z: fest's auto-stage branches route through camp's staging
// guard, so an untracked over-threshold generated file in a festival
// workspace is kept out of git by fest exactly as `camp commit` keeps it out.
// Both fest branches stage repositories that are not a campaign root, so the
// guard applies project-side policy in each: exclude the file, commit the
// rest, and say so with the undo attached.
//
// The big file is sparse (truncate): the guard measures lstat size, never
// content, so 512 MB costs no disk or time.

// ensureGuardGit installs git into the shared Alpine container once and gives
// it a global identity; every fixture repo inherits it.
func ensureGuardGit(t *testing.T, tc *TestContainer) {
	t.Helper()
	_, err := tc.Exec("sh", "-c",
		"command -v git >/dev/null 2>&1 || apk add --no-cache git >/dev/null 2>&1")
	require.NoError(t, err, "git must be installable in the test container")
	_, err = tc.Exec("sh", "-c",
		"git config --global user.email fest@test && git config --global user.name fest-test && git config --global init.defaultBranch main && git config --global protocol.file.allow always")
	require.NoError(t, err)
}

// guardFestYAML is the minimal festival marker: enough for fest's scope
// resolution to treat the directory as a festival root.
const guardFestYAML = "version: \"1.0\"\nname: guard-festival\n"

func TestCommitGuard_LargeFileKeptOutInStandaloneWorkspace(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)

	dir := "/guard-standalone"
	// festivals/.festival is what marks a valid festivals dir; the festival
	// itself lives under a status directory, as fest lays them out.
	_, err := tc.Exec("sh", "-c",
		"mkdir -p "+dir+"/festivals/.festival && cd "+dir+" && git init -q")
	require.NoError(t, err)
	require.NoError(t, tc.WriteFile(dir+"/festivals/active/guard-festival/fest.yaml", guardFestYAML))
	_, err = tc.Exec("sh", "-c",
		"truncate -s 512M "+dir+"/render-output.bin && echo notes > "+dir+"/notes.md")
	require.NoError(t, err)

	out, err := tc.RunFestInDir(dir+"/festivals/active/guard-festival",
		"commit", "--no-tag", "-m", "guarded standalone commit")
	require.NoError(t, err, "the commit must succeed without the big file: %s", out)

	// The exclusion is said out loud with its undo, from fest's own renderer.
	assert.Contains(t, out, "render-output.bin", "fest must name the file it kept out of git")
	assert.Contains(t, out, "fest commit --commit-large", "the exclusion must carry fest's own working undo")

	// The commit exists and carries the small file only.
	committed, err := tc.Exec("git", "-C", dir, "ls-tree", "-r", "--name-only", "HEAD")
	require.NoError(t, err)
	assert.Contains(t, committed, "notes.md", "the ordinary file must be committed")
	assert.NotContains(t, committed, "render-output.bin", "the over-threshold file must stay out of history")

	// Still on disk and untracked: excluded, never deleted.
	status, err := tc.Exec("git", "-C", dir, "status", "--porcelain")
	require.NoError(t, err)
	assert.Contains(t, status, "?? render-output.bin", "the excluded file must remain untracked on disk")

	// Exclude-everything: grow only the big file and commit again. The guard
	// excludes the sole change, so nothing is left to commit; that must read
	// as the guard's doing, not as the commit failing for no reason.
	_, err = tc.Exec("sh", "-c", "truncate -s 513M "+dir+"/render-output.bin")
	require.NoError(t, err)
	out2, err := tc.RunFestInDir(dir+"/festivals/active/guard-festival",
		"commit", "--no-tag", "-m", "only excluded changes")
	require.Error(t, err, "an exclude-everything commit has nothing to commit")
	assert.Contains(t, out2, "kept out of git by the size/bulk guard",
		"the no-op must be attributed to the guard: %s", out2)

	// The advertised retry must actually work: the same commit with
	// --commit-large forces the excluded file into git, exactly as camp's
	// own flag would.
	out3, err := tc.RunFestInDir(dir+"/festivals/active/guard-festival",
		"commit", "--no-tag", "--commit-large", "-m", "committed anyway")
	require.NoError(t, err, "the --commit-large retry must succeed: %s", out3)
	committed2, err := tc.Exec("git", "-C", dir, "ls-tree", "-r", "--name-only", "HEAD")
	require.NoError(t, err)
	assert.Contains(t, committed2, "render-output.bin",
		"--commit-large must force the over-threshold file into the commit")
}

// The campaign-root branch stages a synthesized path list (the festival
// directory, .campaign/fest, festivals/.festival/.state) rather than sweeping
// the worktree, and camp's guard deliberately skips explicit path lists:
// naming a path is read as the user's own intent, so only sweep forms are
// checked (camp internal/git/stageguard.go, isStageEverything). The list fest
// builds is not something the user named, so nothing on this branch is
// guarded today: an over-threshold file inside a festival directory reaches
// git with no exclusion and no report.
//
// This test pins that as the observed truth rather than as the desired one.
// Closing the gap needs a camp-side way to guard a synthesized list; when that
// lands, this test should fail and be rewritten as the exclusion assertion its
// two neighbours already make.
func TestCommitGuard_CampaignRootFileListIsNotGuarded(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)

	dir := "/guard-root"
	// .campaign is what makes this a campaign workspace rather than a bare
	// repo, which is what routes the commit down the file-list branch.
	_, err := tc.Exec("sh", "-c",
		"mkdir -p "+dir+"/festivals/.festival "+dir+"/.campaign && cd "+dir+" && git init -q")
	require.NoError(t, err)
	require.NoError(t, tc.WriteFile(dir+"/festivals/active/guard-festival/fest.yaml", guardFestYAML))

	festDir := dir + "/festivals/active/guard-festival"
	_, err = tc.Exec("sh", "-c",
		"truncate -s 512M "+festDir+"/render-output.bin && echo notes > "+festDir+"/notes.md")
	require.NoError(t, err)

	out, err := tc.RunFestInDir(festDir, "commit", "--no-tag", "-m", "campaign root festival commit")
	require.NoError(t, err, "the campaign root commit must succeed: %s", out)

	assert.NotContains(t, out, "Kept out of git",
		"the guard does not run on an explicit path list, so nothing is reported as excluded")

	committed, err := tc.Exec("git", "-C", dir, "ls-tree", "-r", "--name-only", "HEAD")
	require.NoError(t, err)
	assert.Contains(t, committed, "festivals/active/guard-festival/notes.md",
		"the ordinary festival file must be committed")
	assert.Contains(t, committed, "festivals/active/guard-festival/render-output.bin",
		"the over-threshold file reaches git unguarded on the file-list branch; "+
			"if this now fails, camp guards synthesized lists and this test should assert exclusion instead")
}

func TestCommitGuard_LargeFileKeptOutInLinkedSubmodule(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)

	// A campaign root holding a project submodule, the shape fest's
	// in-submodule branch serves. The submodule source needs one commit
	// before it can be added.
	_, err := tc.Exec("sh", "-c", strings.Join([]string{
		"mkdir -p /guard-src && cd /guard-src && git init -q",
		"echo app > app.txt && git add -A && git commit -qm init",
		"mkdir -p /guard-camp/festivals/.festival && cd /guard-camp && git init -q && mkdir .campaign",
		"git submodule add -q /guard-src projects/app",
		"git add -A && git commit -qm 'campaign with project'",
	}, " && "))
	require.NoError(t, err)
	require.NoError(t, tc.WriteFile("/guard-camp/festivals/active/guard-festival/fest.yaml", guardFestYAML))

	// The link is what gives a project directory festival context.
	out, err := tc.RunFestInDir("/guard-camp/festivals/active/guard-festival", "link", "/guard-camp/projects/app")
	require.NoError(t, err, "linking the project should succeed: %s", out)

	_, err = tc.Exec("sh", "-c",
		"truncate -s 512M /guard-camp/projects/app/render-output.bin && echo change > /guard-camp/projects/app/notes.md")
	require.NoError(t, err)

	out, err = tc.RunFestInDir("/guard-camp/projects/app",
		"commit", "--no-tag", "-m", "guarded project commit")
	require.NoError(t, err, "the project commit must succeed without the big file: %s", out)
	assert.Contains(t, out, "render-output.bin", "fest must name the file it kept out of the project repo")
	assert.Contains(t, out, "fest commit --commit-large", "the exclusion must carry fest's own working undo")

	committed, err := tc.Exec("git", "-C", "/guard-camp/projects/app", "ls-tree", "-r", "--name-only", "HEAD")
	require.NoError(t, err)
	assert.Contains(t, committed, "notes.md", "the ordinary project file must be committed")
	assert.NotContains(t, committed, "render-output.bin", "project-side policy excludes and asks; git never sees the file")

	status, err := tc.Exec("git", "-C", "/guard-camp/projects/app", "status", "--porcelain")
	require.NoError(t, err)
	assert.Contains(t, status, "?? render-output.bin", "the excluded file must remain on disk, untracked")
}
