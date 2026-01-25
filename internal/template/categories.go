// Package template provides marker categorization for auto-fill functionality.
package template

// MarkerCategory represents the type of marker and how it should be filled.
type MarkerCategory int

const (
	// CategoryAutoFill markers are filled from creation flags (name, ID, etc.)
	// These values are known at the time of template creation.
	CategoryAutoFill MarkerCategory = iota

	// CategoryConfig markers are filled from fest.yaml project config.
	// These are project-wide values like lint commands, test commands.
	CategoryConfig

	// CategoryManual markers require human/agent input.
	// These remain as markers until explicitly replaced.
	CategoryManual
)

// String returns a human-readable string for the marker category.
func (c MarkerCategory) String() string {
	switch c {
	case CategoryAutoFill:
		return "auto-fill"
	case CategoryConfig:
		return "config"
	case CategoryManual:
		return "manual"
	default:
		return "unknown"
	}
}

// MarkerDefinition defines a marker pattern and how it should be handled.
type MarkerDefinition struct {
	Pattern      string         // The marker pattern, e.g., "[REPLACE: Festival Name]"
	Category     MarkerCategory // How this marker should be filled
	ContextField string         // Field path in Context for CategoryAutoFill markers
	ConfigKey    string         // Key in fest.yaml for CategoryConfig markers
	Description  string         // Human-readable description of this marker
}

// DefaultMarkerDefinitions returns all known markers with their categories.
// Category A (auto-fill) and Category B (config) markers are included.
// Category C (manual) markers are NOT listed - they remain as markers by default.
func DefaultMarkerDefinitions() []MarkerDefinition {
	return []MarkerDefinition{
		// ============================================
		// Category A: Auto-fill from creation flags
		// ============================================

		// Festival-level markers
		{
			Pattern:      "[REPLACE: Festival Name]",
			Category:     CategoryAutoFill,
			ContextField: "FestivalName",
			Description:  "Name of the festival",
		},
		{
			Pattern:      "[REPLACE: FESTIVAL_ID]",
			Category:     CategoryAutoFill,
			ContextField: "FestivalID",
			Description:  "Festival identifier (e.g., TF0001)",
		},
		{
			Pattern:      "[REPLACE: Festival goal]",
			Category:     CategoryAutoFill,
			ContextField: "FestivalGoal",
			Description:  "Festival goal/objective",
		},

		// Phase-level markers
		{
			Pattern:      "[REPLACE: Phase name]",
			Category:     CategoryAutoFill,
			ContextField: "PhaseName",
			Description:  "Name of the current phase",
		},
		{
			Pattern:      "[REPLACE: Phase Name]",
			Category:     CategoryAutoFill,
			ContextField: "PhaseName",
			Description:  "Name of the current phase (alternate casing)",
		},
		{
			Pattern:      "[REPLACE: PHASE_NAME]",
			Category:     CategoryAutoFill,
			ContextField: "PhaseName",
			Description:  "Name of the current phase (screaming case)",
		},
		{
			Pattern:      "[REPLACE: Phase type]",
			Category:     CategoryAutoFill,
			ContextField: "PhaseType",
			Description:  "Type of phase (planning, implementation, etc.)",
		},
		{
			Pattern:      "[REPLACE: Phase order]",
			Category:     CategoryAutoFill,
			ContextField: "PhaseNumber",
			Description:  "Order/number of the phase",
		},
		{
			Pattern:      "[REPLACE: Phase ID]",
			Category:     CategoryAutoFill,
			ContextField: "PhaseID",
			Description:  "Formatted phase ID (e.g., 001_PLANNING)",
		},
		{
			Pattern:      "[REPLACE: First phase name]",
			Category:     CategoryAutoFill,
			ContextField: "PhaseName",
			Description:  "Name of the first phase",
		},

		// Sequence-level markers
		{
			Pattern:      "[REPLACE: Sequence name]",
			Category:     CategoryAutoFill,
			ContextField: "SequenceName",
			Description:  "Name of the current sequence",
		},
		{
			Pattern:      "[REPLACE: Sequence Name]",
			Category:     CategoryAutoFill,
			ContextField: "SequenceName",
			Description:  "Name of the current sequence (alternate casing)",
		},
		{
			Pattern:      "[REPLACE: SEQUENCE_NAME]",
			Category:     CategoryAutoFill,
			ContextField: "SequenceName",
			Description:  "Name of the current sequence (screaming case)",
		},
		{
			Pattern:      "[REPLACE: Sequence order]",
			Category:     CategoryAutoFill,
			ContextField: "SequenceNumber",
			Description:  "Order/number of the sequence",
		},
		{
			Pattern:      "[REPLACE: Sequence ID]",
			Category:     CategoryAutoFill,
			ContextField: "SequenceID",
			Description:  "Formatted sequence ID (e.g., 01_requirements)",
		},
		{
			Pattern:      "[REPLACE: First sequence name]",
			Category:     CategoryAutoFill,
			ContextField: "SequenceName",
			Description:  "Name of the first sequence",
		},

		// Task-level markers
		{
			Pattern:      "[REPLACE: Task name]",
			Category:     CategoryAutoFill,
			ContextField: "TaskName",
			Description:  "Name of the current task",
		},
		{
			Pattern:      "[REPLACE: Task Name]",
			Category:     CategoryAutoFill,
			ContextField: "TaskName",
			Description:  "Name of the current task (alternate casing)",
		},
		{
			Pattern:      "[REPLACE: TASK_NAME]",
			Category:     CategoryAutoFill,
			ContextField: "TaskName",
			Description:  "Name of the current task (screaming case)",
		},
		{
			Pattern:      "[REPLACE: Task order]",
			Category:     CategoryAutoFill,
			ContextField: "TaskNumber",
			Description:  "Order/number of the task",
		},
		{
			Pattern:      "[REPLACE: Task ID]",
			Category:     CategoryAutoFill,
			ContextField: "TaskID",
			Description:  "Formatted task ID (e.g., 01_user_research.md)",
		},
		{
			Pattern:      "[REPLACE: NN_task_name]",
			Category:     CategoryAutoFill,
			ContextField: "TaskID",
			Description:  "Formatted task ID without extension",
		},

		// Structure/parent markers
		{
			Pattern:      "[REPLACE: Parent phase ID]",
			Category:     CategoryAutoFill,
			ContextField: "ParentPhaseID",
			Description:  "ID of the parent phase",
		},
		{
			Pattern:      "[REPLACE: Parent sequence ID]",
			Category:     CategoryAutoFill,
			ContextField: "ParentSequenceID",
			Description:  "ID of the parent sequence",
		},

		// Timestamp markers
		{
			Pattern:      "[REPLACE: Created date]",
			Category:     CategoryAutoFill,
			ContextField: "CreatedDate",
			Description:  "Creation timestamp",
		},

		// ============================================
		// Category B: Config-based markers (fest.yaml)
		// ============================================
		{
			Pattern:     "[REPLACE: Run your project's lint command]",
			Category:    CategoryConfig,
			ConfigKey:   "lint_command",
			Description: "Lint command from fest.yaml project config",
		},
		{
			Pattern:     "[REPLACE: Run your project's test command]",
			Category:    CategoryConfig,
			ConfigKey:   "test_command",
			Description: "Test command from fest.yaml project config",
		},
		{
			Pattern:     "[REPLACE: Run your project's integration test command]",
			Category:    CategoryConfig,
			ConfigKey:   "integration_test_command",
			Description: "Integration test command from fest.yaml project config",
		},
		{
			Pattern:     "[REPLACE: coverage threshold, e.g., 80%]",
			Category:    CategoryConfig,
			ConfigKey:   "coverage_threshold",
			Description: "Coverage threshold from fest.yaml project config",
		},
		{
			Pattern:     "[REPLACE: Run your project's coverage command]",
			Category:    CategoryConfig,
			ConfigKey:   "coverage_command",
			Description: "Coverage command from fest.yaml project config",
		},
		{
			Pattern:     "[REPLACE: Run your project's build command]",
			Category:    CategoryConfig,
			ConfigKey:   "build_command",
			Description: "Build command from fest.yaml project config",
		},
	}
}

// MarkerRegistry provides lookup functionality for marker definitions.
type MarkerRegistry struct {
	definitions map[string]MarkerDefinition
	byCategory  map[MarkerCategory][]MarkerDefinition
}

// NewMarkerRegistry creates a new registry with all default marker definitions.
func NewMarkerRegistry() *MarkerRegistry {
	r := &MarkerRegistry{
		definitions: make(map[string]MarkerDefinition),
		byCategory:  make(map[MarkerCategory][]MarkerDefinition),
	}
	for _, def := range DefaultMarkerDefinitions() {
		r.definitions[def.Pattern] = def
		r.byCategory[def.Category] = append(r.byCategory[def.Category], def)
	}
	return r
}

// Lookup returns the definition for a marker pattern if it exists.
func (r *MarkerRegistry) Lookup(pattern string) (MarkerDefinition, bool) {
	def, ok := r.definitions[pattern]
	return def, ok
}

// IsAutoFill returns true if the marker can be auto-filled from context.
func (r *MarkerRegistry) IsAutoFill(pattern string) bool {
	def, ok := r.Lookup(pattern)
	return ok && def.Category == CategoryAutoFill
}

// IsConfig returns true if the marker should be filled from config.
func (r *MarkerRegistry) IsConfig(pattern string) bool {
	def, ok := r.Lookup(pattern)
	return ok && def.Category == CategoryConfig
}

// GetByCategory returns all marker definitions for a given category.
func (r *MarkerRegistry) GetByCategory(category MarkerCategory) []MarkerDefinition {
	return r.byCategory[category]
}

// GetAutoFillMarkers returns all auto-fill marker definitions.
func (r *MarkerRegistry) GetAutoFillMarkers() []MarkerDefinition {
	return r.GetByCategory(CategoryAutoFill)
}

// GetConfigMarkers returns all config-based marker definitions.
func (r *MarkerRegistry) GetConfigMarkers() []MarkerDefinition {
	return r.GetByCategory(CategoryConfig)
}

// All returns all registered marker definitions.
func (r *MarkerRegistry) All() []MarkerDefinition {
	defs := make([]MarkerDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		defs = append(defs, def)
	}
	return defs
}

// Count returns the total number of registered markers.
func (r *MarkerRegistry) Count() int {
	return len(r.definitions)
}
