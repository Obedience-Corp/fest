// Package scaffold provides plan document parsing and festival structure generation.
// It reads markdown plan documents (like STRUCTURE.md) and extracts the festival
// hierarchy: phases, sequences, tasks, and their descriptions.
package scaffold

// ParsedPlan represents a complete festival structure extracted from a plan document.
type ParsedPlan struct {
	// FestivalName is the name of the festival.
	FestivalName string

	// Goal is the festival goal statement.
	Goal string

	// Phases contains the ordered phases.
	Phases []ParsedPhase
}

// ParsedPhase represents a phase within the plan.
type ParsedPhase struct {
	// Number is the phase number (e.g., 1, 2, 3).
	Number int

	// Name is the phase name (e.g., "INGEST", "PLAN", "IMPLEMENT").
	Name string

	// Type is the phase type (e.g., "ingest", "planning", "implementation").
	Type string

	// Description contains any descriptive text for this phase.
	Description string

	// Status is the phase status if specified (e.g., "COMPLETED", "IN PROGRESS").
	Status string

	// Sequences contains the ordered sequences within this phase.
	Sequences []ParsedSequence
}

// ParsedSequence represents a sequence within a phase.
type ParsedSequence struct {
	// Number is the sequence number (e.g., 1, 2, 3).
	Number int

	// Name is the sequence name (e.g., "phase_chaining").
	Name string

	// Requirement is the associated requirement ID (e.g., "REQ-003").
	Requirement string

	// Tasks contains the ordered tasks within this sequence.
	Tasks []ParsedTask
}

// ParsedTask represents a task within a sequence.
type ParsedTask struct {
	// Number is the task number (e.g., 1, 2, 3).
	Number int

	// Name is the task description/name.
	Name string
}
