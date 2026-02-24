//go:build integration
// +build integration

package integration

import (
	"testing"
)

// TestReviewMode_InitialDisplay verifies fest next in review phase shows review items.
func TestReviewMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupReviewFestival(t, container, "test-review-initial")
	phasePath := festPath + "/001_REVIEW"

	// Run execute from within the phase directory for phase-type-aware navigation
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Should show review phase content
	verifyOutputContains(t, output, "Review")
}

// TestReviewMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestReviewMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupReviewFestival(t, container, "test-review-detect")
	phasePath := festPath + "/001_REVIEW"

	// Run execute from within the phase directory to detect phase type
	// The mode detection reads PHASE_GOAL.md frontmatter's fest_phase_type field
	output := runExecuteModeFromPhase(t, container, phasePath)

	// Should show review mode - output includes mode-specific instructions
	verifyOutputContains(t, output, "Review")
}
