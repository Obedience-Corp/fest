// Package run implements `fest run`: a leaveable driver for fest next.
package run

import (
	"os"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/runloop"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

type options struct {
	dry        bool
	status     bool
	json       bool
	agent      string
	maxTasks   int
	maxMinutes int
	resume     bool
}

// NewRunCommand creates the fest run command.
func NewRunCommand() *cobra.Command {
	opts := &options{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Drive fest next until the plan is done, blocked on you, or the cap hits",
		Long: `Drive the current festival or tracked WORKFLOW.md without babysitting the loop.

fest run inspects the same slice fest next would show. Human gates, blocked
tasks, and live judges stop the run — that stop is a successful night, not a
failure. Successful slices are committed when the working directory is a git
repo. The campaign is never git-reset.

Use --dry to classify the next slice without invoking an agent.
Use --status to print the morning report without appending to the ledger.

v1 drives standalone tracked WORKFLOW.md files. Festival task execution is
classified by --dry; driving those slices is not enabled yet.`,
		Example: `  fest run --dry
  fest run --status
  fest run --agent claude --max-tasks 8 --max-minutes 240
  fest run --resume`,
		Annotations: map[string]string{
			"scope": string(scope.Global),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.dry, "dry", false, "classify the next slice and exit")
	cmd.Flags().BoolVar(&opts.status, "status", false, "print the morning report without driving")
	cmd.Flags().BoolVar(&opts.json, "json", false, "machine-readable status")
	cmd.Flags().StringVar(&opts.agent, "agent", "claude", "agent binary (claude uses -p; anything else gets the prompt on stdin)")
	cmd.Flags().IntVar(&opts.maxTasks, "max-tasks", runloop.DefaultMaxTasks, "stop after this many driven slices")
	cmd.Flags().IntVar(&opts.maxMinutes, "max-minutes", runloop.DefaultMaxMinutes, "stop after this many minutes")
	cmd.Flags().BoolVar(&opts.resume, "resume", false, "continue the existing ledger (default: always resumes if present)")
	return cmd
}

func runRun(cmd *cobra.Command, opts *options) error {
	ctx := cmd.Context()
	if err := ctx.Err(); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}
	return runloop.Drive(ctx, cwd, runloop.Options{
		Dry:        opts.dry,
		StatusOnly: opts.status,
		JSON:       opts.json,
		Agent:      opts.agent,
		MaxTasks:   opts.maxTasks,
		MaxMinutes: opts.maxMinutes,
		Stdout:     cmd.OutOrStdout(),
	})
}
