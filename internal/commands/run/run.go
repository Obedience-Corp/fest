// Package run implements `fest run`: a leaveable classifier and optional exec loop.
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
	exec       string
	maxTasks   int
	maxMinutes int
	resume     bool
}

// NewRunCommand creates the fest run command.
func NewRunCommand() *cobra.Command {
	opts := &options{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Report whether the next slice is leaveable; optionally loop a caller-supplied command",
		Long: `Inspect the same slice fest next would show and say whether you can leave.

fest run does not launch an agent. Festival is agent-agnostic: the plan and
the loop state live here; whoever executes a slice is the caller's business.

Default (and --dry): classify the next slice, record it, print a report.
Human gates, blocked tasks, and live judges are successful stops.

--exec <command> loops: run that command with the slice on stdin, then
advance. The command can be any worker. Fest does not name or launch a harness.

--status prints the morning report without appending to the ledger.

v1 --exec drives standalone tracked WORKFLOW.md files. Festival tasks are
classified only.`,
		Example: `  fest run
  fest run --dry
  fest run --status --json
  fest run --exec ./my-worker --max-tasks 8 --max-minutes 240`,
		Annotations: map[string]string{
			"scope": string(scope.Global),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.dry, "dry", false, "classify the next slice and exit (default when --exec is omitted)")
	cmd.Flags().BoolVar(&opts.status, "status", false, "print the morning report without driving")
	cmd.Flags().BoolVar(&opts.json, "json", false, "machine-readable status")
	cmd.Flags().StringVar(&opts.exec, "exec", "", "optional worker command; slice prompt is on stdin. omitted: classify only")
	cmd.Flags().IntVar(&opts.maxTasks, "max-tasks", runloop.DefaultMaxTasks, "stop after this many driven slices (with --exec)")
	cmd.Flags().IntVar(&opts.maxMinutes, "max-minutes", runloop.DefaultMaxMinutes, "stop after this many minutes (with --exec)")
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
		Exec:       opts.exec,
		MaxTasks:   opts.maxTasks,
		MaxMinutes: opts.maxMinutes,
		Stdout:     cmd.OutOrStdout(),
	})
}
