package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunner_ClosedFailShortCircuits(t *testing.T) {
	var called []string
	r := &Runner{
		WorkDir: t.TempDir(),
		Exec: func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
			called = append(called, command)
			if command == "fail" {
				return CommandResult{ExitCode: 1, Err: errors.New("exit 1")}
			}
			return CommandResult{ExitCode: 0}
		},
	}
	planned := []PlannedHook{
		{Name: "a", Timing: TimingPre, Hook: ResolvedHook{Name: "a", Command: "fail", Fail: FailClosed, Source: LayerFestival}},
		{Name: "b", Timing: TimingPre, Hook: ResolvedHook{Name: "b", Command: "ok", Fail: FailClosed, Source: LayerFestival}},
	}
	runs, blocked, err := r.Run(context.Background(), LevelTask, VerbTaskComplete, planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Fatal("expected blocked")
	}
	if len(called) != 1 || called[0] != "fail" {
		t.Fatalf("called = %v", called)
	}
	if runs[0].Outcome != OutcomeFail || !runs[0].Blocked {
		t.Fatalf("run0 = %+v", runs[0])
	}
	if runs[1].Outcome != OutcomeSkipped || runs[1].Skip != SkipShortCircuit {
		t.Fatalf("run1 = %+v", runs[1])
	}
}

func TestRunner_OpenFailContinues(t *testing.T) {
	var called []string
	r := &Runner{
		Exec: func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
			called = append(called, command)
			if command == "fail" {
				return CommandResult{ExitCode: 2, Err: errors.New("exit 2")}
			}
			return CommandResult{ExitCode: 0}
		},
	}
	planned := []PlannedHook{
		{Name: "a", Timing: TimingPost, Hook: ResolvedHook{Command: "fail", Fail: FailOpen}},
		{Name: "b", Timing: TimingPost, Hook: ResolvedHook{Command: "ok", Fail: FailClosed}},
	}
	runs, blocked, err := r.Run(context.Background(), LevelGate, VerbGateApprove, planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	if blocked {
		t.Fatal("open fail must not block")
	}
	if len(called) != 2 {
		t.Fatalf("called = %v", called)
	}
	if runs[0].Outcome != OutcomeFail || runs[0].Blocked {
		t.Fatalf("run0 = %+v", runs[0])
	}
	if runs[1].Outcome != OutcomePass {
		t.Fatalf("run1 = %+v", runs[1])
	}
}

func TestRunner_TimeoutClosedBlocks(t *testing.T) {
	r := &Runner{
		Exec: func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
			<-ctx.Done()
			return CommandResult{ExitCode: -1, Err: ctx.Err()}
		},
	}
	planned := []PlannedHook{
		{Name: "slow", Timing: TimingPre, Hook: ResolvedHook{
			Command: "slow", Fail: FailClosed, Timeout: 5 * time.Millisecond,
		}},
	}
	runs, blocked, err := r.Run(context.Background(), LevelTask, VerbTaskComplete, planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !blocked || runs[0].Outcome != OutcomeTimeout {
		t.Fatalf("runs=%+v blocked=%v", runs, blocked)
	}
}

func TestRunner_TimeoutZeroNoDeadline(t *testing.T) {
	r := &Runner{
		Exec: func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
			if _, ok := ctx.Deadline(); ok {
				t.Error("expected no deadline for timeout 0")
			}
			return CommandResult{ExitCode: 0}
		},
	}
	planned := []PlannedHook{
		{Name: "n", Timing: TimingPre, Hook: ResolvedHook{Command: "n", Fail: FailClosed, Timeout: 0}},
	}
	_, _, err := r.Run(context.Background(), LevelTask, VerbTaskComplete, planned, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunner_CancellationBetweenHooks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Runner{
		Exec: func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
			cancel()
			return CommandResult{ExitCode: 0}
		},
	}
	planned := []PlannedHook{
		{Name: "a", Timing: TimingPre, Hook: ResolvedHook{Command: "a", Fail: FailClosed}},
		{Name: "b", Timing: TimingPre, Hook: ResolvedHook{Command: "b", Fail: FailClosed}},
	}
	runs, _, err := r.Run(ctx, LevelTask, VerbTaskComplete, planned, nil)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %+v", runs)
	}
}

func TestRunner_SkippedNeverExec(t *testing.T) {
	called := false
	r := &Runner{
		Exec: func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
			called = true
			return CommandResult{}
		},
	}
	planned := []PlannedHook{
		{Name: "x", Timing: TimingPre, Skip: SkipUndeclared},
		{Name: "y", Timing: TimingPre, Skip: SkipDisabled},
	}
	runs, blocked, err := r.Run(context.Background(), LevelTask, VerbTaskComplete, planned, nil)
	if err != nil || blocked || called {
		t.Fatalf("err=%v blocked=%v called=%v runs=%+v", err, blocked, called, runs)
	}
	if runs[0].Skip != SkipUndeclared || runs[1].Skip != SkipDisabled {
		t.Fatalf("runs = %+v", runs)
	}
}

func TestRunner_CapturesExitAndDuration(t *testing.T) {
	r := &Runner{
		Exec: func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
			time.Sleep(2 * time.Millisecond)
			return CommandResult{ExitCode: 7, Err: errors.New("x"), Stdout: []byte("out")}
		},
	}
	planned := []PlannedHook{
		{Name: "a", Timing: TimingPost, Hook: ResolvedHook{Command: "a", Fail: FailOpen}},
	}
	runs, _, err := r.Run(context.Background(), LevelGate, VerbGateApprove, planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	if runs[0].ExitCode != 7 || runs[0].Duration <= 0 || string(runs[0].Stdout) != "out" {
		t.Fatalf("run = %+v", runs[0])
	}
}

func TestRunner_PrePostFilters(t *testing.T) {
	var cmds []string
	r := &Runner{
		Exec: func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
			cmds = append(cmds, command)
			return CommandResult{ExitCode: 0}
		},
	}
	planned := []PlannedHook{
		{Name: "pre", Timing: TimingPre, Hook: ResolvedHook{Command: "pre-cmd"}},
		{Name: "post", Timing: TimingPost, Hook: ResolvedHook{Command: "post-cmd"}},
	}
	_, _, _ = r.RunPre(context.Background(), LevelTask, VerbTaskComplete, planned, nil)
	if len(cmds) != 1 || cmds[0] != "pre-cmd" {
		t.Fatalf("pre cmds = %v", cmds)
	}
	cmds = nil
	_, _, _ = r.RunPost(context.Background(), LevelTask, VerbTaskComplete, planned, nil)
	if len(cmds) != 1 || cmds[0] != "post-cmd" {
		t.Fatalf("post cmds = %v", cmds)
	}
}

func TestDefaultExec_FailureCarriesStderrTail(t *testing.T) {
	res := defaultExec(context.Background(), "sh -c this-command-does-not-exist-xyz", nil, t.TempDir())
	if res.Err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(res.Err.Error(), "not found") && !strings.Contains(res.Err.Error(), "exit") {
		t.Fatalf("err should carry diagnostics: %v", res.Err)
	}
}

func TestRunner_FillsContextStdinWhenNil(t *testing.T) {
	var got []byte
	r := NewRunner("")
	r.Coord = Coord{
		FestivalPath: "/fest",
		FestivalID:   "AB0001",
		Phase:        "001_PHASE",
		Task:         "001_PHASE/01_seq/01_task.md",
	}
	r.Exec = func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
		got = append([]byte(nil), stdin...)
		return CommandResult{ExitCode: 0}
	}
	planned := []PlannedHook{{
		Name: "buzz_status", Timing: TimingPost,
		Hook: ResolvedHook{Command: "camp-buzz"},
	}}
	if _, _, err := r.Run(context.Background(), LevelTask, VerbTaskComplete, planned, nil); err != nil {
		t.Fatal(err)
	}
	var payload Payload
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("stdin is not JSON: %v (%q)", err, got)
	}
	if payload.SchemaVersion != ContextSchemaVersion {
		t.Fatalf("schema = %q", payload.SchemaVersion)
	}
	if payload.Task != "001_PHASE/01_seq/01_task.md" {
		t.Fatalf("task = %q", payload.Task)
	}
	if payload.Verb != string(VerbTaskComplete) || payload.Level != string(LevelTask) {
		t.Fatalf("verb/level = %+v", payload)
	}
	if payload.Hook != "buzz_status" || payload.Timing != string(TimingPost) {
		t.Fatalf("hook/timing = %+v", payload)
	}
	if payload.FestivalID != "AB0001" || payload.FestivalPath != "/fest" {
		t.Fatalf("festival = %+v", payload)
	}
}

func TestRunner_PreservesCallerStdin(t *testing.T) {
	want := []byte(`{"schema_version":"fest.approval.judge/v1"}` + "\n")
	var got []byte
	r := NewRunner("")
	r.Coord = Coord{Task: "must-not-overwrite"}
	r.Exec = func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
		got = append([]byte(nil), stdin...)
		return CommandResult{ExitCode: 0}
	}
	planned := []PlannedHook{{
		Name: "approval_judge", Timing: TimingPost,
		Hook: ResolvedHook{Command: "ob-judge"},
	}}
	if _, _, err := r.Run(context.Background(), LevelGate, VerbGateApprove, planned, want); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("stdin = %q, want caller payload", got)
	}
}

func TestRunner_DefaultExecInjectsEnvAndStdin(t *testing.T) {
	r := NewRunner(t.TempDir())
	r.Coord = Coord{Task: "001/01/task.md", FestivalID: "AB0001"}
	planned := []PlannedHook{{
		Name: "dump", Timing: TimingPre,
		Hook: ResolvedHook{Command: "/bin/cat"},
	}}
	runs, _, err := r.Run(context.Background(), LevelTask, VerbTaskStart, planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Outcome != OutcomePass {
		t.Fatalf("runs = %+v", runs)
	}
	var payload Payload
	if err := json.Unmarshal(runs[0].Stdout, &payload); err != nil {
		t.Fatalf("cat stdout is not context JSON: %v (%q)", err, runs[0].Stdout)
	}
	if payload.Task != "001/01/task.md" || payload.Verb != string(VerbTaskStart) {
		t.Fatalf("payload = %+v", payload)
	}

	r = NewRunner(t.TempDir())
	r.Coord = Coord{Task: "001/01/task.md", FestivalID: "AB0001"}
	planned = []PlannedHook{{
		Name: "dump-env", Timing: TimingPre,
		Hook: ResolvedHook{Command: "/usr/bin/env"},
	}}
	runs, _, err = r.Run(context.Background(), LevelTask, VerbTaskStart, planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := string(runs[0].Stdout)
	for _, want := range []string{
		"FEST_HOOK=1",
		"FEST_TASK=001/01/task.md",
		"FEST_FESTIVAL=AB0001",
		"FEST_VERB=task_start",
		"FEST_HOOK_NAME=dump-env",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("env dump missing %q:\n%s", want, out)
		}
	}
}

func TestStderrTail_Bounds(t *testing.T) {
	long := strings.Repeat("x", stderrTailLimit+100)
	if got := stderrTail(long); len(got) != stderrTailLimit {
		t.Fatalf("tail length = %d, want %d", len(got), stderrTailLimit)
	}
	if got := stderrTail("  short  "); got != "short" {
		t.Fatalf("tail = %q", got)
	}
}
