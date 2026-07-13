package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newRejectCmd() *cobra.Command {
	var reason string
	var remediationPhase string
	var actor string
	var summary string

	cmd := &cobra.Command{
		Use:   "reject",
		Short: "Reject checkpoint with feedback",
		Long: `Reject a blocking checkpoint and provide feedback.

When a step's work doesn't meet requirements, use this command
to reject and request revisions.

The feedback will be recorded in the workflow state for reference.

Failed gates with remediation:
  Use --remediation-phase to record that a phase gate did not pass and
  to link a remediation phase that will correct the underlying issues.
  After the remediation phase completes, 'fest next' routes back to the
  failed gate for re-evaluation rather than treating it as approved.

Examples:
  fest workflow reject --reason "needs revision"
  fest workflow reject --reason "PR not ready" --remediation-phase 005_FIX_PR_302
  fest workflow reject --reason "missing acceptance proof" --summary "reviewed the diff against the task spec"`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reason == "" {
				return festerrors.Validation("--reason is required").
					WithHint("Usage: fest workflow reject --reason \"your feedback here\"")
			}
			decision, err := normalizeDecision("rejection", actor, summary)
			if err != nil {
				return err
			}
			return runRejectWithRemediationDecision(cmd.Context(), reason, remediationPhase, decision)
		},
	}

	cmd.Flags().StringVarP(&reason, "reason", "r", "", "reason for rejection (required)")
	cmd.Flags().StringVar(&remediationPhase, "remediation-phase", "", "link a remediation phase for a failed gate (e.g. 005_FIX_PR_302)")
	cmd.Flags().StringVar(&actor, "as", decisionActorUser, "deprecated: manual rejections are always recorded as the user; agent decisions require 'fest workflow approve --auto' with a configured judge")
	_ = cmd.Flags().MarkHidden("as")
	cmd.Flags().StringVar(&summary, "summary", "", "decision summary or rationale")
	_ = cmd.MarkFlagRequired("reason")

	return cmd
}

func runReject(ctx context.Context, reason string) error {
	return runRejectWithRemediation(ctx, reason, "")
}

func runRejectWithRemediation(ctx context.Context, reason, remediationPhase string) error {
	return runRejectWithRemediationDecision(ctx, reason, remediationPhase, wf.DecisionMetadata{Actor: decisionActorUser})
}

func runRejectWithRemediationDecision(ctx context.Context, reason, remediationPhase string, decision wf.DecisionMetadata) error {
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}

	if err := lifecycle.EnforcePreActive(ctx, nav.Ctx.FestivalPath, lifecycle.EnforceOptions{
		PhasePath: nav.Ctx.PhasePath,
		Reason:    "fest workflow reject",
	}); err != nil {
		return err
	}

	state := nav.GetWorkflowState()
	steps := nav.GetSteps()

	if state.IsComplete() {
		return festerrors.Validation("workflow is already complete")
	}

	currentStepNum := state.CurrentStep
	if currentStepNum < 1 || currentStepNum > len(steps) {
		return festerrors.Validation("invalid workflow state").
			WithField("current_step", currentStepNum).
			WithField("total_steps", len(steps))
	}

	step := steps[currentStepNum-1]

	if !step.Checkpoint.IsBlocking() {
		return festerrors.Validation("step does not have a blocking checkpoint").
			WithField("step", currentStepNum).
			WithHint("Reject is only for checkpoint steps")
	}

	return withJudgeStepLock(ctx, nav.Ctx.PhasePath, currentStepNum, func() error {
		fresh, err := getWorkflowNavigator(ctx)
		if err != nil {
			return err
		}
		if fresh.GetWorkflowState().CurrentStep != currentStepNum {
			return festerrors.Validation("checkpoint changed before rejection").
				WithField("expected_step", currentStepNum).
				WithField("current_step", fresh.GetWorkflowState().CurrentStep)
		}
		return applyRejectDecision(ctx, fresh, currentStepNum, step, reason, remediationPhase, decision)
	})
}

func applyRejectDecision(ctx context.Context, nav *wf.Navigator, currentStepNum int, step wf.WorkflowStep, reason, remediationPhase string, decision wf.DecisionMetadata) error {
	if remediationPhase != "" {
		if !nav.IsGate() {
			return festerrors.Validation("--remediation-phase is only valid for phase gates").
				WithField("phase", filepath.Base(nav.Ctx.PhasePath)).
				WithHint("failed-gate remediation routes via 'fest next' only for GATES.md gates; for a regular checkpoint use 'fest workflow reject --reason' (revise in place) or 'fest workflow skip' (done elsewhere)")
		}
		if err := validateRemediationPhase(nav.Ctx.FestivalPath, remediationPhase); err != nil {
			return err
		}
		if err := nav.RejectWithRemediationDecision(ctx, reason, remediationPhase, decision); err != nil {
			return festerrors.Wrap(err, "recording failed gate with remediation")
		}
		fmt.Printf("%s Step %d: %s failed with remediation\n", ui.Warning("⚠"), currentStepNum, step.Name)
		fmt.Printf("  %s: %s\n", ui.Label("Reason"), reason)
		fmt.Printf("  %s: %s\n\n", ui.Label("Remediation phase"), remediationPhase)
		fmt.Println("Run " + ui.Accent("fest next") + " to advance into the remediation phase.")
		fmt.Println("After remediation completes, " + ui.Accent("fest next") + " will route back to this gate for re-evaluation.")
		return nil
	}

	if err := nav.RejectWithDecision(ctx, reason, decision); err != nil {
		return festerrors.Wrap(err, "rejecting checkpoint")
	}

	fmt.Printf("%s Step %d: %s rejected\n", ui.Warning("⚠"), currentStepNum, step.Name)
	fmt.Printf("  %s: %s\n\n", ui.Label("Feedback"), reason)
	if decision.Actor != "" {
		fmt.Printf("  %s: %s\n", ui.Label("Rejected by"), decision.Actor)
	}
	if decision.Summary != "" && decision.Summary != reason {
		fmt.Printf("  %s: %s\n", ui.Label("Summary"), decision.Summary)
	}
	fmt.Println("The step is now blocked. Address the feedback and revise the work.")
	fmt.Println("When ready, run " + ui.Accent("fest workflow advance") + " to resubmit.")

	return nil
}

func validateRemediationPhase(festivalPath, phaseName string) error {
	candidate := filepath.Join(festivalPath, phaseName)
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return festerrors.NotFound("remediation phase").
			WithField("phase", phaseName).
			WithHint("create the phase first (e.g. 'fest create phase --name " + phaseName + " --type implementation')")
	}
	if !shared.IsNumberedDir(phaseName) {
		return festerrors.Validation("remediation phase name must be a numbered phase directory").
			WithField("phase", phaseName).
			WithHint("expected format: NNN_NAME (e.g. 005_FIX_PR_302)")
	}
	hasWorkflow := false
	if info, statErr := os.Stat(filepath.Join(candidate, "WORKFLOW.md")); statErr == nil && !info.IsDir() {
		hasWorkflow = true
	}
	if !hasWorkflow && !shared.HasSequenceDirs(candidate) {
		return festerrors.Validation("remediation phase has no actionable work").
			WithField("phase", phaseName).
			WithHint("a remediation phase must contain a WORKFLOW.md or numbered sequence directories so 'fest next' has remediation work to route into; a GATES.md alone is not a remediation phase")
	}
	return nil
}
