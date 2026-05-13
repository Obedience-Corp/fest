//go:build integration && dev
// +build integration,dev

package integration

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFestWatchExplicitAndDirectContexts(t *testing.T) {
	container := GetSharedContainer(t)
	workspaceRoot, festPath := setupWatchFixture(t, container)
	festivalName := filepath.Base(festPath)

	t.Run("explicit selector from workspace root", func(t *testing.T) {
		output := runFestWatchForInitialRender(t, container, workspaceRoot, festivalName)
		requireFestWatchInitialRender(t, output, festivalName)
	})

	t.Run("direct festival root cwd", func(t *testing.T) {
		output := runFestWatchForInitialRender(t, container, festPath)
		requireFestWatchInitialRender(t, output, festivalName)
	})

	t.Run("nested phase cwd", func(t *testing.T) {
		phasePath := filepath.Join(festPath, "001_IMPLEMENT")

		phaseOutput := runFestWatchForInitialRender(t, container, phasePath)
		requireFestWatchInitialRender(t, phaseOutput, festivalName)
	})
}

func setupWatchFixture(t *testing.T, tc *TestContainer) (string, string) {
	t.Helper()

	workspaceRoot := "/workspace"
	festivalsRoot := filepath.Join(workspaceRoot, "festivals")
	festivalName := "fest-watch-fixture-FW0001"
	festivalPath := filepath.Join(festivalsRoot, "active", festivalName)
	phasePath := filepath.Join(festivalPath, "001_IMPLEMENT")
	sequencePath := filepath.Join(phasePath, "01_core")

	_, err := tc.runCommand([]string{
		"sh", "-c",
		fmt.Sprintf(
			"mkdir -p %[1]s/.campaign %[2]s/.festival/.state %[2]s/active %[2]s/planning %[2]s/ready %[2]s/ritual %[2]s/dungeon/completed %[2]s/dungeon/archived %[2]s/dungeon/someday %[3]s",
			workspaceRoot,
			festivalsRoot,
			sequencePath,
		),
	})
	require.NoError(t, err, "should create watch fixture directories")

	err = writeFileInContainer(tc, filepath.Join(festivalsRoot, ".festival", ".state", ".workspace"), `{"workspace":"workspace","registered":"2024-01-01T00:00:00Z"}`)
	require.NoError(t, err, "should create workspace marker")

	err = writeFileInContainer(tc, filepath.Join(festivalPath, "fest.yaml"), `version: "1.0"
metadata:
  id: FW0001
  name: fest-watch-fixture
auto_link:
  enabled: false
`)
	require.NoError(t, err, "should create fest.yaml")

	err = writeFileInContainer(tc, filepath.Join(festivalPath, "FESTIVAL_GOAL.md"), `# Fest Watch Fixture

## Goal

Exercise watch command context resolution.
`)
	require.NoError(t, err, "should create festival goal")

	err = writeFileInContainer(tc, filepath.Join(festivalPath, "FESTIVAL_RULES.md"), "# Festival Rules\n\n- Keep watch tests containerized.\n")
	require.NoError(t, err, "should create festival rules")

	err = writeFileInContainer(tc, filepath.Join(phasePath, "PHASE_GOAL.md"), `---
fest_type: phase
fest_phase_type: implementation
---

# Implement Phase

## Objective

Build the watch command.
`)
	require.NoError(t, err, "should create phase goal")

	err = writeFileInContainer(tc, filepath.Join(sequencePath, "SEQUENCE_GOAL.md"), "# Core Sequence\n\nImplement the core watch flow.\n")
	require.NoError(t, err, "should create sequence goal")

	err = writeFileInContainer(tc, filepath.Join(sequencePath, "01_first_task.md"), "# First Watch Task\n\n## Definition of Done\n\n- [ ] Initial watch render appears.\n")
	require.NoError(t, err, "should create first task")

	err = writeFileInContainer(tc, filepath.Join(sequencePath, "02_second_task.md"), "# Second Watch Task\n\n## Definition of Done\n\n- [ ] Watch footer appears.\n")
	require.NoError(t, err, "should create second task")

	return workspaceRoot, festivalPath
}

func runFestWatchForInitialRender(t *testing.T, tc *TestContainer, cwd string, args ...string) string {
	t.Helper()

	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		quotedArgs = append(quotedArgs, shellQuote(arg))
	}

	cmd := "cd " + shellQuote(cwd) + " && timeout 2s /fest watch"
	if len(quotedArgs) > 0 {
		cmd += " " + strings.Join(quotedArgs, " ")
	}

	output, err := tc.runCommand([]string{"sh", "-c", cmd})
	require.NoError(t, err, "fest watch should start")
	return output
}

func requireFestWatchInitialRender(t *testing.T, output, festivalName string) {
	t.Helper()

	require.Contains(t, output, festivalName, "watch output should render the resolved festival")
	require.Contains(t, output, "001_IMPLEMENT", "watch output should render phase tree content")
	require.Contains(t, output, "01_core", "watch output should render sequence tree content")
	require.Contains(t, output, "01_first_task.md", "watch output should render task tree content")
	require.True(t,
		strings.Contains(output, "Watching for changes") || strings.Contains(output, "Polling for changes"),
		"watch output should include watch footer, got:\n%s",
		output,
	)
	require.NotContains(t, output, "unknown command")
	require.NotContains(t, output, "festival could not be resolved")
}
