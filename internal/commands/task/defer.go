package task

import (
	"fmt"
	"os"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

var (
	deferReason string
	deferYes    bool
	deferJSON   bool
)

func newDeferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "defer [task]",
		Short: "Defer a blocked task's blocker so the festival can keep moving",
		Long: `Defer a blocker you cannot clear right now.

The task stays blocked and still renders as blocked everywhere. Its dependents
become ready, so the rest of the festival proceeds. When everything else is
settled, fest next brings the deferred tasks back for another attempt, and the
festival cannot be promoted to completed while any blocker is deferred unless
you pass --force.

By default, a person confirms a deferral by typing the task number in a
terminal. Pass --yes to defer non-interactively from an agent or app; the event
records automation as the actor. --json requires --yes. Agents should first
work ready tasks and record actual unblock attempts. Defer only a documented
external blocker when specific independent work can proceed; name that work
and why the blocker can wait in --reason. To resume this task instead, use
'fest task unblock --note "<what to try>"'.`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: runDefer,
	}

	cmd.Flags().StringVar(&deferReason, "reason", "", "why this blocker can wait until the end (required)")
	cmd.Flags().BoolVarP(&deferYes, "yes", "y", false, "skip the interactive task-number confirmation")
	cmd.Flags().BoolVar(&deferJSON, "json", false, "output as JSON (requires --yes)")
	_ = cmd.MarkFlagRequired("reason")

	return cmd
}

func runDefer(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	festivalPath, ok := scope.FestivalFrom(ctx)
	if !ok {
		return errors.Validation("no festival context")
	}

	var arg string
	if len(args) > 0 {
		arg = args[0]
	}

	taskID, _, err := resolveTask(ctx, festivalPath, arg)
	if err != nil {
		return err
	}

	if err := lifecycle.EnforcePreActive(ctx, festivalPath, lifecycle.EnforceOptions{
		TaskID: taskID,
		Reason: "fest task defer",
	}); err != nil {
		return err
	}

	needPrompt, err := resolveConfirmation(deferYes, deferJSON, taskID,
		"defer this blocker", "fest task defer --reason <why> --yes")
	if err != nil {
		return err
	}

	var audit *OperatorAudit
	if needPrompt {
		audit, err = OperatorGuard(ctx, "deferral")
		if err != nil {
			return err
		}
	} else {
		audit = &OperatorAudit{Actor: "automation", DeferredBy: "automation"}
	}

	mgr, err := progress.NewManagerWithGate(ctx, festivalPath,
		lifecycle.NewGateWithReason(festivalPath, "fest task defer"))
	if err != nil {
		return errors.Wrap(err, "loading progress")
	}

	task, ok := mgr.GetTaskProgress(taskID)
	if !ok || task == nil || task.Status != progress.StatusBlocked {
		return errors.Validation("only a blocked task can be deferred").
			WithField("task", taskID).
			WithHint("report the blocker first with 'fest task blocked --reason'")
	}

	if needPrompt && !confirmDeferral(taskID, task) {
		return errors.Validation("deferral cancelled")
	}

	if err := applyDeferral(ctx, mgr, festivalPath, taskID, deferReason, audit); err != nil {
		return err
	}
	if deferJSON {
		return shared.EncodeJSON(os.Stdout, map[string]any{
			"success":         true,
			"task":            taskID,
			"status":          progress.StatusBlocked,
			"deferred":        true,
			"deferral_reason": deferReason,
			"actor":           audit.Actor,
		})
	}

	fmt.Printf("%s %s (%s)\n", ui.Label("Task"), ui.Value(taskID, ui.TaskColor),
		ui.GetStateStyle(progress.StatusBlocked).Render(progress.StatusBlocked))
	fmt.Println(ui.Success("Blocker deferred: " + taskID))
	fmt.Println(ui.Dim("fest next will bring this task back when everything else is settled."))
	return nil
}
