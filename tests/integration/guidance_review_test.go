//go:build integration
// +build integration

package integration

import (
	"testing"
)

// TestReviewMode_InitialDisplay verifies fest execute in review phase shows review items.
func TestReviewMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupReviewFestival(t, container, "test-review-initial")

	// Run execute
	output := runExecuteMode(t, container, festPath)

	// Should show review phase content
	verifyOutputContains(t, output, "Review")
}

// TestReviewMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestReviewMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupReviewFestival(t, container, "test-review-detect")

	// Run roadmap to see phase type
	output := runRoadmap(t, container, festPath)

	// Should show review mode
	verifyOutputContains(t, output, "REVIEW")
	verifyOutputContains(t, output, "review")
}
