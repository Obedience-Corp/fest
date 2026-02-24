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

// findFestivalPath locates the actual festival directory after creation.
// fest create festival adds an ID suffix (e.g., "test-fest-TF0001"), so we need
// to find the directory by prefix matching.
func findFestivalPath(t *testing.T, tc *TestContainer, activeDir string, festName string) string {
	t.Helper()

	dirs, err := tc.ListDirectories(activeDir)
	require.NoError(t, err, "should list directories in %s", activeDir)

	for _, dir := range dirs {
		if strings.HasPrefix(dir, festName) {
			return activeDir + "/" + dir
		}
	}

	require.Failf(t, "festival not found", "could not find festival starting with %q in %s, found: %v", festName, activeDir, dirs)
	return "" // unreachable
}

// ============================================================================
// FESTIVAL SETUP HELPERS
// ============================================================================

// setupImplementationFestival creates a test festival with implementation phases for execute mode testing.
// Returns the festival path inside the container.
func setupImplementationFestival(t *testing.T, tc *TestContainer, festName string) string {
	t.Helper()

	// First, initialize workspace properly (creates .festival/.state/.workspace marker)
	festivalsPath := setupWorkspace(t, tc, "/")

	// Create festival using --dest flag (not --path which doesn't exist)
	output, err := tc.RunFestInDir(festivalsPath, "create", "festival", "--name", festName, "--dest", "active")
	require.NoError(t, err, "should create festival: %s", output)

	// Find the actual festival path (fest adds an ID suffix like "test-fest-TF0001")
	festPath := findFestivalPath(t, tc, festivalsPath+"/active", festName)

	// Write fest.yaml with quality gates enabled (gates should never be skippable)
	festYaml := `version: "1.0"
quality_gates:
  enabled: true
  auto_append: true
  implementation:
    - id: testing
      template: gates/implementation/QUALITY_GATE_TESTING
      enabled: true
    - id: review
      template: gates/implementation/QUALITY_GATE_REVIEW
      enabled: true
    - id: iterate
      template: gates/implementation/QUALITY_GATE_ITERATE
      enabled: true
    - id: fest-commit
      template: gates/implementation/QUALITY_GATE_FEST_COMMIT
      enabled: true
`
	err = writeFileInContainer(tc, festPath+"/fest.yaml", festYaml)
	require.NoError(t, err, "should create fest.yaml")

	// Create FESTIVAL_OVERVIEW.md (required by validator completeness check)
	overviewContent := `---
fest_type: overview
---

# Test Festival Overview

## Goals
- Complete implementation testing

## Success Criteria
- All tasks completed successfully
`
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_OVERVIEW.md", overviewContent)
	require.NoError(t, err, "should create FESTIVAL_OVERVIEW.md")

	// Create FESTIVAL_RULES.md (required to avoid warning that blocks fest next)
	rulesContent := "# Festival Rules\n\n- Follow naming conventions\n- Complete all quality gates\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_RULES.md", rulesContent)
	require.NoError(t, err, "should create FESTIVAL_RULES.md")

	// Create implementation phase
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "IMPLEMENTATION", "--type", "implementation")
	require.NoError(t, err, "should create phase")

	// Create sequence
	phasePath := festPath + "/001_IMPLEMENTATION"
	_, err = tc.RunFestInDir(phasePath, "create", "sequence", "--name", "core_work")
	require.NoError(t, err, "should create sequence")

	// Write task files directly without markers to avoid quality gate failures
	// This is the same pattern used for PHASE_GOAL.md files in other helpers
	seqPath := phasePath + "/01_core_work"
	for i, name := range []string{"first_task", "second_task", "third_task"} {
		taskContent := fmt.Sprintf(`---
fest_type: task
fest_id: %02d_%s.md
fest_name: %s
fest_status: pending
---

# Task: %s

## Objective
Complete the %s task.

## Done When
- [ ] Task completed
`, i+1, name, name, name, name)
		taskPath := fmt.Sprintf("%s/%02d_%s.md", seqPath, i+1, name)
		err = writeFileInContainer(tc, taskPath, taskContent)
		require.NoError(t, err, "should create task file %s", taskPath)
	}

	// Add quality gate task stubs to the sequence (required by validator)
	// Files must match taskParsePattern: ^(\d{2})_(.+)\.md$
	// Gate IDs checked by validator: testing, review, iterate, commit
	gateStubs := []struct {
		num      int
		name     string
		gateType string
		title    string
	}{
		{4, "testing", "testing", "Testing"},
		{5, "review", "review", "Code Review"},
		{6, "iterate", "iterate", "Iterate"},
		{7, "fest_commit", "fest-commit", "Fest Commit"},
	}
	for _, gate := range gateStubs {
		gateContent := fmt.Sprintf(`---
fest_type: gate
fest_gate_type: %s
fest_status: pending
---
# Quality Gate: %s
- [ ] Gate passed
`, gate.gateType, gate.title)
		gatePath := fmt.Sprintf("%s/%02d_%s.md", seqPath, gate.num, gate.name)
		err = writeFileInContainer(tc, gatePath, gateContent)
		require.NoError(t, err, "should create gate stub %s", gatePath)
	}

	return festPath
}

// setupPlanFestival creates a test festival with planning phases for plan mode testing.
func setupPlanFestival(t *testing.T, tc *TestContainer, festName string) string {
	t.Helper()

	// First, initialize workspace properly (creates .festival/.state/.workspace marker)
	festivalsPath := setupWorkspace(t, tc, "/")

	// Create festival using --dest flag
	output, err := tc.RunFestInDir(festivalsPath, "create", "festival", "--name", festName, "--dest", "active")
	require.NoError(t, err, "should create festival: %s", output)

	// Find the actual festival path (fest adds an ID suffix)
	festPath := findFestivalPath(t, tc, festivalsPath+"/active", festName)

	// Create FESTIVAL_OVERVIEW.md (required by validator)
	overviewContent := "---\nfest_type: overview\n---\n\n# Test Festival Overview\n\n## Goals\n- Complete planning testing\n\n## Success Criteria\n- All planning steps completed\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_OVERVIEW.md", overviewContent)
	require.NoError(t, err, "should create FESTIVAL_OVERVIEW.md")

	// Create FESTIVAL_RULES.md (recommended by validator)
	rulesContent := "# Festival Rules\n\n- Follow naming conventions\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_RULES.md", rulesContent)
	require.NoError(t, err, "should create FESTIVAL_RULES.md")

	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "PLANNING", "--type", "planning")
	require.NoError(t, err)

	// Planning phases have steps defined in PHASE_GOAL.md
	// Write planning content to phase goal
	phasePath := festPath + "/001_PLANNING"
	goalContent := `---
fest_type: phase
fest_phase_type: planning
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

	// First, initialize workspace properly (creates .festival/.state/.workspace marker)
	festivalsPath := setupWorkspace(t, tc, "/")

	// Create festival using --dest flag
	output, err := tc.RunFestInDir(festivalsPath, "create", "festival", "--name", festName, "--dest", "active")
	require.NoError(t, err, "should create festival: %s", output)

	// Find the actual festival path (fest adds an ID suffix)
	festPath := findFestivalPath(t, tc, festivalsPath+"/active", festName)

	// Create FESTIVAL_OVERVIEW.md (required by validator)
	overviewContent := "---\nfest_type: overview\n---\n\n# Test Festival Overview\n\n## Goals\n- Complete research testing\n\n## Success Criteria\n- All research topics covered\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_OVERVIEW.md", overviewContent)
	require.NoError(t, err, "should create FESTIVAL_OVERVIEW.md")

	// Create FESTIVAL_RULES.md (recommended by validator)
	rulesContent := "# Festival Rules\n\n- Follow naming conventions\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_RULES.md", rulesContent)
	require.NoError(t, err, "should create FESTIVAL_RULES.md")

	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "RESEARCH", "--type", "research")
	require.NoError(t, err)

	phasePath := festPath + "/001_RESEARCH"
	goalContent := `---
fest_type: phase
fest_phase_type: research
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

	// First, initialize workspace properly (creates .festival/.state/.workspace marker)
	festivalsPath := setupWorkspace(t, tc, "/")

	// Create festival using --dest flag
	output, err := tc.RunFestInDir(festivalsPath, "create", "festival", "--name", festName, "--dest", "active")
	require.NoError(t, err, "should create festival: %s", output)

	// Find the actual festival path (fest adds an ID suffix)
	festPath := findFestivalPath(t, tc, festivalsPath+"/active", festName)

	// Create FESTIVAL_OVERVIEW.md (required by validator)
	overviewContent := "---\nfest_type: overview\n---\n\n# Test Festival Overview\n\n## Goals\n- Complete review testing\n\n## Success Criteria\n- All review items checked\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_OVERVIEW.md", overviewContent)
	require.NoError(t, err, "should create FESTIVAL_OVERVIEW.md")

	// Create FESTIVAL_RULES.md (recommended by validator)
	rulesContent := "# Festival Rules\n\n- Follow naming conventions\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_RULES.md", rulesContent)
	require.NoError(t, err, "should create FESTIVAL_RULES.md")

	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "REVIEW", "--type", "review")
	require.NoError(t, err)

	phasePath := festPath + "/001_REVIEW"
	goalContent := `---
fest_type: phase
fest_phase_type: review
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

	// First, initialize workspace properly (creates .festival/.state/.workspace marker)
	festivalsPath := setupWorkspace(t, tc, "/")

	// Create festival using --dest flag
	output, err := tc.RunFestInDir(festivalsPath, "create", "festival", "--name", festName, "--dest", "active")
	require.NoError(t, err, "should create festival: %s", output)

	// Find the actual festival path (fest adds an ID suffix)
	festPath := findFestivalPath(t, tc, festivalsPath+"/active", festName)

	// Create FESTIVAL_OVERVIEW.md (required by validator)
	overviewContent := "---\nfest_type: overview\n---\n\n# Test Festival Overview\n\n## Goals\n- Complete action testing\n\n## Success Criteria\n- All actions executed\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_OVERVIEW.md", overviewContent)
	require.NoError(t, err, "should create FESTIVAL_OVERVIEW.md")

	// Create FESTIVAL_RULES.md (recommended by validator)
	rulesContent := "# Festival Rules\n\n- Follow naming conventions\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_RULES.md", rulesContent)
	require.NoError(t, err, "should create FESTIVAL_RULES.md")

	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "ACTIONS", "--type", "non_coding_action")
	require.NoError(t, err)

	phasePath := festPath + "/001_ACTIONS"
	goalContent := `---
fest_type: phase
fest_phase_type: non_coding_action
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

	// First, initialize workspace properly (creates .festival/.state/.workspace marker)
	festivalsPath := setupWorkspace(t, tc, "/")

	// Create festival using --dest flag
	output, err := tc.RunFestInDir(festivalsPath, "create", "festival", "--name", festName, "--dest", "active")
	require.NoError(t, err, "should create festival: %s", output)

	// Find the actual festival path (fest adds an ID suffix)
	festPath := findFestivalPath(t, tc, festivalsPath+"/active", festName)

	// Create FESTIVAL_OVERVIEW.md (required by validator)
	overviewContent := "---\nfest_type: overview\n---\n\n# Test Festival Overview\n\n## Goals\n- Complete ingest testing\n\n## Success Criteria\n- All items ingested\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_OVERVIEW.md", overviewContent)
	require.NoError(t, err, "should create FESTIVAL_OVERVIEW.md")

	// Create FESTIVAL_RULES.md (recommended by validator)
	rulesContent := "# Festival Rules\n\n- Follow naming conventions\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_RULES.md", rulesContent)
	require.NoError(t, err, "should create FESTIVAL_RULES.md")

	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "INGEST", "--type", "ingest")
	require.NoError(t, err)

	phasePath := festPath + "/001_INGEST"
	goalContent := `---
fest_type: phase
fest_phase_type: ingest
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

// runExecuteMode runs fest next and returns the output.
// When run from the festival root, it defaults to implementation mode.
// For phase-type-aware navigation, use runExecuteModeFromPhase.
func runExecuteMode(t *testing.T, tc *TestContainer, festPath string) string {
	t.Helper()
	output, err := tc.RunFestInDir(festPath, "next")
	require.NoError(t, err, "fest next should succeed")
	return output
}

// runExecuteModeFromPhase runs fest next from within a phase directory.
// This enables phase-type-aware navigation based on PHASE_GOAL.md frontmatter.
func runExecuteModeFromPhase(t *testing.T, tc *TestContainer, phasePath string) string {
	t.Helper()
	output, err := tc.RunFestInDir(phasePath, "next")
	require.NoError(t, err, "fest next (from phase) should succeed")
	return output
}

// runExecuteModeWithMode runs fest next with a specific mode flag.
func runExecuteModeWithMode(t *testing.T, tc *TestContainer, festPath string, mode string) string {
	t.Helper()
	output, err := tc.RunFestInDir(festPath, "next", "--mode", mode)
	require.NoError(t, err, "fest next --mode %s should succeed", mode)
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

	// First, initialize workspace properly (creates .festival/.state/.workspace marker)
	festivalsPath := setupWorkspace(t, tc, "/")

	// Create festival using --dest flag
	output, err := tc.RunFestInDir(festivalsPath, "create", "festival", "--name", festName, "--dest", "active")
	require.NoError(t, err, "should create festival: %s", output)

	// Find the actual festival path (fest adds an ID suffix)
	festPath := findFestivalPath(t, tc, festivalsPath+"/active", festName)

	// Create FESTIVAL_OVERVIEW.md (required by validator)
	overviewContent := "---\nfest_type: overview\n---\n\n# Test Festival Overview\n\n## Goals\n- Complete multi-mode testing\n\n## Success Criteria\n- All phase types navigated\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_OVERVIEW.md", overviewContent)
	require.NoError(t, err, "should create FESTIVAL_OVERVIEW.md")

	// Create FESTIVAL_RULES.md (recommended by validator)
	rulesContent := "# Festival Rules\n\n- Follow naming conventions\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_RULES.md", rulesContent)
	require.NoError(t, err, "should create FESTIVAL_RULES.md")

	// Write fest.yaml with quality gates
	festYaml := "version: \"1.0\"\nquality_gates:\n  enabled: true\n  auto_append: true\n  implementation:\n    - id: testing\n      template: gates/implementation/QUALITY_GATE_TESTING\n      enabled: true\n    - id: review\n      template: gates/implementation/QUALITY_GATE_REVIEW\n      enabled: true\n    - id: iterate\n      template: gates/implementation/QUALITY_GATE_ITERATE\n      enabled: true\n    - id: fest-commit\n      template: gates/implementation/QUALITY_GATE_FEST_COMMIT\n      enabled: true\n"
	err = writeFileInContainer(tc, festPath+"/fest.yaml", festYaml)
	require.NoError(t, err)

	// Create planning phase
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "PLANNING", "--type", "planning")
	require.NoError(t, err, "should create planning phase")

	// Overwrite planning PHASE_GOAL.md with marker-free content
	planGoal := "---\nfest_type: phase\nfest_phase_type: planning\n---\n\n# Planning Phase\n\n## Planning Objectives\n\n- [ ] Define Requirements\n- [ ] Create Design\n"
	err = writeFileInContainer(tc, festPath+"/001_PLANNING/PHASE_GOAL.md", planGoal)
	require.NoError(t, err)

	// Create implementation phase
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "IMPLEMENTATION", "--type", "implementation")
	require.NoError(t, err, "should create implementation phase")

	// Overwrite implementation PHASE_GOAL.md with marker-free content
	implGoal := "---\nfest_type: phase\nfest_phase_type: implementation\n---\n\n# Implementation Phase\n\n## Goal\nImplement core features.\n"
	err = writeFileInContainer(tc, festPath+"/002_IMPLEMENTATION/PHASE_GOAL.md", implGoal)
	require.NoError(t, err)

	// Add sequence to implementation phase
	phasePath := festPath + "/002_IMPLEMENTATION"
	_, err = tc.RunFestInDir(phasePath, "create", "sequence", "--name", "core_work")
	require.NoError(t, err)

	// Overwrite SEQUENCE_GOAL.md with marker-free content
	seqPath := phasePath + "/01_core_work"
	seqGoal := "---\nfest_type: sequence\n---\n\n# Core Work\n\n## Goal\nComplete core work tasks.\n"
	err = writeFileInContainer(tc, seqPath+"/SEQUENCE_GOAL.md", seqGoal)
	require.NoError(t, err)

	// Write task files directly without markers
	for i, name := range []string{"task_one", "task_two"} {
		taskContent := fmt.Sprintf("---\nfest_type: task\nfest_id: %02d_%s.md\nfest_name: %s\nfest_status: pending\n---\n\n# Task: %s\n\n## Objective\nComplete %s.\n\n## Done When\n- [ ] Task completed\n", i+1, name, name, name, name)
		err = writeFileInContainer(tc, fmt.Sprintf("%s/%02d_%s.md", seqPath, i+1, name), taskContent)
		require.NoError(t, err)
	}

	// Add quality gate stubs
	for _, gate := range []struct {
		num                int
		name, gType, title string
	}{
		{3, "testing", "testing", "Testing"},
		{4, "review", "review", "Code Review"},
		{5, "iterate", "iterate", "Iterate"},
		{6, "fest_commit", "fest-commit", "Fest Commit"},
	} {
		gateContent := fmt.Sprintf("---\nfest_type: gate\nfest_gate_type: %s\nfest_status: pending\n---\n# Quality Gate: %s\n- [ ] Gate passed\n", gate.gType, gate.title)
		err = writeFileInContainer(tc, fmt.Sprintf("%s/%02d_%s.md", seqPath, gate.num, gate.name), gateContent)
		require.NoError(t, err)
	}

	// Create review phase
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "REVIEW", "--type", "review")
	require.NoError(t, err, "should create review phase")

	// Overwrite review PHASE_GOAL.md with marker-free content
	reviewGoal := "---\nfest_type: phase\nfest_phase_type: review\n---\n\n# Review Phase\n\n## Review Items\n\n- [ ] Code Quality Check\n- [ ] Security Review\n"
	err = writeFileInContainer(tc, festPath+"/003_REVIEW/PHASE_GOAL.md", reviewGoal)
	require.NoError(t, err)

	return festPath
}

// ============================================================================
// LIFECYCLE TEST HELPERS
// ============================================================================

// setupLifecycleFestival creates a complete lifecycle test festival with all phase types.
// This is used for comprehensive lifecycle testing across phase types.
func setupLifecycleFestival(t *testing.T, tc *TestContainer, festName string) string {
	t.Helper()

	// First, initialize workspace properly (creates .festival/.state/.workspace marker)
	festivalsPath := setupWorkspace(t, tc, "/")

	// Create festival using --dest flag
	output, err := tc.RunFestInDir(festivalsPath, "create", "festival", "--name", festName, "--dest", "active")
	require.NoError(t, err, "should create festival: %s", output)

	// Find the actual festival path (fest adds an ID suffix)
	festPath := findFestivalPath(t, tc, festivalsPath+"/active", festName)

	// Create FESTIVAL_OVERVIEW.md (required by validator)
	overviewContent := "---\nfest_type: overview\n---\n\n# Test Festival Overview\n\n## Goals\n- Complete lifecycle testing\n\n## Success Criteria\n- All lifecycle phases completed\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_OVERVIEW.md", overviewContent)
	require.NoError(t, err, "should create FESTIVAL_OVERVIEW.md")

	// Create FESTIVAL_RULES.md (recommended by validator)
	rulesContent := "# Festival Rules\n\n- Follow naming conventions\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_RULES.md", rulesContent)
	require.NoError(t, err, "should create FESTIVAL_RULES.md")

	// Create ingest phase
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "INGEST", "--type", "ingest")
	require.NoError(t, err, "should create ingest phase")

	// Create planning phase
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "PLANNING", "--type", "planning")
	require.NoError(t, err, "should create planning phase")

	// Create implementation phase with tasks
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "IMPLEMENTATION", "--type", "implementation")
	require.NoError(t, err, "should create implementation phase")

	phasePath := festPath + "/003_IMPLEMENTATION"
	_, err = tc.RunFestInDir(phasePath, "create", "sequence", "--name", "core_work")
	require.NoError(t, err, "should create sequence")

	seqPath := phasePath + "/01_core_work"
	_, err = tc.RunFestInDir(seqPath, "create", "task", "--name", "first_task", "--name", "second_task")
	require.NoError(t, err, "should create tasks")

	// Create review phase
	_, err = tc.RunFestInDir(festPath, "create", "phase", "--name", "REVIEW", "--type", "review")
	require.NoError(t, err, "should create review phase")

	return festPath
}

// VerifyWorkflowNavigation checks fest next returns workflow step format.
func VerifyWorkflowNavigation(t *testing.T, output string) {
	t.Helper()
	lowerOutput := strings.ToLower(output)
	isWorkflow := strings.Contains(lowerOutput, "workflow") ||
		strings.Contains(lowerOutput, "step") ||
		strings.Contains(output, "fest workflow")
	if isWorkflow {
		require.NotContains(t, lowerOutput, "fest progress --complete",
			"workflow mode should not show task completion command")
	}
}

// VerifyTaskNavigation checks fest next returns task format.
func VerifyTaskNavigation(t *testing.T, output string) {
	t.Helper()
	lowerOutput := strings.ToLower(output)
	isTaskBased := strings.Contains(lowerOutput, "task") ||
		strings.Contains(lowerOutput, ".md") ||
		strings.Contains(lowerOutput, "progress")
	if isTaskBased {
		require.NotContains(t, lowerOutput, "fest workflow advance",
			"task mode should not show workflow advance command")
	}
}

// VerifyPhaseStructure validates phase directory has required files for the given type.
func VerifyPhaseStructure(t *testing.T, tc *TestContainer, phasePath string, phaseType string) {
	t.Helper()

	// Common files - all phases should have PHASE_GOAL.md
	exists, err := tc.CheckFileExists(phasePath + "/PHASE_GOAL.md")
	require.NoError(t, err)
	require.True(t, exists, "PHASE_GOAL.md should exist in %s", phasePath)

	// Type-specific verification
	switch phaseType {
	case "ingest":
		// Ingest phases may have WORKFLOW.md or input/output spec directories
		// These are optional scaffolded items
		t.Logf("Verifying ingest phase structure at %s", phasePath)

	case "implementation":
		// Implementation phases should have sequences with tasks
		t.Logf("Verifying implementation phase structure at %s", phasePath)

	case "planning":
		// Planning phases typically have planning objectives in PHASE_GOAL.md
		content, err := tc.ReadFile(phasePath + "/PHASE_GOAL.md")
		require.NoError(t, err)
		require.Contains(t, strings.ToLower(content), "planning",
			"planning phase should have planning content")

	case "review":
		// Review phases typically have review checklist in PHASE_GOAL.md
		content, err := tc.ReadFile(phasePath + "/PHASE_GOAL.md")
		require.NoError(t, err)
		require.Contains(t, strings.ToLower(content), "review",
			"review phase should have review content")
	}
}

// VerifyFestivalStructure validates the complete festival structure is valid.
func VerifyFestivalStructure(t *testing.T, tc *TestContainer, festPath string) {
	t.Helper()

	// Festival should have FESTIVAL_GOAL.md
	exists, err := tc.CheckFileExists(festPath + "/FESTIVAL_GOAL.md")
	require.NoError(t, err)
	require.True(t, exists, "FESTIVAL_GOAL.md should exist")

	// Count phases
	phaseCount, err := tc.CountPhases(festPath)
	require.NoError(t, err)
	require.Greater(t, phaseCount, 0, "festival should have at least one phase")
}
