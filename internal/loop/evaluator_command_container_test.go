//go:build container_fs

package loop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCommandEvaluatorContainerSemantics(t *testing.T) {
	phasePath := t.TempDir()
	evaluator := &CommandEvaluator{}
	shell := "sh"
	outputCommand := "printf 'alpha beta; literal'"
	stderrCommand := "printf 'stderr-marker' >&2"
	exitCommand := "exit 7"
	cwdCommand := "pwd"
	sleepCommand := "sleep 5"
	if runtime.GOOS == "windows" {
		shell = "powershell"
		outputCommand = `Write-Output 'alpha beta; literal'`
		stderrCommand = `[Console]::Error.WriteLine('stderr-marker')`
		cwdCommand = "Write-Output (Get-Location).Path"
		sleepCommand = "Start-Sleep -Seconds 5"
	}
	evaluate := func(ctx context.Context, command, expression string, timeout int) *ConditionResult {
		t.Helper()
		params := map[string]any{"command": command, "expression": expression, "shell": shell}
		if timeout > 0 {
			params["timeout"] = timeout
		}
		result, err := evaluator.Evaluate(ctx, phasePath, params)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	if got := evaluate(context.Background(), outputCommand, "contains:beta; literal", 0); !got.Pass {
		t.Fatalf("literal output should satisfy contains: %+v", got)
	}
	if got := evaluate(context.Background(), outputCommand, "contains:missing", 0); got.Pass || !strings.Contains(got.FailureContext, "alpha beta; literal") {
		t.Fatalf("failed expression should retain output: %+v", got)
	}
	if got := evaluate(context.Background(), stderrCommand, "contains:stderr-marker", 0); !got.Pass {
		t.Fatalf("stderr should be included in expression output: %+v", got)
	}
	if got := evaluate(context.Background(), exitCommand, "exitcode:7", 0); !got.Pass {
		t.Fatalf("nonzero exit expression should pass: %+v", got)
	}
	if got := evaluate(context.Background(), cwdCommand, "contains:"+filepath.Base(phasePath), 0); !got.Pass {
		t.Fatalf("command should run from phase directory: %+v", got)
	}

	started := time.Now()
	if got := evaluate(context.Background(), sleepCommand, "exitcode:-1", 1); got.Pass || !strings.Contains(got.FailureContext, "timed out") {
		t.Fatalf("timeout must fail even when killed exit code matches: %+v", got)
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("timed-out command took %s", elapsed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	started = time.Now()
	if got := evaluate(ctx, sleepCommand, "exitcode:-1", 0); got.Pass || !strings.Contains(got.FailureContext, "cancelled") {
		t.Fatalf("cancelled command must fail: %+v", got)
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("cancelled command took %s", elapsed)
	}
}

func TestStoredCommandConditionContainerReadback(t *testing.T) {
	phasePath := t.TempDir()
	shell, command := "sh", "printf loop-ok"
	if runtime.GOOS == "windows" {
		shell, command = "powershell", "Write-Output loop-ok"
	}
	loopConfig := fmt.Sprintf(`loops:
  - after_step: 1
    condition: command
    params:
      shell: %s
      command: %s
      expression: contains:loop-ok
    on_failure:
      action: retry_step
      context: "Command output: {validation_output}"
    max_iterations: 2
    on_max_reached: escalate
`, shell, command)
	if err := os.WriteFile(filepath.Join(phasePath, "LOOP.yaml"), []byte(loopConfig), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := CheckLoopCondition(context.Background(), phasePath, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.NextStep != 2 || result.RetryContext != "" {
		t.Fatalf("stored command did not advance: %+v", result)
	}
	events, err := NewStateStore(phasePath).Events()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "loop_check" || !events[0].Passed {
		t.Fatalf("persisted command check = %+v", events)
	}
}
