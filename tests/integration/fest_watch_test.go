//go:build integration && dev
// +build integration,dev

package integration

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// These tests create festival trees, navigation links, and watch state.
// They must remain in the containerized integration harness, not host t.TempDir tests.
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

func TestFestWatchDirectMarkerOnlyFestival(t *testing.T) {
	container := GetSharedContainer(t)
	_, festPath := setupWatchMarkerOnlyFixture(t, container)
	festivalName := filepath.Base(festPath)

	_, err := container.runCommand([]string{"test", "!", "-e", filepath.Join(festPath, "fest.yaml")})
	require.NoError(t, err, "fixture should exercise legacy marker-only festival detection")

	output := runFestWatchForInitialRender(t, container, festPath)
	requireFestWatchInitialRender(t, output, festivalName)
}

func TestFestWatchLinkedProjectContexts(t *testing.T) {
	container := GetSharedContainer(t)
	_, festPath := setupWatchFixture(t, container)
	festivalName := filepath.Base(festPath)
	projectRoot := "/workspace/projects/demo-app"
	projectSubdir := filepath.Join(projectRoot, "cmd", "demo")

	_, err := container.runCommand([]string{"mkdir", "-p", projectSubdir})
	require.NoError(t, err, "should create fake project tree")

	linkOutput, err := container.RunFestInDir(festPath, "link", projectRoot)
	require.NoError(t, err, "fest link should create container-local navigation state: %s", linkOutput)

	navContent, err := container.ReadFile("/workspace/.campaign/fest/navigation.yaml")
	require.NoError(t, err, "navigation state should be written inside the container campaign")
	require.Contains(t, navContent, festivalName)
	require.Contains(t, navContent, "projects/demo-app")

	t.Run("project root", func(t *testing.T) {
		output := runFestWatchForInitialRender(t, container, projectRoot)
		requireFestWatchInitialRender(t, output, festivalName)
	})

	t.Run("project subdirectory", func(t *testing.T) {
		output := runFestWatchForInitialRender(t, container, projectSubdir)
		requireFestWatchInitialRender(t, output, festivalName)
	})
}

func TestFestWatchNonInteractiveNoContextFailsFast(t *testing.T) {
	container := GetSharedContainer(t)
	workspaceRoot, _ := setupWatchFixture(t, container)

	output, exitCode := runFestWatchBounded(t, container, workspaceRoot)
	require.NotZero(t, exitCode, "no-context watch should fail")
	require.NotEqual(t, 124, exitCode, "no-context watch should fail before timeout")
	require.NotEqual(t, 143, exitCode, "no-context watch should fail before timeout")
	require.Contains(t, output, "festival could not be resolved from this directory")
	require.Contains(t, output, "linked project")
	require.Contains(t, output, "interactive terminal")
	require.NotContains(t, output, "cd ")
	require.NotContains(t, output, "Watching for changes")
}

// Picker selection is covered by internal/commands/watch resolver unit tests.
// The current Testcontainers TTY helper can allocate a TTY, but Exec does not
// expose deterministic stdin writes for selecting an item from the picker.
// Keep this documented here so the remaining gap is explicit in PR notes.
func TestFestWatchPickerSelectionHarnessLimitation(t *testing.T) {
	t.Log("picker selection requires a TTY harness that can send deterministic stdin; resolver picker path is covered by pure tests")
}

func setupWatchFixture(t *testing.T, tc *TestContainer) (string, string) {
	return setupWatchFixtureWithConfig(t, tc, "fest-watch-fixture-FW0001", true)
}

func setupWatchMarkerOnlyFixture(t *testing.T, tc *TestContainer) (string, string) {
	return setupWatchFixtureWithConfig(t, tc, "legacy-watch-fixture-LW0001", false)
}

func setupWatchFixtureWithConfig(t *testing.T, tc *TestContainer, festivalName string, includeConfig bool) (string, string) {
	t.Helper()

	workspaceRoot := "/workspace"
	festivalsRoot := filepath.Join(workspaceRoot, "festivals")
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

	if includeConfig {
		logicalID := festivalName
		name := festivalName
		if idx := strings.LastIndex(festivalName, "-"); idx >= 0 {
			logicalID = festivalName[idx+1:]
			name = festivalName[:idx]
		}
		err = writeFileInContainer(tc, filepath.Join(festivalPath, "fest.yaml"), fmt.Sprintf(`version: "1.0"
metadata:
  id: %s
  name: %s
auto_link:
  enabled: false
`, logicalID, name))
		require.NoError(t, err, "should create fest.yaml")
	}

	err = writeFileInContainer(tc, filepath.Join(festivalPath, "FESTIVAL_GOAL.md"), fmt.Sprintf(`# %s

## Goal

Exercise watch command context resolution.
`, festivalName))
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

	output, exitCode := runFestWatchBounded(t, tc, cwd, args...)
	require.Contains(t, []int{124, 143}, exitCode, "watch should stay running until bounded timeout after initial render")
	return output
}

func runFestWatchBounded(t *testing.T, tc *TestContainer, cwd string, args ...string) (string, int) {
	t.Helper()

	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		quotedArgs = append(quotedArgs, shellQuote(arg))
	}

	outputPath := "/tmp/fest-watch-output"
	cmd := "cd " + shellQuote(cwd) + " && rm -f " + outputPath + " && set +e; timeout 2s /fest watch"
	if len(quotedArgs) > 0 {
		cmd += " " + strings.Join(quotedArgs, " ")
	}
	cmd += " > " + outputPath + " 2>&1; code=$?; cat " + outputPath + "; printf '\\n__FEST_WATCH_EXIT_CODE:%d\\n' \"$code\""

	output, err := tc.runCommand([]string{"sh", "-c", cmd})
	require.NoError(t, err, "fest watch should start")

	marker := "\n__FEST_WATCH_EXIT_CODE:"
	idx := strings.LastIndex(output, marker)
	require.NotEqual(t, -1, idx, "bounded watch output should include exit marker: %s", output)

	codeText := strings.TrimSpace(output[idx+len(marker):])
	exitCode, err := strconv.Atoi(codeText)
	require.NoError(t, err, "bounded watch exit code should parse")

	return output[:idx], exitCode
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
