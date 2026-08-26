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

// Isolated git identity for this file. Never `git config --global`: that
// writes ~/.gitconfig, and if Exec ever misses the container it rewrites the
// host (2026-08-21: user.email became fest@test; later commits in repos
// without a local identity were authored lancekrogers <fest@test>).
const guardGitConfig = "/tmp/fest-itest.gitconfig"

func withGuardGit(script string) string {
	return "GIT_CONFIG_GLOBAL=" + guardGitConfig + " GIT_CONFIG_NOSYSTEM=1 " + script
}

// ensureGuardGit installs git into the shared Alpine container once and gives
// it an identity that lives only in guardGitConfig, never ~/.gitconfig.
func ensureGuardGit(t *testing.T, tc *TestContainer) {
	t.Helper()
	_, err := tc.Exec("sh", "-c",
		"command -v git >/dev/null 2>&1 || apk add --no-cache git >/dev/null 2>&1")
	require.NoError(t, err, "git must be installable in the test container")
	_, err = tc.Exec("sh", "-c", withGuardGit(
		"git config --file "+guardGitConfig+" user.email fest@test && "+
			"git config --file "+guardGitConfig+" user.name fest-test && "+
			"git config --file "+guardGitConfig+" init.defaultBranch main && "+
			"git config --file "+guardGitConfig+" protocol.file.allow always"))
	require.NoError(t, err)
}

func guardFestInDir(tc *TestContainer, dir string, args ...string) (string, error) {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = "'" + strings.ReplaceAll(a, "'", `'"'"'`) + "'"
	}
	return tc.Exec("sh", "-c", withGuardGit("cd "+dir+" && /fest "+strings.Join(parts, " ")))
}

func guardGit(tc *TestContainer, gitArgs string) (string, error) {
	return tc.Exec("sh", "-c", withGuardGit("git "+gitArgs))
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
		withGuardGit("mkdir -p "+dir+"/festivals/.festival && cd "+dir+" && git init -q"))
	require.NoError(t, err)
	require.NoError(t, tc.WriteFile(dir+"/festivals/active/guard-festival/fest.yaml", guardFestYAML))
	_, err = tc.Exec("sh", "-c",
		"truncate -s 512M "+dir+"/render-output.bin && echo notes > "+dir+"/notes.md")
	require.NoError(t, err)

	out, err := guardFestInDir(tc, dir+"/festivals/active/guard-festival",
		"commit", "--no-tag", "-m", "guarded standalone commit")
	require.NoError(t, err, "the commit must succeed without the big file: %s", out)

	// The exclusion is said out loud with its undo, from fest's own renderer.
	assert.Contains(t, out, "render-output.bin", "fest must name the file it kept out of git")
	assert.Contains(t, out, "fest commit --commit-large", "the exclusion must carry fest's own working undo")

	// The commit exists and carries the small file only.
	committed, err := guardGit(tc, "-C "+dir+" ls-tree -r --name-only HEAD")
	require.NoError(t, err)
	assert.Contains(t, committed, "notes.md", "the ordinary file must be committed")
	assert.NotContains(t, committed, "render-output.bin", "the over-threshold file must stay out of history")

	// Still on disk and untracked: excluded, never deleted.
	status, err := guardGit(tc, "-C "+dir+" status --porcelain")
	require.NoError(t, err)
	assert.Contains(t, status, "?? render-output.bin", "the excluded file must remain untracked on disk")

	// Exclude-everything: grow only the big file and commit again. The guard
	// excludes the sole change, so nothing is left to commit; that must read
	// as the guard's doing, not as the commit failing for no reason.
	_, err = tc.Exec("sh", "-c", "truncate -s 513M "+dir+"/render-output.bin")
	require.NoError(t, err)
	out2, err := guardFestInDir(tc, dir+"/festivals/active/guard-festival",
		"commit", "--no-tag", "-m", "only excluded changes")
	require.Error(t, err, "an exclude-everything commit has nothing to commit")
	assert.Contains(t, out2, "kept out of git by the staging guard",
		"the no-op must be attributed to the guard: %s", out2)
	assert.Contains(t, out2, "render-output.bin",
		"the no-op must name what was held back: %s", out2)
	assert.Contains(t, out2, "fest commit --commit-large",
		"the no-op must carry the flag that commits it anyway: %s", out2)

	// The advertised retry must actually work: the same commit with
	// --commit-large forces the excluded file into git, exactly as camp's
	// own flag would.
	out3, err := guardFestInDir(tc, dir+"/festivals/active/guard-festival",
		"commit", "--no-tag", "--commit-large", "-m", "committed anyway")
	require.NoError(t, err, "the --commit-large retry must succeed: %s", out3)
	committed2, err := guardGit(tc, "-C "+dir+" ls-tree -r --name-only HEAD")
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
		withGuardGit("mkdir -p "+dir+"/festivals/.festival "+dir+"/.campaign && cd "+dir+" && git init -q"))
	require.NoError(t, err)
	require.NoError(t, tc.WriteFile(dir+"/festivals/active/guard-festival/fest.yaml", guardFestYAML))

	festDir := dir + "/festivals/active/guard-festival"
	_, err = tc.Exec("sh", "-c",
		"truncate -s 512M "+festDir+"/render-output.bin && echo notes > "+festDir+"/notes.md")
	require.NoError(t, err)

	out, err := guardFestInDir(tc, festDir, "commit", "--no-tag", "-m", "campaign root festival commit")
	require.NoError(t, err, "the campaign root commit must succeed: %s", out)

	assert.NotContains(t, out, "Kept out of git",
		"the guard does not run on an explicit path list, so nothing is reported as excluded")

	committed, err := guardGit(tc, "-C "+dir+" ls-tree -r --name-only HEAD")
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
	_, err := tc.Exec("sh", "-c", withGuardGit(strings.Join([]string{
		"mkdir -p /guard-src && cd /guard-src && git init -q",
		"echo app > app.txt && git add -A && git commit -qm init",
		"mkdir -p /guard-camp/festivals/.festival && cd /guard-camp && git init -q && mkdir .campaign",
		"git submodule add -q /guard-src projects/app",
		"git add -A && git commit -qm 'campaign with project'",
	}, " && ")))
	require.NoError(t, err)
	require.NoError(t, tc.WriteFile("/guard-camp/festivals/active/guard-festival/fest.yaml", guardFestYAML))

	// The link is what gives a project directory festival context.
	out, err := guardFestInDir(tc, "/guard-camp/festivals/active/guard-festival", "link", "/guard-camp/projects/app")
	require.NoError(t, err, "linking the project should succeed: %s", out)

	_, err = tc.Exec("sh", "-c",
		"truncate -s 512M /guard-camp/projects/app/render-output.bin && echo change > /guard-camp/projects/app/notes.md")
	require.NoError(t, err)

	out, err = guardFestInDir(tc, "/guard-camp/projects/app",
		"commit", "--no-tag", "-m", "guarded project commit")
	require.NoError(t, err, "the project commit must succeed without the big file: %s", out)
	assert.Contains(t, out, "render-output.bin", "fest must name the file it kept out of the project repo")
	assert.Contains(t, out, "fest commit --commit-large", "the exclusion must carry fest's own working undo")

	committed, err := guardGit(tc, "-C /guard-camp/projects/app ls-tree -r --name-only HEAD")
	require.NoError(t, err)
	assert.Contains(t, committed, "notes.md", "the ordinary project file must be committed")
	assert.NotContains(t, committed, "render-output.bin", "project-side policy excludes and asks; git never sees the file")

	status, err := guardGit(tc, "-C /guard-camp/projects/app status --porcelain")
	require.NoError(t, err)
	assert.Contains(t, status, "?? render-output.bin", "the excluded file must remain on disk, untracked")
}

// A nested git repository is kept out of the commit by camp's guard, and fest
// must say so. The failure this guards against is silence: the commit succeeds,
// the foreign checkout is absent, and nothing on stderr explains why. It is a
// separate outcome field from the size exclusions above and was dropped on the
// floor once already when the guard grew it.
func TestCommitGuard_NestedRepoReportedAndOverridable(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)

	dir := "/guard-nested"
	_, err := tc.Exec("sh", "-c",
		withGuardGit("mkdir -p "+dir+"/festivals/.festival && cd "+dir+" && git init -q"))
	require.NoError(t, err)
	require.NoError(t, tc.WriteFile(dir+"/festivals/active/guard-festival/fest.yaml", guardFestYAML))

	// The parent directory must be tracked first, or git reports the parent as
	// the untracked entry and never names the nested repository at all.
	_, err = tc.Exec("sh", "-c",
		withGuardGit("mkdir -p "+dir+"/work && echo base > "+dir+"/work/notes.md && "+
			"git -C "+dir+" add -A && git -C "+dir+" commit -q -m base"))
	require.NoError(t, err)

	_, err = tc.Exec("sh", "-c",
		withGuardGit("mkdir -p "+dir+"/work/vendored && cd "+dir+"/work/vendored && git init -q && "+
			"echo lib > lib.txt && git add -A && git commit -q -m vendored && "+
			"echo edited >> "+dir+"/work/notes.md"))
	require.NoError(t, err)

	out, err := guardFestInDir(tc, dir+"/festivals/active/guard-festival",
		"commit", "--no-tag", "-m", "edit beside a nested repo")
	require.NoError(t, err, "the commit must succeed without the nested repo: %s", out)

	assert.Contains(t, out, "work/vendored", "fest must name the nested repository it kept out")
	assert.Contains(t, out, "not declared in .gitmodules", "fest must say why it was held back")
	assert.Contains(t, out, "fest commit --commit-nested", "the exclusion must carry fest's own working undo")

	committed, err := guardGit(tc, "-C "+dir+" ls-tree -r --name-only HEAD")
	require.NoError(t, err)
	assert.Contains(t, committed, "work/notes.md", "the ordinary edit must be committed")
	assert.NotContains(t, committed, "work/vendored", "the nested repo must stay out of history")

	// The advertised retry must actually work, recording the checkout as a
	// gitlink rather than as files.
	out2, err := guardFestInDir(tc, dir+"/festivals/active/guard-festival",
		"commit", "--no-tag", "--commit-nested", "-m", "embed the nested repo")
	require.NoError(t, err, "the --commit-nested retry must succeed: %s", out2)

	gitlink, err := guardGit(tc, "-C "+dir+" ls-tree -r HEAD")
	require.NoError(t, err)
	assert.True(t, strings.Contains(gitlink, "160000 commit") && strings.Contains(gitlink, "work/vendored"),
		"--commit-nested must record the nested repo as a gitlink: %s", gitlink)
}
