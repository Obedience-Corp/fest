//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type validateStructureOutput struct {
	OK     bool   `json:"ok"`
	Action string `json:"action"`
	Valid  bool   `json:"valid"`
	Issues []struct {
		Level   string `json:"level"`
		Code    string `json:"code"`
		Path    string `json:"path"`
		Message string `json:"message"`
	} `json:"issues"`
}

func TestValidateStructureNamingViolations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)

	// Create workspace structure manually (fest init requires network access unavailable in containers)
	exitCode, _, err := container.container.Exec(container.ctx, []string{
		"sh", "-c",
		"mkdir -p /festivals/.festival/.state /festivals/active /festivals/planning",
	})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)

	// Create .workspace marker file to register as workspace
	err = writeFileInContainer(container, "/festivals/.festival/.state/.workspace",
		`{"workspace": "root", "registered": "2024-01-01T00:00:00Z"}`)
	require.NoError(t, err, "should create workspace marker")

	festivalPath := "/festivals/naming-violations"
	phasePath := filepath.Join(festivalPath, "001_impl")
	sequencePath := filepath.Join(phasePath, "01_Design")

	exitCode, _, err = container.container.Exec(container.ctx, []string{"mkdir", "-p", sequencePath})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)

	writeFile := func(path, content string) {
		exitCode, _, err := container.container.Exec(container.ctx, []string{
			"sh", "-c",
			"printf '%s' '" + content + "' > " + path,
		})
		require.NoError(t, err)
		require.Equal(t, 0, exitCode)
	}

	writeFile(filepath.Join(festivalPath, "FESTIVAL_OVERVIEW.md"), "# Naming Violations\n")
	writeFile(filepath.Join(sequencePath, "SEQUENCE_GOAL.md"), "# Sequence Goal\n")
	writeFile(filepath.Join(sequencePath, "01_Task.md"), "# Task\n")

	output, err := container.RunFest("validate", "structure", festivalPath, "--json")
	require.NoError(t, err, "validate structure should not fail")

	var result validateStructureOutput
	err = json.Unmarshal([]byte(strings.TrimSpace(output)), &result)
	require.NoError(t, err, "validate structure output should be valid JSON")
	require.False(t, result.Valid, "structure validation should be invalid when warning-level issues are present")

	var hasPhase, hasSequence, hasTask bool
	for _, issue := range result.Issues {
		if issue.Code != "naming_convention" || issue.Level != "warning" {
			continue
		}
		if strings.Contains(issue.Path, "001_impl") {
			hasPhase = true
		}
		if strings.Contains(issue.Path, "01_Design") {
			hasSequence = true
		}
		if strings.Contains(issue.Path, "01_Task.md") {
			hasTask = true
		}
	}

	require.True(t, hasPhase, "phase naming violation should be reported")
	require.True(t, hasSequence, "sequence naming violation should be reported")
	require.True(t, hasTask, "task naming violation should be reported")
}

func TestValidateJSONWarningsSetValidFalseWithoutFailingCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)

	exitCode, _, err := container.container.Exec(container.ctx, []string{
		"sh", "-c",
		"mkdir -p /festivals/.festival/.state /festivals/active /festivals/planning",
	})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)

	err = writeFileInContainer(container, "/festivals/.festival/.state/.workspace",
		`{"workspace": "root", "registered": "2024-01-01T00:00:00Z"}`)
	require.NoError(t, err, "should create workspace marker")

	festivalPath := "/festivals/full-validate-warning-only"
	phasePath := filepath.Join(festivalPath, "001_planning")
	sequencePath := filepath.Join(phasePath, "01_Design")

	exitCode, _, err = container.container.Exec(container.ctx, []string{"mkdir", "-p", sequencePath})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)

	writeFile := func(path, content string) {
		exitCode, _, err := container.container.Exec(container.ctx, []string{
			"sh", "-c",
			"printf '%s' '" + content + "' > " + path,
		})
		require.NoError(t, err)
		require.Equal(t, 0, exitCode)
	}

	writeFile(filepath.Join(festivalPath, "FESTIVAL_OVERVIEW.md"), "# Warning Only\n")
	writeFile(filepath.Join(festivalPath, "FESTIVAL_RULES.md"), "# Rules\n")
	writeFile(filepath.Join(phasePath, "PHASE_GOAL.md"), `---
fest_type: phase
fest_phase_type: planning
fest_status: pending
---
# Phase Goal
Actual content here.
`)
	writeFile(filepath.Join(sequencePath, "SEQUENCE_GOAL.md"), "# Sequence Goal\nActual content here.\n")
	writeFile(filepath.Join(sequencePath, "01_Task.md"), "# Task\nActual content here.\n")

	output, err := container.RunFest("validate", festivalPath, "--json")
	require.NoError(t, err, "full validate should not fail for warning-only findings")

	var result validateStructureOutput
	err = json.Unmarshal([]byte(strings.TrimSpace(output)), &result)
	require.NoError(t, err, "full validate output should be valid JSON")
	require.False(t, result.Valid, "full validate should report valid=false when warnings are present")
	require.NotEmpty(t, result.Issues, "warning-only validate result should include issues")
}
