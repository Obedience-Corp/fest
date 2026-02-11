package template

import (
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/stretchr/testify/assert"
)

func TestFrontmatterToReplacements_Nil(t *testing.T) {
	result := FrontmatterToReplacements(nil)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestFrontmatterToReplacements_Festival(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:    frontmatter.TypeFestival,
		ID:      "TF0001",
		Name:    "Test Festival",
		Status:  frontmatter.StatusPlanning,
		Created: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	result := FrontmatterToReplacements(fm)

	assert.Equal(t, "Test Festival", result["[REPLACE: Festival Name]"])
	assert.Equal(t, "TF0001", result["[REPLACE: FESTIVAL_ID]"])
	assert.Equal(t, "planning", result["[REPLACE: Status]"])
	assert.Equal(t, "festival", result["[REPLACE: Document type]"])
	assert.NotEmpty(t, result["[REPLACE: Created date]"])
}

func TestFrontmatterToReplacements_Phase(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:      frontmatter.TypePhase,
		ID:        "001_PLANNING",
		Name:      "Planning",
		Parent:    "my-festival",
		Order:     1,
		PhaseType: frontmatter.PhaseTypePlanning,
		Status:    frontmatter.StatusPending,
	}

	result := FrontmatterToReplacements(fm)

	assert.Equal(t, "Planning", result["[REPLACE: Phase name]"])
	assert.Equal(t, "Planning", result["[REPLACE: Phase Name]"])
	assert.Equal(t, "Planning", result["[REPLACE: PHASE_NAME]"])
	assert.Equal(t, "001_PLANNING", result["[REPLACE: Phase ID]"])
	assert.Equal(t, "1", result["[REPLACE: Phase order]"])
	assert.Equal(t, "planning", result["[REPLACE: Phase type]"])
}

func TestFrontmatterToReplacements_Sequence(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:   frontmatter.TypeSequence,
		ID:     "01_requirements",
		Name:   "Requirements Gathering",
		Parent: "001_PLANNING",
		Order:  1,
		Status: frontmatter.StatusPending,
	}

	result := FrontmatterToReplacements(fm)

	assert.Equal(t, "Requirements Gathering", result["[REPLACE: Sequence name]"])
	assert.Equal(t, "Requirements Gathering", result["[REPLACE: Sequence Name]"])
	assert.Equal(t, "Requirements Gathering", result["[REPLACE: SEQUENCE_NAME]"])
	assert.Equal(t, "01_requirements", result["[REPLACE: Sequence ID]"])
	assert.Equal(t, "1", result["[REPLACE: Sequence order]"])
	assert.Equal(t, "001_PLANNING", result["[REPLACE: Parent phase ID]"])
}

func TestFrontmatterToReplacements_Task(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:     frontmatter.TypeTask,
		ID:       "01_user_research.md",
		Name:     "User Research",
		Parent:   "01_requirements",
		Order:    1,
		Autonomy: frontmatter.AutonomyMedium,
		Status:   frontmatter.StatusPending,
	}

	result := FrontmatterToReplacements(fm)

	assert.Equal(t, "User Research", result["[REPLACE: Task name]"])
	assert.Equal(t, "User Research", result["[REPLACE: Task Name]"])
	assert.Equal(t, "User Research", result["[REPLACE: TASK_NAME]"])
	assert.Equal(t, "01_user_research.md", result["[REPLACE: Task ID]"])
	assert.Equal(t, "01_user_research", result["[REPLACE: NN_task_name]"])
	assert.Equal(t, "1", result["[REPLACE: Task order]"])
	assert.Equal(t, "01", result["[REPLACE: NN]"])
	assert.Equal(t, "01_requirements", result["[REPLACE: Parent sequence ID]"])
	assert.Equal(t, "medium", result["[REPLACE: Autonomy]"])
}

func TestFrontmatterToReplacements_Gate(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:     frontmatter.TypeGate,
		ID:       "03_testing.md",
		Name:     "Testing Gate",
		Parent:   "01_requirements",
		Order:    3,
		GateType: frontmatter.GateTesting,
		Status:   frontmatter.StatusPending,
	}

	result := FrontmatterToReplacements(fm)

	assert.Equal(t, "Testing Gate", result["[REPLACE: Task name]"])
	assert.Equal(t, "03_testing.md", result["[REPLACE: Task ID]"])
	assert.Equal(t, "03_testing", result["[REPLACE: NN_task_name]"])
	assert.Equal(t, "testing", result["[REPLACE: Gate type]"])
}

func TestFrontmatterToReplacements_TaskIDWithoutExtension(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:   frontmatter.TypeTask,
		ID:     "01_setup", // No .md extension
		Name:   "Setup",
		Parent: "01_init",
		Order:  1,
	}

	result := FrontmatterToReplacements(fm)

	assert.Equal(t, "01_setup", result["[REPLACE: Task ID]"])
	assert.Equal(t, "01_setup", result["[REPLACE: NN_task_name]"])
}

func TestFrontmatterToReplacements_EmptyValues(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type: frontmatter.TypeTask,
		// All other fields empty
	}

	result := FrontmatterToReplacements(fm)

	// Should only have document type
	assert.Equal(t, "task", result["[REPLACE: Document type]"])

	// Empty fields should not be in map
	_, hasName := result["[REPLACE: Task name]"]
	assert.False(t, hasName)

	_, hasID := result["[REPLACE: Task ID]"]
	assert.False(t, hasID)
}

func TestFrontmatterToReplacements_Priority(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:     frontmatter.TypeFestival,
		Priority: frontmatter.PriorityHigh,
	}

	result := FrontmatterToReplacements(fm)

	assert.Equal(t, "high", result["[REPLACE: Priority]"])
}

func TestMergeFrontmatterIntoContext_Nil(t *testing.T) {
	ctx := NewContext()
	MergeFrontmatterIntoContext(ctx, nil)
	// Should not panic, context unchanged
	assert.Empty(t, ctx.FestivalName)

	MergeFrontmatterIntoContext(nil, &frontmatter.Frontmatter{})
	// Should not panic
}

func TestMergeFrontmatterIntoContext_Festival(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:    frontmatter.TypeFestival,
		ID:      "TF0001",
		Name:    "Test Festival",
		Created: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	ctx := NewContext()
	MergeFrontmatterIntoContext(ctx, fm)

	assert.Equal(t, "Test Festival", ctx.FestivalName)
	assert.Equal(t, "TF0001", ctx.FestivalID)
	assert.Equal(t, "festival", ctx.CurrentLevel)
}

func TestMergeFrontmatterIntoContext_Phase(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:      frontmatter.TypePhase,
		ID:        "001_PLANNING",
		Name:      "Planning",
		Parent:    "my-festival",
		Order:     1,
		PhaseType: frontmatter.PhaseTypePlanning,
	}

	ctx := NewContext()
	MergeFrontmatterIntoContext(ctx, fm)

	assert.Equal(t, 1, ctx.PhaseNumber)
	assert.Equal(t, "Planning", ctx.PhaseName)
	assert.Equal(t, "planning", ctx.PhaseType)
	assert.Equal(t, "phase", ctx.CurrentLevel)
}

func TestMergeFrontmatterIntoContext_Sequence(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:   frontmatter.TypeSequence,
		ID:     "01_requirements",
		Name:   "Requirements",
		Parent: "001_PLANNING",
		Order:  1,
	}

	ctx := NewContext()
	MergeFrontmatterIntoContext(ctx, fm)

	assert.Equal(t, 1, ctx.SequenceNumber)
	assert.Equal(t, "Requirements", ctx.SequenceName)
	assert.Equal(t, "001_PLANNING", ctx.ParentPhaseID)
	assert.Equal(t, "sequence", ctx.CurrentLevel)
}

func TestMergeFrontmatterIntoContext_Task(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:   frontmatter.TypeTask,
		ID:     "01_research.md",
		Name:   "Research",
		Parent: "01_requirements",
		Order:  1,
	}

	ctx := NewContext()
	MergeFrontmatterIntoContext(ctx, fm)

	assert.Equal(t, 1, ctx.TaskNumber)
	assert.Equal(t, "Research", ctx.TaskName)
	assert.Equal(t, "01_requirements", ctx.ParentSequenceID)
	assert.Equal(t, "task", ctx.CurrentLevel)
}

func TestContextFromFrontmatter(t *testing.T) {
	fm := &frontmatter.Frontmatter{
		Type:      frontmatter.TypePhase,
		ID:        "002_IMPLEMENTATION",
		Name:      "Implementation",
		Parent:    "my-festival",
		Order:     2,
		PhaseType: frontmatter.PhaseTypeImplementation,
		Created:   time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC),
	}

	ctx := ContextFromFrontmatter(fm)

	assert.NotNil(t, ctx)
	assert.Equal(t, 2, ctx.PhaseNumber)
	assert.Equal(t, "Implementation", ctx.PhaseName)
	assert.Equal(t, "implementation", ctx.PhaseType)
	assert.Equal(t, "002_IMPLEMENTATION", ctx.PhaseID)
	assert.Equal(t, "phase", ctx.CurrentLevel)
}

func TestContextFromFrontmatter_Nil(t *testing.T) {
	ctx := ContextFromFrontmatter(nil)

	assert.NotNil(t, ctx)
	assert.Empty(t, ctx.FestivalName)
	assert.Empty(t, ctx.CurrentLevel)
}

func TestFrontmatterToReplacements_AllFields(t *testing.T) {
	tracking := true
	fm := &frontmatter.Frontmatter{
		Type:     frontmatter.TypeTask,
		ID:       "05_implement.md",
		Name:     "Implement Feature",
		Parent:   "02_core_logic",
		Order:    5,
		Status:   frontmatter.StatusInProgress,
		Autonomy: frontmatter.AutonomyHigh,
		Priority: frontmatter.PriorityMedium,
		Created:  time.Date(2024, 3, 15, 14, 30, 0, 0, time.FixedZone("PST", -8*3600)),
		Tracking: &tracking,
	}

	result := FrontmatterToReplacements(fm)

	// Verify all expected mappings
	assert.Equal(t, "Implement Feature", result["[REPLACE: Task name]"])
	assert.Equal(t, "05_implement.md", result["[REPLACE: Task ID]"])
	assert.Equal(t, "05_implement", result["[REPLACE: NN_task_name]"])
	assert.Equal(t, "5", result["[REPLACE: Task order]"])
	assert.Equal(t, "05", result["[REPLACE: NN]"])
	assert.Equal(t, "02_core_logic", result["[REPLACE: Parent sequence ID]"])
	assert.Equal(t, "in_progress", result["[REPLACE: Status]"])
	assert.Equal(t, "high", result["[REPLACE: Autonomy]"])
	assert.Equal(t, "medium", result["[REPLACE: Priority]"])
	assert.Equal(t, "task", result["[REPLACE: Document type]"])
	assert.NotEmpty(t, result["[REPLACE: Created date]"])
}
