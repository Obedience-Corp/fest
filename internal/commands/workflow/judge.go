package workflow

import (
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// newJudgeCmd explicitly runs (or re-runs) the configured approval judge for
// the current blocking checkpoint. It is the agent-facing recovery command
// after a judge rejection; ordinary operator rejections remain human-owned.
func newJudgeCmd() *cobra.Command {
	opts := approvalJudgeOptions{Auto: true, Rejudge: true}
	cmd := &cobra.Command{
		Use:   "judge",
		Short: "Run the approval judge for the current checkpoint",
		Long: `Run the configured approval judge for the current blocking checkpoint.

Use this after revising evidence following a judge rejection. A judge-owned
rejection is reopened automatically; ordinary operator rejections still
require 'fest workflow approve'. By default the judge runs in the background;
use --wait when this command should wait for the verdict.

The judge command is resolved from --judge-command or the
hooks.approval_judge.command workspace configuration hook.`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runApproveWithOptions(cmd.Context(), wf.DecisionMetadata{}, opts)
		},
	}
	cmd.Flags().StringVar(&opts.JudgeCommand, "judge-command", "", "approval judge command (overrides hooks.approval_judge.command)")
	cmd.Flags().DurationVar(&opts.Timeout, "judge-timeout", 0, "maximum time to wait for the approval judge (0 waits until it returns)")
	cmd.Flags().BoolVar(&opts.Wait, "wait", false, "block until the judge returns instead of launching it in the background")
	return cmd
}
