//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestImplementationMode_InitialTask verifies that fest next shows the first task.
func TestImplementationMode_InitialTask(t *testing.T) {
	container := GetSharedContainer(t)

	// Setup: Create festival with tasks
	festPath := setupImplementationFestival(t, container, "test-impl-initial")

	// Act: Run fest next
	output := runExecuteMode(t, container, festPath)

	// Assert: Should show first task
	verifyOutputContains(t, output, "first_task")
	verifyOutputContains(t, output, "fest task completed")
}

// TestImplementationMode_Navigation verifies fest next advances through tasks.
func TestImplementationMode_Navigation(t *testing.T) {
	container := GetSharedContainer(t)

	festPath := setupImplementationFestival(t, container, "test-impl-nav")

	// Initial execute to set state
	_ = runExecuteMode(t, container, festPath)

	// Complete first task
	_, err := container.RunFestInDir(festPath, "progress", "--complete", "--task", "001_IMPLEMENTATION/01_core_work/01_first_task.md")
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
	output, err := container.RunFestInDir(festPath, "progress", "--complete", "--task", "001_IMPLEMENTATION/01_core_work/01_first_task.md")
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
	_, err := container.RunFestInDir(festPath, "progress", "--complete", "--task", "001_IMPLEMENTATION/01_core_work/01_first_task.md")
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
	_, err := container.RunFestInDir(festivalsPath, "create", "festival", "--name", "test-impl-phases", "--dest", "planning")
	require.NoError(t, err)

	festPath := findFestivalPath(t, container, festivalsPath+"/planning", "test-impl-phases")

	// Write fest.yaml with quality gates enabled and auto-link disabled (no real project dirs in container)
	festYaml := `version: "1.0"
auto_link:
  enabled: false
quality_gates:
  enabled: true
  auto_append: true
  implementation:
    - id: testing
      template: gates/implementation/QUALITY_GATE_TESTING
      enabled: true
    - id: review
      template: gates/implementation/QUALITY_GATE_REVIEW
      enabled: true
    - id: iterate
      template: gates/implementation/QUALITY_GATE_ITERATE
      enabled: true
    - id: fest-commit
      template: gates/implementation/QUALITY_GATE_FEST_COMMIT
      enabled: true
`
	err = writeFileInContainer(container, festPath+"/fest.yaml", festYaml)
	require.NoError(t, err)

	// Create FESTIVAL_OVERVIEW.md (required by validator)
	overviewContent := `---
fest_type: overview
---

# Test Festival Overview

## Goals
- Test phase boundary navigation
`
	err = writeFileInContainer(container, festPath+"/FESTIVAL_OVERVIEW.md", overviewContent)
	require.NoError(t, err)

	// Create FESTIVAL_RULES.md (required to avoid warning that blocks fest next)
	rulesContent := "# Festival Rules\n\n- Follow naming conventions\n- Complete all quality gates\n"
	err = writeFileInContainer(container, festPath+"/FESTIVAL_RULES.md", rulesContent)
	require.NoError(t, err)

	// Create two phases
	_, err = container.RunFestInDir(festPath, "create", "phase", "--name", "PHASE_ONE", "--type", "implementation")
	require.NoError(t, err)
	_, err = container.RunFestInDir(festPath, "create", "phase", "--name", "PHASE_TWO", "--type", "implementation")
	require.NoError(t, err)

	// Create sequences and tasks using the real CLI workflow
	_, err = container.RunFestInDir(festPath+"/001_PHASE_ONE", "create", "sequence", "--name", "seq1")
	require.NoError(t, err)

	_, err = container.RunFestInDir(festPath+"/001_PHASE_ONE/01_seq1", "create", "task", "--name", "phase1_task")
	require.NoError(t, err)

	_, err = container.RunFestInDir(festPath+"/002_PHASE_TWO", "create", "sequence", "--name", "seq2")
	require.NoError(t, err)

	_, err = container.RunFestInDir(festPath+"/002_PHASE_TWO/01_seq2", "create", "task", "--name", "phase2_task")
	require.NoError(t, err)

	// Ensure gate templates exist and apply quality gates
	ensureGateTemplates(t, container, festPath)
	_, err = container.RunFestInDir(festPath, "gates", "apply", "--approve")
	require.NoError(t, err, "should apply quality gates")

	// Replace markers across entire festival (simulates user/agent filling them in)
	err = replaceMarkersInContainer(container, festPath)
	require.NoError(t, err, "should replace markers")

	// Run execute - should show phase 1 task
	output := runExecuteMode(t, container, festPath)
	verifyOutputContains(t, output, "phase1_task")

	// Complete all phase 1 tasks (real task + gate stubs)
	phase1Tasks := []string{
		"001_PHASE_ONE/01_seq1/01_phase1_task.md",
		"001_PHASE_ONE/01_seq1/02_testing.md",
		"001_PHASE_ONE/01_seq1/03_review.md",
		"001_PHASE_ONE/01_seq1/04_iterate.md",
		"001_PHASE_ONE/01_seq1/05_fest_commit.md",
	}
	for _, task := range phase1Tasks {
		_, err = container.RunFestInDir(festPath, "progress", "--complete", "--task", task)
		require.NoError(t, err, "should complete %s", task)
	}

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
	_, err := container.RunFestInDir(festivalsPath, "create", "festival", "--name", "test-impl-done", "--dest", "planning")
	require.NoError(t, err)

	festPath := findFestivalPath(t, container, festivalsPath+"/planning", "test-impl-done")

	// Write fest.yaml with quality gates enabled and auto-link disabled (no real project dirs in container)
	festYaml := "version: \"1.0\"\nauto_link:\n  enabled: false\nquality_gates:\n  enabled: true\n  auto_append: true\n  implementation:\n    - id: testing\n      template: gates/implementation/QUALITY_GATE_TESTING\n      enabled: true\n    - id: review\n      template: gates/implementation/QUALITY_GATE_REVIEW\n      enabled: true\n    - id: iterate\n      template: gates/implementation/QUALITY_GATE_ITERATE\n      enabled: true\n    - id: fest-commit\n      template: gates/implementation/QUALITY_GATE_FEST_COMMIT\n      enabled: true\n"
	err = writeFileInContainer(container, festPath+"/fest.yaml", festYaml)
	require.NoError(t, err)

	// Create FESTIVAL_OVERVIEW.md (required by validator)
	overviewContent := "---\nfest_type: overview\n---\n\n# Test Festival Overview\n\n## Goals\n- Test completion behavior\n\n## Success Criteria\n- All tasks completed\n"
	err = writeFileInContainer(container, festPath+"/FESTIVAL_OVERVIEW.md", overviewContent)
	require.NoError(t, err)

	// Create FESTIVAL_RULES.md (recommended by validator)
	rulesContent := "# Festival Rules\n\n- Follow naming conventions\n"
	err = writeFileInContainer(container, festPath+"/FESTIVAL_RULES.md", rulesContent)
	require.NoError(t, err)

	_, err = container.RunFestInDir(festPath, "create", "phase", "--name", "ONLY_PHASE", "--type", "implementation")
	require.NoError(t, err)
	_, err = container.RunFestInDir(festPath+"/001_ONLY_PHASE", "create", "sequence", "--name", "only_seq")
	require.NoError(t, err)

	// Create task using the real CLI workflow
	_, err = container.RunFestInDir(festPath+"/001_ONLY_PHASE/01_only_seq", "create", "task", "--name", "only_task")
	require.NoError(t, err)

	// Ensure gate templates exist and apply quality gates
	ensureGateTemplates(t, container, festPath)
	_, err = container.RunFestInDir(festPath, "gates", "apply", "--approve")
	require.NoError(t, err, "should apply quality gates")

	// Replace markers across entire festival (simulates user/agent filling them in)
	err = replaceMarkersInContainer(container, festPath)
	require.NoError(t, err, "should replace markers")

	// Run execute
	_ = runExecuteMode(t, container, festPath)

	// Complete all tasks (real task + gate stubs)
	allTasks := []string{
		"001_ONLY_PHASE/01_only_seq/01_only_task.md",
		"001_ONLY_PHASE/01_only_seq/02_testing.md",
		"001_ONLY_PHASE/01_only_seq/03_review.md",
		"001_ONLY_PHASE/01_only_seq/04_iterate.md",
		"001_ONLY_PHASE/01_only_seq/05_fest_commit.md",
	}
	for _, task := range allTasks {
		_, err = container.RunFestInDir(festPath, "progress", "--complete", "--task", task)
		require.NoError(t, err, "should complete %s", task)
	}

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
	_, err := container.RunFestInDir(festivalsPath, "create", "festival", "--name", "test-impl-empty", "--dest", "planning")
	require.NoError(t, err)

	festPath := findFestivalPath(t, container, festivalsPath+"/planning", "test-impl-empty")

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
