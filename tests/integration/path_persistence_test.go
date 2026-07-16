//go:build integration
// +build integration

package integration

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPromotePersistsCampaignRelativeStatusHistory verifies the real CLI move
// path in the containerized filesystem harness. The fixture deliberately uses
// a campaign root so the persisted YAML must not contain the container's
// absolute host-equivalent path.
func TestPromotePersistsCampaignRelativeStatusHistory(t *testing.T) {
	container := GetSharedContainer(t)
	root := "/path-leakage-campaign"
	festivalsPath := setupWorkspace(t, container, root)
	_, err := container.Exec("mkdir", "-p", filepath.Join(root, ".campaign"))
	require.NoError(t, err, "should create campaign marker")

	output, err := container.RunFestInDir(festivalsPath, "create", "festival", "--name", "path-leakage", "--dest", "planning")
	require.NoError(t, err, "should create fixture festival: %s", output)

	festPath := findFestivalPath(t, container, festivalsPath+"/planning", "path-leakage")
	output, err = container.RunFestInDir(festPath, "promote", "--force", "--no-commit")
	require.NoError(t, err, "should promote fixture festival: %s", output)

	readyPath := filepath.Join(festivalsPath, "ready", filepath.Base(festPath))
	festYAML, err := container.ReadFile(filepath.Join(readyPath, "fest.yaml"))
	require.NoError(t, err, "should read promoted fest.yaml")
	require.NotContains(t, festYAML, root+"/", "status history must not persist an absolute campaign path")
	require.Contains(t, festYAML, "path: festivals/", "status history should persist a campaign-relative path")
	require.NotContains(t, strings.TrimSpace(festYAML), "path: /", "status history path must not be root-absolute")
}
