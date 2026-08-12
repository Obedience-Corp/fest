//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A sequence's fest_working_dir is the explicit repository target for its
// implementation work. Running the commit gate from that sequence must commit
// the linked project first, then commit festival state and the updated
// submodule pointer at the campaign root. Requiring the caller to cd into the
// project loses the sequence context that selected the target in the first
// place and makes the gate's plain `fest commit` instruction incomplete.
func TestCommitFromSequenceTargetsWorkingDirectory(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)

	const (
		campaign = "/commit-linked-campaign"
		festival = campaign + "/festivals/active/linked-commit-LC0001"
		sequence = festival + "/001_IMPLEMENT/01_change_app"
		project  = campaign + "/projects/app"
	)

	_, err := tc.Exec("sh", "-c", strings.Join([]string{
		"mkdir -p /commit-linked-src && cd /commit-linked-src && git init -q",
		"echo initial > app.txt && git add -A && git commit -q -m initial",
		"mkdir -p " + campaign + "/festivals/.festival " + campaign + "/.campaign",
		"cd " + campaign + " && git init -q",
		"git submodule add -q /commit-linked-src projects/app",
		"git add -A && git commit -q -m 'campaign baseline'",
	}, " && "))
	require.NoError(t, err)

	require.NoError(t, tc.WriteFile(festival+"/fest.yaml", `version: "1.0"
metadata:
  id: LC0001
  name: linked-commit
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
	require.NoError(t, tc.WriteFile(project+"/app.txt", "changed by festival\n"))

	out, err := tc.Exec("sh", "-c", "cd "+sequence+" && /fest commit -m 'fix: change linked app'")
	require.NoError(t, err, "plain fest commit from the sequence must target fest_working_dir: %s", out)

	projectLog, err := tc.Exec("git", "-C", project, "log", "-1", "--format=%s")
	require.NoError(t, err)
	assert.Contains(t, projectLog, "FE-LC0001", "project commit must carry the festival reference")
	assert.Contains(t, projectLog, "fix: change linked app")

	projectFile, err := tc.Exec("git", "-C", project, "show", "HEAD:app.txt")
	require.NoError(t, err)
	assert.Contains(t, projectFile, "changed by festival", "the project change must be committed in the project")

	rootLog, err := tc.Exec("git", "-C", campaign, "log", "-1", "--format=%s")
	require.NoError(t, err)
	assert.Contains(t, rootLog, "FE-LC0001", "campaign commit must carry the same festival reference")
	assert.Contains(t, rootLog, "fest: fix: change linked app")

	rootFiles, err := tc.Exec("git", "-C", campaign, "show", "--name-only", "--format=", "HEAD")
	require.NoError(t, err)
	assert.Contains(t, rootFiles, "festivals/active/linked-commit-LC0001")
	assert.Contains(t, rootFiles, "projects/app", "campaign commit must advance the project pointer")
}

// A festival-level navigation link provides the same target when the caller is
// at the festival root, where there is no sequence fest_working_dir to prefer.
func TestCommitFromFestivalTargetsNavigationLink(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)

	const (
		campaign = "/commit-navigation-campaign"
		festival = campaign + "/festivals/active/navigation-commit-NC0001"
		project  = campaign + "/projects/app"
	)

	_, err := tc.Exec("sh", "-c", strings.Join([]string{
		"mkdir -p /commit-navigation-src && cd /commit-navigation-src && git init -q",
		"echo initial > app.txt && git add -A && git commit -q -m initial",
		"mkdir -p " + campaign + "/festivals/.festival " + campaign + "/.campaign",
		"cd " + campaign + " && git init -q",
		"git submodule add -q /commit-navigation-src projects/app",
		"git add -A && git commit -q -m 'campaign baseline'",
	}, " && "))
	require.NoError(t, err)

	require.NoError(t, tc.WriteFile(festival+"/fest.yaml", `version: "1.0"
metadata:
  id: NC0001
  name: navigation-commit
`))
	out, err := tc.RunFestInDir(festival, "link", project)
	require.NoError(t, err, "festival link must be created: %s", out)
	require.NoError(t, tc.WriteFile(project+"/app.txt", "changed through navigation link\n"))

	out, err = tc.Exec("sh", "-c", "cd "+festival+" && /fest commit -m 'fix: honor navigation link'")
	require.NoError(t, err, "plain fest commit from the festival must target its navigation link: %s", out)

	projectLog, err := tc.Exec("git", "-C", project, "log", "-1", "--format=%s")
	require.NoError(t, err)
	assert.Contains(t, projectLog, "FE-NC0001")
	assert.Contains(t, projectLog, "fix: honor navigation link")

	rootLog, err := tc.Exec("git", "-C", campaign, "log", "-1", "--format=%s")
	require.NoError(t, err)
	assert.Contains(t, rootLog, "FE-NC0001")
	assert.Contains(t, rootLog, "fest: fix: honor navigation link")
}

// --festival accepts a path as scope input. Previously the command deferred
// scope resolution because the flag was present, but tag detection only
// searched festival names and IDs. An absolute path therefore committed the
// project with a campaign-only tag and skipped the campaign-root commit.
func TestCommitFromProjectResolvesAbsoluteFestivalPath(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)

	const (
		campaign = "/commit-flag-campaign"
		festival = campaign + "/festivals/active/flag-commit-FC0001"
		project  = campaign + "/projects/app"
	)

	_, err := tc.Exec("sh", "-c", strings.Join([]string{
		"mkdir -p /commit-flag-src && cd /commit-flag-src && git init -q",
		"echo initial > app.txt && git add -A && git commit -q -m initial",
		"mkdir -p " + campaign + "/festivals/.festival " + campaign + "/.campaign",
		"cd " + campaign + " && git init -q",
		"git submodule add -q /commit-flag-src projects/app",
		"git add -A && git commit -q -m 'campaign baseline'",
	}, " && "))
	require.NoError(t, err)

	require.NoError(t, tc.WriteFile(festival+"/fest.yaml", `version: "1.0"
metadata:
  id: FC0001
  name: flag-commit
`))
	require.NoError(t, tc.WriteFile(festival+"/NOTES.md", "festival evidence\n"))
	require.NoError(t, tc.WriteFile(project+"/app.txt", "staged project change\n"))
	_, err = tc.Exec("git", "-C", project, "add", "app.txt")
	require.NoError(t, err)

	out, err := tc.Exec("sh", "-c", "cd "+project+" && /fest commit --stage=false --festival "+festival+" -m 'fix: preserve explicit festival path'")
	require.NoError(t, err, "absolute --festival path must resolve full festival context: %s", out)

	projectLog, err := tc.Exec("git", "-C", project, "log", "-1", "--format=%s")
	require.NoError(t, err)
	assert.Contains(t, projectLog, "FE-FC0001", "path-based commit must not degrade to a campaign-only tag")

	rootLog, err := tc.Exec("git", "-C", campaign, "log", "-1", "--format=%s")
	require.NoError(t, err)
	assert.Contains(t, rootLog, "FE-FC0001")
	assert.Contains(t, rootLog, "fest: fix: preserve explicit festival path")
}
