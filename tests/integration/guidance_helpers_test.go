//go:build integration
// +build integration

package integration

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ============================================================================
// FESTIVAL SETUP HELPERS
// ============================================================================

// setupImplementationFestival creates a test festival with implementation phases for execute mode testing.
// Returns the festival path inside the container.
func setupImplementationFestival(t *testing.T, tc *TestContainer, festName string) string {
	t.Helper()
	festPath := "/festivals/" + festName

	// Create festival
	_, err := tc.RunFest("create", "festival", "--name", festName, "--path", "/festivals")
	require.NoError(t, err, "should create festival")

	// Create implementation phase
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "IMPLEMENTATION", "--type", "implementation")
	require.NoError(t, err, "should create phase")

	// Create sequence
	phasePath := festPath + "/001_IMPLEMENTATION"
	_, err = tc.RunFestInDir(phasePath, "create", "sequence", "--name", "core_work")
	require.NoError(t, err, "should create sequence")

	// Create tasks
	seqPath := phasePath + "/01_core_work"
	_, err = tc.RunFestInDir(seqPath, "create", "task", "--name", "first_task")
	require.NoError(t, err, "should create first task")
	_, err = tc.RunFestInDir(seqPath, "create", "task", "--name", "second_task")
	require.NoError(t, err, "should create second task")
	_, err = tc.RunFestInDir(seqPath, "create", "task", "--name", "third_task")
	require.NoError(t, err, "should create third task")

	return festPath
}

// setupPlanFestival creates a test festival with planning phases for plan mode testing.
func setupPlanFestival(t *testing.T, tc *TestContainer, festName string) string {
	t.Helper()
	festPath := "/festivals/" + festName

	_, err := tc.RunFest("create", "festival", "--name", festName, "--path", "/festivals")
	require.NoError(t, err)

	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "PLANNING", "--type", "planning")
	require.NoError(t, err)

	// Planning phases have steps defined in PHASE_GOAL.md
	// Write planning content to phase goal
	phasePath := festPath + "/001_PLANNING"
	goalContent := `---
fest_type: phase
phase_type: planning
---

# Planning Phase

## Planning Objectives

- [ ] Define Requirements
- [ ] Create Design
- [ ] Get Approval
`
	err = writeFileInContainer(tc, phasePath+"/PHASE_GOAL.md", goalContent)
	require.NoError(t, err)

	return festPath
}

// setupResearchFestival creates a test festival with research phases for research mode testing.
func setupResearchFestival(t *testing.T, tc *TestContainer, festName string) string {
	t.Helper()
	festPath := "/festivals/" + festName

	_, err := tc.RunFest("create", "festival", "--name", festName, "--path", "/festivals")
	require.NoError(t, err)

	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "RESEARCH", "--type", "research")
	require.NoError(t, err)

	phasePath := festPath + "/001_RESEARCH"
	goalContent := `---
fest_type: phase
phase_type: research
---

# Research Phase

## Research Topics

- [ ] Market Analysis
- [ ] Technical Feasibility
`
	err = writeFileInContainer(tc, phasePath+"/PHASE_GOAL.md", goalContent)
	require.NoError(t, err)

	return festPath
}

// setupReviewFestival creates a test festival with review phases for review mode testing.
func setupReviewFestival(t *testing.T, tc *TestContainer, festName string) string {
	t.Helper()
	festPath := "/festivals/" + festName

	_, err := tc.RunFest("create", "festival", "--name", festName, "--path", "/festivals")
	require.NoError(t, err)

	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "REVIEW", "--type", "review")
	require.NoError(t, err)

	phasePath := festPath + "/001_REVIEW"
	goalContent := `---
fest_type: phase
phase_type: review
---

# Review Phase

## Review Items

- [ ] Code Quality Check
- [ ] Security Review
- [ ] Performance Check
`
	err = writeFileInContainer(tc, phasePath+"/PHASE_GOAL.md", goalContent)
	require.NoError(t, err)

	return festPath
}

// setupActionFestival creates a test festival with action phases for action mode testing.
func setupActionFestival(t *testing.T, tc *TestContainer, festName string) string {
	t.Helper()
	festPath := "/festivals/" + festName

	_, err := tc.RunFest("create", "festival", "--name", festName, "--path", "/festivals")
	require.NoError(t, err)

	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "ACTIONS", "--type", "action")
	require.NoError(t, err)

	phasePath := festPath + "/001_ACTIONS"
	goalContent := `---
fest_type: phase
phase_type: action
---

# Action Phase

## Actions

- [ ] Deploy to Staging
- [ ] Run Smoke Tests
- [ ] Deploy to Production
`
	err = writeFileInContainer(tc, phasePath+"/PHASE_GOAL.md", goalContent)
	require.NoError(t, err)

	return festPath
}

// setupIngestFestival creates a test festival with ingest phases for ingest mode testing.
func setupIngestFestival(t *testing.T, tc *TestContainer, festName string) string {
	t.Helper()
	festPath := "/festivals/" + festName

	_, err := tc.RunFest("create", "festival", "--name", festName, "--path", "/festivals")
	require.NoError(t, err)

	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "INGEST", "--type", "ingest")
	require.NoError(t, err)

	phasePath := festPath + "/001_INGEST"
	goalContent := `---
fest_type: phase
phase_type: ingest
---

# Ingest Phase

## Items to Ingest

- [ ] Import User Data
- [ ] Import Products
`
	err = writeFileInContainer(tc, phasePath+"/PHASE_GOAL.md", goalContent)
	require.NoError(t, err)

	return festPath
}

// ============================================================================
// STATE VERIFICATION HELPERS
// ============================================================================

// GuidanceState represents the persisted guidance state from .guidance_state file.
type GuidanceState struct {
	Mode           string         `yaml:"mode"`
	CurrentPhase   string         `yaml:"current_phase"`
	CurrentItem    string         `yaml:"current_item"`
	CompletedItems []string       `yaml:"completed_items"`
	Progress       map[string]any `yaml:"progress"`
}

// readGuidanceState reads and parses the .guidance_state file from a festival.
func readGuidanceState(t *testing.T, tc *TestContainer, festPath string) *GuidanceState {
	t.Helper()
	statePath := festPath + "/.guidance_state"

	content, err := tc.ReadFile(statePath)
	require.NoError(t, err, "should read guidance state file")

	var state GuidanceState
	err = yaml.Unmarshal([]byte(content), &state)
	require.NoError(t, err, "should parse guidance state YAML")

	return &state
}

// verifyStateExists checks that a .guidance_state file exists in the festival.
func verifyStateExists(t *testing.T, tc *TestContainer, festPath string) {
	t.Helper()
	exists, err := tc.CheckFileExists(festPath + "/.guidance_state")
	require.NoError(t, err)
	require.True(t, exists, "guidance state file should exist at %s/.guidance_state", festPath)
}

// verifyStateMode checks that the guidance state has the expected mode.
func verifyStateMode(t *testing.T, tc *TestContainer, festPath string, expectedMode string) {
	t.Helper()
	state := readGuidanceState(t, tc, festPath)
	require.Equal(t, expectedMode, state.Mode, "guidance mode should match")
}

// verifyCompletedItems checks that specific items are marked as completed.
func verifyCompletedItems(t *testing.T, tc *TestContainer, festPath string, expectedItems []string) {
	t.Helper()
	state := readGuidanceState(t, tc, festPath)
	for _, item := range expectedItems {
		require.Contains(t, state.CompletedItems, item, "item %s should be completed", item)
	}
}

// ============================================================================
// OUTPUT VERIFICATION HELPERS
// ============================================================================

// verifyOutputContains checks that command output contains all expected strings.
func verifyOutputContains(t *testing.T, output string, expectedStrings ...string) {
	t.Helper()
	for _, expected := range expectedStrings {
		require.Contains(t, output, expected, "output should contain: %s", expected)
	}
}

// verifyOutputNotContains checks that command output does NOT contain any of the specified strings.
func verifyOutputNotContains(t *testing.T, output string, unexpectedStrings ...string) {
	t.Helper()
	for _, unexpected := range unexpectedStrings {
		require.NotContains(t, output, unexpected, "output should NOT contain: %s", unexpected)
	}
}

// verifyCommandInOutput checks that the expected command appears in guidance output.
func verifyCommandInOutput(t *testing.T, output string, command string) {
	t.Helper()
	require.Contains(t, output, command, "output should show command: %s", command)
}

// extractCurrentTask extracts the current task name from execute mode output.
func extractCurrentTask(t *testing.T, output string) string {
	t.Helper()
	// Look for pattern like "Current Task: 01_first_task.md" or task names in output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Task") && strings.Contains(line, ".md") {
			// Try to extract the task name
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasSuffix(part, ".md") {
					return strings.TrimSpace(part)
				}
			}
		}
	}
	t.Logf("Could not find current task in output:\n%s", output)
	return ""
}

// ============================================================================
// COMMAND EXECUTION HELPERS
// ============================================================================

// runExecuteMode runs fest execute and returns the output.
func runExecuteMode(t *testing.T, tc *TestContainer, festPath string) string {
	t.Helper()
	output, err := tc.RunFestInDir(festPath, "execute")
	require.NoError(t, err, "fest execute should succeed")
	return output
}

// runExecuteModeWithMode runs fest execute with a specific mode flag.
func runExecuteModeWithMode(t *testing.T, tc *TestContainer, festPath string, mode string) string {
	t.Helper()
	output, err := tc.RunFestInDir(festPath, "execute", "--mode", mode)
	require.NoError(t, err, "fest execute --mode %s should succeed", mode)
	return output
}

// runRoadmap runs fest execute --roadmap and returns the output.
func runRoadmap(t *testing.T, tc *TestContainer, festPath string) string {
	t.Helper()
	output, err := tc.RunFestInDir(festPath, "execute", "--roadmap")
	require.NoError(t, err, "fest execute --roadmap should succeed")
	return output
}

// runNext runs fest next and returns the output.
func runNext(t *testing.T, tc *TestContainer, festPath string) string {
	t.Helper()
	output, err := tc.RunFestInDir(festPath, "next")
	require.NoError(t, err, "fest next should succeed")
	return output
}

// runProgress runs fest progress with the specified flags.
func runProgress(t *testing.T, tc *TestContainer, festPath string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"progress"}, args...)
	output, err := tc.RunFestInDir(festPath, cmdArgs...)
	require.NoError(t, err, "fest progress should succeed")
	return output
}

// ============================================================================
// CONTAINER FILE HELPERS
// ============================================================================

// writeFileInContainer writes content to a file in the container using heredoc.
func writeFileInContainer(tc *TestContainer, path, content string) error {
	// Use heredoc to write file content safely
	cmd := fmt.Sprintf("cat > %s << 'EOFMARKER'\n%s\nEOFMARKER", path, content)
	_, err := tc.runCommand([]string{"sh", "-c", cmd})
	return err
}

// appendFileInContainer appends content to a file in the container.
func appendFileInContainer(tc *TestContainer, path, content string) error {
	cmd := fmt.Sprintf("cat >> %s << 'EOFMARKER'\n%s\nEOFMARKER", path, content)
	_, err := tc.runCommand([]string{"sh", "-c", cmd})
	return err
}

// ============================================================================
// MULTI-MODE FESTIVAL SETUP
// ============================================================================

// setupMultiModeFestival creates a festival with multiple phase types for comprehensive testing.
func setupMultiModeFestival(t *testing.T, tc *TestContainer, festName string) string {
	t.Helper()
	festPath := "/festivals/" + festName

	// Create festival
	_, err := tc.RunFest("create", "festival", "--name", festName, "--path", "/festivals")
	require.NoError(t, err, "should create festival")

	// Create planning phase
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "PLANNING", "--type", "planning")
	require.NoError(t, err, "should create planning phase")

	// Create implementation phase
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "IMPLEMENTATION", "--type", "implementation")
	require.NoError(t, err, "should create implementation phase")

	// Add sequence and tasks to implementation phase
	phasePath := festPath + "/002_IMPLEMENTATION"
	_, err = tc.RunFestInDir(phasePath, "create", "sequence", "--name", "core_work")
	require.NoError(t, err)

	seqPath := phasePath + "/01_core_work"
	_, err = tc.RunFestInDir(seqPath, "create", "task", "--name", "task_one")
	require.NoError(t, err)
	_, err = tc.RunFestInDir(seqPath, "create", "task", "--name", "task_two")
	require.NoError(t, err)

	// Create review phase
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "REVIEW", "--type", "review")
	require.NoError(t, err, "should create review phase")

	return festPath
}
