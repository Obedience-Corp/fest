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
	blockedReason string
	blockedJSON   bool
	blockedYes    bool
)

func newBlockedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocked [task]",
		Short: "Mark a task as blocked",
		Long: `Mark a task as blocked, pausing work and notifying the user.

By default a confirmation prompt is shown; pass --yes to skip it for
non-interactive or agent use. --json emits a structured result and requires
--yes.`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: runBlocked,
	}

	cmd.Flags().StringVar(&blockedReason, "reason", "", "reason for the blocker (required)")
	cmd.Flags().BoolVar(&blockedJSON, "json", false, "output as JSON (requires --yes)")
	cmd.Flags().BoolVarP(&blockedYes, "yes", "y", false, "skip the interactive confirmation prompt")
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

	if err := lifecycle.EnforcePreActive(ctx, festivalPath, lifecycle.EnforceOptions{
		TaskID: taskID,
		Reason: "fest task blocked",
	}); err != nil {
		return err
	}

	// Refuse before doing any work when confirmation cannot be obtained.
	needPrompt, err := resolveConfirmation(blockedYes, blockedJSON, taskID,
		"report a blocker", "fest task blocked --reason <msg> --yes")
	if err != nil {
		return err
	}

	mgr, err := progress.NewManagerWithGate(ctx, festivalPath,
		lifecycle.NewGateWithReason(festivalPath, "fest task blocked"))
	if err != nil {
		return errors.Wrap(err, "loading progress")
	}

	if !blockedJSON {
		task, _ := mgr.GetTaskProgress(taskID)
		status := progress.StatusPending
		if task != nil {
			status = task.Status
		}
		fmt.Printf("%s %s (%s)\n", ui.Label("Task"), ui.Value(taskID, ui.TaskColor),
			ui.GetStateStyle(status).Render(status))
	}

	if needPrompt && !confirmBlocked(taskID, blockedReason) {
		fmt.Println(ui.Info("Cancelled."))
		return nil
	}

	if err := mgr.ReportBlocker(ctx, taskID, blockedReason); err != nil {
		return err
	}

	if blockedJSON {
		result := map[string]any{
			"success": true,
			"task":    taskID,
			"status":  progress.StatusBlocked,
			"blocker": blockedReason,
		}
		return shared.EncodeJSON(os.Stdout, result)
	}

	fmt.Println()
	fmt.Println(ui.Warning("Task blocked: " + taskID))
	fmt.Printf("%s %s\n", ui.Label("Reason"), blockedReason)
	return nil
}
