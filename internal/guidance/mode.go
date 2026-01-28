package guidance

// Mode represents the guidance operating mode.
// Each mode corresponds to a category of phase types and determines
// which navigator implementation handles the guidance.
type Mode string

const (
	// ModeImplementation is for implementation phases.
	// Tasks are code-focused with testing, review, iterate, commit gates.
	ModeImplementation Mode = "implementation"

	// ModePlan is for planning phases.
	// Tasks create festival structure (phases, sequences, task documents).
	ModePlan Mode = "plan"

	// ModeIngest is for ingest phases.
	// Tasks normalize raw input into planning-ready representations.
	// Supports user-approved loops for iterative refinement.
	ModeIngest Mode = "ingest"

	// ModeResearch is for research/discovery phases.
	// Tasks are topic-based with findings_review, documentation gates.
	ModeResearch Mode = "research"

	// ModeReview is for review/QA phases.
	// Tasks are checklist-based with sign_off gates.
	ModeReview Mode = "review"

	// ModeAction is for deployment and non-coding action phases.
	// Tasks are operational with action_verify, completion gates.
	ModeAction Mode = "action"
)

// String returns the string representation of the mode.
func (m Mode) String() string {
	return string(m)
}

// IsValid returns true if the mode is a known valid mode.
func (m Mode) IsValid() bool {
	switch m {
	case ModeImplementation, ModePlan, ModeIngest, ModeResearch, ModeReview, ModeAction:
		return true
	default:
		return false
	}
}

// AutonomyLevel represents how autonomous a step can be executed.
// This determines whether human intervention is required.
type AutonomyLevel string

const (
	// AutonomyHigh indicates the task can be fully automated.
	// Agents can complete without human review.
	AutonomyHigh AutonomyLevel = "high"

	// AutonomyMedium indicates some human oversight recommended.
	// Agents complete but human reviews output.
	AutonomyMedium AutonomyLevel = "medium"

	// AutonomyLow indicates human must be involved.
	// Agent provides guidance but human executes.
	AutonomyLow AutonomyLevel = "low"
)

// String returns the string representation of the autonomy level.
func (a AutonomyLevel) String() string {
	return string(a)
}

// ModeFromPhaseType converts a PhaseType string to the appropriate guidance Mode.
// This is the primary routing mechanism for phase-type-aware navigation.
func ModeFromPhaseType(phaseType string) Mode {
	switch phaseType {
	case "planning":
		return ModePlan
	case "implementation":
		return ModeImplementation
	case "research":
		return ModeResearch
	case "review":
		return ModeReview
	case "deployment", "action", "non_coding_action":
		return ModeAction
	case "ingest":
		return ModeIngest
	default:
		// Default to execution for unknown types
		return ModeImplementation
	}
}

// PhaseTypeFromMode returns a canonical phase type for a mode.
// Used for reverse lookups when creating new phases.
func PhaseTypeFromMode(mode Mode) string {
	switch mode {
	case ModePlan:
		return "planning"
	case ModeImplementation:
		return "implementation"
	case ModeResearch:
		return "research"
	case ModeReview:
		return "review"
	case ModeAction:
		return "deployment"
	case ModeIngest:
		return "ingest"
	default:
		return "implementation"
	}
}

// AllModes returns all valid modes.
func AllModes() []Mode {
	return []Mode{
		ModeImplementation,
		ModePlan,
		ModeIngest,
		ModeResearch,
		ModeReview,
		ModeAction,
	}
}
