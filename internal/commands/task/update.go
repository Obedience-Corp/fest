package task

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

var updateJSON bool

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [task] <percent>",
		Short: "Update a task's progress percentage",
		Long: `Update a task's progress percentage (0-100).

This is a frictionless forward-motion signal and does not prompt for
confirmation. When [task] is omitted the current task is auto-detected.`,
		Args: cobra.RangeArgs(1, 2),
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: runUpdate,
	}

	cmd.Flags().BoolVar(&updateJSON, "json", false, "output as JSON")

	return cmd
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	festivalPath, ok := scope.FestivalFrom(ctx)
	if !ok {
		return errors.Validation("no festival context")
	}

	var taskArg, pctArg string
	switch len(args) {
	case 1:
		pctArg = args[0]
	default:
		taskArg = args[0]
		pctArg = args[1]
	}

	pct, err := parsePercent(pctArg)
	if err != nil {
		return err
	}

	taskID, _, err := resolveTask(ctx, festivalPath, taskArg)
	if err != nil {
		return err
	}

	if err := lifecycle.EnforcePreActive(ctx, festivalPath, lifecycle.EnforceOptions{
		TaskID: taskID,
		Reason: "fest task update",
	}); err != nil {
		return err
	}

	mgr, err := progress.NewManagerWithGate(ctx, festivalPath,
		lifecycle.NewGateWithReason(festivalPath, "fest task update"))
	if err != nil {
		return errors.Wrap(err, "loading progress")
	}

	if err := mgr.UpdateProgress(ctx, taskID, pct); err != nil {
		return err
	}

	status := statusForPercent(pct)
	if task, ok := mgr.GetTaskProgress(taskID); ok && task != nil {
		status = task.Status
	}

	if updateJSON {
		result := map[string]any{
			"success":  true,
			"task":     taskID,
			"progress": pct,
			"status":   status,
		}
		return shared.EncodeJSON(os.Stdout, result)
	}

	fmt.Printf("%s %s (%s)\n", ui.Label("Task"), ui.Value(taskID, ui.TaskColor),
		ui.GetStateStyle(status).Render(status))
	fmt.Printf("%s %s\n", ui.Label("Progress"), ui.Value(fmt.Sprintf("%d%%", pct)))
	return nil
}

// parsePercent parses a percentage string like "50" or "50%" into an int in the
// range 0-100.
func parsePercent(s string) (int, error) {
	s = strings.TrimSuffix(strings.TrimSpace(s), "%")
	pct, err := strconv.Atoi(s)
	if err != nil {
		return 0, errors.Validation("invalid percentage").WithField("value", s)
	}
	if pct < 0 || pct > 100 {
		return 0, errors.Validation("percentage must be 0-100").WithField("value", pct)
	}
	return pct, nil
}

// statusForPercent maps a progress percentage to a task status.
func statusForPercent(pct int) string {
	switch {
	case pct >= 100:
		return progress.StatusCompleted
	case pct > 0:
		return progress.StatusInProgress
	default:
		return progress.StatusPending
	}
}
