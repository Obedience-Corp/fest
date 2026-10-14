//go:build windows && container_fs

package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestCommandEvaluatorWindowsLiteralExpression(t *testing.T) {
	phasePath := t.TempDir()
	outputPath := filepath.Join(phasePath, "literal-values.json")
	command := fmt.Sprintf(`$values = [string[]]@('two words', 'O''Brien', '日本語', 'a;b', '$(Write-Output injected)', ''); [IO.File]::WriteAllText('%s', (ConvertTo-Json -InputObject $values -Compress)); Write-Output '日本語'`, powerShellQuote(outputPath))
	result, err := (&CommandEvaluator{}).Evaluate(context.Background(), phasePath, map[string]any{
		"command":    command,
		"expression": "contains:日本語",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pass {
		t.Fatalf("literal command expression failed: %+v", result)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("literal command output is not JSON: %v: %s", err, data)
	}
	want := []string{"two words", "O'Brien", "日本語", "a;b", "$(Write-Output injected)", ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("literal command values = %#v, want %#v", got, want)
	}
	t.Logf("Windows condition literal output: %#v", got)
}

func TestCommandConditionCancellationStopsDescendantAndRecordsFailure(t *testing.T) {
	phasePath := t.TempDir()
	pidPath := filepath.Join(phasePath, "descendant.pid")
	followupPath := filepath.Join(phasePath, "unexpected.txt")
	command := fmt.Sprintf(`$childInfo = New-Object System.Diagnostics.ProcessStartInfo
$childInfo.FileName = 'ping.exe'
$childInfo.Arguments = '-n 31 127.0.0.1'
$childInfo.UseShellExecute = $false
$childInfo.CreateNoWindow = $true
$child = [System.Diagnostics.Process]::Start($childInfo)
[IO.File]::WriteAllText('%s', [string]$child.Id)
Start-Sleep -Seconds 30
[IO.File]::WriteAllText('%s', 'ran')`, powerShellQuote(pidPath), powerShellQuote(followupPath))
	loopConfig := fmt.Sprintf(`loops:
  - after_step: 1
    condition: command
    params:
      command: |
        %s
      expression: exitcode:0
    on_failure:
      action: retry_step
      context: "Cancelled command: {validation_output}"
    max_iterations: 2
    on_max_reached: escalate
`, strings.ReplaceAll(command, "\n", "\n        "))
	if err := os.WriteFile(filepath.Join(phasePath, "LOOP.yaml"), []byte(loopConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		result *CheckResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := CheckLoopCondition(ctx, phasePath, 1)
		done <- outcome{result, err}
	}()

	var childPID int
	deadline := time.After(10 * time.Second)
	for childPID == 0 {
		if data, err := os.ReadFile(pidPath); err == nil {
			childPID, err = strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatalf("invalid descendant PID %q: %v", data, err)
			}
		}
		if childPID != 0 {
			break
		}
		select {
		case got := <-done:
			t.Fatalf("command exited before cancellation: result=%+v err=%v", got.result, got.err)
		case <-deadline:
			t.Fatal("condition never started its descendant")
		case <-time.After(25 * time.Millisecond):
		}
	}
	t.Cleanup(func() {
		_ = exec.Command("taskkill.exe", "/PID", strconv.Itoa(childPID), "/F").Run()
	})
	child, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(childPID))
	if err != nil {
		t.Fatalf("open running descendant %d: %v", childPID, err)
	}
	defer windows.CloseHandle(child)
	if event, err := windows.WaitForSingleObject(child, 0); err != nil || event != uint32(windows.WAIT_TIMEOUT) {
		t.Fatalf("descendant %d was not running before cancellation: event=%d err=%v", childPID, event, err)
	}

	started := time.Now()
	cancel()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.result.NextStep != 1 || got.result.RetryContext == "" {
			t.Fatalf("cancelled condition advanced the step: %+v", got.result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled condition did not return promptly")
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("cancelled condition took %s", elapsed)
	}
	if event, err := windows.WaitForSingleObject(child, 5000); err != nil || event != uint32(windows.WAIT_OBJECT_0) {
		t.Fatalf("descendant %d survived cancellation: event=%d err=%v", childPID, event, err)
	}
	if _, err := os.Stat(followupPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled command ran follow-up: stat error = %v", err)
	}
	// Reopen persisted state after the check, as a separate reader would.
	events, err := NewStateStore(phasePath).Events()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Type != "loop_check" || events[0].Passed || events[1].Type != "loop_retry" || events[1].Passed {
		t.Fatalf("cancelled condition recorded success: %+v", events)
	}
	t.Logf("Windows condition cancellation: descendant PID %d exited in %s; persisted loop_check=false and loop_retry=false", childPID, time.Since(started))
}

func powerShellQuote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
