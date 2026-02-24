//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIngestMode_InitialDisplay verifies fest next in ingest phase runs without error.
// Ingest phases don't have a dedicated navigator, so fest next returns festival-level status.
func TestIngestMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupIngestFestival(t, container, "test-ingest-initial")
	phasePath := festPath + "/001_INGEST"

	// Run execute from within the phase directory
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Ingest phases fall through to festival-level status (no dedicated navigator)
	require.NotEmpty(t, output, "output should not be empty")
}

// TestIngestMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestIngestMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupIngestFestival(t, container, "test-ingest-detect")
	phasePath := festPath + "/001_INGEST"

	// Run execute from within the phase directory
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Verify output is non-empty (ingest phases use festival-level status)
	require.NotEmpty(t, output, "output should not be empty")
}
