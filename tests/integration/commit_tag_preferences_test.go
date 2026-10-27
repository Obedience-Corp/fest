//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Exercise the real commit command and read back both histories. Each case has
// its own Camp configuration, so settings cannot leak between shared-container
// tests. All repository and configuration mutations stay inside the container.
func TestCommitTagPreferences(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)
	for i, tt := range []struct {
		name, global, local                                        string
		noTag, tagged, invalid, cleanProject, rootOnly, standalone bool
	}{
		{name: "default tags", tagged: true},
		{name: "global disabled", global: `{"commit":{"disable_commit_tags":true}}`},
		{name: "local disabled", local: `{"commit":{"disable_commit_tags":true}}`},
		{name: "local overrides global", global: `{"commit":{"disable_commit_tags":true}}`, local: `{"commit":{"disable_commit_tags":false}}`, tagged: true},
		{name: "flag overrides enabled settings", local: `{"commit":{"disable_commit_tags":false}}`, noTag: true},
		{name: "clean project root commit", local: `{"commit":{"disable_commit_tags":true}}`, cleanProject: true},
		{name: "camp root only", local: `{"commit":{"disable_commit_tags":true}}`, rootOnly: true},
		{name: "standalone global disabled", global: `{"commit":{"disable_commit_tags":true}}`, standalone: true, rootOnly: true},
		{name: "invalid global", global: `{`, invalid: true},
		{name: "invalid local", local: `{`, invalid: true},
		{name: "flag does not hide invalid settings", local: `{`, noTag: true, invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := fmt.Sprintf("/commit-tag-prefs-%d", i)
			configRoot := root + "-config"
			festival := root + "/festivals/active/tag-test-TG0001"
			sequence := festival + "/001_IMPLEMENT/01_change"
			project := root + "/projects/app"
			_, err := tc.Exec("sh", "-c", "mkdir -p "+root+"/festivals/.festival && git init -q "+root)
			require.NoError(t, err)
			if !tt.standalone {
				require.NoError(t, tc.WriteFile(root+"/.campaign/campaign.yaml", "id: 12345678-1234-1234-1234-123456789012\nname: tag-test\ntype: product\n"))
			}
			require.NoError(t, tc.WriteFile(festival+"/fest.yaml", "version: \"1.0\"\nmetadata:\n  id: TG0001\n  name: tag-test\n"))
			workingDir := ""
			if !tt.rootOnly {
				_, err = tc.Exec("sh", "-c", strings.Join([]string{
					"mkdir -p " + root + "-src && cd " + root + "-src && git init -q",
					"echo initial > app.txt && git add -A && git commit -q -m initial",
					"cd " + root + " && git submodule add -q " + root + "-src projects/app",
				}, " && "))
				require.NoError(t, err)
				workingDir = "fest_working_dir: projects/app\n"
			}
			require.NoError(t, tc.WriteFile(sequence+"/SEQUENCE_GOAL.md", "---\nfest_type: sequence\nfest_id: 01_change\nfest_parent: 001_IMPLEMENT\nfest_order: 1\nfest_status: in_progress\n"+workingDir+"---\n\n# Change\n"))
			_, err = tc.Exec("sh", "-c", "cd "+root+" && git add -A && git commit -q -m baseline")
			require.NoError(t, err)
			rootBefore, err := tc.Exec("git", "-C", root, "rev-parse", "HEAD")
			require.NoError(t, err)
			var projectBefore string
			if !tt.rootOnly {
				projectBefore, err = tc.Exec("git", "-C", project, "rev-parse", "HEAD")
				require.NoError(t, err)
				if !tt.cleanProject {
					require.NoError(t, tc.WriteFile(project+"/app.txt", "changed\n"))
				}
			}
			require.NoError(t, tc.WriteFile(sequence+"/NOTES.md", "festival evidence\n"))
			if tt.global != "" {
				require.NoError(t, tc.WriteFile(configRoot+"/obey/campaign/config.json", tt.global))
			}
			if tt.local != "" {
				require.NoError(t, tc.WriteFile(root+"/.campaign/settings/local.json", tt.local))
			}
			flag := ""
			if tt.noTag {
				flag = " --no-tag"
			}
			out, commitErr := tc.Exec("sh", "-c", "cd "+sequence+" && XDG_CONFIG_HOME="+configRoot+" /fest commit --json"+flag+" -m 'fix: tag policy'")
			var result struct {
				Success     bool   `json:"success"`
				Message     string `json:"message"`
				CampaignTag string `json:"campaign_tag"`
				TaskRef     string `json:"task_ref"`
				Phase       string `json:"phase"`
				Sequence    string `json:"sequence"`
				Error       string `json:"error"`
			}
			if tt.invalid {
				require.NoError(t, commitErr, "JSON mode reports failures in the result: %s", out)
				require.NoError(t, json.Unmarshal([]byte(out), &result))
				require.False(t, result.Success)
				require.Contains(t, result.Error, "resolving commit tag preference")
				for repo, before := range map[string]string{root: rootBefore, project: projectBefore} {
					head, err := tc.Exec("git", "-C", repo, "rev-parse", "HEAD")
					require.NoError(t, err)
					require.Equal(t, before, head)
					staged, err := tc.Exec("git", "-C", repo, "diff", "--cached", "--name-only")
					require.NoError(t, err)
					require.Empty(t, strings.TrimSpace(staged), "invalid settings must fail before staging")
				}
				return
			}
			require.NoError(t, commitErr, out)
			require.NoError(t, json.Unmarshal([]byte(out), &result), "JSON must agree with the stored commit")
			require.True(t, result.Success)
			prefix := ""
			if tt.tagged {
				prefix = "[tag-test:12345678-FE-TG0001-PH-001-SQ-01] "
				require.Equal(t, strings.TrimSpace(prefix), result.CampaignTag)
				require.Equal(t, "FE-TG0001", result.TaskRef)
			} else {
				require.Empty(t, result.CampaignTag)
				require.Empty(t, result.TaskRef)
				require.Empty(t, result.Phase)
				require.Empty(t, result.Sequence)
			}
			rootLabel := "fest: "
			if tt.rootOnly {
				rootLabel = ""
			}
			rootLog, err := tc.Exec("git", "-C", root, "log", "-1", "--format=%s")
			require.NoError(t, err)
			require.Equal(t, prefix+rootLabel+"fix: tag policy", strings.TrimSpace(rootLog))
			if !tt.rootOnly {
				projectLog, err := tc.Exec("git", "-C", project, "log", "-1", "--format=%s")
				require.NoError(t, err)
				if tt.cleanProject {
					require.Equal(t, "initial", strings.TrimSpace(projectLog))
				} else {
					require.Equal(t, prefix+"fix: tag policy", strings.TrimSpace(projectLog))
				}
			}
			wantMessage := prefix + "fix: tag policy"
			if tt.cleanProject {
				wantMessage = prefix + "fest: fix: tag policy"
			}
			require.Equal(t, wantMessage, result.Message)
		})
	}
}
