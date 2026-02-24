//go:build integration
// +build integration

package integration

import (
	"testing"
)

// TestIngestMode_InitialDisplay verifies fest next in ingest phase shows ingest items.
func TestIngestMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupIngestFestival(t, container, "test-ingest-initial")
	phasePath := festPath + "/001_INGEST"

	// Run execute from within the phase directory for phase-type-aware navigation
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Should show ingest phase content
	verifyOutputContains(t, output, "Ingest")
}

// TestIngestMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestIngestMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupIngestFestival(t, container, "test-ingest-detect")
	phasePath := festPath + "/001_INGEST"

	// Run execute from within the phase directory to detect phase type
	// The mode detection reads PHASE_GOAL.md frontmatter's fest_phase_type field
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Should show ingest mode - output includes mode-specific instructions
	verifyOutputContains(t, output, "Ingest")
}
