package loop

import (
	"context"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestCommandInterpreterKeepsLiteralExpression(t *testing.T) {
	const command = `Write-Output 'a b; "quoted"'`
	executable, args, _, err := commandInterpreter(map[string]any{"shell": "powershell"}, command)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" && executable != "powershell.exe" {
		t.Fatalf("Windows interpreter = %q, want powershell.exe", executable)
	}
	if want := []string{"-NoProfile", "-NonInteractive", "-Command", command}; !reflect.DeepEqual(args, want) {
		t.Fatalf("interpreter args = %#v, want %#v", args, want)
	}
	if _, _, _, err := commandInterpreter(map[string]any{"shell": "cmd"}, command); err == nil {
		t.Fatal("unsupported interpreter should return an error")
	}
}

func TestCommandEvaluatorMissingInterpreter(t *testing.T) {
	t.Setenv("PATH", "")
	result, err := (&CommandEvaluator{}).Evaluate(context.Background(), ".", map[string]any{
		"command":    "exit 0",
		"expression": "exitcode:0",
		"shell":      "sh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Pass || !strings.Contains(result.FailureContext, "PATH") || !strings.Contains(result.FailureContext, "sh") {
		t.Fatalf("missing POSIX interpreter should fail with installation hint: %+v", result)
	}
	if runtime.GOOS == "windows" && !strings.Contains(result.FailureContext, "Git Bash") {
		t.Fatalf("Windows missing-shell hint should name Git Bash: %+v", result)
	}
}

func TestCommandEvaluatorCancelledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := (&CommandEvaluator{}).Evaluate(ctx, ".", map[string]any{
		"command":    "exit 0",
		"expression": "exitcode:0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Pass || !strings.Contains(result.FailureContext, "cancelled") {
		t.Fatalf("cancelled command should fail without executing: %+v", result)
	}
}
