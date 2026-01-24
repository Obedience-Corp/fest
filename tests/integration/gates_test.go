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

	// Test 4: gates init - create fest.yaml at festival level
	t.Run("GatesInitFestival", func(t *testing.T) {
		// First verify fest.yaml doesn't exist
		festYAMLPath := "/festivals/test-gates-festival/fest.yaml"
		exists, _ := container.CheckFileExists(festYAMLPath)
		require.False(t, exists, "fest.yaml should not exist initially")

		// Run init
		output, err := container.RunFestInDir("/festivals/test-gates-festival", "gates", "init")
		require.NoError(t, err, "gates init should not fail")
		require.Contains(t, output, "Festival configuration created", "Should confirm fest.yaml creation")
		t.Logf("gates init: %s", output)

		// Verify file was created
		exists, err = container.CheckFileExists(festYAMLPath)
		require.NoError(t, err)
		require.True(t, exists, "fest.yaml should be created")

		// Verify file content - should have phase-type sections
		content, err := container.ReadFile(festYAMLPath)
		require.NoError(t, err)
		require.Contains(t, content, "quality_gates:", "fest.yaml should have quality_gates section")
		require.Contains(t, content, "implementation:", "fest.yaml should have implementation section")
		require.Contains(t, content, "id: testing", "fest.yaml should have testing gate")
		t.Logf("fest.yaml content: %s", content)
	})

	// Test 5: gates init --phase - create override at phase level
	t.Run("GatesInitPhase", func(t *testing.T) {
		overridePath := "/festivals/test-gates-festival/002_IMPLEMENT/.fest.gates.yml"

		// Verify file doesn't exist
		exists, _ := container.CheckFileExists(overridePath)
		require.False(t, exists, "Phase override should not exist initially")

		// Run init with --phase flag
		output, err := container.RunFestInDir("/festivals/test-gates-festival", "gates", "init", "--phase", "002_IMPLEMENT")
		require.NoError(t, err, "gates init --phase should not fail")
		require.Contains(t, output, "Override file created", "Should confirm creation")
		t.Logf("gates init --phase: %s", output)

		// Verify file was created
		exists, err = container.CheckFileExists(overridePath)
		require.NoError(t, err)
		require.True(t, exists, "Phase override should be created")
	})

	// Test 6: gates init --sequence - create override at sequence level
	t.Run("GatesInitSequence", func(t *testing.T) {
		overridePath := "/festivals/test-gates-festival/002_IMPLEMENT/01_core/.fest.gates.yml"

		// Verify file doesn't exist
		exists, _ := container.CheckFileExists(overridePath)
		require.False(t, exists, "Sequence override should not exist initially")

		// Run init with --sequence flag
		output, err := container.RunFestInDir("/festivals/test-gates-festival", "gates", "init", "--sequence", "002_IMPLEMENT/01_core")
		require.NoError(t, err, "gates init --sequence should not fail")
		require.Contains(t, output, "Override file created", "Should confirm creation")
		t.Logf("gates init --sequence: %s", output)

		// Verify file was created
		exists, err = container.CheckFileExists(overridePath)
		require.NoError(t, err)
		require.True(t, exists, "Sequence override should be created")
	})

	// Test 7: gates validate
	t.Run("GatesValidate", func(t *testing.T) {
		output, err := container.RunFestInDir("/festivals/test-gates-festival", "gates", "validate")
		require.NoError(t, err, "gates validate should not fail")
		// Either "valid" or lists issues
		if strings.Contains(output, "valid") {
			t.Logf("Validation passed: %s", output)
		} else {
			t.Logf("Validation found issues: %s", output)
		}
	})

	// Test 8: gates validate --json
	t.Run("GatesValidateJSON", func(t *testing.T) {
		output, err := container.RunFestInDir("/festivals/test-gates-festival", "gates", "validate", "--json")
		require.NoError(t, err, "gates validate --json should not fail")
		require.Contains(t, output, `"valid"`, "JSON should contain valid field")
		require.Contains(t, output, `"issues"`, "JSON should contain issues field")
		t.Logf("gates validate --json: %s", output)
	})

	// Test 9: gates show with phase override in effect
	t.Run("GatesShowWithOverride", func(t *testing.T) {
		// The phase override was created in test 5
		// Show gates for that phase
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

	expected := []string{
		"_quality_gate_testing.md",
		"_quality_gate_review.md",
		"_quality_gate_iterate.md",
		"_quality_gate_commit.md",
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
	// Create base structure
	dirs := []string{
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
	// Create directories including the root .festival (required by FindFestivalsRoot)
	dirs := []string{
		"/festivals/.festival", // Root .festival directory
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

	return nil
}
