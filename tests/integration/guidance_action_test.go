//go:build integration
// +build integration

package integration

import (
	"testing"
)

// TestActionMode_InitialDisplay verifies fest next in action phase shows actions.
func TestActionMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupActionFestival(t, container, "test-action-initial")
	phasePath := festPath + "/001_ACTIONS"

	// Run execute from within the phase directory for phase-type-aware navigation
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Should show action phase content
	verifyOutputContains(t, output, "Action")
}

// TestActionMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestActionMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupActionFestival(t, container, "test-action-detect")
	phasePath := festPath + "/001_ACTIONS"

	// Run execute from within the phase directory to detect phase type
	// The mode detection reads PHASE_GOAL.md frontmatter's fest_phase_type field
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Should show action mode - output includes mode-specific instructions
	// Non-coding action phases show action-focused output
	verifyOutputContains(t, output, "Action")
}
