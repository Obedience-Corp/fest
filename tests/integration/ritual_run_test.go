//go:build integration
// +build integration

package integration

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/stretchr/testify/require"
)

// TestRitualRun_AutoCommit verifies that `fest ritual run` copies the ritual
// template into active/, increments run_count in the source fest.yaml, and
// auto-commits only the scoped filesystem changes (leaving unrelated worktree
// modifications uncommitted). It covers both standalone and campaign workspace
// detection so the name-style campaign tag path is exercised end-to-end.
//
// This test must live in the container harness because it exercises real git
// and real filesystem mutation. CLAUDE.md §11 forbids running this class of
// test on the host against t.TempDir().
func TestRitualRun_AutoCommit(t *testing.T) {
	cases := []struct {
		name          string
		setupCampaign bool
		subjectPrefix string
	}{
		{name: "StandaloneWorkspace", setupCampaign: false, subjectPrefix: ""},
		{name: "CampaignWorkspace", setupCampaign: true, subjectPrefix: "[testcampaign:12345678]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			container := GetSharedContainer(t)
			ensureGit(t, container)

			const (
				workspaceRoot = "/workspace"
				festivalsRoot = workspaceRoot + "/festivals"
				ritualName    = "daily-job-search-RI-DJ0001"
				ritualPath    = festivalsRoot + "/ritual/" + ritualName
				activeRunName = ritualName + "-0001"
				activeRunPath = festivalsRoot + "/active/" + activeRunName
			)

			setupRitualWorkspace(t, container, workspaceRoot, festivalsRoot, ritualPath)
			if tc.setupCampaign {
				writeCampaignConfig(t, container, workspaceRoot)
			}
			setupGitRepo(t, container, workspaceRoot)
			modifyReadmeUnstaged(t, container, workspaceRoot)

			output, err := container.RunFestInDir(workspaceRoot, "ritual", "run", "daily-job")
			require.NoError(t, err, "fest ritual run should succeed: %s", output)

			// Copy landed in active/.
			exists, err := container.CheckFileExists(activeRunPath + "/FESTIVAL_OVERVIEW.md")
			require.NoError(t, err)
			require.True(t, exists, "expected copied ritual run file at %s", activeRunPath)

			// Source ritual fest.yaml run_count incremented.
			config, err := container.ReadFile(ritualPath + "/fest.yaml")
			require.NoError(t, err)
			require.Contains(t, config, "run_count: 1",
				"expected ritual config to increment run_count, got:\n%s", config)

			// Commit subject matches the documented format.
			subject := strings.TrimSpace(runGit(t, container, workspaceRoot, "log", "-1", "--pretty=%s"))
			wantSubject := "chore(fest): ritual run: daily-job-search-RI-DJ0001 (DJ0001) -> daily-job-search-RI-DJ0001-0001"
			if tc.subjectPrefix != "" {
				wantSubject = tc.subjectPrefix + " " + wantSubject
			}
			require.Equal(t, wantSubject, subject, "commit subject mismatch")

			// Commit contains ritual config and active run files only.
			changedFiles := runGit(t, container, workspaceRoot, "show", "--name-only", "--pretty=", "HEAD")
			require.Contains(t, changedFiles,
				"festivals/ritual/daily-job-search-RI-DJ0001/fest.yaml",
				"expected ritual config in commit, got:\n%s", changedFiles)
			require.Contains(t, changedFiles,
				"festivals/active/daily-job-search-RI-DJ0001-0001/FESTIVAL_OVERVIEW.md",
				"expected active ritual run files in commit, got:\n%s", changedFiles)
			require.NotContains(t, changedFiles, "README.md",
				"expected unrelated README change to remain uncommitted, got:\n%s", changedFiles)

			// Unrelated README change is still pending in the worktree.
			status := runGit(t, container, workspaceRoot, "status", "--short")
			require.True(t,
				strings.Contains(status, " M README.md") || strings.Contains(status, "M README.md"),
				"expected unrelated README change to remain in worktree, got:\n%s", status)
		})
	}
}

// ensureGit installs git into the shared alpine container (idempotent).
func ensureGit(t *testing.T, tc *TestContainer) {
	t.Helper()
	_, err := tc.runCommand([]string{
		"sh", "-c",
		"command -v git >/dev/null 2>&1 || apk add --no-cache git >/dev/null",
	})
	require.NoError(t, err, "should install git in container")
}

// setupRitualWorkspace creates a festivals/ tree with a registered workspace
// marker and a ritual template directory ready for `fest ritual run` to copy.
func setupRitualWorkspace(t *testing.T, tc *TestContainer, workspaceRoot, festivalsRoot, ritualPath string) {
	t.Helper()

	_, err := tc.runCommand([]string{
		"sh", "-c",
		fmt.Sprintf(
			"mkdir -p %s/.festival/.state %s/active %s/planning %s",
			festivalsRoot, festivalsRoot, festivalsRoot, ritualPath,
		),
	})
	require.NoError(t, err, "should create festivals directories")

	writeContainerFile(t, tc,
		festivalsRoot+"/.festival/.state/.workspace",
		`{"workspace": "workspace", "registered": "2024-01-01T00:00:00Z"}`)
	writeContainerFile(t, tc,
		ritualPath+"/FESTIVAL_OVERVIEW.md",
		"# Overview\n")
	writeContainerFile(t, tc,
		ritualPath+"/fest.yaml",
		"version: \"1.0\"\nmetadata:\n  name: daily-job-search\nritual_config:\n  run_count: 0\n")
}

// writeCampaignConfig drops a minimal campaign.yaml so scope detection
// classifies the workspace as WorkspaceTypeCampaign and LoadCampaignID
// returns the expected identifier.
func writeCampaignConfig(t *testing.T, tc *TestContainer, workspaceRoot string) {
	t.Helper()

	campaignYAML := "id: 12345678-1234-1234-1234-123456789abc\n" +
		"name: TestCampaign\n" +
		"type: product\n"

	_, err := tc.runCommand([]string{
		"sh", "-c",
		fmt.Sprintf("mkdir -p %s/.campaign/settings", workspaceRoot),
	})
	require.NoError(t, err, "should create .campaign directory")

	writeContainerFile(t, tc, workspaceRoot+"/.campaign/campaign.yaml", campaignYAML)
}

// setupGitRepo initializes a git repo with committer identity and seeds an
// initial commit so subsequent commits produce a clean diff.
func setupGitRepo(t *testing.T, tc *TestContainer, root string) {
	t.Helper()

	writeContainerFile(t, tc, root+"/README.md", "base\n")

	runGit(t, tc, root, "init")
	runGit(t, tc, root, "config", "user.name", "Test User")
	runGit(t, tc, root, "config", "user.email", "test@example.com")
	runGit(t, tc, root, "config", "commit.gpgsign", "false")
	runGit(t, tc, root, "add", ".")
	runGit(t, tc, root, "commit", "-m", "initial state")
}

// modifyReadmeUnstaged writes an unrelated worktree modification that must
// survive the ritual auto-commit without being staged.
func modifyReadmeUnstaged(t *testing.T, tc *TestContainer, root string) {
	t.Helper()
	writeContainerFile(t, tc, root+"/README.md", "base\nunrelated change\n")
}

// runGit executes a git command in the given directory inside the container
// and returns the combined output. Test fails if the command errors or exits
// non-zero. Uses stdcopy demultiplexing so large outputs (e.g. git show) are
// captured in full.
func runGit(t *testing.T, tc *TestContainer, dir string, args ...string) string {
	t.Helper()

	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}
	cmd := fmt.Sprintf("cd %s && git %s", shellQuote(dir), strings.Join(quoted, " "))

	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{"sh", "-c", cmd})
	require.NoError(t, err, "git exec failed")

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, reader)
	require.NoError(t, err, "git output demux failed")

	output := stdout.String() + stderr.String()
	require.Equal(t, 0, exitCode, "git %s failed: %s", strings.Join(args, " "), output)

	return output
}

// writeContainerFile writes content to a file in the container, creating
// parent directories as needed. Uses a heredoc with a random marker to avoid
// shell-escaping issues.
func writeContainerFile(t *testing.T, tc *TestContainer, path, content string) {
	t.Helper()

	mkdir := fmt.Sprintf("mkdir -p %s", shellQuote(parentDir(path)))
	_, err := tc.runCommand([]string{"sh", "-c", mkdir})
	require.NoError(t, err, "should create parent dir for %s", path)

	cmd := fmt.Sprintf("cat > %s <<'FEST_WRITE_EOF'\n%sFEST_WRITE_EOF\n", shellQuote(path), content)
	_, err = tc.runCommand([]string{"sh", "-c", cmd})
	require.NoError(t, err, "should write file %s", path)
}

// parentDir returns the parent path component for a unix-style path.
func parentDir(path string) string {
	if idx := strings.LastIndex(path, "/"); idx > 0 {
		return path[:idx]
	}
	return "/"
}

// shellQuote single-quotes a shell argument, escaping embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
