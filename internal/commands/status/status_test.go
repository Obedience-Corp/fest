package status

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/workflow"
)

func TestValidStatuses(t *testing.T) {
	tests := []struct {
		entityType EntityType
		status     string
		expected   bool
	}{
		// Festival statuses
		{EntityFestival, "planning", true},
		{EntityFestival, "ready", true},
		{EntityFestival, "active", true},
		{EntityFestival, "ritual", true},
		{EntityFestival, "dungeon", true},
		{EntityFestival, "dungeon/completed", true},
		{EntityFestival, "dungeon/archived", true},
		{EntityFestival, "dungeon/someday", true},
		{EntityFestival, "invalid", false},

		// Phase statuses
		{EntityPhase, "pending", true},
		{EntityPhase, "in_progress", true},
		{EntityPhase, "completed", true},
		{EntityPhase, "blocked", false},

		// Sequence statuses
		{EntitySequence, "pending", true},
		{EntitySequence, "in_progress", true},
		{EntitySequence, "completed", true},
		{EntitySequence, "blocked", false},

		// Task statuses
		{EntityTask, "pending", true},
		{EntityTask, "in_progress", true},
		{EntityTask, "blocked", true},
		{EntityTask, "completed", true},
		{EntityTask, "invalid", false},

		// Gate statuses
		{EntityGate, "pending", true},
		{EntityGate, "passed", true},
		{EntityGate, "failed", true},
		{EntityGate, "completed", false},
	}

	for _, tc := range tests {
		t.Run(string(tc.entityType)+"/"+tc.status, func(t *testing.T) {
			result := isValidStatus(tc.entityType, tc.status)
			if result != tc.expected {
				t.Errorf("isValidStatus(%q, %q) = %v, want %v",
					tc.entityType, tc.status, result, tc.expected)
			}
		})
	}
}

func TestEntityTypes(t *testing.T) {
	// Verify all entity types have valid statuses defined
	entityTypes := []EntityType{
		EntityFestival,
		EntityPhase,
		EntitySequence,
		EntityTask,
		EntityGate,
	}

	for _, et := range entityTypes {
		statuses, ok := ValidStatuses[et]
		if !ok {
			t.Errorf("EntityType %q has no valid statuses defined", et)
			continue
		}
		if len(statuses) == 0 {
			t.Errorf("EntityType %q has empty valid statuses", et)
		}
	}
}

func TestIsValidStatus_UnknownEntityType(t *testing.T) {
	result := isValidStatus("unknown", "pending")
	if result {
		t.Error("isValidStatus should return false for unknown entity type")
	}
}

func TestIsValidFestivalStatus_NoSchema(t *testing.T) {
	// With empty festivalsRoot (no schema), should fall back to defaults
	tests := []struct {
		status string
		want   bool
	}{
		{"active", true},
		{"ready", true},
		{"planning", true},
		{"ritual", true},
		{"dungeon", true},
		{"dungeon/completed", true},
		{"dungeon/archived", true},
		{"dungeon/someday", true},
		{"invalid", false},
		{"custom", false},
	}

	for _, tt := range tests {
		t.Run("no_schema/"+tt.status, func(t *testing.T) {
			if got := isValidFestivalStatus("", tt.status); got != tt.want {
				t.Errorf("isValidFestivalStatus(\"\", %q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestIsValidFestivalStatus_WithSchema(t *testing.T) {
	// Create a temp directory with a custom .workflow.yaml
	tmpDir := t.TempDir()
	schemaContent := `version: 1
type: status-workflow
name: test
directories:
  active:
    description: Active items
    order: 1
  custom_status:
    description: Custom status
    order: 2
  vault:
    description: Vault
    order: 3
    nested: true
    children:
      archive:
        description: Archived
      hold:
        description: On hold
`
	schemaPath := filepath.Join(tmpDir, workflow.SchemaFileName)
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	tests := []struct {
		status string
		want   bool
	}{
		{"active", true},
		{"custom_status", true},
		{"vault/archive", true},
		{"vault/hold", true},
		{"planning", false},     // Not in custom schema
		{"dungeon", false},     // Not in custom schema
		{"nonexistent", false},
	}

	for _, tt := range tests {
		t.Run("with_schema/"+tt.status, func(t *testing.T) {
			if got := isValidFestivalStatus(tmpDir, tt.status); got != tt.want {
				t.Errorf("isValidFestivalStatus(%q, %q) = %v, want %v", tmpDir, tt.status, got, tt.want)
			}
		})
	}
}

func TestGetValidFestivalStatuses_NoSchema(t *testing.T) {
	// Without schema, should return defaults
	statuses := getValidFestivalStatuses("")
	if len(statuses) == 0 {
		t.Error("expected default statuses, got empty")
	}

	// Check that defaults include expected entries
	expected := map[string]bool{
		"active":            true,
		"ready":             true,
		"planning":          true,
		"ritual":            true,
		"dungeon":           true,
		"dungeon/completed": true,
		"dungeon/archived":  true,
		"dungeon/someday":   true,
	}
	for _, s := range statuses {
		delete(expected, s)
	}
	if len(expected) > 0 {
		t.Errorf("missing expected default statuses: %v", expected)
	}
}

func TestGetValidFestivalStatuses_WithSchema(t *testing.T) {
	// Create a temp directory with a custom .workflow.yaml
	tmpDir := t.TempDir()
	schemaContent := `version: 1
type: status-workflow
name: test
directories:
  inbox:
    description: New items
    order: 1
  doing:
    description: In progress
    order: 2
`
	schemaPath := filepath.Join(tmpDir, workflow.SchemaFileName)
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	statuses := getValidFestivalStatuses(tmpDir)
	statusSet := make(map[string]bool)
	for _, s := range statuses {
		statusSet[s] = true
	}

	if !statusSet["inbox"] {
		t.Error("expected 'inbox' from schema, not found")
	}
	if !statusSet["doing"] {
		t.Error("expected 'doing' from schema, not found")
	}
	if statusSet["active"] {
		t.Error("should NOT have 'active' when custom schema is used")
	}
}

func TestFestivalsRootFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		status   string
		expected string
	}{
		{
			name:     "simple status",
			path:     "/workspace/festivals/active/my-fest",
			status:   "active",
			expected: "/workspace/festivals",
		},
		{
			name:     "nested dungeon status",
			path:     "/workspace/festivals/dungeon/completed/my-fest",
			status:   "dungeon/completed",
			expected: "/workspace/festivals",
		},
		{
			name:     "nested dungeon archived",
			path:     "/workspace/festivals/dungeon/archived/my-fest",
			status:   "dungeon/archived",
			expected: "/workspace/festivals",
		},
		{
			name:     "planning status",
			path:     "/workspace/festivals/planning/my-fest",
			status:   "planning",
			expected: "/workspace/festivals",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := festivalsRootFromPath(tc.path, tc.status)
			if result != tc.expected {
				t.Errorf("festivalsRootFromPath(%q, %q) = %q, want %q",
					tc.path, tc.status, result, tc.expected)
			}
		})
	}
}

func TestValidateTransition_NoSchema(t *testing.T) {
	// Without a schema, all transitions should be allowed
	err := validateTransition(context.Background(), t.TempDir(), "active", "completed")
	if err != nil {
		t.Errorf("expected nil error without schema, got: %v", err)
	}
}

func TestValidateTransition_WithSchema(t *testing.T) {
	tmpDir := t.TempDir()
	schemaContent := `version: 1
type: status-workflow
name: test
directories:
  active:
    description: Active items
    order: 1
    transition_opts:
      - completed
      - dungeon
  completed:
    description: Done
    order: 2
  dungeon:
    description: Archived
    order: 3
    nested: true
    children:
      archived:
        description: Archived items
`
	schemaPath := filepath.Join(tmpDir, workflow.SchemaFileName)
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"allowed transition", "active", "completed", false},
		{"allowed nested transition", "active", "dungeon/archived", false},
		{"disallowed transition", "active", "planning", true},
		{"no transition_opts defined", "completed", "active", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTransition(context.Background(), tmpDir, tc.from, tc.to)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateTransition(%q, %q) error = %v, wantErr %v",
					tc.from, tc.to, err, tc.wantErr)
			}
		})
	}
}

func TestTransitionOptsForStatus(t *testing.T) {
	tmpDir := t.TempDir()
	schemaContent := `version: 1
type: status-workflow
name: test
directories:
  active:
    description: Active
    order: 1
    transition_opts:
      - completed
      - dungeon
  planning:
    description: Planning
    order: 2
  completed:
    description: Done
    order: 3
  dungeon:
    description: Archived
    order: 4
    nested: true
    children:
      archived:
        description: Archived items
`
	schemaPath := filepath.Join(tmpDir, workflow.SchemaFileName)
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	schema, err := workflow.LoadSchema(context.Background(), schemaPath)
	if err != nil {
		t.Fatalf("failed to load schema: %v", err)
	}

	// Status with transition_opts should return those opts
	opts := transitionOptsForStatus(schema, "active")
	if len(opts) != 2 {
		t.Errorf("expected 2 transition opts for active, got %d: %v", len(opts), opts)
	}

	// Status without transition_opts should return all directories
	opts = transitionOptsForStatus(schema, "planning")
	if len(opts) == 0 {
		t.Error("expected all directories for planning (no transition_opts), got empty")
	}
}
