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
	unblockJSON bool
	unblockNote string
)

func newUnblockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unblock [task]",
		Short: "Clear a task's blocker and resume work",
		Long: `Clear a task's blocker, returning it to in_progress.

This is a frictionless forward-motion signal and does not prompt for
confirmation. When [task] is omitted the current task is auto-detected.

Pass --note to tell the executor what to try. The note is stored on the task and
rendered under it by the next fest next, so the executor sees it even if it is
not running right now.`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: runUnblock,
	}

	cmd.Flags().BoolVar(&unblockJSON, "json", false, "output as JSON")
	cmd.Flags().StringVar(&unblockNote, "note", "",
		"feedback for the executor, shown under the task on the next fest next")

	return cmd
}

func runUnblock(cmd *cobra.Command, args []string) error {
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
		Reason: "fest task unblock",
	}); err != nil {
		return err
	}

	mgr, err := progress.NewManagerWithGate(ctx, festivalPath,
		lifecycle.NewGateWithReason(festivalPath, "fest task unblock"))
	if err != nil {
		return errors.Wrap(err, "loading progress")
	}

	if err := mgr.ClearBlocker(ctx, taskID, unblockNote); err != nil {
		return err
	}

	if unblockJSON {
		result := map[string]any{
			"success": true,
			"task":    taskID,
			"cleared": true,
		}
		if unblockNote != "" {
			result["note"] = unblockNote
		}
		return shared.EncodeJSON(os.Stdout, result)
	}

	status := progress.StatusInProgress
	if task, ok := mgr.GetTaskProgress(taskID); ok && task != nil {
		status = task.Status
	}
	fmt.Printf("%s %s (%s)\n", ui.Label("Task"), ui.Value(taskID, ui.TaskColor),
		ui.GetStateStyle(status).Render(status))
	fmt.Println(ui.Success("Blocker cleared: " + taskID))
	return nil
}
