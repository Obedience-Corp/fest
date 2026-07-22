package hooks

import (
	"context"
	"errors"
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
