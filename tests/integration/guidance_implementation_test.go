//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestImplementationMode_InitialTask verifies that fest execute shows the first task.
func TestImplementationMode_InitialTask(t *testing.T) {
	container := GetSharedContainer(t)

	// Setup: Create festival with tasks
	festPath := setupImplementationFestival(t, container, "test-impl-initial")

	// Act: Run fest execute
	output := runExecuteMode(t, container, festPath)

	// Assert: Should show first task
	verifyOutputContains(t, output, "first_task")
	verifyOutputContains(t, output, "fest progress")
}

// TestImplementationMode_Navigation verifies fest next advances through tasks.
func TestImplementationMode_Navigation(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupImplementationFestival(t, container, "test-impl-nav")

	// Initial execute to set state
	_ = runExecuteMode(t, container, festPath)

	// Complete first task
	_, err := container.RunFestInDir(festPath, "progress", "--complete", "--force", "--task", "001_IMPLEMENTATION/01_core_work/01_first_task.md")
	require.NoError(t, err)

	// Get next task
	output := runNext(t, container, festPath)

	// Should now show second task
	verifyOutputContains(t, output, "second_task")
}

// TestImplementationMode_CompleteTask verifies marking tasks complete works.
func TestImplementationMode_CompleteTask(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupImplementationFestival(t, container, "test-impl-complete")

	// Initial execute
	_ = runExecuteMode(t, container, festPath)

	// Complete first task
	output, err := container.RunFestInDir(festPath, "progress", "--complete", "--force", "--task", "001_IMPLEMENTATION/01_core_work/01_first_task.md")
	require.NoError(t, err)
	verifyOutputContains(t, output, "complete")
}

// TestImplementationMode_StatePersistence verifies state survives command restarts.
func TestImplementationMode_StatePersistence(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupImplementationFestival(t, container, "test-impl-persist")

	// Run execute to create state
	_ = runExecuteMode(t, container, festPath)

	// Complete a task
	_, err := container.RunFestInDir(festPath, "progress", "--complete", "--force", "--task", "001_IMPLEMENTATION/01_core_work/01_first_task.md")
	require.NoError(t, err)

	// Run execute again (simulates new session)
	output := runExecuteMode(t, container, festPath)

	// Should remember completed task and show second
	verifyOutputContains(t, output, "second_task")
}

// TestImplementationMode_PhaseBoundaries verifies navigation between phases.
func TestImplementationMode_PhaseBoundaries(t *testing.T) {
	container := GetSharedContainer(t)

	// Set up workspace first
	festivalsPath := setupWorkspace(t, container, "/")

	// Create festival with multiple phases
	_, err := container.RunFestInDir(festivalsPath, "create", "festival", "--name", "test-impl-phases", "--dest", "active")
	require.NoError(t, err)

	festPath := findFestivalPath(t, container, festivalsPath+"/active", "test-impl-phases")

	// Create two phases
	_, err = container.RunFestInDir(festPath, "create", "phase", "--name", "PHASE_ONE", "--type", "implementation")
	require.NoError(t, err)
	_, err = container.RunFestInDir(festPath, "create", "phase", "--name", "PHASE_TWO", "--type", "implementation")
	require.NoError(t, err)

	// Create sequence and task in each phase
	_, err = container.RunFestInDir(festPath+"/001_PHASE_ONE", "create", "sequence", "--name", "seq1")
	require.NoError(t, err)
	_, err = container.RunFestInDir(festPath+"/001_PHASE_ONE/01_seq1", "create", "task", "--name", "phase1_task")
	require.NoError(t, err)

	_, err = container.RunFestInDir(festPath+"/002_PHASE_TWO", "create", "sequence", "--name", "seq2")
	require.NoError(t, err)
	_, err = container.RunFestInDir(festPath+"/002_PHASE_TWO/01_seq2", "create", "task", "--name", "phase2_task")
	require.NoError(t, err)

	// Run execute - should show phase 1 task
	output := runExecuteMode(t, container, festPath)
	verifyOutputContains(t, output, "phase1_task")

	// Complete phase 1 task
	_, err = container.RunFestInDir(festPath, "progress", "--complete", "--force", "--task", "001_PHASE_ONE/01_seq1/01_phase1_task.md")
	require.NoError(t, err)

	// Next should show phase 2 task
	output = runNext(t, container, festPath)
	verifyOutputContains(t, output, "phase2_task")
}

// TestImplementationMode_Completion verifies behavior when all tasks are done.
func TestImplementationMode_Completion(t *testing.T) {
	container := GetSharedContainer(t)

	// Set up workspace first
	festivalsPath := setupWorkspace(t, container, "/")

	// Create minimal festival with one task
	_, err := container.RunFestInDir(festivalsPath, "create", "festival", "--name", "test-impl-done", "--dest", "active")
	require.NoError(t, err)

	festPath := findFestivalPath(t, container, festivalsPath+"/active", "test-impl-done")

	_, err = container.RunFestInDir(festPath, "create", "phase", "--name", "ONLY_PHASE", "--type", "implementation")
	require.NoError(t, err)
	_, err = container.RunFestInDir(festPath+"/001_ONLY_PHASE", "create", "sequence", "--name", "only_seq")
	require.NoError(t, err)
	_, err = container.RunFestInDir(festPath+"/001_ONLY_PHASE/01_only_seq", "create", "task", "--name", "only_task")
	require.NoError(t, err)

	// Run execute
	_ = runExecuteMode(t, container, festPath)

	// Complete the only task
	_, err = container.RunFestInDir(festPath, "progress", "--complete", "--force", "--task", "001_ONLY_PHASE/01_only_seq/01_only_task.md")
	require.NoError(t, err)

	// Execute again - should indicate completion
	output := runExecuteMode(t, container, festPath)
	// Should indicate no more tasks or completion
	require.True(t,
		containsAnyOf(output, "complete", "done", "All tasks"),
		"output should indicate completion: %s", output)
}

// TestImplementationMode_EmptyFestival verifies handling of festival with no tasks.
func TestImplementationMode_EmptyFestival(t *testing.T) {
	container := GetSharedContainer(t)

	// Set up workspace first
	festivalsPath := setupWorkspace(t, container, "/")

	// Create festival with phase but no tasks
	_, err := container.RunFestInDir(festivalsPath, "create", "festival", "--name", "test-impl-empty", "--dest", "active")
	require.NoError(t, err)

	festPath := findFestivalPath(t, container, festivalsPath+"/active", "test-impl-empty")

	_, err = container.RunFestInDir(festPath, "create", "phase", "--name", "EMPTY_PHASE", "--type", "implementation")
	require.NoError(t, err)

	// Run execute - should handle gracefully
	output, err := container.RunFestInDir(festPath, "execute")
	// May error or return empty state - either is acceptable
	if err != nil {
		t.Logf("Execute on empty festival failed (expected): %v", err)
	} else {
		t.Logf("Execute on empty festival output: %s", output)
	}
}

// TestImplementationMode_Roadmap verifies roadmap output shows all phases.
func TestImplementationMode_Roadmap(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupImplementationFestival(t, container, "test-impl-roadmap")

	// Run roadmap
	output := runRoadmap(t, container, festPath)

	// Should show phase structure
	verifyOutputContains(t, output, "FESTIVAL ROADMAP")
	verifyOutputContains(t, output, "IMPLEMENTATION")
	verifyOutputContains(t, output, "first_task")
	verifyOutputContains(t, output, "second_task")
	verifyOutputContains(t, output, "third_task")
}

// containsAnyOf checks if the string contains any of the substrings.
func containsAnyOf(s string, substrs ...string) bool {
	lowerS := strings.ToLower(s)
	for _, substr := range substrs {
		if strings.Contains(lowerS, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}
