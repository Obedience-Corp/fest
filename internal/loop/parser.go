package loop

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseLoopConfig reads and validates a LOOP.yaml file.
func ParseLoopConfig(path string) (*LoopConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read loop config: %w", err)
	}

	var config LoopConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse loop config: %w", err)
	}

	for i, loop := range config.Loops {
		if loop.MaxIterations <= 0 {
			return nil, fmt.Errorf("loop %d: max_iterations is required and must be > 0", i)
		}

		if !isValidConditionType(loop.Condition) {
			return nil, fmt.Errorf("loop %d: unknown condition type '%s'", i, loop.Condition)
		}

		if loop.OnFailure.Action != "retry_step" && loop.OnFailure.Action != "return_to_step" {
			return nil, fmt.Errorf("loop %d: on_failure.action must be 'retry_step' or 'return_to_step'", i)
		}

		if loop.OnFailure.Action == "return_to_step" && loop.OnFailure.Step == nil {
			return nil, fmt.Errorf("loop %d: on_failure.step is required when action is 'return_to_step'", i)
		}

		if loop.OnMaxReached != "escalate" {
			return nil, fmt.Errorf("loop %d: on_max_reached must be 'escalate'", i)
		}
	}

	return &config, nil
}

func isValidConditionType(condType string) bool {
	validTypes := map[string]bool{
		"validate":     true,
		"markers":      true,
		"file_exists":  true,
		"schema_valid": true,
		"command":      true,
	}
	return validTypes[condType]
}

// FindLoopForStep returns the loop configured for the given step, if any.
func (c *LoopConfig) FindLoopForStep(step int) *Loop {
	for i := range c.Loops {
		if c.Loops[i].AfterStep == step {
			return &c.Loops[i]
		}
	}
	return nil
}
