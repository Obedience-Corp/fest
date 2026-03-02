package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFestivalName(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		expected string
	}{
		{
			name:     "simple name with ID",
			dirName:  "my-festival-AF0001",
			expected: "my festival",
		},
		{
			name:     "long name with ID",
			dirName:  "address-feedback-from-last-fest-festival-AF0001",
			expected: "address feedback from last fest festival",
		},
		{
			name:     "no ID suffix",
			dirName:  "simple-festival",
			expected: "simple festival",
		},
		{
			name:     "underscores",
			dirName:  "my_festival_GU0002",
			expected: "my festival",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFestivalName(tt.dirName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractFestivalID(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		expected string
	}{
		{
			name:     "standard ID",
			dirName:  "my-festival-AF0001",
			expected: "AF0001",
		},
		{
			name:     "GU ID",
			dirName:  "guild-fest-GU0002",
			expected: "GU0002",
		},
		{
			name:     "no ID",
			dirName:  "simple-festival",
			expected: "",
		},
		{
			name:     "3-letter prefix",
			dirName:  "test-ABC1234",
			expected: "ABC1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFestivalID(tt.dirName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePhaseInfo(t *testing.T) {
	tests := []struct {
		name         string
		dirName      string
		expectedNum  int
		expectedName string
	}{
		{
			name:         "standard phase",
			dirName:      "001_PLANNING",
			expectedNum:  1,
			expectedName: "Planning",
		},
		{
			name:         "implementation phase",
			dirName:      "002_IMPLEMENTATION",
			expectedNum:  2,
			expectedName: "Implementation",
		},
		{
			name:         "lowercase phase",
			dirName:      "01_research",
			expectedNum:  1,
			expectedName: "research",
		},
		{
			name:         "multi-word phase",
			dirName:      "003_MARKER_AUTOFILL",
			expectedNum:  3,
			expectedName: "Marker autofill",
		},
		{
			name:         "no underscore",
			dirName:      "invalid",
			expectedNum:  0,
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, name := parsePhaseInfo(tt.dirName)
			assert.Equal(t, tt.expectedNum, num)
			assert.Equal(t, tt.expectedName, name)
		})
	}
}

func TestParseSequenceInfo(t *testing.T) {
	tests := []struct {
		name         string
		dirName      string
		expectedNum  int
		expectedName string
	}{
		{
			name:         "simple sequence",
			dirName:      "01_requirements",
			expectedNum:  1,
			expectedName: "requirements",
		},
		{
			name:         "multi-word sequence",
			dirName:      "02_api_design",
			expectedNum:  2,
			expectedName: "api design",
		},
		{
			name:         "no underscore",
			dirName:      "invalid",
			expectedNum:  0,
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, name := parseSequenceInfo(tt.dirName)
			assert.Equal(t, tt.expectedNum, num)
			assert.Equal(t, tt.expectedName, name)
		})
	}
}

func TestDetectPhaseType(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		expected string
	}{
		{name: "planning", dirName: "001_PLANNING", expected: "planning"},
		{name: "research", dirName: "002_RESEARCH", expected: "research"},
		{name: "implementation", dirName: "003_IMPLEMENTATION", expected: "implementation"},
		{name: "review", dirName: "004_REVIEW", expected: "review"},
		{name: "deployment dir maps to non_coding_action", dirName: "005_DEPLOYMENT", expected: "non_coding_action"},
		{name: "unknown", dirName: "006_CUSTOM", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectPhaseType(tt.dirName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildContextFromPath(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create festival directory structure
	festivalDir := filepath.Join(tmpDir, "my-festival-AF0001")
	phaseDir := filepath.Join(festivalDir, "001_PLANNING")
	sequenceDir := filepath.Join(phaseDir, "01_requirements")

	require.NoError(t, os.MkdirAll(sequenceDir, 0755))

	// Create fest.yaml to mark festival root
	festYaml := filepath.Join(festivalDir, "fest.yaml")
	require.NoError(t, os.WriteFile(festYaml, []byte("version: \"1.0\"\n"), 0644))

	t.Run("builds context from sequence path", func(t *testing.T) {
		ctx, err := BuildContextFromPath(sequenceDir, festivalDir)
		require.NoError(t, err)

		assert.Equal(t, "my festival", ctx.FestivalName)
		assert.Equal(t, "AF0001", ctx.FestivalID)
		assert.Equal(t, 1, ctx.PhaseNumber)
		assert.Equal(t, "Planning", ctx.PhaseName)
		assert.Equal(t, "planning", ctx.PhaseType)
		assert.Equal(t, 1, ctx.SequenceNumber)
		assert.Equal(t, "requirements", ctx.SequenceName)
		assert.Equal(t, "sequence", ctx.CurrentLevel)
	})

	t.Run("builds context from phase path", func(t *testing.T) {
		ctx, err := BuildContextFromPath(phaseDir, festivalDir)
		require.NoError(t, err)

		assert.Equal(t, "my festival", ctx.FestivalName)
		assert.Equal(t, 1, ctx.PhaseNumber)
		assert.Equal(t, "Planning", ctx.PhaseName)
		assert.Equal(t, 0, ctx.SequenceNumber) // No sequence
		assert.Equal(t, "phase", ctx.CurrentLevel)
	})

	t.Run("builds minimal context outside festival", func(t *testing.T) {
		ctx, err := BuildContextFromPath("/tmp", "")
		require.NoError(t, err)

		assert.Empty(t, ctx.FestivalName)
		assert.Empty(t, ctx.FestivalID)
	})
}

func TestBuildContextFromPath_WithFestYAMLMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festival with a directory name that differs from the original name
	festivalDir := filepath.Join(tmpDir, "my-great-festival-MG0001")
	phaseDir := filepath.Join(festivalDir, "001_PLANNING")
	require.NoError(t, os.MkdirAll(phaseDir, 0755))

	t.Run("reads name and goal from fest.yaml metadata", func(t *testing.T) {
		// Write fest.yaml with full metadata (original name preserved with casing)
		festYaml := `version: "1.0"
metadata:
  id: "MG0001"
  name: "My Great Festival"
  goal: "Build something amazing"
  festival_type: "standard"
`
		require.NoError(t, os.WriteFile(filepath.Join(festivalDir, "fest.yaml"), []byte(festYaml), 0644))

		ctx, err := BuildContextFromPath(phaseDir, festivalDir)
		require.NoError(t, err)

		// Should use exact name from fest.yaml, not lossy directory-name parse
		assert.Equal(t, "My Great Festival", ctx.FestivalName)
		assert.Equal(t, "Build something amazing", ctx.FestivalGoal)
		assert.Equal(t, "MG0001", ctx.FestivalID)

		// Phase parsing from directory name should still work
		assert.Equal(t, 1, ctx.PhaseNumber)
		assert.Equal(t, "Planning", ctx.PhaseName)
	})

	t.Run("falls back to directory parsing when fest.yaml has no metadata", func(t *testing.T) {
		// Write minimal fest.yaml without metadata
		require.NoError(t, os.WriteFile(filepath.Join(festivalDir, "fest.yaml"), []byte("version: \"1.0\"\n"), 0644))

		ctx, err := BuildContextFromPath(festivalDir, festivalDir)
		require.NoError(t, err)

		// Should fall back to directory name parsing (lossy but functional)
		assert.Equal(t, "my great festival", ctx.FestivalName)
		assert.Equal(t, "", ctx.FestivalGoal)
		assert.Equal(t, "MG0001", ctx.FestivalID)
	})

	t.Run("falls back to directory parsing when fest.yaml is missing", func(t *testing.T) {
		noYamlDir := filepath.Join(tmpDir, "no-yaml-fest-NY0001")
		require.NoError(t, os.MkdirAll(noYamlDir, 0755))

		ctx, err := BuildContextFromPath(noYamlDir, noYamlDir)
		require.NoError(t, err)

		assert.Equal(t, "no yaml fest", ctx.FestivalName)
		assert.Equal(t, "NY0001", ctx.FestivalID)
	})
}

func TestBuildTaskContext(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	festivalDir := filepath.Join(tmpDir, "test-festival-TF0001")
	phaseDir := filepath.Join(festivalDir, "002_IMPLEMENTATION")
	sequenceDir := filepath.Join(phaseDir, "03_core_logic")

	require.NoError(t, os.MkdirAll(sequenceDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(festivalDir, "fest.yaml"), []byte("version: \"1.0\"\n"), 0644))

	ctx, err := BuildTaskContext(sequenceDir, festivalDir, 5, "implement feature")
	require.NoError(t, err)

	// Verify full hierarchy
	assert.Equal(t, "test festival", ctx.FestivalName)
	assert.Equal(t, "TF0001", ctx.FestivalID)
	assert.Equal(t, 2, ctx.PhaseNumber)
	assert.Equal(t, "Implementation", ctx.PhaseName)
	assert.Equal(t, "implementation", ctx.PhaseType)
	assert.Equal(t, 3, ctx.SequenceNumber)
	assert.Equal(t, "core logic", ctx.SequenceName)
	assert.Equal(t, 5, ctx.TaskNumber)
	assert.Equal(t, "implement feature", ctx.TaskName)
	assert.Equal(t, "task", ctx.CurrentLevel)

	// Verify computed variables
	assert.NotEmpty(t, ctx.TaskID)
	assert.NotEmpty(t, ctx.ParentSequenceID)
	assert.NotEmpty(t, ctx.ParentPhaseID)
	assert.NotEmpty(t, ctx.FullPath)
}
