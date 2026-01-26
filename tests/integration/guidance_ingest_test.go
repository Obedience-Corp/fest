//go:build integration
// +build integration

package integration

import (
	"testing"
)

// TestIngestMode_InitialDisplay verifies fest execute in ingest phase shows ingest items.
func TestIngestMode_InitialDisplay(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupIngestFestival(t, container, "test-ingest-initial")

	// Run execute
	output := runExecuteMode(t, container, festPath)

	// Should show ingest phase content
	verifyOutputContains(t, output, "Ingest")
}

// TestIngestMode_PhaseTypeDetection verifies phase type is correctly detected.
func TestIngestMode_PhaseTypeDetection(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupIngestFestival(t, container, "test-ingest-detect")

	// Run roadmap to see phase type
	output := runRoadmap(t, container, festPath)

	// Should show ingest mode
	verifyOutputContains(t, output, "INGEST")
	verifyOutputContains(t, output, "ingest")
}
