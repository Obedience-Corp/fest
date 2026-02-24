//go:build integration
// +build integration

package integration

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatesCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Get shared container (reset between tests)
	container := GetSharedContainer(t)

	// Setup: Create festivals directory structure with .festival/gates
	t.Run("Setup", func(t *testing.T) {
		// Create the base festivals structure
		err := setupGatesTestFestival(container)
		require.NoError(t, err, "Failed to setup test festival")

		// Verify structure
		exists, err := container.CheckDirExists("/festivals/.festival")
		require.NoError(t, err)
		require.True(t, exists, ".festival directory should exist")
	})

	// Test 1: gates --help
	t.Run("GatesHelp", func(t *testing.T) {
		output, err := container.RunFest("gates", "--help")
		require.NoError(t, err, "gates --help should not fail")
		require.Contains(t, output, "gates", "Help should mention gates")
		require.Contains(t, output, "show", "Help should mention show subcommand")
		require.Contains(t, output, "apply", "Help should mention apply subcommand")
		require.Contains(t, output, "init", "Help should mention init subcommand")
		require.Contains(t, output, "validate", "Help should mention validate subcommand")
		t.Logf("gates help: %s", output)
	})

	// Test 2: gates show - show effective policy for festival
	t.Run("GatesShow", func(t *testing.T) {
		output, err := container.RunFestInDir("/festivals/test-gates-festival", "gates", "show")
		require.NoError(t, err, "gates show should not fail")
		require.Contains(t, output, "GATE POLICY", "Should show gate policy header")
		require.Contains(t, output, "Active Gates", "Should list active gates")
		require.Contains(t, output, "testing", "Should show testing gate")
		require.Contains(t, output, "review", "Should show review gate")
		t.Logf("gates show: %s", output)
	})

	// Test 3: gates show --json
	t.Run("GatesShowJSON", func(t *testing.T) {
		output, err := container.RunFestInDir("/festivals/test-gates-festival", "gates", "show", "--json")
		require.NoError(t, err, "gates show --json should not fail")
		require.Contains(t, output, `"gates"`, "JSON should contain gates array")
		require.Contains(t, output, `"sources"`, "JSON should contain sources array")
		require.Contains(t, output, `"level"`, "JSON should contain level field")
		t.Logf("gates show --json: %s", output)
	})

	// Test 4: gates show with phase flag
	t.Run("GatesShowWithPhase", func(t *testing.T) {
		output, err := container.RunFestInDir("/festivals/test-gates-festival", "gates", "show", "--phase", "002_IMPLEMENT")
		require.NoError(t, err, "gates show --phase should not fail")
		require.Contains(t, output, "GATE POLICY", "Should show gate policy header")
		require.Contains(t, output, "Configuration Sources", "Should list configuration sources")
		t.Logf("gates show --phase: %s", output)
	})

	// Test 10: gates show for sequence
	t.Run("GatesShowSequence", func(t *testing.T) {
		output, err := container.RunFestInDir("/festivals/test-gates-festival", "gates", "show", "--sequence", "002_IMPLEMENT/01_core")
		require.NoError(t, err, "gates show --sequence should not fail")
		require.Contains(t, output, "GATE POLICY", "Should show gate policy header")
		require.Contains(t, output, "Configuration Sources", "Should list configuration sources")
		t.Logf("gates show --sequence: %s", output)
	})

	// Test 11: Verify hierarchical merge behavior with custom override file
	t.Run("HierarchicalMerge", func(t *testing.T) {
		// Write a custom policy at festival level
		customPolicy := `version: 1
name: custom
inherit: true
append:
  - id: testing
    template: gates/implementation/QUALITY_GATE_TESTING
    name: Testing and Verification
    enabled: true
  - id: review
    template: gates/implementation/QUALITY_GATE_REVIEW
    name: Code Review
    enabled: true
`
		// Create a new festival for this test
		err := createMinimalFestival(container, "/festivals/hierarchy-test")
		require.NoError(t, err)

		// Write custom policy
		policyPath := "/festivals/hierarchy-test/.festival/gates.yml"
		exitCode, _, err := container.container.Exec(container.ctx, []string{
			"sh", "-c",
			"mkdir -p /festivals/hierarchy-test/.festival && printf '%s' '" + customPolicy + "' > " + policyPath,
		})
		require.NoError(t, err)
		require.Equal(t, 0, exitCode)

		// Verify the policy is loaded correctly
		output, err := container.RunFestInDir("/festivals/hierarchy-test", "gates", "show", "--json")
		require.NoError(t, err, "gates show should work with custom policy")
		require.Contains(t, output, "testing", "Should show gates from custom policy")
		t.Logf("Hierarchical merge test: %s", output)
	})
}

func TestGatesApplyVaryingSequenceCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)
	setupFestivalTemplates(t, container)

	output, err := container.RunFestInDir(
		"/festivals",
		"create", "festival",
		"--name", "gates-apply-varied",
		"--goal", "Gates-apply-varied",
		"--json",
		"--skip-markers",
	)
	require.NoError(t, err, "create festival should succeed")

	result := parseCreateFestivalOutput(t, output)
	festivalPath := filepath.Join("/festivals", result.Festival.Dest, result.Festival.Directory)
	phasePath := filepath.Join(festivalPath, "002_IMPLEMENT")

	writeFile := func(path, content string) {
		exitCode, _, err := container.container.Exec(container.ctx, []string{
			"sh", "-c",
			"printf '%s' '" + content + "' > " + path,
		})
		require.NoError(t, err)
		require.Equal(t, 0, exitCode)
	}

	exitCode, _, err := container.container.Exec(container.ctx, []string{"mkdir", "-p", phasePath})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)

	// Create PHASE_GOAL.md with fest_phase_type frontmatter
	phaseGoalContent := `---
fest_phase_type: implementation
---

# Implementation Phase

Implement the features.
`
	writeFile(filepath.Join(phasePath, "PHASE_GOAL.md"), phaseGoalContent)

	sequences := []struct {
		name      string
		taskCount int
	}{
		{name: "01_small", taskCount: 1},
		{name: "02_medium", taskCount: 2},
		{name: "03_large", taskCount: 0},
	}

	for _, seq := range sequences {
		seqPath := filepath.Join(phasePath, seq.name)
		exitCode, _, err := container.container.Exec(container.ctx, []string{"mkdir", "-p", seqPath})
		require.NoError(t, err)
		require.Equal(t, 0, exitCode)

		writeFile(filepath.Join(seqPath, "SEQUENCE_GOAL.md"), "# Sequence Goal\n")

		for i := 1; i <= seq.taskCount; i++ {
			taskName := fmt.Sprintf("%02d_seed.md", i)
			writeFile(filepath.Join(seqPath, taskName), fmt.Sprintf("# Task %02d\n", i))
		}
	}

	output, err = container.RunFestInDir(festivalPath, "gates", "apply", "--approve")
	require.NoError(t, err, "gates apply --approve should not fail")
	require.NotContains(t, output, "No implementation sequences found", "should process sequences")

	for _, seq := range sequences {
		seqPath := filepath.Join(phasePath, seq.name)
		files, err := container.ListDirectory(seqPath)
		require.NoError(t, err)
		requireGateTasks(t, files)
	}
}

func requireGateTasks(t *testing.T, files []string) {
	t.Helper()

	// Gate IDs from DefaultFestivalConfig: testing, review, iterate, fest-commit
	// Note: FormatTaskID normalizes hyphens to underscores, so fest-commit → fest_commit
	expected := []string{
		"_testing.md",
		"_review.md",
		"_iterate.md",
		"_fest_commit.md",
	}

	for _, suffix := range expected {
		found := false
		for _, file := range files {
			if strings.HasSuffix(file, suffix) {
				found = true
				break
			}
		}
		require.True(t, found, "expected gate task %s", suffix)
	}
}

// setupGatesTestFestival creates a test festival structure for gates testing
func setupGatesTestFestival(tc *TestContainer) error {
	// Create base structure (including .state directory for workspace marker)
	dirs := []string{
		"/festivals/.festival/.state",
		"/festivals/.festival/gates/policies",
		"/festivals/.festival/templates",
		"/festivals/.festival/templates/gates",
		"/festivals/test-gates-festival/001_DESIGN/01_planning",
		"/festivals/test-gates-festival/002_IMPLEMENT/01_core",
		"/festivals/test-gates-festival/002_IMPLEMENT/02_features",
		"/festivals/test-gates-festival/003_TEST/01_unit",
	}

	for _, dir := range dirs {
		exitCode, _, err := tc.container.Exec(tc.ctx, []string{"mkdir", "-p", dir})
		if err != nil || exitCode != 0 {
			return err
		}
	}

	// Create FESTIVAL_GOAL.md
	goalContent := `---
id: TEST_GATES_FESTIVAL
---

# Test Gates Festival

## Goal

Test the hierarchical quality gates system.
`
	goalPath := "/festivals/test-gates-festival/FESTIVAL_GOAL.md"
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{
		"sh", "-c",
		"printf '%s' '" + goalContent + "' > " + goalPath,
	})
	if err != nil || exitCode != 0 {
		return err
	}

	// Create fest.yaml (required for FindFestivalRoot)
	festYAMLContent := `version: "1.0"
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
	festYAMLPath := "/festivals/test-gates-festival/fest.yaml"
	exitCode, _, err = tc.container.Exec(tc.ctx, []string{
		"sh", "-c",
		"printf '%s' '" + festYAMLContent + "' > " + festYAMLPath,
	})
	if err != nil || exitCode != 0 {
		return err
	}

	// Create task files
	taskContent := `---
id: TASK_01
---

# Task 01

Implementation task.
`
	tasks := []string{
		"/festivals/test-gates-festival/002_IMPLEMENT/01_core/01_setup.md",
		"/festivals/test-gates-festival/002_IMPLEMENT/01_core/02_implement.md",
		"/festivals/test-gates-festival/002_IMPLEMENT/02_features/01_feature_a.md",
	}

	for _, task := range tasks {
		exitCode, _, err := tc.container.Exec(tc.ctx, []string{
			"sh", "-c",
			"printf '%s' '" + taskContent + "' > " + task,
		})
		if err != nil || exitCode != 0 {
			return err
		}
	}

	return nil
}

// createMinimalFestival creates a minimal festival structure
func createMinimalFestival(tc *TestContainer, path string) error {
	// Create directories including the root .festival/.state (required by FindFestivalsRoot)
	dirs := []string{
		"/festivals/.festival/.state", // Root .festival/.state directory for workspace marker
		filepath.Join(path, ".festival"),
		filepath.Join(path, "001_PHASE/01_seq"),
	}

	for _, dir := range dirs {
		exitCode, _, err := tc.container.Exec(tc.ctx, []string{"mkdir", "-p", dir})
		if err != nil || exitCode != 0 {
			return err
		}
	}

	// Create FESTIVAL_GOAL.md
	goalPath := filepath.Join(path, "FESTIVAL_GOAL.md")
	goalContent := "# Minimal Test Festival\n\nTest festival for gates.\n"
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{
		"sh", "-c",
		"printf '%s' '" + goalContent + "' > " + goalPath,
	})
	if err != nil || exitCode != 0 {
		return err
	}

	// Create fest.yaml (required for FindFestivalRoot)
	festYAMLContent := `version: "1.0"
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
	festYAMLPath := filepath.Join(path, "fest.yaml")
	exitCode, _, err = tc.container.Exec(tc.ctx, []string{
		"sh", "-c",
		"printf '%s' '" + festYAMLContent + "' > " + festYAMLPath,
	})
	if err != nil || exitCode != 0 {
		return err
	}

	return nil
}
