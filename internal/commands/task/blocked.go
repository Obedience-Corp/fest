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

var (
	blockedReason string
	blockedJSON   bool
)

func newBlockedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocked [task]",
		Short: "Mark a task as blocked (requires confirmation)",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: runBlocked,
	}

	cmd.Flags().StringVar(&blockedReason, "reason", "", "reason for the blocker (required)")
	cmd.Flags().BoolVar(&blockedJSON, "json", false, "output as JSON (blocks: interactive confirmation required)")
	_ = cmd.MarkFlagRequired("reason")

	return cmd
}

func runBlocked(cmd *cobra.Command, args []string) error {
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
	if blockedJSON {
		result := map[string]any{
			"error":   "interactive confirmation required",
			"task":    taskID,
			"message": "Use 'fest task blocked' without --json to report a blocker interactively",
		}
		if encErr := shared.EncodeJSON(os.Stdout, result); encErr != nil {
			return errors.Wrap(encErr, "encoding JSON output")
		}
		return errors.Validation("interactive confirmation required for reporting blockers")
	}

	mgr, err := progress.NewManager(ctx, festivalPath)
	if err != nil {
		return errors.Wrap(err, "loading progress")
	}

	// Show current status
	task, _ := mgr.GetTaskProgress(taskID)
	if task != nil {
		fmt.Printf("%s %s (%s)\n", ui.Label("Task"), ui.Value(taskID, ui.TaskColor),
			ui.GetStateStyle(task.Status).Render(task.Status))
	} else {
		fmt.Printf("%s %s (%s)\n", ui.Label("Task"), ui.Value(taskID, ui.TaskColor),
			ui.GetStateStyle(progress.StatusPending).Render(progress.StatusPending))
	}

	// Interactive confirmation
	if !confirmBlocked(taskID, blockedReason) {
		fmt.Println(ui.Info("Cancelled."))
		return nil
	}

	if err := mgr.ReportBlocker(ctx, taskID, blockedReason); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(ui.Warning("Task blocked: " + taskID))
	fmt.Printf("%s %s\n", ui.Label("Reason"), blockedReason)
	return nil
}
