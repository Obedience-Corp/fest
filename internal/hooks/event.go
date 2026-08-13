package hooks

type Timing string

const (
	TimingPre  Timing = "pre"
	TimingPost Timing = "post"
)

type Level string

const (
	LevelPhase    Level = "phase"
	LevelSequence Level = "sequence"
	LevelTask     Level = "task"
	LevelGate     Level = "gate"
)

type Verb string

const (
	VerbTaskStart        Verb = "task_start"
	VerbTaskComplete     Verb = "task_complete"
	VerbSequenceComplete Verb = "sequence_complete"
	VerbPhaseComplete    Verb = "phase_complete"
	VerbGateApprove      Verb = "gate_approve"
)

// Event is a lifecycle coordinate a hook binding fires at.
type Event struct {
	Timing Timing
	Level  Level
	Verb   Verb
}

// V1Verbs is the closed set of lifecycle verbs supported in v1.
func V1Verbs() []Verb {
	return []Verb{
		VerbTaskStart,
		VerbTaskComplete,
		VerbSequenceComplete,
		VerbPhaseComplete,
		VerbGateApprove,
	}
}
