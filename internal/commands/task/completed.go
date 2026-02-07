package task

import (
	"fmt"
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/gates"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

var completedJSON bool

func newCompletedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completed [task]",
		Short: "Mark a task as complete (requires confirmation)",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: runCompleted,
	}

	cmd.Flags().BoolVar(&completedJSON, "json", false, "output as JSON (blocks: interactive confirmation required)")

	return cmd
}

func runCompleted(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	festivalPath, ok := scope.FestivalFrom(ctx)
	if !ok {
		return errors.Validation("no festival context")
	}

	var arg string
	if len(args) > 0 {
		arg = args[0]
	}

	taskID, taskFilePath, err := resolveTask(ctx, festivalPath, arg)
	if err != nil {
		return err
	}

	// Require interactive mode - no --json bypass for mutations
	if completedJSON {
		result := map[string]any{
			"error":   "interactive confirmation required",
			"task":    taskID,
			"message": "Use 'fest task completed' without --json to complete a task interactively",
		}
		if encErr := shared.EncodeJSON(os.Stdout, result); encErr != nil {
			return errors.Wrap(encErr, "encoding JSON output")
		}
		return errors.Validation("interactive confirmation required for task completion")
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

	// Evaluate quality gates
	if !strings.HasSuffix(taskFilePath, ".md") {
		taskFilePath += ".md"
	}

	phasePath, sequencePath := resolveTaskLocationPaths(festivalPath, taskID)
	evaluator := gates.NewGateEvaluator(festivalPath, phasePath, sequencePath)
	gateResult, gateErr := evaluator.EvaluateForTask(ctx, taskFilePath)
	if gateErr != nil {
		return errors.Wrap(gateErr, "evaluating quality gates")
	}

	if !gateResult.Passed {
		printGateFailures(gateResult)
		return errors.Validation("task completion blocked by quality gates").
			WithField("task", taskID).
			WithField("failed_gates", len(gateResult.FailedGates))
	}

	// Interactive confirmation
	if !confirmCompletion(taskID) {
		fmt.Println(ui.Info("Cancelled."))
		return nil
	}

	if err := mgr.MarkComplete(ctx, taskID); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(ui.Success("Task completed: " + taskID))
	return nil
}

// printGateFailures displays gate failures in a human-readable format.
func printGateFailures(result *gates.GateEvaluationResult) {
	fmt.Println()
	fmt.Println(ui.H1("Quality Gates Failed"))
	fmt.Println()
	fmt.Println(ui.Error(fmt.Sprintf("Task completion BLOCKED - %d gate(s) failed", len(result.FailedGates))))
	fmt.Println()

	for _, failure := range result.FailedGates {
		var sb strings.Builder
		sb.WriteString(failure.Reason)
		if len(failure.Details) > 0 {
			sb.WriteString("\n\nDetails:\n")
			for _, detail := range failure.Details {
				sb.WriteString("  • ")
				sb.WriteString(detail)
				sb.WriteString("\n")
			}
		}
		fmt.Println(ui.ErrorPanel(failure.Name, sb.String()))
		fmt.Println()
	}

	fmt.Println(ui.Info("Fix the issues above and try again. Quality gates enforce code quality."))
	fmt.Println()
}
