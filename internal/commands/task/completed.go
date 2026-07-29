package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/gates"
	"github.com/Obedience-Corp/fest/internal/hooks"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

// newTaskHookRunner is an injectable seam so tests can fake hook execution.
var newTaskHookRunner = hooks.NewRunner

// blockedHookName returns the first fail-closed hook that blocked the verb.
func blockedHookName(runs []hooks.HookRun) string {
	for _, run := range runs {
		if run.Blocked {
			return run.Name
		}
	}
	return ""
}

var (
	completedJSON bool
	completedYes  bool
)

func newCompletedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completed [task]",
		Short: "Mark a task as complete",
		Long: `Mark a task as complete.

Quality gates are evaluated first and block completion on failure. By default a
confirmation prompt is shown; pass --yes to skip it for non-interactive or agent
use. --json emits a structured result and requires --yes.`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: runCompleted,
	}

	cmd.Flags().BoolVar(&completedJSON, "json", false, "output as JSON (requires --yes)")
	cmd.Flags().BoolVarP(&completedYes, "yes", "y", false, "skip the interactive confirmation prompt")

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

	if err := lifecycle.EnforcePreActive(ctx, festivalPath, lifecycle.EnforceOptions{
		TaskID: taskID,
		Reason: "fest task completed",
	}); err != nil {
		return err
	}

	// Refuse before doing any work when confirmation cannot be obtained.
	needPrompt, err := resolveConfirmation(completedYes, completedJSON, taskID,
		"complete this task", "fest task completed --yes")
	if err != nil {
		return err
	}

	mgr, err := progress.NewManagerWithGate(ctx, festivalPath,
		lifecycle.NewGateWithReason(festivalPath, "fest task completed"))
	if err != nil {
		return errors.Wrap(err, "loading progress")
	}

	if !completedJSON {
		task, _ := mgr.GetTaskProgress(taskID)
		status := progress.StatusPending
		if task != nil {
			status = task.Status
		}
		fmt.Printf("%s %s (%s)\n", ui.Label("Task"), ui.Value(taskID, ui.TaskColor),
			ui.GetStateStyle(status).Render(status))
	}

	// Evaluate quality gates - these BLOCK completion if they fail.
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
		if completedJSON {
			result := map[string]any{
				"success":      false,
				"task":         taskID,
				"blocked":      true,
				"failed_gates": gateResult.FailedGates,
				"message":      "Task completion blocked by quality gates",
			}
			if encErr := shared.EncodeJSON(os.Stdout, result); encErr != nil {
				return errors.Wrap(encErr, "encoding JSON output")
			}
		} else {
			printGateFailures(gateResult)
		}
		return errors.Validation("task completion blocked by quality gates").
			WithField("task", taskID).
			WithField("failed_gates", len(gateResult.FailedGates))
	}

	if needPrompt && !confirmCompletion(taskID) {
		fmt.Println(ui.Info("Cancelled."))
		return nil
	}

	hookReq := progress.LifecycleHookRequest{
		FestivalPath: festivalPath,
		Phase:        filepath.Base(phasePath),
		Task:         taskID,
		Level:        hooks.LevelTask,
		Verb:         hooks.VerbTaskComplete,
	}
	if content, readErr := os.ReadFile(taskFilePath); readErr == nil {
		hookReq.Pre, hookReq.Post, hookReq.HumanGate = progress.StepHookBindingsFromFrontmatter(content)
	}
	planned, err := progress.PlanLifecycleHooks(ctx, hookReq)
	if err != nil {
		return err
	}
	runner := newTaskHookRunner(festivalPath)
	preRuns, preBlocked, preErr := progress.RunLifecycleStage(ctx, mgr.Store(), runner, hookReq, planned, hooks.TimingPre)
	if saveErr := mgr.Store().SaveEvents(ctx); saveErr != nil && preErr == nil {
		preErr = saveErr
	}
	if preErr != nil {
		return errors.Wrap(preErr, "running pre-completion hooks")
	}
	if preBlocked {
		blockErr := progress.BlockedHookError(hooks.VerbTaskComplete, preRuns)
		if completedJSON {
			result := map[string]any{
				"success":     false,
				"task":        taskID,
				"blocked":     true,
				"failed_hook": blockedHookName(preRuns),
				"message":     "Task completion blocked by fail-closed hook",
			}
			if encErr := shared.EncodeJSON(os.Stdout, result); encErr != nil {
				return errors.Wrap(encErr, "encoding JSON output")
			}
		} else {
			fmt.Println()
			fmt.Println(ui.Error("Task completion BLOCKED by fail-closed hook: " + blockedHookName(preRuns)))
		}
		return blockErr
	}

	if err := mgr.MarkComplete(ctx, taskID); err != nil {
		return err
	}

	_, postBlocked, postErr := progress.RunLifecycleStage(ctx, mgr.Store(), runner, hookReq, planned, hooks.TimingPost)
	if saveErr := mgr.Store().SaveEvents(ctx); saveErr != nil && postErr == nil {
		postErr = saveErr
	}
	if postErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: post-completion hooks: %v\n", postErr)
	} else if postBlocked {
		fmt.Fprintln(os.Stderr, "Warning: a fail-closed post hook failed; the task remains completed (see wf_hook_run in the festival audit trail)")
	}

	if completedJSON {
		task, _ := mgr.GetTaskProgress(taskID)
		timeSpent := 0
		if task != nil {
			timeSpent = task.TimeSpentMinutes
		}
		result := map[string]any{
			"success":            true,
			"task":               taskID,
			"status":             progress.StatusCompleted,
			"time_spent_minutes": timeSpent,
		}
		return shared.EncodeJSON(os.Stdout, result)
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
