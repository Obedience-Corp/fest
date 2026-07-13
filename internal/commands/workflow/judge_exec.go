package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

const judgeExecPayloadSchema = "fest.approval.judge-exec/v1"

// judgeExecPayload is the handoff from 'fest workflow approve --auto' to the
// detached judge-exec runner. The runner rebuilds the judge request from live
// workflow state; the payload only pins which checkpoint was delegated and
// how to run the judge.
type judgeExecPayload struct {
	SchemaVersion string `json:"schema_version"`
	StepNumber    int    `json:"step_number"`
	StepName      string `json:"step_name"`
	JudgeCommand  string `json:"judge_command"`
	Timeout       string `json:"timeout,omitempty"`
}

// launchJudgeProcess is a seam for tests; production detaches a judge-exec
// child so approve --auto returns immediately instead of blocking on a
// potentially long-running judge evaluation.
var launchJudgeProcess = launchJudgeProcessDefault

// launchApproveAuto starts the judge in the background and returns. The
// checkpoint stays blocked until the detached runner applies the verdict
// through the same fail-closed path as --wait; watchers render the
// waiting-on-judge state in the meantime.
func launchApproveAuto(ctx context.Context, nav *wf.Navigator, currentStepNum int, step wf.WorkflowStep, opts approvalJudgeOptions) error {
	payload := judgeExecPayload{
		SchemaVersion: judgeExecPayloadSchema,
		StepNumber:    currentStepNum,
		StepName:      step.Name,
		JudgeCommand:  opts.JudgeCommand,
	}
	if opts.Timeout > 0 {
		payload.Timeout = opts.Timeout.String()
	}

	festDir := filepath.Join(nav.Ctx.PhasePath, ".fest")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		return festerrors.IO("creating .fest directory for judge handoff", err).WithField("path", festDir)
	}
	payloadPath := filepath.Join(festDir, fmt.Sprintf("judge-step-%d.json", currentStepNum))
	data, err := json.Marshal(payload)
	if err != nil {
		return festerrors.Wrap(err, "encoding judge-exec payload")
	}
	if err := os.WriteFile(payloadPath, data, 0o600); err != nil {
		return festerrors.IO("writing judge-exec payload", err).WithField("path", payloadPath)
	}

	pid, err := launchJudgeProcess(payloadPath, nav.Ctx.PhasePath, filepath.Join(festDir, "judge.log"))
	if err != nil {
		return festerrors.Wrap(err, "launching approval judge").
			WithHint("the checkpoint was not approved; fix the judge command or run 'fest workflow approve' manually")
	}

	if err := nav.BeginJudge(ctx, currentStepNum, opts.JudgeCommand, pid); err != nil {
		return festerrors.Wrap(err, "recording judge start")
	}

	fmt.Printf("%s Judge launched: %s (pid %d)\n", ui.Value("⚖", ui.JudgeColor), opts.JudgeCommand, pid)
	fmt.Println("  The checkpoint stays blocked until the verdict lands.")
	fmt.Printf("  Watch progress: %s\n", ui.Accent("fest show"))
	fmt.Printf("  After it returns, run %s — approved continues the workflow;\n", ui.Accent("fest next"))
	fmt.Printf("  rejected records feedback to address, then %s to resubmit.\n", ui.Accent("fest workflow advance"))
	return nil
}

func launchJudgeProcessDefault(payloadPath, phaseDir, logPath string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, festerrors.Wrap(err, "resolving fest binary for judge runner")
	}
	// Hard safety: go test package binaries are named "<pkg>.test". Re-execing
	// them with "workflow judge-exec ..." does NOT run the hidden command —
	// it re-enters the test harness, re-runs every Test*, and each auto-approve
	// test detaches another child → process storm / machine freeze.
	// Tests must mock launchJudgeProcess; production always uses the fest binary.
	if looksLikeGoTestBinary(exe) {
		return 0, festerrors.Validation("refusing to detach judge-exec from a go test binary").
			WithField("executable", exe).
			WithHint("tests must mock launchJudgeProcess; never call the default launcher under go test")
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, festerrors.IO("opening judge log", err).WithField("path", logPath)
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.Command(exe, "workflow", "judge-exec", "--payload", payloadPath)
	cmd.Dir = phaseDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// New session so the runner outlives the parent fest process (fire-and-forget).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, festerrors.Wrap(err, "starting judge runner")
	}
	pid := cmd.Process.Pid
	// Detach: do not Wait; the parent returns after logging BeginJudge.
	_ = cmd.Process.Release()
	return pid, nil
}

// looksLikeGoTestBinary reports whether exe is a `go test` package binary.
// Those binaries re-run the test suite when invoked with extra argv, so they
// must never be used as the judge-exec re-exec target.
func looksLikeGoTestBinary(exe string) bool {
	base := filepath.Base(exe)
	return strings.HasSuffix(base, ".test") || strings.HasSuffix(base, ".test.exe")
}

// judgeProcessAlive reports whether the recorded judge runner pid is still
// running, so a stale running record from a crashed runner does not block
// relaunching the judge forever.
func judgeProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func newJudgeExecCmd() *cobra.Command {
	var payloadPath string
	cmd := &cobra.Command{
		Use:    "judge-exec",
		Short:  "Internal: run a delegated approval judge and apply its verdict",
		Hidden: true,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runJudgeExec(cmd.Context(), payloadPath)
		},
	}
	cmd.Flags().StringVar(&payloadPath, "payload", "", "path to the payload written by 'fest workflow approve --auto'")
	_ = cmd.MarkFlagRequired("payload")
	return cmd
}

func runJudgeExec(ctx context.Context, payloadPath string) error {
	raw, err := os.ReadFile(payloadPath)
	if err != nil {
		return festerrors.IO("reading judge-exec payload", err).WithField("path", payloadPath)
	}
	var payload judgeExecPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return festerrors.Parse("parsing judge-exec payload", err)
	}
	if payload.SchemaVersion != judgeExecPayloadSchema {
		return festerrors.Validation("unsupported judge-exec payload schema").
			WithField("schema_version", payload.SchemaVersion)
	}

	opts := approvalJudgeOptions{Auto: true, Wait: true, JudgeCommand: payload.JudgeCommand}
	if payload.Timeout != "" {
		timeout, err := time.ParseDuration(payload.Timeout)
		if err != nil {
			return festerrors.Validation("invalid judge-exec timeout").WithField("timeout", payload.Timeout)
		}
		opts.Timeout = timeout
	}

	defer func() { _ = os.Remove(payloadPath) }()

	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}
	state := nav.GetWorkflowState()
	if state.CurrentStep != payload.StepNumber {
		return nav.RecordJudgeOutcome(ctx, payload.StepNumber, wf.JudgeFailed,
			fmt.Sprintf("checkpoint moved before the judge ran (workflow now on step %d); verdict not applied", state.CurrentStep))
	}
	steps := nav.GetSteps()
	if payload.StepNumber < 1 || payload.StepNumber > len(steps) {
		return festerrors.Validation("judge-exec payload step out of range").
			WithField("step", fmt.Sprintf("%d", payload.StepNumber))
	}
	step := steps[payload.StepNumber-1]

	decision, audit, err := judgeApproval(ctx, nav, step, opts)
	if err != nil {
		if recErr := nav.RecordJudgeOutcome(ctx, payload.StepNumber, wf.JudgeFailed, err.Error()); recErr != nil {
			fmt.Printf("%s failed to record judge outcome: %v\n", ui.Warning("⚠"), recErr)
		}
		return err
	}

	// The judge may have run for a long time; re-read durable state and only
	// apply the verdict if the delegated checkpoint is still the current one
	// (an operator may have approved or reset it manually in the meantime).
	fresh, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}
	if fresh.GetWorkflowState().CurrentStep != payload.StepNumber {
		return fresh.RecordJudgeOutcome(ctx, payload.StepNumber, wf.JudgeFailed,
			fmt.Sprintf("checkpoint changed while the judge was running (workflow now on step %d); verdict %q discarded",
				fresh.GetWorkflowState().CurrentStep, decision.Decision))
	}

	return applyApproveAutoVerdict(ctx, fresh, payload.StepNumber, step, decision, audit)
}
