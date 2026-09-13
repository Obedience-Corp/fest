//go:build integration

package integration

import (
	"image/gif"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGifPathOutsideWorkspace(t *testing.T) {
	tc := GetSharedContainer(t)
	// Neither the caller nor the festival lives in a fest workspace. Exercise
	// the real root command's scope check, not just NewGifCommand in isolation.
	const root = "/tmp/gif-path-outside-workspace"
	const festival = root + "/demo-DM0001"
	const caller = root + "/caller"
	require.NoError(t, tc.WriteFile(festival+"/FESTIVAL_GOAL.md", "# Demo\n"))
	require.NoError(t, tc.WriteFile(festival+"/001_IMPLEMENT/01_build/01_task.md", "---\nfest_type: task\nfest_tracking: true\n---\n# Task\n"))
	require.NoError(t, tc.WriteFile(caller+"/plain/README.md", "Not a festival\n"))

	for _, target := range []string{festival, "../demo-DM0001", festival + "/001_IMPLEMENT"} {
		t.Run(target, func(t *testing.T) {
			output, err := tc.RunFestInDir(caller, "gif", target)
			require.NoError(t, err, output)
			const out = caller + "/demo-DM0001.gif"
			require.Contains(t, output, "Wrote "+out)
			data, err := tc.ReadFile(out)
			require.NoError(t, err)
			_, err = gif.DecodeAll(strings.NewReader(data))
			require.NoError(t, err, "output must be a readable GIF")
			mode, err := tc.Exec("stat", "-c", "%a", out)
			require.NoError(t, err)
			require.Equal(t, "644", strings.TrimSpace(mode))
			_, err = tc.Exec("rm", out)
			require.NoError(t, err)
		})
	}

	for _, args := range [][]string{{}, {"missing"}, {"plain"}, {festival, "--festival", "DM0001"}} {
		t.Run("invalid/"+strings.Join(args, "_"), func(t *testing.T) {
			output, err := tc.RunFestInDir(caller, append([]string{"gif"}, args...)...)
			require.Error(t, err, output)
			require.NotContains(t, output, "Wrote ")
			exists, err := tc.CheckFileExists(caller + "/demo-DM0001.gif")
			require.NoError(t, err)
			require.False(t, exists, "invalid input must not create an output file")
		})
	}
}
