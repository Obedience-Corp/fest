package task

import (
	"fmt"
	"os"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

var resetJSON bool

func newResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset [task]",
		Short: "Reset a task to pending (requires confirmation)",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runReset,
	}

	cmd.Flags().BoolVar(&resetJSON, "json", false, "output as JSON (blocks: interactive confirmation required)")

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

	// Require interactive mode
	if resetJSON {
		result := map[string]any{
			"error":   "interactive confirmation required",
			"task":    taskID,
			"message": "Use 'fest task reset' without --json to reset a task interactively",
		}
		if encErr := shared.EncodeJSON(os.Stdout, result); encErr != nil {
			return errors.Wrap(encErr, "encoding JSON output")
		}
		return errors.Validation("interactive confirmation required for task reset")
	}

	mgr, err := progress.NewManager(ctx, festivalPath)
	if err != nil {
		return errors.Wrap(err, "loading progress")
	}

	// Show current status before reset
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

	// Interactive confirmation
	if !confirmReset(taskID) {
		fmt.Println(ui.Info("Cancelled."))
		return nil
	}

	if err := mgr.ResetTask(ctx, taskID); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(ui.Success("Task reset to pending: " + taskID))
	return nil
}
