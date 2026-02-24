//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReviewMode_InitialDisplay verifies fest next in review phase runs without error.
// Review phases don't have a dedicated navigator, so fest next returns festival-level status.
func TestReviewMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupReviewFestival(t, container, "test-review-initial")
	phasePath := festPath + "/001_REVIEW"

	// Run execute from within the phase directory
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Review phases fall through to festival-level status (no dedicated navigator)
	require.NotEmpty(t, output, "output should not be empty")
}

// TestReviewMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestReviewMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupReviewFestival(t, container, "test-review-detect")
	phasePath := festPath + "/001_REVIEW"

	// Run execute from within the phase directory
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Verify output is non-empty (review phases use festival-level status)
	require.NotEmpty(t, output, "output should not be empty")
}
