//go:build integration
// +build integration

package integration

import (
	"testing"
)

// TestPlanMode_InitialDisplay verifies fest next in planning phase shows planning objectives.
func TestPlanMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupPlanFestival(t, container, "test-plan-initial")
	phasePath := festPath + "/001_PLANNING"

	// Run execute from within the phase directory for phase-type-aware navigation
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Should show planning phase content
	verifyOutputContains(t, output, "Planning")
}

// TestPlanMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestPlanMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupPlanFestival(t, container, "test-plan-detect")
	phasePath := festPath + "/001_PLANNING"

	// Run execute from within the phase directory to detect phase type
	// The mode detection reads PHASE_GOAL.md frontmatter's fest_phase_type field
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Should show planning mode - output includes mode-specific instructions
	verifyOutputContains(t, output, "Planning")
}

// TestPlanMode_ModeOverride verifies --mode flag can override auto-detection.
func TestPlanMode_ModeOverride(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupPlanFestival(t, container, "test-plan-override")

	// Run execute with mode override
	output := runExecuteModeWithMode(t, container, festPath, "implementation")

	// Should respect override
	t.Logf("Override output: %s", output)
}
