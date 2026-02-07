package loop

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CommandEvaluator runs an arbitrary command and evaluates an expression on the output.
type CommandEvaluator struct{}

func (e *CommandEvaluator) Evaluate(ctx context.Context, phasePath string, params map[string]any) (*ConditionResult, error) {
	cmdStr, ok := params["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command condition requires 'command' parameter")
	}

	expr, ok := params["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("command condition requires 'expression' parameter")
	}

	timeout := 30 * time.Second
	if t, ok := params["timeout"].(int); ok {
		timeout = time.Duration(t) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = phasePath
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return &ConditionResult{
				Pass:           false,
				FailureContext: fmt.Sprintf("Command failed: %v\nOutput: %s", err, string(output)),
			}, nil
		}
	}

	passed := evaluateExpression(expr, string(output), exitCode)

	if passed {
		return &ConditionResult{
			Pass:           true,
			FailureContext: "",
		}, nil
	}

	return &ConditionResult{
		Pass:           false,
		FailureContext: fmt.Sprintf("Expression '%s' not satisfied\nCommand output: %s", expr, string(output)),
	}, nil
}

func evaluateExpression(expr, output string, exitCode int) bool {
	parts := strings.SplitN(expr, ":", 2)
	if len(parts) != 2 {
		return false
	}

	switch parts[0] {
	case "contains":
		return strings.Contains(output, parts[1])
	case "not_contains":
		return !strings.Contains(output, parts[1])
	case "exitcode":
		return fmt.Sprintf("%d", exitCode) == parts[1]
	default:
		return false
	}
}
