package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/config"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

const approvalJudgeSchemaVersion = "fest.approval.judge/v1"

type approvalJudgeOptions struct {
	Auto          bool
	Rejudge       bool
	JudgeCommand  string
	Timeout       time.Duration
	WorkDir       string // festival path; set as judge process cwd
	OverrideJudge bool
	Summary       string // manual path summary (also used with --override-judge)
	Wait          bool
}

type approvalJudgeRequest struct {
	SchemaVersion string   `json:"schema_version"`
	FestivalPath  string   `json:"festival_path"`
	PhasePath     string   `json:"phase_path"`
	Document      string   `json:"document"`
	StepNumber    int      `json:"step_number"`
	StepName      string   `json:"step_name"`
	Goal          string   `json:"goal,omitempty"`
	Actions       []string `json:"actions,omitempty"`
	Output        string   `json:"output,omitempty"`
	Checkpoint    string   `json:"checkpoint"`
	// Evidence lists phase-relative deliverable files the judge should read as
	// evidence, beyond Document (which is only the step definition for a
	// WORKFLOW.md checkpoint). These are the same existing, non-empty artifacts
	// the readiness gate validated. Additive to fest.approval.judge/v1: an older
	// judge ignores it and sees only Document, as before.
	Evidence []string `json:"evidence,omitempty"`
}

type approvalJudgeResponse struct {
	SchemaVersion string   `json:"schema_version"`
	Decision      string   `json:"decision"`
	Reason        string   `json:"reason"`
	Confidence    *float64 `json:"confidence,omitempty"`
	Followups     []string `json:"followups,omitempty"`
}

type approvalJudgeRunner func(ctx context.Context, command string, stdin []byte, dir string) ([]byte, error)

var runApprovalJudgeCommand approvalJudgeRunner = runApprovalJudgeCommandDefault

func newApproveCmd() *cobra.Command {
	var actor string
	opts := approvalJudgeOptions{}

	cmd := &cobra.Command{
		Use:   "approve",
		Short: "Approve a blocking checkpoint",
		Long: `Approve a blocking checkpoint and proceed to the next step.

Some workflow steps require explicit user approval before proceeding.
This is typically used for review gates or major decision points.

After approval:
  - The current step is marked as approved
  - The workflow advances to the next step

Auto approval:
  Configuring hooks.approval_judge.command is the operator opt-in that
  delegates blocking checkpoints away from human review. With that hook set,
  fest next auto-invokes the judge on blocking WORKFLOW.md / GATES.md steps.

  Use 'fest workflow judge' to re-run the judge explicitly after a rejection;
  '--auto' remains a backwards-compatible alias. Agents must not clear checkpoints with --as agent;
  agent-actor decisions are recorded only via the judge path.

  Checkpoint classes:
    artifact_review         — deliverables can be auto-judged when evidence is ready
    operator_attestation    — human must approve; --auto is refused and plain
                              manual approval requires an interactive TTY

  Presentation-like steps require non-empty evidence (e.g. output_specs/PRESENTATION.md)
  before the judge is invoked. Missing evidence blocks deterministically without a model call.

  After a judge reject, re-submit with: fest workflow judge
  Operator override (interactive TTY, or --override-judge --summary "..."):
  records decision_actor=user_override.

  When an approval judge is configured, non-interactive manual approve is refused
  so agents cannot mint decision_actor=user. Use a real terminal and type APPROVE.

  The judge command receives JSON on stdin using schema fest.approval.judge/v1
  and must return JSON on stdout with decision "approve" or "reject" and a
  reason. Missing commands, timeouts, non-zero exits, malformed JSON, unknown
  decisions, and empty reasons fail closed and do not approve the checkpoint.

  The judge command is resolved as: --judge-command flag, else the
  hooks.approval_judge.command hook in .festival/config.yaml. If neither is
  set, --auto fails closed and leaves the checkpoint unchanged.

      hooks:
        approval_judge:
          command: ob judge

  By default --auto launches the judge in the background and returns
  immediately; the checkpoint stays blocked until the verdict lands, and
  'fest show' renders the waiting-on-judge state while it runs. Use --wait
  to block until the judge returns instead.`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Auto {
				opts.Rejudge = true
				return runApproveWithOptions(cmd.Context(), wf.DecisionMetadata{}, opts)
			}
			decision, err := normalizeDecision("approval", actor, opts.Summary)
			if err != nil {
				return err
			}
			return runApproveWithOptions(cmd.Context(), decision, opts)
		},
	}

	cmd.Flags().StringVar(&actor, "as", decisionActorUser, "deprecated: manual approvals are always recorded as the user; agent decisions require --auto with a configured judge")
	_ = cmd.Flags().MarkHidden("as")
	cmd.Flags().StringVar(&opts.Summary, "summary", "", "approval summary or rationale (required with --override-judge)")
	cmd.Flags().BoolVar(&opts.Auto, "auto", false, "delegate this checkpoint decision to the configured approval judge command")
	cmd.Flags().StringVar(&opts.JudgeCommand, "judge-command", opts.JudgeCommand, "approval judge command for --auto (overrides the .festival/config.yaml hooks.approval_judge.command hook)")
	cmd.Flags().DurationVar(&opts.Timeout, "judge-timeout", opts.Timeout, "maximum time to wait for the approval judge (0 waits until it returns)")
	cmd.Flags().BoolVar(&opts.OverrideJudge, "override-judge", false, "operator override of a judge/readiness reject (requires --summary; for non-interactive use)")
	cmd.Flags().BoolVar(&opts.Wait, "wait", false, "block until the judge returns instead of launching it in the background")
	cmd.MarkFlagsMutuallyExclusive("auto", "as")
	cmd.MarkFlagsMutuallyExclusive("auto", "summary")
	cmd.MarkFlagsMutuallyExclusive("auto", "override-judge")

	return cmd
}

func runApprove(ctx context.Context) error {
	return runApproveWithDecision(ctx, wf.DecisionMetadata{Actor: decisionActorUser})
}

func runApproveWithDecision(ctx context.Context, decision wf.DecisionMetadata) error {
	return runApproveWithOptions(ctx, decision, approvalJudgeOptions{})
}

func runApproveWithOptions(ctx context.Context, decision wf.DecisionMetadata, opts approvalJudgeOptions) error {
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}

	if err := lifecycle.EnforcePreActive(ctx, nav.Ctx.FestivalPath, lifecycle.EnforceOptions{
		PhasePath: nav.Ctx.PhasePath,
		Reason:    "fest workflow approve",
	}); err != nil {
		return err
	}

	state := nav.GetWorkflowState()
	steps := nav.GetSteps()

	// Check if already complete
	if state.IsComplete() {
		fmt.Println(ui.Success("✓ Workflow already complete!"))
		return nil
	}

	// Get current step info
	currentStepNum := state.CurrentStep
	if currentStepNum < 1 || currentStepNum > len(steps) {
		return festerrors.Validation("invalid workflow state").
			WithField("current_step", currentStepNum).
			WithField("total_steps", len(steps))
	}

	step := steps[currentStepNum-1]

	// Verify this is a checkpoint step
	if !step.Checkpoint.IsBlocking() {
		return festerrors.Validation("step does not have a blocking checkpoint").
			WithField("step", currentStepNum).
			WithHint("Use 'fest workflow advance' for regular steps")
	}

	if opts.Auto {
		judgeCommand, err := resolveApprovalJudgeCommandFor(ctx, nav, opts.JudgeCommand)
		if err != nil {
			return err
		}
		opts.JudgeCommand = judgeCommand
		opts.WorkDir = nav.Ctx.FestivalPath
		return runApproveAuto(ctx, nav, currentStepNum, step, opts)
	}

	return withJudgeStepLock(ctx, nav.Ctx.PhasePath, currentStepNum, func() error {
		fresh, err := reloadWorkflowNavigator(ctx, nav)
		if err != nil {
			return err
		}
		freshState := fresh.GetWorkflowState()
		if freshState.CurrentStep != currentStepNum {
			return festerrors.Validation("checkpoint changed before approval").
				WithField("expected_step", currentStepNum).
				WithField("current_step", freshState.CurrentStep)
		}
		freshSteps := fresh.GetSteps()
		if currentStepNum < 1 || currentStepNum > len(freshSteps) {
			return festerrors.Validation("checkpoint changed before approval")
		}
		freshStep := freshSteps[currentStepNum-1]
		judgeConfigured, err := approvalJudgeConfiguredFor(ctx, fresh)
		if err != nil {
			return err
		}
		decision, err = resolveManualApprovalDecision(
			decision,
			judgeConfigured,
			wf.ClassifyCheckpoint(freshStep) == wf.CheckpointClassOperatorAttestation,
			opts.OverrideJudge,
			freshState.GetCurrentStepState(),
			currentStepNum,
			freshStep.Name,
		)
		if err != nil {
			return err
		}
		if err := fresh.ApproveWithDecision(ctx, decision); err != nil {
			return festerrors.Wrap(err, "approving checkpoint")
		}

		fmt.Printf("%s Step %d: %s approved\n", ui.Success("✓"), currentStepNum, freshStep.Name)
		if decision.Actor != "" {
			fmt.Printf("  %s: %s\n", ui.Label("Approved by"), decision.Actor)
		}
		if decision.Summary != "" {
			fmt.Printf("  %s: %s\n", ui.Label("Summary"), decision.Summary)
		}
		return showNextStep(ctx, fresh, fresh.GetSteps())
	})
}

type workspaceConfigLoader func(string) (*config.WorkspaceConfig, error)

func approvalJudgeConfiguredFor(ctx context.Context, nav *wf.Navigator) (bool, error) {
	festivalsRoot, err := approvalJudgeFestivalsRoot(ctx, nav)
	if err != nil {
		return false, err
	}
	if festivalsRoot == "" {
		return false, nil
	}
	cfg, err := config.LoadWorkspaceConfig(festivalsRoot)
	if err != nil {
		return false, festerrors.Wrap(err, "loading approval judge authority configuration")
	}
	if cfg == nil {
		return false, festerrors.Validation("approval judge authority configuration is empty")
	}
	return strings.TrimSpace(cfg.Hooks.ApprovalJudge.Command) != "", nil
}

func approvalJudgeFestivalsRoot(ctx context.Context, nav *wf.Navigator) (string, error) {
	if ws, ok := scope.WorkspaceFrom(ctx); ok && ws != nil && strings.TrimSpace(ws.FestivalsPath) != "" {
		return ws.FestivalsPath, nil
	}
	if nav == nil || nav.Ctx == nil || strings.TrimSpace(nav.Ctx.FestivalPath) == "" {
		return "", nil
	}
	ws, err := workspace.FindWorkspace(ctx, nav.Ctx.FestivalPath)
	if err != nil {
		if errors.Is(err, workspace.ErrNoWorkspace) || errors.Is(err, workspace.ErrNoCampaign) {
			return "", nil
		}
		return "", festerrors.Wrap(err, "resolving approval judge authority configuration")
	}
	return ws.FestivalsPath, nil
}

func approvalJudgeConfiguredWithLoader(ctx context.Context, load workspaceConfigLoader) (bool, error) {
	ws, ok := scope.WorkspaceFrom(ctx)
	if !ok || ws == nil || strings.TrimSpace(ws.FestivalsPath) == "" {
		return false, nil
	}
	cfg, err := load(ws.FestivalsPath)
	if err != nil {
		return false, festerrors.Wrap(err, "loading approval judge authority configuration")
	}
	if cfg == nil {
		return false, festerrors.Validation("approval judge authority configuration is empty")
	}
	return strings.TrimSpace(cfg.Hooks.ApprovalJudge.Command) != "", nil
}

// resolveApprovalJudgeCommand resolves the command used for --auto approval.
// Precedence: the --judge-command flag, then the hooks.approval_judge.command
// hook in .festival/config.yaml (via workspace context or navigator festival
// path). It fails closed when neither is set so no checkpoint is delegated to
// an unconfigured (or assumed) command.
func resolveApprovalJudgeCommand(ctx context.Context, flagValue string) (string, error) {
	return resolveApprovalJudgeCommandFor(ctx, nil, flagValue)
}

func resolveApprovalJudgeCommandFor(ctx context.Context, nav *wf.Navigator, flagValue string) (string, error) {
	if cmd := strings.TrimSpace(flagValue); cmd != "" {
		return cmd, nil
	}

	festivalsRoot, err := approvalJudgeFestivalsRoot(ctx, nav)
	if err != nil {
		return "", err
	}
	if festivalsRoot != "" {
		cfg, err := config.LoadWorkspaceConfig(festivalsRoot)
		if err != nil {
			return "", festerrors.Wrap(err, "loading approval judge hook")
		}
		if cfg == nil {
			return "", festerrors.Validation("approval judge authority configuration is empty")
		}
		if cmd := strings.TrimSpace(cfg.Hooks.ApprovalJudge.Command); cmd != "" {
			return cmd, nil
		}
	}

	return "", festerrors.Validation("approval judge command is not configured").
		WithHint(`--auto delegates this checkpoint to an approval judge command, but none is configured.

Configure a command in .festival/config.yaml. It receives the approval request
as JSON on stdin and must print a JSON verdict on stdout (schema
fest.approval.judge/v1):

hooks:
  approval_judge:
    command: <your-approval-judge-tool>

Example (using the obey CLI):

hooks:
  approval_judge:
    command: ob judge

Or pass --judge-command <cmd> for a one-off.`)
}

// AutoDelegateBlockingCheckpoints runs the configured approval judge for each
// consecutive blocking checkpoint while a judge is configured.
//
// This is the fest next integration path: configuring hooks.approval_judge is
// an operator opt-in to skip human pings on GATES.md / WORKFLOW.md checkpoints.
// When no judge is configured, this is a no-op and the caller formats the
// manual checkpoint instructions.
//
// Fail-closed: judge errors leave the checkpoint unchanged and return the
// error. Reject verdicts are recorded by runApproveAuto and return nil so the
// agent can address feedback.
func AutoDelegateBlockingCheckpoints(ctx context.Context, nav *wf.Navigator) error {
	if nav == nil {
		return nil
	}
	if err := nav.EnsureInitialized(); err != nil {
		return err
	}

	// Cap iterations so a misbehaving state machine cannot loop forever.
	const maxAutoSteps = 32
	for i := 0; i < maxAutoSteps; i++ {
		state := nav.GetWorkflowState()
		steps := nav.GetSteps()
		if state == nil || state.IsComplete() || len(steps) == 0 {
			return nil
		}
		current := state.CurrentStep
		if current < 1 || current > len(steps) {
			return nil
		}
		step := steps[current-1]
		if wf.ClassifyCheckpoint(step) == wf.CheckpointClassOperatorAttestation {
			// Operator attestations are intentionally never delegated. Leave the
			// checkpoint untouched so fest next renders the human approval path.
			return nil
		}
		stepState := state.GetStepState(current)
		// Pending or in_progress blocking checkpoints are eligible. fest next
		// often lands here before GetNext promotes pending → in_progress.
		if stepState == nil || !step.Checkpoint.IsBlocking() {
			return nil
		}
		switch stepState.Status {
		case wf.StepStatusInProgress, wf.StepStatusPending:
			// eligible
		default:
			return nil
		}

		judgeConfigured, err := approvalJudgeConfiguredFor(ctx, nav)
		if err != nil {
			return err
		}
		if !judgeConfigured {
			// No judge configured — leave checkpoint for manual / agent instruction path.
			return nil
		}
		judgeCommand, err := resolveApprovalJudgeCommandFor(ctx, nav, "")
		if err != nil {
			return err
		}

		// fest next must not error while a prior fire-and-forget judge is still
		// alive — report waiting-on-judge and leave the checkpoint blocked.
		if stepState.Judge != nil && stepState.Judge.Status == wf.JudgeRunning &&
			judgeLeaseActive(stepState.Judge) {
			cmd := stepState.Judge.Command
			if cmd == "" {
				cmd = judgeCommand
			}
			fmt.Printf("%s Approval judge still running: %s (pid %d)\n",
				ui.Value("⚖", ui.JudgeColor), cmd, stepState.Judge.Pid)
			fmt.Println("  Checkpoint stays blocked until the verdict lands.")
			fmt.Printf("  Watch progress: %s\n", ui.Accent("fest workflow status"))
			fmt.Printf("  After it returns, run %s again.\n", ui.Accent("fest next"))
			return nil
		}

		fmt.Printf("%s Checkpoint delegated to approval judge (%s)\n", ui.Info("→"), judgeCommand)
		fmt.Printf("  Step %d: %s\n", current, step.Name)

		opts := approvalJudgeOptions{Auto: true, JudgeCommand: judgeCommand}
		if err := runApproveAuto(ctx, nav, current, step, opts); err != nil {
			return err
		}

		// Reject or fail-closed leave state non-progressing; stop so the
		// caller can render the blocked checkpoint / next instructions.
		nextState := nav.GetWorkflowState()
		if nextState == nil || nextState.IsComplete() {
			return nil
		}
		if nextState.CurrentStep == current {
			return nil
		}
		// Approved and advanced — continue if the next step is also a
		// delegated blocking checkpoint.
	}
	return nil
}

// prepareAutoJudgeReadiness runs deterministic authorization and evidence
// checks while the caller holds the step lock. Readiness failures are recorded
// durably as agent decisions before any judge process is launched.
func prepareAutoJudgeReadiness(ctx context.Context, nav *wf.Navigator, currentStepNum int, step wf.WorkflowStep) error {
	if err := checkAutoJudgeAllowed(step); err != nil {
		return err
	}
	if err := checkApprovalReadiness(nav.Ctx.PhasePath, step); err != nil {
		reason := formatReadinessBlockReason(err)
		if recErr := nav.RejectWithDecision(ctx, reason, wf.DecisionMetadata{
			Actor:   decisionActorAgent,
			Summary: reason,
		}); recErr != nil {
			return festerrors.Wrap(recErr, "recording readiness rejection")
		}
		fmt.Printf("%s Step %d: %s blocked (readiness)\n", ui.Warning("⚠"), currentStepNum, step.Name)
		fmt.Printf("  %s: %s\n\n", ui.Label("Reason"), reason)
		printApprovalRecoveryFor(ctx, nav, step)
		return festerrors.Validation(reason).
			WithHint("fix the evidence, then use one of the approval routes shown above")
	}
	return nil
}

func checkAutoJudgePreflight(phasePath string, step wf.WorkflowStep) error {
	if err := checkAutoJudgeAllowed(step); err != nil {
		return err
	}
	return checkApprovalReadiness(phasePath, step)
}

func runApproveAuto(ctx context.Context, nav *wf.Navigator, currentStepNum int, step wf.WorkflowStep, opts approvalJudgeOptions) error {
	if !opts.Wait {
		return launchApproveAuto(ctx, nav, currentStepNum, step, opts)
	}

	runID, err := newJudgeRunID()
	if err != nil {
		return err
	}
	err = withJudgeStepLock(ctx, nav.Ctx.PhasePath, currentStepNum, func() error {
		fresh, err := reloadWorkflowNavigator(ctx, nav)
		if err != nil {
			return err
		}
		if fresh.GetWorkflowState().CurrentStep != currentStepNum {
			return festerrors.Validation("checkpoint changed before approval judge started")
		}
		freshSteps := fresh.GetSteps()
		if currentStepNum < 1 || currentStepNum > len(freshSteps) {
			return festerrors.Validation("checkpoint changed before approval judge started")
		}
		step = freshSteps[currentStepNum-1]
		rejudgePreflighted, err := reopenJudgeRejectionIfRequested(ctx, fresh, currentStepNum, step, opts)
		if err != nil {
			return err
		}
		if err := reconcileJudgeBeforeLaunch(ctx, fresh, currentStepNum); err != nil {
			return err
		}
		if !rejudgePreflighted {
			if err := prepareAutoJudgeReadiness(ctx, fresh, currentStepNum, step); err != nil {
				return err
			}
		}
		if err := fresh.BeginJudge(ctx, currentStepNum, opts.JudgeCommand, runID, os.Getpid()); err != nil {
			return festerrors.Wrap(err, "recording judge start")
		}
		nav = fresh
		return nil
	})
	if err != nil {
		return err
	}

	decision, audit, err := judgeApproval(ctx, nav, step, opts)
	if err != nil {
		// Synchronous --wait path: the agent is still mid-turn, so no detached
		// continuation is delivered (the ad-hoc payload carries no session
		// identity and delivery must not interleave an in-flight turn).
		if _, recErr := recordJudgeFailureIfOwned(ctx, nav, judgeExecPayload{
			StepNumber: currentStepNum, RunID: runID,
		}, err.Error()); recErr != nil {
			fmt.Printf("%s failed to record judge outcome: %v\n", ui.Warning("⚠"), recErr)
		}
		return err
	}

	return withJudgeStepLock(ctx, nav.Ctx.PhasePath, currentStepNum, func() error {
		fresh, err := reloadWorkflowNavigator(ctx, nav)
		if err != nil {
			return err
		}
		_, err = applyApproveAutoVerdict(ctx, fresh, currentStepNum, step, runID, decision, audit)
		return err
	})
}

// reconcileJudgeBeforeLaunch closes a dead detached judge lease before a new
// run starts. The failure is persisted through the event log so another
// process cannot continue to render the checkpoint as waiting forever.
func reconcileJudgeBeforeLaunch(ctx context.Context, nav *wf.Navigator, step int) error {
	state := nav.GetWorkflowState()
	stepState := state.GetStepState(step)
	if stepState == nil || stepState.Judge == nil || stepState.Judge.Status != wf.JudgeRunning {
		return nil
	}
	if judgeLeaseActive(stepState.Judge) {
		return judgeAlreadyRunningError(stepState.Judge)
	}

	recorded, err := nav.RecordJudgeFailure(ctx, step, stepState.Judge.RunID,
		"detached judge process exited before recording a verdict")
	if err != nil {
		return festerrors.Wrap(err, "recording stale judge failure")
	}
	if !recorded {
		return festerrors.Validation("approval judge lease changed before stale-run cleanup").
			WithHint("run fest workflow status, then retry the approval judge")
	}
	return nil
}

// reopenJudgeRejectionIfRequested clears only a judge-owned blocked state
// after the same deterministic preflight used for a fresh auto-judge run.
// Rejected preflight leaves the original blocked state untouched.
func reopenJudgeRejectionIfRequested(ctx context.Context, nav *wf.Navigator, stepNum int, step wf.WorkflowStep, opts approvalJudgeOptions) (bool, error) {
	if !opts.Rejudge {
		return false, nil
	}
	state := nav.GetWorkflowState().GetStepState(stepNum)
	if state == nil || state.Status != wf.StepStatusBlocked {
		return false, nil
	}
	if err := checkAutoJudgePreflight(nav.Ctx.PhasePath, step); err != nil {
		return false, err
	}
	if err := nav.ReopenJudgeRejection(ctx, stepNum); err != nil {
		return false, err
	}
	return true, nil
}

func applyApproveAutoVerdict(ctx context.Context, nav *wf.Navigator, currentStepNum int, step wf.WorkflowStep, runID string, decision *approvalJudgeResponse, audit string) (bool, error) {
	judgeDecision := wf.DecisionMetadata{Actor: decisionActorAgent, Summary: decision.Reason, Followups: decision.Followups}

	switch decision.Decision {
	case "approve":
		applied, err := nav.ApplyJudgeApproval(ctx, currentStepNum, runID, audit, judgeDecision)
		if err != nil {
			return false, festerrors.Wrap(err, "approving checkpoint from judge decision")
		}
		if !applied {
			return false, nil
		}
		fmt.Printf("%s Step %d: %s auto-approved\n", ui.Success("✓"), currentStepNum, step.Name)
		fmt.Printf("  %s: %s\n", ui.Label("Reason"), decision.Reason)
		return true, showNextStep(ctx, nav, nav.GetSteps())
	case "reject":
		applied, err := nav.ApplyJudgeRejection(ctx, currentStepNum, runID, audit, judgeDecision)
		if err != nil {
			return false, festerrors.Wrap(err, "recording judge rejection")
		}
		if !applied {
			return false, nil
		}
		fmt.Printf("%s Step %d: %s auto-rejected\n", ui.Warning("⚠"), currentStepNum, step.Name)
		fmt.Printf("  %s: %s\n", ui.Label("Reason"), decision.Reason)
		printJudgeFollowups(decision.Followups)
		fmt.Println()
		fmt.Println("The step is now blocked. Address the feedback and revise the work.")
		printApprovalRecoveryFor(ctx, nav, step)
		return true, nil
	default:
		// Unreachable today: parseApprovalJudgeResponse already rejects any
		// decision outside {approve, reject} (that path records JudgeFailed via
		// the error branch above). Kept fail-closed so the judge lifecycle is
		// still resolved rather than left "running" if that validation is ever
		// relaxed.
		unsupported := festerrors.Validation("approval judge returned unsupported decision").
			WithField("decision", decision.Decision).
			WithHint("allowed decisions are approve and reject")
		if _, recErr := nav.RecordJudgeOutcome(ctx, currentStepNum, runID, wf.JudgeFailed, unsupported.Error()); recErr != nil {
			fmt.Printf("%s failed to record judge outcome: %v\n", ui.Warning("⚠"), recErr)
		}
		return false, unsupported
	}
}

func judgeApproval(ctx context.Context, nav *wf.Navigator, step wf.WorkflowStep, opts approvalJudgeOptions) (*approvalJudgeResponse, string, error) {
	req := approvalJudgeRequest{
		SchemaVersion: approvalJudgeSchemaVersion,
		FestivalPath:  nav.Ctx.FestivalPath,
		PhasePath:     nav.Ctx.PhasePath,
		Document:      "WORKFLOW.md",
		StepNumber:    step.Number,
		StepName:      step.Name,
		Goal:          step.Goal,
		Actions:       step.Actions,
		Output:        step.Output,
		Checkpoint:    step.Checkpoint.String(),
	}
	if nav.IsGate() {
		req.Document = "GATES.md"
	}
	// Prefer human checkpoint text when available for the judge rubric.
	if text := strings.TrimSpace(step.CheckpointText); text != "" {
		req.Checkpoint = text
	}
	// Attach the step's deliverable files so the judge reads the actual work,
	// not just the step definition. Without this the judge sees only Document
	// and rejects (no evidence) or approves on the agent's self-report.
	req.Evidence = resolveExistingEvidencePaths(nav.Ctx.PhasePath, step)

	return evaluateApprovalJudge(ctx, req, opts)
}

func evaluateApprovalJudge(ctx context.Context, req approvalJudgeRequest, opts approvalJudgeOptions) (*approvalJudgeResponse, string, error) {
	stdin, err := json.Marshal(req)
	if err != nil {
		return nil, "", festerrors.Wrap(err, "encoding approval judge request")
	}

	// No default deadline: judge evaluations can legitimately run long, and
	// killing one mid-inference wastes the run. --judge-timeout opts into a
	// bound; the judge remains observable while it runs (wf_judge_started,
	// waiting-on-judge indicator).
	judgeCtx := ctx
	cancel := context.CancelFunc(func() {})
	if opts.Timeout > 0 {
		judgeCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	}
	defer cancel()

	out, err := runApprovalJudgeCommand(judgeCtx, opts.JudgeCommand, append(stdin, '\n'), opts.WorkDir)
	if err != nil {
		if errors.Is(judgeCtx.Err(), context.DeadlineExceeded) {
			return nil, "", festerrors.Wrap(judgeCtx.Err(), "approval judge timed out").
				WithField("judge_command", opts.JudgeCommand).
				WithHint("the checkpoint was not approved; rerun manually or raise --judge-timeout")
		}
		return nil, "", festerrors.Wrap(err, "approval judge failed").
			WithField("judge_command", opts.JudgeCommand).
			WithHint("the checkpoint was not approved; install the judge command or run 'fest workflow approve' interactively")
	}

	decision, err := parseApprovalJudgeResponse(out)
	if err != nil {
		return nil, "", err
	}

	audit := approvalJudgeAudit(opts.JudgeCommand, decision)
	return decision, audit, nil
}

func runApprovalJudgeCommandDefault(ctx context.Context, command string, stdin []byte, dir string) ([]byte, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil, festerrors.Validation("approval judge command is empty")
	}
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdin = bytes.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	return out, nil
}

func parseApprovalJudgeResponse(out []byte) (*approvalJudgeResponse, error) {
	var resp approvalJudgeResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, festerrors.Parse("parsing approval judge response", err).
			WithHint("judge stdout must be JSON matching schema fest.approval.judge/v1")
	}

	resp.SchemaVersion = strings.TrimSpace(resp.SchemaVersion)
	resp.Decision = strings.ToLower(strings.TrimSpace(resp.Decision))
	resp.Reason = strings.TrimSpace(resp.Reason)

	if resp.SchemaVersion != approvalJudgeSchemaVersion {
		return nil, festerrors.Validation("approval judge returned unsupported schema").
			WithField("schema_version", resp.SchemaVersion).
			WithHint("expected schema_version fest.approval.judge/v1")
	}
	if resp.Decision != "approve" && resp.Decision != "reject" {
		return nil, festerrors.Validation("approval judge returned unsupported decision").
			WithField("decision", resp.Decision).
			WithHint("allowed decisions are approve and reject")
	}
	if resp.Reason == "" {
		return nil, festerrors.Validation("approval judge returned empty reason").
			WithHint("judge responses must include a reason so the approval audit trail is useful")
	}

	return &resp, nil
}

func approvalJudgeAudit(command string, decision *approvalJudgeResponse) string {
	return fmt.Sprintf("approval auto mode: schema_version=%s judge_command=%q decision=%s reason=%q",
		approvalJudgeSchemaVersion, command, decision.Decision, decision.Reason)
}
