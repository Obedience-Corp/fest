package task

import (
	"fmt"
	"os"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

var (
	blockedReason       string
	blockedTried        []string
	blockedJSON         bool
	blockedYes          bool
	blockedList         bool
	blockedListOpen     bool
	blockedListDeferred bool
)

func newBlockedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocked [task]",
		Short: "Mark a task as blocked",
		Long: `Mark a task as blocked, pausing work and notifying the user.

Repeat --tried for each unblock attempt that failed. The operator sees these
when deciding whether to defer the blocker, and a block with no recorded
attempts is likely to be sent back.

By default a confirmation prompt is shown; pass --yes to skip it for
non-interactive or agent use. --json emits a structured result and requires
--yes.

--list reports the festival's blockers instead of reporting one. It takes no
task and no --reason, writes nothing, and never prompts. Open blockers are
listed before deferred ones; --open and --deferred narrow the list to one of
them.`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		PreRunE: blockedPreRun,
		RunE:    runBlocked,
	}

	blockedTried = nil

	cmd.Flags().StringVar(&blockedReason, "reason", "", "reason for the blocker (required)")
	cmd.Flags().StringArrayVar(&blockedTried, "tried", nil,
		"an unblock attempt that failed; repeat for each attempt")
	cmd.Flags().BoolVar(&blockedJSON, "json", false, "output as JSON (requires --yes)")
	cmd.Flags().BoolVarP(&blockedYes, "yes", "y", false, "skip the interactive confirmation prompt")
	cmd.Flags().BoolVar(&blockedList, "list", false, "list the festival's blockers instead of reporting one")
	cmd.Flags().BoolVar(&blockedListOpen, "open", false, "with --list, show only blockers no operator has deferred")
	cmd.Flags().BoolVar(&blockedListDeferred, "deferred", false, "with --list, show only deferred blockers")
	cmd.MarkFlagsMutuallyExclusive("open", "deferred")
	_ = cmd.MarkFlagRequired("reason")

	return cmd
}

// blockedPreRun lifts the required --reason for the reporting path only. Cobra
// checks required flags after PreRunE, so clearing the annotation here leaves
// the message an operator who forgets --reason sees today exactly as it was.
func blockedPreRun(cmd *cobra.Command, args []string) error {
	if !blockedList {
		if blockedListOpen || blockedListDeferred {
			return errors.Validation("--open and --deferred filter the blocker list").
				WithHint("pass --list to report the festival's blockers")
		}
		return nil
	}
	if len(args) > 0 {
		return errors.Validation("--list reports every blocker in the festival and takes no task").
			WithField("task", args[0]).
			WithHint("drop the task, or use 'fest task show' for one task")
	}
	return cmd.Flags().SetAnnotation("reason", cobra.BashCompOneRequiredFlag, []string{"false"})
}

func runBlocked(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	festivalPath, ok := scope.FestivalFrom(ctx)
	if !ok {
		return errors.Validation("no festival context")
	}

	if blockedList {
		return runBlockedList(ctx, os.Stdout, festivalPath, time.Now())
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

	if err := mgr.ReportBlocker(ctx, taskID, blockedReason, blockedTried); err != nil {
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
