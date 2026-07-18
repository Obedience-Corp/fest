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
	resetJSON bool
	resetYes  bool
)

func newResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset [task]",
		Short: "Reset a task to pending",
		Long: `Reset a task to pending, clearing all progress, time, and blocker data.

By default a confirmation prompt is shown; pass --yes to skip it for
non-interactive or agent use. --json emits a structured result and requires
--yes.`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: runReset,
	}

	cmd.Flags().BoolVar(&resetJSON, "json", false, "output as JSON (requires --yes)")
	cmd.Flags().BoolVarP(&resetYes, "yes", "y", false, "skip the interactive confirmation prompt")

	return cmd
}

func runReset(cmd *cobra.Command, args []string) error {
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
		Reason: "fest task reset",
	}); err != nil {
		return err
	}

	// Refuse before doing any work when confirmation cannot be obtained.
	needPrompt, err := resolveConfirmation(resetYes, resetJSON, taskID,
		"reset this task", "fest task reset --yes")
	if err != nil {
		return err
	}

	mgr, err := progress.NewManagerWithGate(ctx, festivalPath,
		lifecycle.NewGateWithReason(festivalPath, "fest task reset"))
	if err != nil {
		return errors.Wrap(err, "loading progress")
	}

	if !resetJSON {
		task, _ := mgr.GetTaskProgress(taskID)
		if task != nil {
			fmt.Printf("%s %s (%s)\n", ui.Label("Task"), ui.Value(taskID, ui.TaskColor),
				ui.GetStateStyle(task.Status).Render(task.Status))
			if task.Progress > 0 {
				fmt.Printf("%s %s\n", ui.Label("Progress"), ui.Value(fmt.Sprintf("%d%%", task.Progress)))
			}
			if task.TimeSpentMinutes > 0 {
				fmt.Printf("%s %s\n", ui.Label("Time"), ui.Value(ui.FormatDuration(task.TimeSpentMinutes)))
			}
		} else {
			fmt.Printf("%s %s (%s)\n", ui.Label("Task"), ui.Value(taskID, ui.TaskColor),
				ui.GetStateStyle(progress.StatusPending).Render(progress.StatusPending))
		}
	}

	if needPrompt && !confirmReset(taskID) {
		fmt.Println(ui.Info("Cancelled."))
		return nil
	}

	if err := mgr.ResetTask(ctx, taskID); err != nil {
		return err
	}

	if resetJSON {
		result := map[string]any{
			"success": true,
			"task":    taskID,
			"status":  progress.StatusPending,
		}
		return shared.EncodeJSON(os.Stdout, result)
	}

	fmt.Println()
	fmt.Println(ui.Success("Task reset to pending: " + taskID))
	return nil
}
