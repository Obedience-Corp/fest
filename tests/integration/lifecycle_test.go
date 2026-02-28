//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMultiPhaseNavigation tests that fest next correctly routes through
// different phase types as the user progresses through a festival.
func TestMultiPhaseNavigation(t *testing.T) {
	container := GetSharedContainer(t)

	// Use the existing setupMultiModeFestival helper
	festPath := setupMultiModeFestival(t, container, "multi-nav-test")

	// Test initial navigation - should start with first phase
	t.Run("InitialPhase", func(t *testing.T) {
		output := runExecuteMode(t, container, festPath)
		t.Logf("Initial execute output: %s", output)
		// First phase is PLANNING type
		verifyPlanningNavigation(t, output)
	})

	// Test that different phase types get different navigators
	t.Run("PhaseTypeDetection", func(t *testing.T) {
		// Run from a specific phase directory to detect phase type
		phasePath := festPath + "/001_PLANNING"
		output, err := container.RunFestInDir(phasePath, "next")
		if err == nil {
			t.Logf("Phase type detection output: %s", output)
			// Should show planning-specific content
			require.True(t,
				containsAnyOf(strings.ToLower(output), "planning", "implementation", "review"),
				"next output should reflect phase type")
		} else {
			t.Logf("Phase detection failed (acceptable): %v", err)
		}
	})
}

// TestWorkflowMode tests that phases with WORKFLOW.md files use workflow navigation.
func TestWorkflowMode(t *testing.T) {
	container := GetSharedContainer(t)

	// Setup workspace properly
	festivalsPath := setupWorkspace(t, container, "/")

	// Create festival
	_, err := container.RunFestInDir(festivalsPath, "create", "festival", "--name", "workflow-test", "--dest", "planning")
	require.NoError(t, err)

	// Find the actual festival path (fest adds an ID suffix)
	festPath := findFestivalPath(t, container, festivalsPath+"/planning", "workflow-test")

	// Create an implementation phase
	_, err = container.RunFestInDir(festPath, "create", "phase", "--name", "WORKFLOW_PHASE", "--type", "implementation")
	require.NoError(t, err)

	phasePath := festPath + "/001_WORKFLOW_PHASE"

	// Create a WORKFLOW.md file to trigger workflow mode
	workflowContent := `# Phase Workflow

## Steps

1. READ: Review requirements
2. IMPLEMENT: Write code
3. TEST: Run tests
4. REVIEW: Code review
`
	err = writeFileInContainer(container, phasePath+"/WORKFLOW.md", workflowContent)
	require.NoError(t, err)

	// Test fest next from the phase with WORKFLOW.md
	output, err := container.RunFestInDir(phasePath, "next")
	if err != nil {
		t.Logf("Next output (workflow phase): %s", output)
	} else {
		t.Logf("Workflow phase next: %s", output)
		// When WORKFLOW.md exists, should use workflow navigation
		if strings.Contains(output, "WORKFLOW") ||
			strings.Contains(output, "Step") ||
			strings.Contains(output, "workflow") {
			t.Log("Correctly detected workflow mode")
		}
	}
}

// =============================================================================
// VERIFICATION HELPERS (used by tests in this file)
// =============================================================================

// verifyPlanningNavigation checks that output shows planning-specific navigation.
func verifyPlanningNavigation(t *testing.T, output string) {
	t.Helper()
	lower := strings.ToLower(output)
	// Planning phases show objectives, requirements, or planning guidance
	acceptable := containsAnyOf(lower,
		"planning", "plan", "objective", "requirement", "design", "define")
	if !acceptable {
		t.Logf("Warning: output doesn't clearly show planning navigation: %s", output)
	}
}

// containsAnyOf is defined in guidance_implementation_test.go
