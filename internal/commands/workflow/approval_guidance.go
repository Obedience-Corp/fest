package workflow

import (
	"context"
	"fmt"
	"strings"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/ui"
)

func approvalRecoveryLines(ctx context.Context, step wf.WorkflowStep) []string {
	configured, err := approvalJudgeConfigured(ctx)
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
		"Re-submit to the judge: " + ui.Accent("fest workflow approve --auto"),
		"Operator override:       " + ui.Accent("fest workflow approve") + " (interactive)",
	}
}

func approvalRecoveryText(ctx context.Context, step wf.WorkflowStep) string {
	return strings.Join(approvalRecoveryLines(ctx, step), "\n")
}

func printApprovalRecovery(ctx context.Context, step wf.WorkflowStep) {
	for _, line := range approvalRecoveryLines(ctx, step) {
		fmt.Println(line)
	}
}
