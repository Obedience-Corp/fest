//go:build integration

package integration

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoveConfirmation(t *testing.T) {
	tc := GetSharedContainer(t)
	for _, kind := range []string{"phase", "sequence", "task"} {
		for _, scenario := range []struct {
			name   string
			target string
			flags  string
			input  string
			apply  bool
			prompt bool
		}{
			{"force_last", "3", "--force --dry-run=false", "", true, false},
			{"force_middle", "2", "--force --dry-run=false", "", true, false},
			{"force_default_preview", "2", "--force", "y\n", false, false},
			{"force_explicit_preview", "2", "--force --dry-run=true", "y\n", false, false},
			{"default_preview", "2", "", "y\n", false, false},
			{"decline", "2", "--dry-run=false", "n\n", false, true},
			{"eof", "2", "--dry-run=false", "", false, true},
			{"approve_once", "2", "--dry-run=false", "y\n", true, true},
		} {
			t.Run(kind+"/"+scenario.name, func(t *testing.T) {
				root := setupWorkspaceWithoutTemplates(t, tc, "/remove-"+kind+"-"+scenario.name)
				festival := filepath.Join(root, "planning", "remove-test")
				require.NoError(t, tc.WriteFile(festival+"/FESTIVAL_GOAL.md", "# Remove test\n"))
				dir := festival
				if kind != "phase" {
					dir = filepath.Join(dir, "001_IMPLEMENT")
					require.NoError(t, tc.WriteFile(dir+"/PHASE_GOAL.md", "# Implement\n"))
				}
				if kind == "task" {
					dir = filepath.Join(dir, "01_work")
					require.NoError(t, tc.WriteFile(dir+"/SEQUENCE_GOAL.md", "# Work\n"))
				}
				name := func(n int, label string) string {
					if kind == "phase" {
						return fmt.Sprintf("%03d_%s", n, strings.ToUpper(label))
					}
					s := fmt.Sprintf("%02d_%s", n, label)
					if kind == "task" {
						s += ".md"
					}
					return s
				}
				file := func(element string) string {
					if kind == "phase" {
						return element + "/PHASE_GOAL.md"
					}
					if kind == "sequence" {
						return element + "/SEQUENCE_GOAL.md"
					}
					return element
				}
				for n, label := range []string{"first", "middle", "last"} {
					require.NoError(t, tc.WriteFile(filepath.Join(dir, file(name(n+1, label))), "# "+label+"\n\nKeep this content.\n"))
				}
				before := snapshotFestivalWorkspace(t, tc, festival)
				inputPath := "/remove-input-" + kind + "-" + scenario.name
				require.NoError(t, tc.WriteFile(inputPath, scenario.input))
				// Redirect a finite input file: an unexpected second prompt sees
				// EOF and fails assertions instead of hanging the suite.
				output, err := tc.Exec("sh", "-c", "cd "+dir+" && /fest remove "+kind+" "+scenario.target+" "+scenario.flags+" --no-color < "+inputPath)
				require.NoError(t, err, "%s", output)
				require.NotContains(t, output, "Proceed with renumbering?")
				require.NotContains(t, output, "Apply these changes?")
				require.Equal(t, scenario.prompt, strings.Contains(output, "Are you sure?"), "%s", output)
				if !scenario.apply {
					require.Equal(t, before, snapshotFestivalWorkspace(t, tc, festival), "preview/decline must preserve all paths and bytes")
					return
				}
				require.Contains(t, output, "Successfully applied")
				removed := name(3, "last")
				if scenario.target == "2" {
					removed = name(2, "middle")
					last, err := tc.ReadFile(filepath.Join(dir, file(name(2, "last"))))
					require.NoError(t, err)
					require.Contains(t, last, "# last\n\nKeep this content.")
				}
				_, err = tc.Exec("test", "!", "-e", filepath.Join(dir, removed))
				require.NoError(t, err, "removed element still exists")
				_, err = tc.Exec("test", "!", "-e", filepath.Join(dir, name(3, "last")))
				require.NoError(t, err, "old last path still exists")
				first, err := tc.ReadFile(filepath.Join(dir, file(name(1, "first"))))
				require.NoError(t, err)
				require.Equal(t, "# first\n\nKeep this content.\n", first)
				if scenario.target == "3" {
					middle, err := tc.ReadFile(filepath.Join(dir, file(name(2, "middle"))))
					require.NoError(t, err)
					require.Equal(t, "# middle\n\nKeep this content.\n", middle)
				}
			})
		}
	}
}
