//go:build integration
// +build integration

package integration

import (
	"testing"
)

// TestResearchMode_InitialDisplay verifies fest execute in research phase shows topics.
func TestResearchMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupResearchFestival(t, container, "test-research-initial")
	phasePath := festPath + "/001_RESEARCH"

	// Run execute from within the phase directory for phase-type-aware navigation
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Should show research phase content
	verifyOutputContains(t, output, "Research")
}

// TestResearchMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestResearchMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupResearchFestival(t, container, "test-research-detect")
	phasePath := festPath + "/001_RESEARCH"

	// Run execute from within the phase directory to detect phase type
	// The mode detection reads PHASE_GOAL.md frontmatter's fest_phase_type field
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Should show research mode - output includes mode-specific instructions
	verifyOutputContains(t, output, "Research")
}
