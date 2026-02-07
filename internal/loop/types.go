package loop

import (
	"time"
)

// LoopConfig represents a LOOP.yaml file.
type LoopConfig struct {
	Loops []Loop `yaml:"loops"`
}

// Loop represents a single loop condition.
type Loop struct {
	AfterStep     int       `yaml:"after_step"`
	Condition     string    `yaml:"condition"`
	OnFailure     OnFailure `yaml:"on_failure"`
	MaxIterations int       `yaml:"max_iterations"`
	OnMaxReached  string    `yaml:"on_max_reached"`
}

// OnFailure defines what to do when a condition fails.
type OnFailure struct {
	Action  string `yaml:"action"`         // "retry_step" or "return_to_step"
	Step    *int   `yaml:"step,omitempty"` // Required for return_to_step
	Context string `yaml:"context"`        // Template string with {validation_output}, {marker_list}
}

// ConditionResult is returned by all condition evaluators.
type ConditionResult struct {
	Pass           bool
	FailureContext string
}

// LoopState tracks loop execution state.
type LoopState struct {
	LoopIndex   int
	CurrentStep int
	Iterations  int
	LastChecked time.Time
	LastResult  *ConditionResult
}
