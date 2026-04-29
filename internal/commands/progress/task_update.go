// Package progress implements the fest progress command for tracking execution progress.
package progress

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/gates"
	"github.com/Obedience-Corp/fest/internal/lifecycle"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
)

func handleTaskUpdate(ctx context.Context, mgr *progress.Manager, festivalPath string, opts *progressOptions) error {
	taskID, err := resolveTaskID(festivalPath, opts)
	if err != nil {
		return err
	}

	if err := lifecycle.EnforcePreActive(ctx, festivalPath, lifecycle.EnforceOptions{
		TaskID: taskID,
		Reason: progressReasonFor(opts),
	}); err != nil {
		return err
	}

	// Handle blocker report
	if opts.blocker != "" {
		if err := mgr.ReportBlocker(ctx, taskID, opts.blocker); err != nil {
			return err
		}
		if opts.json {
			result := map[string]interface{}{
				"success": true,
				"task":    taskID,
				"status":  progress.StatusBlocked,
				"blocker": opts.blocker,
			}
			if err := shared.EncodeJSON(os.Stdout, result); err != nil {
				return errors.Wrap(err, "encoding JSON output")
			}
		} else {
			task, _ := mgr.GetTaskProgress(taskID)
			task = ensureTaskProgress(taskID, task, &progress.TaskProgress{
				Status:         progress.StatusBlocked,
				Progress:       0,
				BlockerMessage: opts.blocker,
			})
			printTaskProgress("Task Blocked", task)
		}
		return nil
	}

	// Handle clear blocker
	if opts.clear {
		if err := mgr.ClearBlocker(ctx, taskID); err != nil {
			return err
		}
		if opts.json {
			result := map[string]interface{}{
				"success": true,
				"task":    taskID,
				"cleared": true,
			}
			if err := shared.EncodeJSON(os.Stdout, result); err != nil {
				return errors.Wrap(err, "encoding JSON output")
			}
		} else {
			task, _ := mgr.GetTaskProgress(taskID)
			task = ensureTaskProgress(taskID, task, &progress.TaskProgress{
				Status:   progress.StatusInProgress,
				Progress: 0,
			})
			printTaskProgress("Blocker Cleared", task)
		}
		return nil
	}

	// Handle complete
	if opts.complete {
		// Evaluate verification gates
		// Resolve task file path for gate evaluation
		taskFilePath := filepath.Join(festivalPath, taskID)
		if !strings.HasSuffix(taskFilePath, ".md") {
			taskFilePath += ".md"
		}

		// Resolve phase and sequence paths from task path
		phasePath, sequencePath := resolveTaskLocationPaths(festivalPath, taskID)

		// Evaluate verification gates - these BLOCK completion if they fail
		evaluator := gates.NewGateEvaluator(festivalPath, phasePath, sequencePath)
		gateResult, err := evaluator.EvaluateForTask(ctx, taskFilePath)
		if err != nil {
			return errors.Wrap(err, "evaluating quality gates")
		}

		if !gateResult.Passed {
			// Gates failed - BLOCK completion
			if opts.json {
				result := map[string]any{
					"success":      false,
					"task":         taskID,
					"blocked":      true,
					"failed_gates": gateResult.FailedGates,
					"message":      "Task completion blocked by quality gates",
				}
				if err := shared.EncodeJSON(os.Stdout, result); err != nil {
					return errors.Wrap(err, "encoding JSON output")
				}
			} else {
				printGateFailures(gateResult)
			}
			return errors.Validation("task completion blocked by quality gates").
				WithField("task", taskID).
				WithField("failed_gates", len(gateResult.FailedGates))
		}

		if err := mgr.MarkComplete(ctx, taskID); err != nil {
			return err
		}
		if opts.json {
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
			if err := shared.EncodeJSON(os.Stdout, result); err != nil {
				return errors.Wrap(err, "encoding JSON output")
			}
		} else {
			task, _ := mgr.GetTaskProgress(taskID)
			task = ensureTaskProgress(taskID, task, &progress.TaskProgress{
				Status:   progress.StatusCompleted,
				Progress: 100,
			})
			printTaskProgress("Task Completed", task)
		}
		return nil
	}

	// Handle in-progress
	if opts.inProgress {
		if err := mgr.MarkInProgress(ctx, taskID); err != nil {
			return err
		}
		if opts.json {
			result := map[string]interface{}{
				"success": true,
				"task":    taskID,
				"status":  progress.StatusInProgress,
			}
			if err := shared.EncodeJSON(os.Stdout, result); err != nil {
				return errors.Wrap(err, "encoding JSON output")
			}
		} else {
			task, _ := mgr.GetTaskProgress(taskID)
			task = ensureTaskProgress(taskID, task, &progress.TaskProgress{
				Status:   progress.StatusInProgress,
				Progress: 0,
			})
			printTaskProgress("Task In Progress", task)
		}
		return nil
	}

	// Handle progress update
	if opts.update != "" {
		pct, err := parsePercentage(opts.update)
		if err != nil {
			return err
		}
		if err := mgr.UpdateProgress(ctx, taskID, pct); err != nil {
			return err
		}
		if opts.json {
			task, _ := mgr.GetTaskProgress(taskID)
			status := statusForProgress(pct)
			if task != nil {
				status = task.Status
			}
			result := map[string]interface{}{
				"success":  true,
				"task":     taskID,
				"progress": pct,
				"status":   status,
			}
			if err := shared.EncodeJSON(os.Stdout, result); err != nil {
				return errors.Wrap(err, "encoding JSON output")
			}
		} else {
			task, _ := mgr.GetTaskProgress(taskID)
			task = ensureTaskProgress(taskID, task, &progress.TaskProgress{
				Status:   statusForProgress(pct),
				Progress: pct,
			})
			printTaskProgress("Progress Updated", task)
		}
		return nil
	}

	// No update flags, show task progress
	task, exists := mgr.GetTaskProgress(taskID)
	if !exists {
		if opts.json {
			result := map[string]interface{}{
				"task":     taskID,
				"progress": 0,
				"status":   progress.StatusPending,
			}
			if err := shared.EncodeJSON(os.Stdout, result); err != nil {
				return errors.Wrap(err, "encoding JSON output")
			}
		} else {
			printTaskProgress("Task Progress", &progress.TaskProgress{
				TaskID:   taskID,
				Status:   progress.StatusPending,
				Progress: 0,
			})
		}
		return nil
	}

	if opts.json {
		if err := shared.EncodeJSON(os.Stdout, task); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		printTaskProgress("Task Progress", task)
	}

	return nil
}

func printTaskProgress(title string, task *progress.TaskProgress) {
	if task == nil {
		return
	}
	fmt.Println(ui.H1(title))

	// Infer work type from task name for display
	workType := frontmatter.InferWorkType(task.TaskID)
	workTypeIndicator := ui.FormatWorkType(string(workType))

	// Show task with work type indicator
	taskDisplay := ui.Value(task.TaskID, ui.TaskColor)
	if workTypeIndicator != "" {
		taskDisplay = workTypeIndicator + " " + taskDisplay
	}
	fmt.Printf("%s %s\n", ui.Label("Task"), taskDisplay)

	fmt.Printf("%s %s\n", ui.Label("Status"), ui.GetStateStyle(task.Status).Render(task.Status))
	fmt.Printf("%s %s\n", ui.Label("Progress"), ui.Value(fmt.Sprintf("%d%%", task.Progress)))
	if task.BlockerMessage != "" {
		fmt.Printf("%s %s\n", ui.Label("Blocker"), ui.Warning(task.BlockerMessage))
	}
	if task.TimeSpentMinutes > 0 {
		fmt.Printf("%s %s\n", ui.Label("Time"), ui.Value(ui.FormatDuration(task.TimeSpentMinutes)))
	}
}

func ensureTaskProgress(taskID string, task *progress.TaskProgress, fallback *progress.TaskProgress) *progress.TaskProgress {
	if task != nil {
		return task
	}
	if fallback == nil {
		fallback = &progress.TaskProgress{
			Status:   progress.StatusPending,
			Progress: 0,
		}
	}
	fallback.TaskID = taskID
	return fallback
}

func statusForProgress(progressPct int) string {
	switch {
	case progressPct >= 100:
		return progress.StatusCompleted
	case progressPct > 0:
		return progress.StatusInProgress
	default:
		return progress.StatusPending
	}
}

// progressReasonFor returns a label for the lifecycle gate based on the
// active task-update flag, so the block message tells the agent which
// command triggered it.
func progressReasonFor(opts *progressOptions) string {
	switch {
	case opts.complete:
		return "fest progress --complete"
	case opts.inProgress:
		return "fest progress --in-progress"
	case opts.update != "":
		return "fest progress --update"
	case opts.blocker != "":
		return "fest progress --blocker"
	case opts.clear:
		return "fest progress --clear"
	}
	return "fest progress"
}

func parsePercentage(s string) (int, error) {
	s = strings.TrimSuffix(s, "%")
	pct, err := strconv.Atoi(s)
	if err != nil {
		return 0, errors.Validation("invalid percentage").WithField("value", s)
	}
	if pct < 0 || pct > 100 {
		return 0, errors.Validation("percentage must be 0-100").WithField("value", pct)
	}
	return pct, nil
}

func validateTaskOptions(opts *progressOptions) error {
	if opts.taskPath != "" && (opts.taskID != "" || opts.phase != "" || opts.sequence != "") {
		return errors.Validation("use --path or --task/--phase/--sequence, not both")
	}

	if (opts.phase != "" || opts.sequence != "") && opts.taskID == "" {
		return errors.Validation("--phase/--sequence require --task")
	}

	if (opts.phase == "") != (opts.sequence == "") {
		return errors.Validation("both --phase and --sequence must be provided together")
	}

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
		fmt.Println(ui.ErrorPanel(failure.Name, formatGateFailure(failure)))
		fmt.Println()
	}

	fmt.Println(ui.Info("Fix the issues above and try again. Quality gates enforce code quality."))
	fmt.Println()
}

// formatGateFailure formats a single gate failure for display.
func formatGateFailure(failure gates.GateFailure) string {
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
	return sb.String()
}
