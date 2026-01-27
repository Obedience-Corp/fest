//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forbiddenTokens are markers that indicate template rendering problems.
// These should never appear in CLI output.
var forbiddenTemplateTokens = []string{
	"{{.",        // Unrendered template variable
	"{{",         // Any unrendered template syntax
	"<no value>", // Go template's default for missing values
}

// TestExecuteTemplateOutputIsValid verifies that fest execute produces
// valid template output without unrendered tokens.
func TestExecuteTemplateOutputIsValid(t *testing.T) {
	tc := GetSharedContainer(t)

	// Create a minimal festival for testing
	_, err := tc.RunFest("create", "festival", "/test/template-test", "--id", "TEMPLATE_TEST")
	require.NoError(t, err, "failed to create festival")

	// Create a phase and sequence with a task
	_, err = tc.RunFestInDir("/test/template-test", "create", "phase", "planning", "--id", "PLANNING_PHASE")
	require.NoError(t, err, "failed to create phase")

	_, err = tc.RunFestInDir("/test/template-test/001_PLANNING", "create", "sequence", "setup", "--id", "SETUP_SEQ")
	require.NoError(t, err, "failed to create sequence")

	_, err = tc.RunFestInDir("/test/template-test/001_PLANNING/01_setup", "create", "task", "first_task", "--id", "FIRST_TASK")
	require.NoError(t, err, "failed to create task")

	// Run execute command - this uses the implementation/instructions template
	output, err := tc.RunFestInDir("/test/template-test", "execute")
	require.NoError(t, err, "fest execute failed")

	// Verify no forbidden tokens in output
	assertNoForbiddenTokens(t, output, "execute")

	// Verify output has expected structural elements (not specific content)
	assert.True(t, len(output) > 50, "execute output should not be empty")
}

// TestExecuteDryRunTemplateOutputIsValid verifies dry run output is valid.
func TestExecuteDryRunTemplateOutputIsValid(t *testing.T) {
	tc := GetSharedContainer(t)

	// Create a festival with multiple phases for dry run
	_, err := tc.RunFest("create", "festival", "/test/dryrun-test", "--id", "DRYRUN_TEST")
	require.NoError(t, err, "failed to create festival")

	_, err = tc.RunFestInDir("/test/dryrun-test", "create", "phase", "phase_one", "--id", "PHASE_ONE")
	require.NoError(t, err)

	_, err = tc.RunFestInDir("/test/dryrun-test/001_PHASE_ONE", "create", "sequence", "seq", "--id", "SEQ")
	require.NoError(t, err)

	_, err = tc.RunFestInDir("/test/dryrun-test/001_PHASE_ONE/01_seq", "create", "task", "task", "--id", "TASK")
	require.NoError(t, err)

	// Run execute with --dry-run flag
	output, err := tc.RunFestInDir("/test/dryrun-test", "execute", "--dry-run")
	require.NoError(t, err, "fest execute --dry-run failed")

	// Verify no forbidden tokens in output
	assertNoForbiddenTokens(t, output, "execute --dry-run")

	// Verify structural elements exist
	assert.True(t, len(output) > 50, "dry run output should not be empty")
}

// TestNextTemplateOutputIsValid verifies fest next produces valid output.
func TestNextTemplateOutputIsValid(t *testing.T) {
	tc := GetSharedContainer(t)

	// Create a festival with tasks
	_, err := tc.RunFest("create", "festival", "/test/next-test", "--id", "NEXT_TEST")
	require.NoError(t, err, "failed to create festival")

	_, err = tc.RunFestInDir("/test/next-test", "create", "phase", "impl", "--id", "IMPL")
	require.NoError(t, err)

	_, err = tc.RunFestInDir("/test/next-test/001_IMPL", "create", "sequence", "core", "--id", "CORE")
	require.NoError(t, err)

	_, err = tc.RunFestInDir("/test/next-test/001_IMPL/01_core", "create", "task", "build_it", "--id", "BUILD")
	require.NoError(t, err)

	// Run next command
	output, err := tc.RunFestInDir("/test/next-test", "next")
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
	_, err := tc.RunFest("create", "festival", "/test/validate-test", "--id", "VALIDATE_TEST")
	require.NoError(t, err, "failed to create festival")

	// Run validate command
	output, err := tc.RunFestInDir("/test/validate-test", "validate")
	// validate may fail due to empty festival - that's OK, we just check output format
	_ = err

	// Verify no forbidden tokens in output
	assertNoForbiddenTokens(t, output, "validate")
}

// TestStatusTemplateOutputIsValid verifies fest status produces valid output.
func TestStatusTemplateOutputIsValid(t *testing.T) {
	tc := GetSharedContainer(t)

	// Create a festival with content
	_, err := tc.RunFest("create", "festival", "/test/status-test", "--id", "STATUS_TEST")
	require.NoError(t, err, "failed to create festival")

	_, err = tc.RunFestInDir("/test/status-test", "create", "phase", "phase", "--id", "PHASE")
	require.NoError(t, err)

	// Run status command
	output, err := tc.RunFestInDir("/test/status-test", "status")
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
	_, err := tc.RunFest("create", "festival", "/test/allcmds-test", "--id", "ALLCMDS_TEST")
	require.NoError(t, err, "failed to create festival")

	_, err = tc.RunFestInDir("/test/allcmds-test", "create", "phase", "phase", "--id", "PHASE")
	require.NoError(t, err)

	_, err = tc.RunFestInDir("/test/allcmds-test/001_PHASE", "create", "sequence", "seq", "--id", "SEQ")
	require.NoError(t, err)

	_, err = tc.RunFestInDir("/test/allcmds-test/001_PHASE/01_seq", "create", "task", "task", "--id", "TASK")
	require.NoError(t, err)

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
			output, _ := tc.RunFestInDir("/test/allcmds-test", cmd.args...)
			// Commands may fail but we still want to check output format
			assertNoForbiddenTokens(t, output, cmd.name)
		})
	}
}

// TestCompleteFestivalTemplateOutput verifies the complete message template.
func TestCompleteFestivalTemplateOutput(t *testing.T) {
	tc := GetSharedContainer(t)

	// Create a minimal festival
	_, err := tc.RunFest("create", "festival", "/test/complete-test", "--id", "COMPLETE_TEST")
	require.NoError(t, err, "failed to create festival")

	_, err = tc.RunFestInDir("/test/complete-test", "create", "phase", "phase", "--id", "PHASE")
	require.NoError(t, err)

	_, err = tc.RunFestInDir("/test/complete-test/001_PHASE", "create", "sequence", "seq", "--id", "SEQ")
	require.NoError(t, err)

	_, err = tc.RunFestInDir("/test/complete-test/001_PHASE/01_seq", "create", "task", "only_task", "--id", "ONLY")
	require.NoError(t, err)

	// Mark the task as complete
	_, err = tc.RunFestInDir("/test/complete-test", "progress", "--task", "001_PHASE/01_seq/01_only_task.md", "--complete")
	require.NoError(t, err)

	// Run execute - should show completion message
	output, _ := tc.RunFestInDir("/test/complete-test", "execute")

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
