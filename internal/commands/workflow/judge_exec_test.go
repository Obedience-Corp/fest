package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// withMockedJudgeLaunch installs a fake launchJudgeProcess for the duration of
// the test. Production code must never detach under go test — this is the
// only safe way to exercise the async --auto path in unit tests.
func withMockedJudgeLaunch(t *testing.T, fn func(payloadPath, phaseDir, logPath string) (int, error)) {
	t.Helper()
	orig := launchJudgeProcess
	launchJudgeProcess = fn
	t.Cleanup(func() { launchJudgeProcess = orig })
}

func TestLooksLikeGoTestBinary(t *testing.T) {
	cases := []struct {
		exe  string
		want bool
	}{
		{"/tmp/go-build123/b001/workflow.test", true},
		{`C:\Users\x\go-build\workflow.test.exe`, true},
		{"/usr/local/bin/fest", false},
		{"/Users/me/bin/fest", false},
		{"workflow.test.backup", false},
	}
	for _, tc := range cases {
		if got := looksLikeGoTestBinary(tc.exe); got != tc.want {
			t.Fatalf("looksLikeGoTestBinary(%q) = %v, want %v", tc.exe, got, tc.want)
		}
	}
}

func TestLaunchJudgeProcessDefault_RefusesGoTestBinary(t *testing.T) {
	// The process under test IS a go test binary, so the default launcher must
	// refuse rather than re-exec and fork-bomb the machine.
	_, err := launchJudgeProcessDefault("/tmp/payload.json", t.TempDir(), filepath.Join(t.TempDir(), "judge.log"))
	if err == nil {
		t.Fatal("expected refusal when executable is a go test binary")
	}
	if !strings.Contains(err.Error(), "go test binary") {
		t.Fatalf("error = %v, want go test binary refusal", err)
	}
}

func TestRunApproveAuto_AsyncFireAndForget(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)
	ctx := context.Background()

	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	steps := nav.GetSteps()

	var gotPayload, gotPhase, gotLog string
	withMockedJudgeLaunch(t, func(payloadPath, phaseDirArg, logPath string) (int, error) {
		gotPayload = payloadPath
		gotPhase = phaseDirArg
		gotLog = logPath
		return 9991, nil
	})

	// Default async path: Wait=false (zero value).
	out := captureStdout(t, func() {
		if err := runApproveAuto(ctx, nav, 2, steps[1], approvalJudgeOptions{
			JudgeCommand: "ob judge",
			Timeout:      30 * time.Second,
		}); err != nil {
			t.Fatalf("runApproveAuto: %v", err)
		}
	})
	if !strings.Contains(out, "Judge launched") {
		t.Fatalf("output missing launch notice: %q", out)
	}
	if !strings.Contains(out, "pid 9991") {
		t.Fatalf("output missing pid: %q", out)
	}

	// Checkpoint still open — fest does not wait for the verdict.
	state := nav.GetWorkflowState()
	if state.CurrentStep != 2 {
		t.Fatalf("current step = %d, want 2", state.CurrentStep)
	}
	judge := state.GetStepState(2).Judge
	if judge == nil || judge.Status != wf.JudgeRunning {
		t.Fatalf("judge = %+v, want running", judge)
	}
	if judge.Pid != 9991 {
		t.Fatalf("pid = %d, want 9991", judge.Pid)
	}
	if judge.FinishedAt != nil {
		t.Fatalf("async launch must not set FinishedAt: %+v", judge)
	}

	if gotPhase != phaseDir {
		t.Fatalf("phaseDir = %q, want %q", gotPhase, phaseDir)
	}
	if !strings.HasSuffix(gotLog, "judge.log") {
		t.Fatalf("log path = %q, want .../judge.log", gotLog)
	}

	raw, err := os.ReadFile(gotPayload)
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}
	var payload judgeExecPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	if payload.SchemaVersion != judgeExecPayloadSchema {
		t.Fatalf("schema = %q", payload.SchemaVersion)
	}
	if payload.StepNumber != 2 || payload.JudgeCommand != "ob judge" {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Timeout != (30 * time.Second).String() {
		t.Fatalf("timeout = %q, want %s", payload.Timeout, 30*time.Second)
	}
}

func TestRunApproveAuto_AsyncRefusesDuplicateWhileAlive(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)
	ctx := context.Background()

	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	steps := nav.GetSteps()

	// Use this test process's own pid so judgeProcessAlive reports true.
	self := os.Getpid()
	withMockedJudgeLaunch(t, func(payloadPath, phaseDir, logPath string) (int, error) {
		return self, nil
	})

	if err := runApproveAuto(ctx, nav, 2, steps[1], approvalJudgeOptions{JudgeCommand: "ob judge"}); err != nil {
		t.Fatalf("first launch: %v", err)
	}

	err := runApproveAuto(ctx, nav, 2, steps[1], approvalJudgeOptions{JudgeCommand: "ob judge"})
	if err == nil {
		t.Fatal("expected duplicate-launch refusal while judge pid is alive")
	}
	if !strings.Contains(err.Error(), "already evaluating") {
		t.Fatalf("error = %v, want already evaluating", err)
	}
}
