package loop

import (
	"context"
	"fmt"
	"os/exec"
)

// ValidateEvaluator runs fest validate and checks for errors.
type ValidateEvaluator struct{}

func (e *ValidateEvaluator) Evaluate(ctx context.Context, phasePath string, params map[string]any) (*ConditionResult, error) {
	cmd := exec.CommandContext(ctx, "fest", "validate", phasePath)
	output, err := cmd.CombinedOutput()

	if err == nil {
		return &ConditionResult{
			Pass:           true,
			FailureContext: "",
		}, nil
	}

	return &ConditionResult{
		Pass:           false,
		FailureContext: fmt.Sprintf("Validation errors:\n%s", string(output)),
	}, nil
}
