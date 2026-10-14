package loop

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
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

	if err := ctx.Err(); err != nil {
		return &ConditionResult{Pass: false, FailureContext: fmt.Sprintf("Command cancelled before execution: %v", err)}, nil
	}

	shell, args, hint, err := commandInterpreter(params, cmdStr)
	if err != nil {
		return nil, err
	}
	shellPath, err := exec.LookPath(shell)
	if err != nil {
		return &ConditionResult{
			Pass:           false,
			FailureContext: fmt.Sprintf("Command interpreter %q is unavailable: %v. %s", shell, err, hint),
		}, nil
	}

	cmd := exec.CommandContext(ctx, shellPath, args...)
	configureCommandCancellation(cmd)
	// A shell may leave a child holding stdout or stderr open after cancellation.
	// Bound the pipe wait so a timed-out condition cannot stall until that child exits.
	cmd.WaitDelay = 250 * time.Millisecond
	cmd.Dir = phasePath
	output, err := cmd.CombinedOutput()
	if ctxErr := ctx.Err(); ctxErr != nil {
		reason := "cancelled"
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			reason = "timed out"
		}
		return &ConditionResult{
			Pass:           false,
			FailureContext: fmt.Sprintf("Command %s: %v\nOutput: %s", reason, ctxErr, string(output)),
		}, nil
	}

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

// commandInterpreter keeps existing POSIX behavior on Unix. On Windows,
// PowerShell is the default; stored POSIX expressions can select sh explicitly.
func commandInterpreter(params map[string]any, command string) (string, []string, string, error) {
	shell := ""
	if raw, exists := params["shell"]; exists {
		var ok bool
		shell, ok = raw.(string)
		if !ok {
			return "", nil, "", fmt.Errorf("command condition 'shell' must be a string")
		}
	}
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = "powershell"
		} else {
			shell = "sh"
		}
	}

	switch shell {
	case "sh":
		hint := "Install a POSIX sh and put it on PATH."
		if runtime.GOOS == "windows" {
			hint = "Install Git for Windows / Git Bash and put sh.exe on PATH for POSIX command conditions, or set shell: powershell for PowerShell syntax."
		}
		return "sh", []string{"-c", command}, hint, nil
	case "powershell":
		executable := "powershell.exe"
		if runtime.GOOS != "windows" {
			executable = "pwsh"
		}
		return executable, []string{"-NoProfile", "-NonInteractive", "-Command", command}, "Install PowerShell and put its executable on PATH, or set shell: sh for POSIX syntax.", nil
	case "pwsh":
		return "pwsh", []string{"-NoProfile", "-NonInteractive", "-Command", command}, "Install PowerShell 7 and put pwsh on PATH, or set shell: sh for POSIX syntax.", nil
	default:
		return "", nil, "", fmt.Errorf("unsupported command condition shell %q (use powershell, pwsh, or sh)", shell)
	}
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
