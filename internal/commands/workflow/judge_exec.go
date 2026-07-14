package workflow

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

const judgeExecPayloadSchema = "fest.approval.judge-exec/v1"

const (
	judgeClaimTimeout = 5 * time.Second
	judgeLockPoll     = 25 * time.Millisecond
)

// judgeExecPayload is the handoff from 'fest workflow judge' (or the legacy
// 'fest workflow approve --auto' alias) to the
// detached judge-exec runner. The runner rebuilds the judge request from live
// workflow state; the payload only pins which checkpoint was delegated and
// how to run the judge.
type judgeExecPayload struct {
	SchemaVersion string `json:"schema_version"`
	StepNumber    int    `json:"step_number"`
	StepName      string `json:"step_name"`
	JudgeCommand  string `json:"judge_command"`
	Timeout       string `json:"timeout,omitempty"`
	RunID         string `json:"run_id"`
}

// launchApproveAuto starts the judge in the background and returns. The
// checkpoint stays blocked until the detached runner applies the verdict
// through the same fail-closed path as --wait; watchers render the
// waiting-on-judge state in the meantime.
func launchApproveAuto(ctx context.Context, nav *wf.Navigator, currentStepNum int, step wf.WorkflowStep, opts approvalJudgeOptions) error {
	return withJudgeStepLock(ctx, nav.Ctx.PhasePath, currentStepNum, func() error {
		fresh, err := reloadWorkflowNavigator(ctx, nav)
		if err != nil {
			return err
		}
		state := fresh.GetWorkflowState()
		if state.CurrentStep != currentStepNum {
			return festerrors.Validation("checkpoint changed before approval judge launch").
				WithField("expected_step", currentStepNum).
				WithField("current_step", state.CurrentStep)
		}
		if err := reopenJudgeRejectionIfRequested(ctx, fresh, currentStepNum, opts); err != nil {
			return err
		}
		if ss := state.GetStepState(currentStepNum); ss != nil && ss.Judge != nil &&
			ss.Judge.Status == wf.JudgeRunning {
			if judgeLeaseActive(ss.Judge) {
				return judgeAlreadyRunningError(ss.Judge)
			}
			if _, err := fresh.RecordJudgeOutcome(ctx, currentStepNum, ss.Judge.RunID, wf.JudgeFailed,
				"detached judge process exited before recording a verdict"); err != nil {
				return festerrors.Wrap(err, "recording stale judge failure")
			}
		}
		freshSteps := fresh.GetSteps()
		if currentStepNum < 1 || currentStepNum > len(freshSteps) {
			return festerrors.Validation("checkpoint changed before approval judge launch")
		}
		step = freshSteps[currentStepNum-1]
		if err := prepareAutoJudgeReadiness(ctx, fresh, currentStepNum, step); err != nil {
			return err
		}

		runID, err := newJudgeRunID()
		if err != nil {
			return err
		}
		return launchApproveAutoLocked(ctx, fresh, currentStepNum, step, opts, runID)
	})
}

func launchApproveAutoLocked(ctx context.Context, nav *wf.Navigator, currentStepNum int, step wf.WorkflowStep, opts approvalJudgeOptions, runID string) error {
	payload := judgeExecPayload{
		SchemaVersion: judgeExecPayloadSchema,
		StepNumber:    currentStepNum,
		StepName:      step.Name,
		JudgeCommand:  opts.JudgeCommand,
		RunID:         runID,
	}
	if opts.Timeout > 0 {
		payload.Timeout = opts.Timeout.String()
	}

	festDir := filepath.Join(nav.Ctx.PhasePath, ".fest")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		return festerrors.IO("creating .fest directory for judge handoff", err).WithField("path", festDir)
	}
	payloadPath := filepath.Join(festDir, fmt.Sprintf("judge-step-%d-%s.json", currentStepNum, runID))
	data, err := json.Marshal(payload)
	if err != nil {
		return festerrors.Wrap(err, "encoding judge-exec payload")
	}
	if err := os.WriteFile(payloadPath, data, 0o600); err != nil {
		return festerrors.IO("writing judge-exec payload", err).WithField("path", payloadPath)
	}

	// Persist the run lease before starting a child. The child will not evaluate
	// until the parent durably claims this lease with its PID.
	if err := nav.BeginJudge(ctx, currentStepNum, opts.JudgeCommand, runID, 0); err != nil {
		_ = os.Remove(payloadPath)
		return festerrors.Wrap(err, "recording judge lease")
	}

	pid, err := launchJudgeProcessDefault(payloadPath, nav.Ctx.PhasePath, filepath.Join(festDir, "judge.log"))
	if err != nil {
		_, recErr := nav.RecordJudgeOutcome(ctx, currentStepNum, runID, wf.JudgeFailed, err.Error())
		_ = os.Remove(payloadPath)
		if recErr != nil {
			return festerrors.Wrap(recErr, "recording judge launch failure")
		}
		return festerrors.Wrap(err, "launching approval judge").
			WithHint("the checkpoint was not approved; fix the judge command or run 'fest workflow approve' manually")
	}

	claimed, err := nav.ClaimJudge(ctx, currentStepNum, runID, pid)
	if err != nil || !claimed {
		_ = terminateJudgeProcess(pid)
		detail := "judge process was terminated because its durable PID claim failed"
		_, recErr := nav.RecordJudgeOutcome(ctx, currentStepNum, runID, wf.JudgeFailed, detail)
		if err != nil {
			return festerrors.Wrap(err, "claiming judge process")
		}
		if recErr != nil {
			return festerrors.Wrap(recErr, "recording judge claim failure")
		}
		return festerrors.Validation("approval judge lease was superseded before launch completed")
	}

	fmt.Printf("%s Judge launched: %s (pid %d)\n", ui.Value("⚖", ui.JudgeColor), opts.JudgeCommand, pid)
	fmt.Println("  The checkpoint stays blocked until the verdict lands.")
	fmt.Printf("  Watch progress: %s\n", ui.Accent("fest show"))
	fmt.Printf("  After it returns, run %s — approved continues the workflow;\n", ui.Accent("fest next"))
	fmt.Println("  rejected records feedback and the valid recovery routes to use next.")
	return nil
}

func newJudgeRunID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", festerrors.Wrap(err, "generating approval judge run id")
	}
	return fmt.Sprintf("%x", raw[:]), nil
}

func judgeAlreadyRunningError(judge *wf.JudgeState) error {
	return festerrors.Validation("an approval judge is already evaluating this checkpoint").
		WithField("judge_command", judge.Command).
		WithField("pid", fmt.Sprintf("%d", judge.Pid)).
		WithField("run_id", judge.RunID).
		WithHint("the checkpoint unblocks when the judge returns; watch it with 'fest show' and re-run 'fest next' afterwards")
}

func judgeLeaseActive(judge *wf.JudgeState) bool {
	if judge == nil || judge.Status != wf.JudgeRunning {
		return false
	}
	if judge.Pid > 0 {
		return judgeProcessAlive(judge.Pid)
	}
	return judge.StartedAt != nil && time.Since(*judge.StartedAt) < 2*judgeClaimTimeout
}

func terminateJudgeProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

func withJudgeStepLock(ctx context.Context, phasePath string, step int, fn func() error) error {
	lockDir := filepath.Join(phasePath, ".fest")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return festerrors.IO("creating judge lock directory", err).WithField("path", lockDir)
	}
	lockPath := filepath.Join(lockDir, fmt.Sprintf("judge-step-%d.lock", step))
	release, err := acquireJudgeStepLock(ctx, lockPath)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return festerrors.Wrap(err, "waiting for judge step lock").WithField("path", lockPath)
		}
		return festerrors.IO("acquiring judge step lock", err).WithField("path", lockPath)
	}
	defer release()
	return fn()
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
	// Host tests exercise this refusal only; real launches run in the container
	// integration harness. Production always uses the fest binary.
	if looksLikeGoTestBinary(exe) {
		return 0, festerrors.Validation("refusing to detach judge-exec from a go test binary").
			WithField("executable", exe).
			WithHint("real judge launches must run in the container integration harness, never under go test")
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
	configureDetachedJudgeProcess(cmd)
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
	cmd.Flags().StringVar(&payloadPath, "payload", "", "path to the payload written by 'fest workflow judge'")
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
	if payload.RunID == "" {
		return festerrors.Validation("judge-exec payload is missing run ownership").
			WithField("payload", payloadPath)
	}

	opts := approvalJudgeOptions{
		Auto:         true,
		Wait:         true,
		JudgeCommand: payload.JudgeCommand,
	}
	if payload.Timeout != "" {
		timeout, err := time.ParseDuration(payload.Timeout)
		if err != nil {
			return festerrors.Validation("invalid judge-exec timeout").WithField("timeout", payload.Timeout)
		}
		opts.Timeout = timeout
	}

	defer func() { _ = os.Remove(payloadPath) }()

	nav, owned, err := waitForJudgeClaim(ctx, payload)
	if err != nil {
		return err
	}
	if !owned {
		return nil
	}
	opts.WorkDir = nav.Ctx.FestivalPath
	steps := nav.GetSteps()
	if payload.StepNumber < 1 || payload.StepNumber > len(steps) {
		err := festerrors.Validation("judge-exec payload step out of range").
			WithField("step", fmt.Sprintf("%d", payload.StepNumber))
		if recErr := recordJudgeFailureIfOwned(ctx, nav, payload, err.Error()); recErr != nil {
			fmt.Printf("%s failed to record judge outcome: %v\n", ui.Warning("⚠"), recErr)
		}
		return err
	}
	step := steps[payload.StepNumber-1]
	if err := checkAutoJudgePreflight(nav.Ctx.PhasePath, step); err != nil {
		reason := formatReadinessBlockReason(err)
		applied := false
		recordErr := withJudgeStepLock(ctx, nav.Ctx.PhasePath, payload.StepNumber, func() error {
			fresh, reloadErr := reloadWorkflowNavigator(ctx, nav)
			if reloadErr != nil {
				return reloadErr
			}
			decision := &approvalJudgeResponse{Decision: "reject", Reason: reason}
			var applyErr error
			applied, applyErr = applyApproveAutoVerdict(
				ctx,
				fresh,
				payload.StepNumber,
				step,
				payload.RunID,
				decision,
				"detached approval judge preflight rejected: "+reason,
			)
			return applyErr
		})
		if recordErr != nil {
			return recordErr
		}
		if !applied {
			return nil
		}
		return festerrors.Validation(reason).
			WithHint("the judge was not invoked; fix the checkpoint class or evidence before resubmitting")
	}

	decision, audit, err := judgeApproval(ctx, nav, step, opts)
	if err != nil {
		if recErr := recordJudgeFailureIfOwned(ctx, nav, payload, err.Error()); recErr != nil {
			fmt.Printf("%s failed to record judge outcome: %v\n", ui.Warning("⚠"), recErr)
		}
		return err
	}

	return withJudgeStepLock(ctx, nav.Ctx.PhasePath, payload.StepNumber, func() error {
		fresh, err := reloadWorkflowNavigator(ctx, nav)
		if err != nil {
			return err
		}
		applied, err := applyApproveAutoVerdict(ctx, fresh, payload.StepNumber, step, payload.RunID, decision, audit)
		if err != nil {
			return err
		}
		if !applied {
			fmt.Printf("%s judge run %s was superseded; verdict %q discarded\n",
				ui.Warning("⚠"), payload.RunID, decision.Decision)
		}
		return nil
	})
}

func waitForJudgeClaim(ctx context.Context, payload judgeExecPayload) (*wf.Navigator, bool, error) {
	deadline := time.Now().Add(judgeClaimTimeout)
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return nil, false, err
	}
	for {
		state := nav.GetWorkflowState()
		ss := state.GetStepState(payload.StepNumber)
		if state.CurrentStep != payload.StepNumber || ss == nil || ss.Judge == nil ||
			ss.Judge.Status != wf.JudgeRunning || ss.Judge.RunID != payload.RunID {
			return nav, false, nil
		}
		if ss.Judge.Pid == os.Getpid() {
			return nav, true, nil
		}
		if ss.Judge.Pid != 0 || time.Now().After(deadline) {
			detail := "judge runner never received its durable PID claim"
			if err := recordUnclaimedJudgeFailure(ctx, nav, payload, detail); err != nil {
				return nil, false, err
			}
			return nav, false, nil
		}
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(judgeLockPoll):
		}
		nav, err = reloadWorkflowNavigator(ctx, nav)
		if err != nil {
			return nil, false, err
		}
	}
}

func recordUnclaimedJudgeFailure(ctx context.Context, nav *wf.Navigator, payload judgeExecPayload, detail string) error {
	return withJudgeStepLock(ctx, nav.Ctx.PhasePath, payload.StepNumber, func() error {
		fresh, err := reloadWorkflowNavigator(ctx, nav)
		if err != nil {
			return err
		}
		ss := fresh.GetWorkflowState().GetStepState(payload.StepNumber)
		if ss == nil || ss.Judge == nil || ss.Judge.RunID != payload.RunID || ss.Judge.Pid != 0 {
			return nil
		}
		_, err = fresh.RecordJudgeOutcome(ctx, payload.StepNumber, payload.RunID, wf.JudgeFailed, detail)
		return err
	})
}

func recordJudgeFailureIfOwned(ctx context.Context, nav *wf.Navigator, payload judgeExecPayload, detail string) error {
	return withJudgeStepLock(ctx, nav.Ctx.PhasePath, payload.StepNumber, func() error {
		fresh, err := reloadWorkflowNavigator(ctx, nav)
		if err != nil {
			return err
		}
		_, err = fresh.RecordJudgeOutcome(ctx, payload.StepNumber, payload.RunID, wf.JudgeFailed, detail)
		return err
	})
}
