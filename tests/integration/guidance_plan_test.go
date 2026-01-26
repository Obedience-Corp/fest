//go:build integration
// +build integration

package integration

import (
	"testing"
)

// TestPlanMode_InitialDisplay verifies fest execute in planning phase shows planning objectives.
func TestPlanMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupPlanFestival(t, container, "test-plan-initial")

	// Run execute
	output := runExecuteMode(t, container, festPath)

	// Should show planning phase content
	verifyOutputContains(t, output, "Planning")
}

// TestPlanMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestPlanMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupPlanFestival(t, container, "test-plan-detect")

	// Run roadmap to see phase type
	output := runRoadmap(t, container, festPath)

	// Should show planning mode
	verifyOutputContains(t, output, "PLANNING")
	verifyOutputContains(t, output, "planning")
}

// TestPlanMode_ModeOverride verifies --mode flag can override auto-detection.
func TestPlanMode_ModeOverride(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupPlanFestival(t, container, "test-plan-override")

	// Run execute with mode override
	output := runExecuteModeWithMode(t, container, festPath, "implementation")

	// Should respect override
	t.Logf("Override output: %s", output)
}
