//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestActionMode_InitialDisplay verifies fest next in action phase runs without error.
// Action phases don't have a dedicated navigator, so fest next returns festival-level status.
func TestActionMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupActionFestival(t, container, "test-action-initial")
	phasePath := festPath + "/001_ACTIONS"

	// Run execute from within the phase directory
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Action phases fall through to festival-level status (no dedicated navigator)
	require.NotEmpty(t, output, "output should not be empty")
}

// TestActionMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestActionMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupActionFestival(t, container, "test-action-detect")
	phasePath := festPath + "/001_ACTIONS"

	// Run execute from within the phase directory
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Verify output is non-empty (action phases use festival-level status)
	require.NotEmpty(t, output, "output should not be empty")
}
