package loop

import (
	"context"
	"fmt"
)

// ConditionEvaluator is the interface all condition evaluators implement.
type ConditionEvaluator interface {
	Evaluate(ctx context.Context, phasePath string, params map[string]any) (*ConditionResult, error)
}

// NewEvaluator creates the appropriate evaluator for a condition type.
func NewEvaluator(condType string) (ConditionEvaluator, error) {
	switch condType {
	case "validate":
		return &ValidateEvaluator{}, nil
	case "markers":
		return &MarkersEvaluator{}, nil
	case "file_exists":
		return &FileExistsEvaluator{}, nil
	case "schema_valid":
		return &SchemaValidEvaluator{}, nil
	case "command":
		return &CommandEvaluator{}, nil
	default:
		return nil, fmt.Errorf("unknown condition type: %s", condType)
	}
}
