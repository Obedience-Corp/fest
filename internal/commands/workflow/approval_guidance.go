package workflow

import (
	"context"
	"fmt"
	"strings"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/ui"
)

func approvalRecoveryLines(ctx context.Context, step wf.WorkflowStep) []string {
	return approvalRecoveryLinesFor(ctx, nil, step)
}

func approvalRecoveryLinesFor(ctx context.Context, nav *wf.Navigator, step wf.WorkflowStep) []string {
	configured, err := approvalJudgeConfiguredFor(ctx, nav)
	if err != nil {
		return []string{
			fmt.Sprintf("Approval judge configuration is invalid: %v", err),
			"Fix .festival/config.yaml before approving this checkpoint.",
		}
	}
	if wf.ClassifyCheckpoint(step) == wf.CheckpointClassOperatorAttestation {
		return []string{
			"Human attestation required: " + ui.Accent("fest workflow approve") + " (interactive)",
		}
	}
	if !configured {
		return []string{
			"Operator approve: " + ui.Accent("fest workflow approve"),
			"To enable auto-judge, configure hooks.approval_judge.command first.",
		}
	}
	return []string{
		"Operator override:       " + ui.Accent("fest workflow approve") + " (interactive)",
		"Re-run approval judge:   " + ui.Accent("fest workflow judge"),
	}
}

func approvalRecoveryTextFor(ctx context.Context, nav *wf.Navigator, step wf.WorkflowStep) string {
	return strings.Join(approvalRecoveryLinesFor(ctx, nav, step), "\n")
}

func printApprovalRecoveryFor(ctx context.Context, nav *wf.Navigator, step wf.WorkflowStep) {
	for _, line := range approvalRecoveryLinesFor(ctx, nav, step) {
		fmt.Println(line)
	}
}
