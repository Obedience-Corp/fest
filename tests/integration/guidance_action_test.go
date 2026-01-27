//go:build integration
// +build integration

package integration

import (
	"testing"
)

// TestActionMode_InitialDisplay verifies fest execute in action phase shows actions.
func TestActionMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupActionFestival(t, container, "test-action-initial")

	// Run execute
	output := runExecuteMode(t, container, festPath)

	// Should show action phase content
	verifyOutputContains(t, output, "Action")
}

// TestActionMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestActionMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupActionFestival(t, container, "test-action-detect")

	// Run roadmap to see phase type
	output := runRoadmap(t, container, festPath)

	// Should show action mode
	verifyOutputContains(t, output, "ACTIONS")
	verifyOutputContains(t, output, "action")
}
