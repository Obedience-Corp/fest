//go:build integration
// +build integration

package integration

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forbiddenTokens are markers that indicate template rendering problems.
// These should never appear in CLI output.
// Note: We check for "{{." specifically (not "{{") because some commands
// legitimately show "{{ placeholder }}" as documentation for marker syntax.
var forbiddenTemplateTokens = []string{
	"{{.",        // Unrendered template variable like {{.Name}}
	"<no value>", // Go template's default for missing values
}

// setupTemplateFestival creates a festival with implementation phase for template testing.
// Returns the festival path and the workspace's festivals/ root so callers can
// promote the festival to active when they need to bypass the pre-active
// lifecycle gate.
func setupTemplateFestival(t *testing.T, tc *TestContainer, festName string) (string, string) {
	t.Helper()

	// Initialize workspace properly
	festivalsPath := setupWorkspace(t, tc, "/")

	// Create festival using correct flags
	_, err := tc.RunFestInDir(festivalsPath, "create", "festival", "--name", festName, "--dest", "planning")
	require.NoError(t, err, "failed to create festival")

	// Find the actual festival path (fest adds an ID suffix)
	festPath := findFestivalPath(t, tc, festivalsPath+"/planning", festName)

	// Append quality_gates and disable auto_link without overwriting the
	// metadata block that fest create festival wrote (status_history is
	// required by the pre-active lifecycle gate and fest status set).
	err = ensureQualityGatesInFestivalConfig(tc, festPath)
	require.NoError(t, err, "should ensure quality gates in fest.yaml")

	// Create FESTIVAL_OVERVIEW.md (required by validator completeness check)
	overviewContent := `---
fest_type: overview
---

# Test Festival Overview

## Goals
- Complete template testing

## Success Criteria
- All templates render correctly
`
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_OVERVIEW.md", overviewContent)
	require.NoError(t, err, "should create FESTIVAL_OVERVIEW.md")

	// Create FESTIVAL_RULES.md (required to avoid warning that blocks fest next)
	rulesContent := "# Festival Rules\n\n- Follow naming conventions\n- Complete all quality gates\n"
	err = writeFileInContainer(tc, festPath+"/FESTIVAL_RULES.md", rulesContent)
	require.NoError(t, err, "should create FESTIVAL_RULES.md")

	return festPath, festivalsPath
}

// TestNextTemplateOutputIsValidBasic verifies that fest next produces
// valid template output without unrendered tokens.
func TestNextTemplateOutputIsValidBasic(t *testing.T) {
	tc := GetSharedContainer(t)

	// Create a minimal festival for testing
	festPath, festivalsPath := setupTemplateFestival(t, tc, "template-test")

	// Create an implementation phase with a task
	_, err := tc.RunFestInDir(festPath, "create", "phase", "--name", "PLANNING", "--type", "implementation")
	require.NoError(t, err, "failed to create phase")

	phasePath := festPath + "/001_PLANNING"
	_, err = tc.RunFestInDir(phasePath, "create", "sequence", "--name", "setup")
	require.NoError(t, err, "failed to create sequence")

	seqPath := phasePath + "/01_setup"
	_, err = tc.RunFestInDir(seqPath, "create", "task", "--name", "first_task")
	require.NoError(t, err, "failed to create task")

	// Ensure gate templates exist and apply quality gates
	ensureGateTemplates(t, tc, festPath)
	_, err = tc.RunFestInDir(festPath, "gates", "apply", "--approve")
	require.NoError(t, err, "failed to apply quality gates")

	// Replace markers across entire festival (simulates user/agent filling them in)
	err = replaceMarkersInContainer(tc, festPath)
	require.NoError(t, err, "failed to replace markers")

	// Promote to active so the pre-active lifecycle gate allows
	// fest next on an implementation phase.
	festPath = promoteToActive(t, tc, festivalsPath, festPath)

	// Run next command - this uses the implementation/instructions template
	output, err := tc.RunFestInDir(festPath, "next")
	require.NoError(t, err, "fest next failed")

	// Verify no forbidden tokens in output
	assertNoForbiddenTokens(t, output, "next")

	// Verify output has expected structural elements (not specific content)
	assert.True(t, len(output) > 50, "next output should not be empty")
}

// TestNextTemplateOutputIsValid verifies fest next produces valid output.
func TestNextTemplateOutputIsValid(t *testing.T) {
	tc := GetSharedContainer(t)

	// Create a festival with tasks
	festPath, festivalsPath := setupTemplateFestival(t, tc, "next-test")

	_, err := tc.RunFestInDir(festPath, "create", "phase", "--name", "IMPL", "--type", "implementation")
	require.NoError(t, err)

	phasePath := festPath + "/001_IMPL"
	_, err = tc.RunFestInDir(phasePath, "create", "sequence", "--name", "core")
	require.NoError(t, err)

	seqPath := phasePath + "/01_core"
	_, err = tc.RunFestInDir(seqPath, "create", "task", "--name", "build_it", "--skip-markers")
	require.NoError(t, err)

	// Add quality gate task stubs (required by validator for fest next)
	// Files must match taskParsePattern: ^(\d{2})_(.+)\.md$
	// Frontmatter must include fest_managed + fest_gate_id (used by gates.GetGateID)
	for _, gate := range []struct {
		num      int
		name, id string
		title    string
	}{
		{2, "testing", "testing", "Testing"},
		{3, "review", "review", "Code Review"},
		{4, "iterate", "iterate", "Iterate"},
		{5, "fest_commit", "fest-commit", "Fest Commit"},
	} {
		gateContent := fmt.Sprintf("---\nfest_managed: true\nfest_gate_id: %s\nfest_type: gate\nfest_status: pending\n---\n# Quality Gate: %s\n- [ ] Gate passed\n", gate.id, gate.title)
		err = writeFileInContainer(tc, fmt.Sprintf("%s/%02d_%s.md", seqPath, gate.num, gate.name), gateContent)
		require.NoError(t, err)
	}

	// Replace any scaffold template markers so "fest next" can execute.
	err = replaceMarkersInContainer(tc, festPath)
	require.NoError(t, err, "failed to replace markers")

	// Promote to active so the pre-active lifecycle gate allows
	// fest next on an implementation phase.
	festPath = promoteToActive(t, tc, festivalsPath, festPath)

	// Run next command
	output, err := tc.RunFestInDir(festPath, "next")
	require.NoError(t, err, "fest next failed")

	// Verify no forbidden tokens in output
	assertNoForbiddenTokens(t, output, "next")

	// Verify output has content
	assert.True(t, len(output) > 20, "next output should not be empty")
}

// TestValidateTemplateOutputIsValid verifies fest validate produces valid output.
func TestValidateTemplateOutputIsValid(t *testing.T) {
	tc := GetSharedContainer(t)

	// Create a festival
	festPath, _ := setupTemplateFestival(t, tc, "validate-test")

	// Run validate command
	output, err := tc.RunFestInDir(festPath, "validate")
	// validate may fail due to empty festival - that's OK, we just check output format
	_ = err

	// Verify no forbidden tokens in output
	assertNoForbiddenTokens(t, output, "validate")
}

// TestStatusTemplateOutputIsValid verifies fest status produces valid output.
func TestStatusTemplateOutputIsValid(t *testing.T) {
	tc := GetSharedContainer(t)

	// Create a festival with content
	festPath, _ := setupTemplateFestival(t, tc, "status-test")

	_, err := tc.RunFestInDir(festPath, "create", "phase", "--name", "PHASE", "--type", "implementation")
	require.NoError(t, err)

	// Run status command
	output, err := tc.RunFestInDir(festPath, "status")
	require.NoError(t, err, "fest status failed")

	// Verify no forbidden tokens in output
	assertNoForbiddenTokens(t, output, "status")

	// Verify output has content
	assert.True(t, len(output) > 10, "status output should not be empty")
}

// TestAllCommandsProduceValidOutput runs common commands and verifies output structure.
func TestAllCommandsProduceValidOutput(t *testing.T) {
	tc := GetSharedContainer(t)

	// Create base festival
	festPath, _ := setupTemplateFestival(t, tc, "allcmds-test")

	_, err := tc.RunFestInDir(festPath, "create", "phase", "--name", "PHASE", "--type", "implementation")
	require.NoError(t, err)

	phasePath := festPath + "/001_PHASE"
	_, err = tc.RunFestInDir(phasePath, "create", "sequence", "--name", "seq")
	require.NoError(t, err)

	seqPath := phasePath + "/01_seq"
	_, err = tc.RunFestInDir(seqPath, "create", "task", "--name", "task", "--skip-markers")
	require.NoError(t, err)

	// Add quality gate task stubs (required by validator for fest next)
	// Files must match taskParsePattern: ^(\d{2})_(.+)\.md$
	// Frontmatter must include fest_managed + fest_gate_id (used by gates.GetGateID)
	for _, gate := range []struct {
		num      int
		name, id string
		title    string
	}{
		{2, "testing", "testing", "Testing"},
		{3, "review", "review", "Code Review"},
		{4, "iterate", "iterate", "Iterate"},
		{5, "fest_commit", "fest-commit", "Fest Commit"},
	} {
		gateContent := fmt.Sprintf("---\nfest_managed: true\nfest_gate_id: %s\nfest_type: gate\nfest_status: pending\n---\n# Quality Gate: %s\n- [ ] Gate passed\n", gate.id, gate.title)
		err = writeFileInContainer(tc, fmt.Sprintf("%s/%02d_%s.md", seqPath, gate.num, gate.name), gateContent)
		require.NoError(t, err)
	}

	// Test various commands that produce templated output
	commands := []struct {
		name string
		args []string
	}{
		{"execute", []string{"execute"}},
		{"execute-dry-run", []string{"execute", "--dry-run"}},
		{"next", []string{"next"}},
		{"status", []string{"status"}},
		{"validate", []string{"validate"}},
	}

	for _, cmd := range commands {
		t.Run(cmd.name, func(t *testing.T) {
			output, _ := tc.RunFestInDir(festPath, cmd.args...)
			// Commands may fail but we still want to check output format
			assertNoForbiddenTokens(t, output, cmd.name)
		})
	}
}

// TestCompleteFestivalTemplateOutput verifies the complete message template.
func TestCompleteFestivalTemplateOutput(t *testing.T) {
	tc := GetSharedContainer(t)

	// Create a minimal festival
	festPath, festivalsPath := setupTemplateFestival(t, tc, "complete-test")

	_, err := tc.RunFestInDir(festPath, "create", "phase", "--name", "PHASE", "--type", "implementation")
	require.NoError(t, err)

	phasePath := festPath + "/001_PHASE"
	_, err = tc.RunFestInDir(phasePath, "create", "sequence", "--name", "seq")
	require.NoError(t, err)

	seqPath := phasePath + "/01_seq"

	// Write task file directly without markers to pass quality gates
	taskContent := `---
fest_type: task
fest_id: 01_only_task.md
fest_name: only_task
fest_status: pending
---

# Task: only_task

## Objective
Complete the only_task task.

## Done When
- [ ] Task completed
`
	err = writeFileInContainer(tc, seqPath+"/01_only_task.md", taskContent)
	require.NoError(t, err)

	// Promote to active so the pre-active lifecycle gate allows
	// progress mutations on the implementation phase.
	festPath = promoteToActive(t, tc, festivalsPath, festPath)

	// Mark the task as complete
	_, err = tc.RunFestInDir(festPath, "progress", "--task", "001_PHASE/01_seq/01_only_task.md", "--complete")
	require.NoError(t, err)

	// Run execute - should show completion message
	output, _ := tc.RunFestInDir(festPath, "execute")

	// Verify no forbidden tokens in complete message
	assertNoForbiddenTokens(t, output, "execute (complete)")
}

// assertNoForbiddenTokens checks that output doesn't contain template rendering problems.
func assertNoForbiddenTokens(t *testing.T, output, cmdName string) {
	t.Helper()
	for _, token := range forbiddenTemplateTokens {
		if strings.Contains(output, token) {
			t.Errorf("command %q output contains forbidden token %q:\n%s",
				cmdName, token, truncateOutput(output, 500))
		}
	}
}

// truncateOutput limits output length for error messages.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}
