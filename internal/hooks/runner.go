package hooks

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

type Outcome string

const (
	OutcomePass    Outcome = "pass"
	OutcomeFail    Outcome = "fail"
	OutcomeTimeout Outcome = "timeout"
	OutcomeSkipped Outcome = "skipped"
)

// SkipShortCircuit marks hooks not run because a prior closed failure blocked the list.
const SkipShortCircuit SkipReason = "short-circuit"

// HookRun is the record of one hook execution (or skip), consumed by the audit task.
type HookRun struct {
	Name     string
	Layer    Layer
	Timing   Timing
	Level    Level
	Verb     Verb
	Outcome  Outcome
	Skip     SkipReason // set when Outcome == OutcomeSkipped
	ExitCode int
	Duration time.Duration
	Fail     FailPolicy
	Blocked  bool // true when a closed failure blocked the verb
	Stdout   []byte
	Err      error // underlying exec error when Outcome is fail/timeout
}

// CommandResult is what the exec seam returns.
type CommandResult struct {
	Stdout   []byte
	ExitCode int
	Err      error
}

// Runner executes hooks. The exec seam is injectable for tests.
type Runner struct {
	WorkDir string // festival root; hook cwd
	Exec    func(ctx context.Context, command string, stdin []byte, dir string) CommandResult
}

// NewRunner creates a runner with the default process-exec seam.
func NewRunner(workDir string) *Runner {
	return &Runner{WorkDir: workDir, Exec: defaultExec}
}

func defaultExec(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return CommandResult{ExitCode: -1, Err: festerrors.Validation("hook command is empty")}
	}
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	if dir != "" {
		cmd.Dir = dir
	}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	res := CommandResult{Stdout: out, Err: err}
	if ee, ok := err.(*exec.ExitError); ok {
		res.ExitCode = ee.ExitCode()
	} else if err != nil {
		res.ExitCode = -1
	}
	if err != nil {
		if tail := stderrTail(stderr.String()); tail != "" {
			res.Err = festerrors.Wrap(err, tail)
		}
	}
	return res
}

// stderrTailLimit bounds how much hook stderr is carried on a failure record.
const stderrTailLimit = 2048

func stderrTail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > stderrTailLimit {
		s = s[len(s)-stderrTailLimit:]
	}
	return s
}

// Run executes the planned hooks for one timing in list order.
// It returns the run records and whether the verb is blocked (a closed failure occurred).
func (r *Runner) Run(ctx context.Context, level Level, verb Verb, planned []PlannedHook, stdin []byte) ([]HookRun, bool, error) {
	if r == nil {
		return nil, false, festerrors.Validation("hooks runner is nil")
	}
	if r.Exec == nil {
		r.Exec = defaultExec
	}
	var runs []HookRun
	blocked := false
	for _, p := range planned {
		if err := ctx.Err(); err != nil {
			return runs, blocked, festerrors.Wrap(err, "hook run cancelled")
		}
		if blocked {
			runs = append(runs, HookRun{
				Name: p.Name, Timing: p.Timing, Level: level, Verb: verb,
				Outcome: OutcomeSkipped, Skip: SkipShortCircuit,
			})
			continue
		}
		if p.Skip != "" {
			runs = append(runs, HookRun{
				Name: p.Name, Timing: p.Timing, Level: level, Verb: verb,
				Outcome: OutcomeSkipped, Skip: p.Skip, Layer: p.Hook.Source,
			})
			continue
		}
		run := r.runOne(ctx, level, verb, p, stdin)
		if run.Outcome != OutcomePass && p.Hook.Fail == FailClosed {
			run.Blocked = true
			blocked = true
		}
		runs = append(runs, run)
	}
	return runs, blocked, nil
}

// RunPre runs only TimingPre planned hooks. If blocked, the caller must not apply the verb.
func (r *Runner) RunPre(ctx context.Context, level Level, verb Verb, planned []PlannedHook, stdin []byte) ([]HookRun, bool, error) {
	return r.Run(ctx, level, verb, filterTiming(planned, TimingPre), stdin)
}

// RunPost runs only TimingPost planned hooks after the verb applied.
func (r *Runner) RunPost(ctx context.Context, level Level, verb Verb, planned []PlannedHook, stdin []byte) ([]HookRun, bool, error) {
	return r.Run(ctx, level, verb, filterTiming(planned, TimingPost), stdin)
}

func filterTiming(planned []PlannedHook, t Timing) []PlannedHook {
	var out []PlannedHook
	for _, p := range planned {
		if p.Timing == t {
			out = append(out, p)
		}
	}
	return out
}

func (r *Runner) runOne(ctx context.Context, level Level, verb Verb, p PlannedHook, stdin []byte) HookRun {
	run := HookRun{
		Name: p.Name, Layer: p.Hook.Source, Timing: p.Timing,
		Level: level, Verb: verb, Fail: p.Hook.Fail,
	}
	runCtx := ctx
	cancel := func() {}
	if p.Hook.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, p.Hook.Timeout)
	}
	defer cancel()
	start := time.Now()
	res := r.Exec(runCtx, p.Hook.Command, stdin, r.WorkDir)
	run.Duration = time.Since(start)
	run.ExitCode = res.ExitCode
	run.Stdout = res.Stdout
	run.Err = res.Err
	switch {
	case runCtx.Err() == context.DeadlineExceeded:
		run.Outcome = OutcomeTimeout
	case res.Err != nil || res.ExitCode != 0:
		run.Outcome = OutcomeFail
	default:
		run.Outcome = OutcomePass
	}
	return run
}
