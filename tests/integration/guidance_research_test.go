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

	// Run execute
	output := runExecuteMode(t, container, festPath)

	// Should show research phase content
	verifyOutputContains(t, output, "Research")
}

// TestResearchMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestResearchMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupResearchFestival(t, container, "test-research-detect")

	// Run roadmap to see phase type
	output := runRoadmap(t, container, festPath)

	// Should show research mode
	verifyOutputContains(t, output, "RESEARCH")
	verifyOutputContains(t, output, "research")
}
