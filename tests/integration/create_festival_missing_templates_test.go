//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCreateFestivalErrorsOnMissingCoreTemplates is a regression test for
// fest#139: fest create festival silently skipped missing core templates and
// reported success. The fix makes missing core templates a hard error so the
// user gets a clear, actionable message instead of a broken scaffold.
func TestCreateFestivalErrorsOnMissingCoreTemplates(t *testing.T) {
	tc := GetSharedContainer(t)
	// Set up a workspace with NO festival templates at all.
	festivalsPath := setupWorkspace(t, tc, "/missing-templates")

	// Non-agent, non-JSON mode: must exit non-zero with a human-readable error
	// and leave no festival directory behind (plain create used to skip rollback).
	output, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "no-templates",
		"--no-color",
	)
	require.Error(t, err, "create must fail when core templates are missing")
	require.Contains(t, output, "missing required core festival templates",
		"error message must name the missing core templates, got: %s", output)
	requireNoPlanningFestivals(t, tc, festivalsPath)

	// JSON mode: must report the error in structured output with ok=false.
	// (Non-agent --json exits 0 by convention; the error is in the payload.)
	jsonOutput, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "no-templates-json",
		"--json",
	)
	_ = err // non-agent --json may exit 0 even on error (existing convention)
	var jsonResult createFestivalMissingTemplateResult
	require.NoError(t, json.Unmarshal([]byte(jsonOutput), &jsonResult), "JSON output should parse: %s", jsonOutput)
	require.False(t, jsonResult.OK, "JSON result must report ok=false, got: %s", jsonOutput)
	require.NotEmpty(t, jsonResult.Errors, "JSON result must contain error details")
	require.Contains(t, jsonResult.Errors[0].Message, "missing required core festival templates",
		"JSON error message must name the missing templates")
	requireNoPlanningFestivals(t, tc, festivalsPath)

	// Agent mode: must exit non-zero with structured failure output and rollback.
	agentOutput, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "no-templates-agent",
		"--agent",
	)
	require.Error(t, err, "agent create must fail when core templates are missing")

	var agentResult createFestivalMissingTemplateResult
	require.NoError(t, json.Unmarshal([]byte(agentOutput), &agentResult), "agent output should parse: %s", agentOutput)
	require.False(t, agentResult.OK)
	require.NotEmpty(t, agentResult.Errors)
	require.Contains(t, agentResult.Errors[0].Message, "missing required core festival templates")

	// Agent mode must roll back the scaffolded directory.
	require.NotNil(t, agentResult.RolledBack, "agent failure must report rollback status")
	require.True(t, *agentResult.RolledBack, "agent failure must roll back the scaffolded directory")
	requireNoPlanningFestivals(t, tc, festivalsPath)
}

// TestCreateFestivalErrorsOnPartialCoreTemplatesLeavesNoDestDir is the
// half-written-scaffold regression: three of four core templates used to be
// written before the missing one failed the create, leaving planning/<slug>-<id>/.
func TestCreateFestivalErrorsOnPartialCoreTemplatesLeavesNoDestDir(t *testing.T) {
	tc := GetSharedContainer(t)
	festivalsPath := setupWorkspace(t, tc, "/partial-templates")

	for _, name := range []string{"OVERVIEW.md", "GOAL.md", "RULES.md"} {
		path := festivalsPath + "/.festival/templates/festival/" + name
		require.NoError(t, tc.WriteFile(path, "# "+name+"\n"), "write partial core template %s", name)
	}

	output, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "partial-templates",
		"--no-color",
	)
	require.Error(t, err, "create must fail when a core template is missing")
	require.Contains(t, output, "missing required core festival templates",
		"error message must name the missing core templates, got: %s", output)
	require.Contains(t, output, "festival/TODO.md",
		"error message must name the missing TODO template, got: %s", output)
	requireNoPlanningFestivals(t, tc, festivalsPath)
}

// TestCreateFestivalDryRunReportsMissingCoreTemplates verifies that --dry-run
// surfaces missing core templates instead of silently omitting them from the
// preview tree (fest#139).
func TestCreateFestivalDryRunReportsMissingCoreTemplates(t *testing.T) {
	tc := GetSharedContainer(t)
	festivalsPath := setupWorkspace(t, tc, "/missing-templates-dry-run")

	// JSON dry-run must list missing core templates.
	output, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "missing-dry",
		"--dry-run",
		"--agent",
	)
	require.NoError(t, err, "dry-run should succeed even with missing templates: %s", output)

	var preview createFestivalPreviewResultWithMissing
	require.NoError(t, json.Unmarshal([]byte(output), &preview), "dry-run JSON should parse: %s", output)
	require.True(t, preview.OK)
	require.NotEmpty(t, preview.MissingCoreTemplates,
		"dry-run must report missing core templates, got: %+v", preview)

	// Human-readable dry-run must warn about missing templates.
	humanOutput, err := tc.RunFestInDir(
		festivalsPath,
		"create", "festival",
		"--name", "missing-dry",
		"--dry-run",
		"--no-color",
	)
	require.NoError(t, err, "human dry-run should succeed: %s", humanOutput)
	require.Contains(t, humanOutput, "MISSING core templates",
		"human dry-run must warn about missing core templates, got: %s", humanOutput)
}

func requireNoPlanningFestivals(t *testing.T, tc *TestContainer, festivalsPath string) {
	t.Helper()
	dirs, err := tc.ListDirectories(festivalsPath + "/planning")
	require.NoError(t, err)
	require.Empty(t, dirs, "no festival directory should remain after missing-template failure, got: %v", dirs)
}

type createFestivalMissingTemplateResult struct {
	OK     bool `json:"ok"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
	RolledBack *bool `json:"rolled_back"`
}

type createFestivalPreviewResultWithMissing struct {
	OK                   bool              `json:"ok"`
	MissingCoreTemplates []string          `json:"missing_core_templates"`
	Festival             map[string]string `json:"festival"`
}
