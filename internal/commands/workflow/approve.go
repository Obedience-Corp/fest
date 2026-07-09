package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/config"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

const approvalJudgeSchemaVersion = "fest.approval.judge/v1"

type approvalJudgeOptions struct {
	Auto         bool
	JudgeCommand string
	Timeout      time.Duration
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
}

type approvalJudgeResponse struct {
	SchemaVersion string   `json:"schema_version"`
	Decision      string   `json:"decision"`
	Reason        string   `json:"reason"`
	Confidence    *float64 `json:"confidence,omitempty"`
	Followups     []string `json:"followups,omitempty"`
}

type approvalJudgeRunner func(ctx context.Context, command string, stdin []byte) ([]byte, error)

var runApprovalJudgeCommand approvalJudgeRunner = runApprovalJudgeCommandDefault

func newApproveCmd() *cobra.Command {
	var actor string
	var summary string
	opts := approvalJudgeOptions{
		Timeout: 2 * time.Minute,
	}

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
  Manual approval is the default. Use --auto only when an operator has explicitly
  delegated this checkpoint decision to an external judge command.

  The judge command receives JSON on stdin using schema fest.approval.judge/v1
  and must return JSON on stdout with decision "approve" or "reject" and a
  reason. Missing commands, timeouts, non-zero exits, malformed JSON, unknown
  decisions, and empty reasons fail closed and do not approve the checkpoint.

  The judge command is resolved as: --judge-command flag, else the
  hooks.approval_judge.command hook in .festival/config.yaml. If neither is
  set, --auto fails closed and leaves the checkpoint unchanged.

      hooks:
        approval_judge:
          command: ob judge`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Auto {
				return runApproveWithOptions(cmd.Context(), wf.DecisionMetadata{}, opts)
			}
			decision, err := normalizeDecision("approval", actor, summary, "")
			if err != nil {
				return err
			}
			return runApproveWithDecision(cmd.Context(), decision)
		},
	}

	cmd.Flags().StringVar(&actor, "as", decisionActorUser, "decision actor: user or agent")
	cmd.Flags().StringVar(&summary, "summary", "", "approval summary or rationale")
	cmd.Flags().BoolVar(&opts.Auto, "auto", false, "delegate this checkpoint decision to the configured approval judge command")
	cmd.Flags().StringVar(&opts.JudgeCommand, "judge-command", opts.JudgeCommand, "approval judge command for --auto (overrides the .festival/config.yaml hooks.approval_judge.command hook)")
	cmd.Flags().DurationVar(&opts.Timeout, "judge-timeout", opts.Timeout, "maximum time to wait for the approval judge")
	cmd.MarkFlagsMutuallyExclusive("auto", "as")
	cmd.MarkFlagsMutuallyExclusive("auto", "summary")

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
		judgeCommand, err := resolveApprovalJudgeCommand(ctx, opts.JudgeCommand)
		if err != nil {
			return err
		}
		opts.JudgeCommand = judgeCommand
		return runApproveAuto(ctx, nav, currentStepNum, step, opts)
	}

	// Approve and advance
	if err := nav.ApproveWithDecision(ctx, decision); err != nil {
		return festerrors.Wrap(err, "approving checkpoint")
	}

	fmt.Printf("%s Step %d: %s approved\n", ui.Success("✓"), currentStepNum, step.Name)
	if decision.Actor != "" {
		fmt.Printf("  %s: %s\n", ui.Label("Approved by"), decision.Actor)
	}
	if decision.Summary != "" {
		fmt.Printf("  %s: %s\n", ui.Label("Summary"), decision.Summary)
	}
	return showNextStep(ctx, nav, steps)
}

// resolveApprovalJudgeCommand resolves the command used for --auto approval.
// Precedence: the --judge-command flag, then the hooks.approval_judge.command
// hook in .festival/config.yaml. It fails closed when neither is set so no
// checkpoint is delegated to an unconfigured (or assumed) command.
func resolveApprovalJudgeCommand(ctx context.Context, flagValue string) (string, error) {
	if cmd := strings.TrimSpace(flagValue); cmd != "" {
		return cmd, nil
	}

	if ws, ok := scope.WorkspaceFrom(ctx); ok && ws != nil && ws.FestivalsPath != "" {
		cfg, err := config.LoadWorkspaceConfig(ws.FestivalsPath)
		if err != nil {
			return "", festerrors.Wrap(err, "loading approval judge hook")
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

func runApproveAuto(ctx context.Context, nav *wf.Navigator, currentStepNum int, step wf.WorkflowStep, opts approvalJudgeOptions) error {
	decision, audit, err := judgeApproval(ctx, nav, step, opts)
	if err != nil {
		return err
	}

	judgeDecision := wf.DecisionMetadata{Actor: decisionActorAgent, Summary: decision.Reason}

	switch decision.Decision {
	case "approve":
		if err := nav.ApproveWithAudit(ctx, audit, judgeDecision); err != nil {
			return festerrors.Wrap(err, "approving checkpoint from judge decision")
		}
		fmt.Printf("%s Step %d: %s auto-approved\n", ui.Success("✓"), currentStepNum, step.Name)
		fmt.Printf("  %s: %s\n", ui.Label("Reason"), decision.Reason)
		return showNextStep(ctx, nav, nav.GetSteps())
	case "reject":
		if err := nav.RejectWithDecision(ctx, audit, judgeDecision); err != nil {
			return festerrors.Wrap(err, "recording judge rejection")
		}
		fmt.Printf("%s Step %d: %s auto-rejected\n", ui.Warning("⚠"), currentStepNum, step.Name)
		fmt.Printf("  %s: %s\n\n", ui.Label("Reason"), decision.Reason)
		fmt.Println("The step is now blocked. Address the feedback and revise the work.")
		fmt.Println("When ready, run " + ui.Accent("fest workflow advance") + " to resubmit.")
		return nil
	default:
		return festerrors.Validation("approval judge returned unsupported decision").
			WithField("decision", decision.Decision).
			WithHint("allowed decisions are approve and reject")
	}
}

func judgeApproval(ctx context.Context, nav *wf.Navigator, step wf.WorkflowStep, opts approvalJudgeOptions) (*approvalJudgeResponse, string, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 2 * time.Minute
	}

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

	return evaluateApprovalJudge(ctx, req, opts)
}

func evaluateApprovalJudge(ctx context.Context, req approvalJudgeRequest, opts approvalJudgeOptions) (*approvalJudgeResponse, string, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 2 * time.Minute
	}

	stdin, err := json.Marshal(req)
	if err != nil {
		return nil, "", festerrors.Wrap(err, "encoding approval judge request")
	}

	judgeCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	out, err := runApprovalJudgeCommand(judgeCtx, opts.JudgeCommand, append(stdin, '\n'))
	if err != nil {
		if judgeCtx.Err() != nil {
			return nil, "", festerrors.Wrap(judgeCtx.Err(), "approval judge timed out").
				WithField("judge_command", opts.JudgeCommand).
				WithHint("the checkpoint was not approved; rerun manually or use a working judge command")
		}
		return nil, "", festerrors.Wrap(err, "approval judge failed").
			WithField("judge_command", opts.JudgeCommand).
			WithHint("the checkpoint was not approved; install the judge command or run 'fest workflow approve' manually")
	}

	decision, err := parseApprovalJudgeResponse(out)
	if err != nil {
		return nil, "", err
	}

	audit := approvalJudgeAudit(opts.JudgeCommand, decision)
	return decision, audit, nil
}

func runApprovalJudgeCommandDefault(ctx context.Context, command string, stdin []byte) ([]byte, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil, festerrors.Validation("approval judge command is empty")
	}
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
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
