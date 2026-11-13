package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/camp/pkg/approvalkit"

	"github.com/Obedience-Corp/fest/internal/campledger"
)

// Reasons recorded when a pending approval ends without a human approving or
// rejecting it.
const (
	approvalReasonJudgeApproved = "judge_approved"
	approvalReasonJudgeRejected = "judge_rejected"
	approvalReasonMovedOn       = "checkpoint_no_longer_waiting"
)

// approvalReport is the outcome the next reportApprovals call attributes to a
// request that stops waiting. A transition that knows why sets it first; the
// default is a withdrawal, which is right for skip, reset, and anything else
// that moves the workflow without a human decision.
type approvalReport struct {
	outcome approvalkit.Outcome
	reason  string
}

// expectApprovalOutcome records why the transition about to be persisted ends
// a pending approval.
func (n *Navigator) expectApprovalOutcome(outcome approvalkit.Outcome, reason string) {
	n.nextApprovalReport = &approvalReport{outcome: outcome, reason: reason}
}

// reportApprovals tells the campaign what this workflow is now waiting on a
// human to approve. It runs after a transition has been persisted and derives
// its answer from that persisted state alone, so every path that changes state
// can call it without knowing what was reported before.
//
// The workflow is waiting when its current step has a blocking checkpoint that
// routes to a human (needsHumanApproval, the same predicate the approval-needed
// notification uses) and the step is not blocked on a revision. A checkpoint a
// configured judge will decide is never reported: it is not waiting on a human.
//
// Reporting is best-effort and silent outside a campaign; see
// campledger.SetWaiting.
func (n *Navigator) reportApprovals(ctx context.Context) {
	if n == nil {
		return
	}
	report := approvalReport{outcome: approvalkit.OutcomeWithdrawn, reason: approvalReasonMovedOn}
	if n.nextApprovalReport != nil {
		report = *n.nextApprovalReport
		n.nextApprovalReport = nil
	}
	if n.workflowState == nil || n.festivalPath == "" || n.Ctx == nil || n.Ctx.PhasePath == "" {
		return
	}
	campledger.SetWaiting(ctx, n.Ctx.PhasePath, n.stateKey(), n.waitingCheckpoint(ctx), report.outcome, report.reason, nil)
}

// waitingCheckpoint returns the checkpoint this workflow is waiting on a human
// to decide, or nil.
func (n *Navigator) waitingCheckpoint(ctx context.Context) *campledger.Waiting {
	if n.workflowState.IsComplete() {
		return nil
	}
	stepNum := n.workflowState.CurrentStep
	if stepNum < 1 || stepNum > len(n.steps) {
		return nil
	}
	step := n.steps[stepNum-1]
	if !n.needsHumanApproval(ctx, step) {
		return nil
	}
	if state := n.workflowState.GetStepState(stepNum); state != nil {
		switch state.Status {
		case StepStatusBlocked, StepStatusFailedRemediation, StepStatusCompleted, StepStatusSkipped:
			// Blocked and failed-with-remediation wait on a revision, not on
			// the human; completed and skipped are not waiting at all.
			return nil
		}
	}

	kind := approvalkit.SourceFestivalCheckpoint
	label := "checkpoint"
	if n.IsGate() {
		kind = approvalkit.SourcePhaseGate
		label = "phase gate"
	}
	festival := filepath.Base(n.festivalPath)
	scope := campledger.FestivalScope(n.festivalPath, "")
	scope.Phase = n.phaseName
	return &campledger.Waiting{
		Kind:     kind,
		Step:     stepNum,
		StepName: step.Name,
		Title:    fmt.Sprintf("%s / %s: %s", festival, n.phaseName, step.Name),
		Body:     checkpointRequestBody(label, stepNum, len(n.steps), step),
		Dir:      n.Ctx.PhasePath,
		Doc:      filepath.Join(n.Ctx.PhasePath, n.filename()),
		Scope:    scope,
	}
}

// checkpointRequestBody is the text a human reads before deciding: which step
// is waiting and what that step was meant to produce.
func checkpointRequestBody(label string, stepNum, total int, step WorkflowStep) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Step %d of %d (%s) is at a %s that needs a human decision.\n", stepNum, total, step.Name, label)
	if goal := strings.TrimSpace(step.Goal); goal != "" {
		fmt.Fprintf(&b, "\nGoal: %s\n", goal)
	}
	if len(step.Actions) > 0 {
		b.WriteString("\nActions:\n")
		for i, action := range step.Actions {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, strings.TrimSpace(action))
		}
	}
	if output := strings.TrimSpace(step.Output); output != "" {
		fmt.Fprintf(&b, "\nOutput: %s\n", output)
	}
	return b.String()
}

// CheckpointRequestBody exposes the request text for sources that are not a
// Navigator (standalone workflows) so every checkpoint reads the same way.
func CheckpointRequestBody(label string, stepNum, total int, step WorkflowStep) string {
	return checkpointRequestBody(label, stepNum, total, step)
}

// ReportApprovals re-reports this workflow's waiting state to the campaign. It
// is for commands that find a checkpoint already waiting without changing
// state (advance at a checkpoint), so a checkpoint that started waiting before
// this reporting existed still shows up.
func (n *Navigator) ReportApprovals(ctx context.Context) {
	n.reportApprovals(ctx)
}
