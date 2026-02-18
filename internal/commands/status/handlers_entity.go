package status

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// handlePathBasedStatusSet handles --path flag by detecting entity type and routing accordingly.
// This allows setting festival status from anywhere in the workspace by passing the festival name.
func handlePathBasedStatusSet(ctx context.Context, display *ui.UI, cwd, newStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// First, try to resolve as a festival
	festivalPath, err := resolveFestivalFromPath(cwd, opts.path)
	if err == nil {
		// Successfully resolved as a festival - detect entity type to confirm
		entityType := detectEntityType(festivalPath)

		switch entityType {
		case EntityFestival:
			// Validate festival status (schema-aware)
			pathFestivalsRoot := findFestivalsRoot(festivalPath)
			if !isValidFestivalStatus(pathFestivalsRoot, newStatus) {
				validOptions := getValidFestivalStatuses(pathFestivalsRoot)
				return errors.Validation("invalid status for festival").
					WithField("status", newStatus).
					WithField("valid_options", strings.Join(validOptions, ", "))
			}

			// Get festival info using DetectCurrentLocation
			loc, locErr := show.DetectCurrentLocation(ctx, festivalPath)
			if locErr != nil {
				return errors.Wrap(locErr, "detecting festival info")
			}
			if loc.Festival == nil {
				return errors.NotFound("festival info").WithField("path", festivalPath)
			}

			return handleFestivalStatusChange(ctx, display, loc.Festival, newStatus, opts)

		case EntityPhase:
			// Path points to a phase - set phase name and route to phase handler
			opts.phase = filepath.Base(festivalPath)
			// Get festival root (parent of phase)
			festivalRoot := filepath.Dir(festivalPath)
			return handlePhaseStatusSetWithPath(ctx, display, festivalRoot, newStatus, opts)

		case EntitySequence:
			// Path points to a sequence - more complex routing needed
			// For now, fall through to task handling which can resolve sequences
			break

		case EntityTask:
			// Path points to a task file - route to task handler
			return handleTaskStatusSet(ctx, display, cwd, newStatus, opts)
		}
	}

	// Path didn't resolve as a festival - try as a task path
	// This maintains backward compatibility for task-level --path usage
	return handleTaskStatusSet(ctx, display, cwd, newStatus, opts)
}

// handlePhaseStatusSetWithPath handles phase status when we already know the festival path.
func handlePhaseStatusSetWithPath(ctx context.Context, display *ui.UI, festivalPath, newStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// Validate status for phases
	if !isValidStatus(EntityPhase, newStatus) {
		validOptions := ValidStatuses[EntityPhase]
		return errors.Validation("invalid status for phase").
			WithField("status", newStatus).
			WithField("valid_options", strings.Join(validOptions, ", "))
	}

	// Find the phase directory
	phasePath, phaseName, err := resolvePhase(festivalPath, opts.phase)
	if err != nil {
		return err
	}

	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")
	oldStatus, err := readGoalStatus(goalPath)
	if err != nil {
		return err
	}

	if string(oldStatus) == newStatus {
		return emitPhaseStatusAlready(display, opts, phaseName, newStatus)
	}

	if err := updateGoalFrontmatter(goalPath, frontmatter.Status(newStatus)); err != nil {
		return err
	}

	return emitPhaseStatusSuccess(display, opts, phaseName, string(oldStatus), newStatus)
}

// handleTaskStatusSet handles setting status for a specific task.
func handleTaskStatusSet(ctx context.Context, display *ui.UI, cwd, newStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// Validate status for tasks
	if !isValidStatus(EntityTask, newStatus) {
		validOptions := ValidStatuses[EntityTask]
		return errors.Validation("invalid status for task").
			WithField("status", newStatus).
			WithField("valid_options", strings.Join(validOptions, ", "))
	}

	// Resolve festival path
	festivalPath, err := shared.ResolveFestivalPath(cwd, "")
	if err != nil {
		return errors.Wrap(err, "not inside a festival").
			WithField("hint", "navigate to a festival directory or use 'fest link'")
	}

	// Determine task ID from flag
	taskID := opts.task
	if opts.path != "" {
		taskID = opts.path
	}

	// Normalize task ID - resolve relative to festival
	taskID, err = resolveTaskID(festivalPath, cwd, taskID)
	if err != nil {
		return err
	}

	// Create progress manager
	mgr, err := progress.NewManager(ctx, festivalPath)
	if err != nil {
		return errors.Wrap(err, "creating progress manager")
	}

	// Get current status for display
	currentTask, exists := mgr.GetTaskProgress(taskID)
	currentStatus := "pending"
	if exists {
		currentStatus = string(currentTask.Status)
	}

	// Check if already at target status
	if currentStatus == newStatus {
		return emitTaskStatusAlready(display, opts, taskID, newStatus)
	}

	// Apply the status change based on target status
	switch newStatus {
	case "pending":
		// Reset to pending - set progress to 0
		if err := mgr.UpdateProgress(ctx, taskID, 0); err != nil {
			return errors.Wrap(err, "resetting task status")
		}
	case "in_progress":
		if err := mgr.MarkInProgress(ctx, taskID); err != nil {
			return errors.Wrap(err, "marking task in progress")
		}
	case "blocked":
		// For blocked, we need a message - use generic if not provided
		if err := mgr.ReportBlocker(ctx, taskID, "Blocked via status set"); err != nil {
			return errors.Wrap(err, "marking task blocked")
		}
	case "completed":
		if err := mgr.MarkComplete(ctx, taskID); err != nil {
			return errors.Wrap(err, "marking task complete")
		}
	}

	return emitTaskStatusSuccess(display, opts, taskID, currentStatus, newStatus)
}

// emitTaskStatusAlready outputs message when task is already at the requested status.
func emitTaskStatusAlready(display *ui.UI, opts *statusOptions, taskID, status string) error {
	if opts.json {
		result := map[string]interface{}{
			"success": true,
			"message": "task already at requested status",
			"task":    taskID,
			"status":  status,
		}
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		fmt.Printf("%s %s\n", ui.Info("Task already at status"), ui.GetStateStyle(status).Render(status))
		fmt.Printf("%s %s\n", ui.Label("Task"), ui.Dim(taskID))
	}
	return nil
}

// emitTaskStatusSuccess outputs success message after changing task status.
func emitTaskStatusSuccess(display *ui.UI, opts *statusOptions, taskID, oldStatus, newStatus string) error {
	if opts.json {
		result := map[string]interface{}{
			"success":    true,
			"task":       taskID,
			"old_status": oldStatus,
			"new_status": newStatus,
		}
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		fmt.Println(ui.Success("✓ Task status updated"))
		fmt.Printf("%s %s\n", ui.Label("Task"), ui.Dim(taskID))
		fmt.Printf("%s %s\n", ui.Label("From"), ui.GetStateStyle(oldStatus).Render(oldStatus))
		fmt.Printf("%s %s\n", ui.Label("To"), ui.GetStateStyle(newStatus).Render(newStatus))
	}
	return nil
}

// handlePhaseStatusSet handles setting status for a specific phase.
func handlePhaseStatusSet(ctx context.Context, display *ui.UI, cwd, newStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// Validate status for phases
	if !isValidStatus(EntityPhase, newStatus) {
		validOptions := ValidStatuses[EntityPhase]
		return errors.Validation("invalid status for phase").
			WithField("status", newStatus).
			WithField("valid_options", strings.Join(validOptions, ", "))
	}

	// Resolve festival path
	festivalPath, err := shared.ResolveFestivalPath(cwd, "")
	if err != nil {
		return errors.Wrap(err, "not inside a festival").
			WithField("hint", "navigate to a festival directory or use 'fest link'")
	}

	// Find the phase directory
	phasePath, phaseName, err := resolvePhase(festivalPath, opts.phase)
	if err != nil {
		return err
	}

	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")
	oldStatus, err := readGoalStatus(goalPath)
	if err != nil {
		return err
	}

	if string(oldStatus) == newStatus {
		return emitPhaseStatusAlready(display, opts, phaseName, newStatus)
	}

	if err := updateGoalFrontmatter(goalPath, frontmatter.Status(newStatus)); err != nil {
		return err
	}

	return emitPhaseStatusSuccess(display, opts, phaseName, string(oldStatus), newStatus)
}

// emitPhaseStatusAlready outputs message when phase is already at the requested status.
func emitPhaseStatusAlready(display *ui.UI, opts *statusOptions, phaseName, status string) error {
	if opts.json {
		result := map[string]interface{}{
			"success": true,
			"message": "phase already at requested status",
			"phase":   phaseName,
			"status":  status,
		}
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		fmt.Printf("%s %s\n", ui.Info("Phase already at status"), ui.GetStateStyle(status).Render(status))
		fmt.Printf("%s %s\n", ui.Label("Phase"), ui.Value(phaseName, ui.PhaseColor))
	}
	return nil
}

// emitPhaseStatusSuccess outputs success message after changing phase status.
func emitPhaseStatusSuccess(display *ui.UI, opts *statusOptions, phaseName, oldStatus, newStatus string) error {
	if opts.json {
		result := map[string]interface{}{
			"success":    true,
			"phase":      phaseName,
			"old_status": oldStatus,
			"new_status": newStatus,
		}
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		fmt.Println(ui.Success("✓ Phase status updated"))
		fmt.Printf("%s %s\n", ui.Label("Phase"), ui.Value(phaseName, ui.PhaseColor))
		fmt.Printf("%s %s\n", ui.Label("From"), ui.GetStateStyle(oldStatus).Render(oldStatus))
		fmt.Printf("%s %s\n", ui.Label("To"), ui.GetStateStyle(newStatus).Render(newStatus))
	}
	return nil
}

// handleSequenceStatusSet handles setting status for a specific sequence.
func handleSequenceStatusSet(ctx context.Context, display *ui.UI, cwd, newStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// Validate status for sequences
	if !isValidStatus(EntitySequence, newStatus) {
		validOptions := ValidStatuses[EntitySequence]
		return errors.Validation("invalid status for sequence").
			WithField("status", newStatus).
			WithField("valid_options", strings.Join(validOptions, ", "))
	}

	// Resolve festival path
	festivalPath, err := shared.ResolveFestivalPath(cwd, "")
	if err != nil {
		return errors.Wrap(err, "not inside a festival").
			WithField("hint", "navigate to a festival directory or use 'fest link'")
	}

	// Find the sequence directory
	seqPath, seqName, err := resolveSequence(festivalPath, cwd, opts.sequence)
	if err != nil {
		return err
	}

	goalPath := filepath.Join(seqPath, "SEQUENCE_GOAL.md")
	oldStatus, err := readGoalStatus(goalPath)
	if err != nil {
		return err
	}

	if string(oldStatus) == newStatus {
		return emitSequenceStatusAlready(display, opts, seqName, newStatus)
	}

	if err := updateGoalFrontmatter(goalPath, frontmatter.Status(newStatus)); err != nil {
		return err
	}

	return emitSequenceStatusSuccess(display, opts, seqName, string(oldStatus), newStatus)
}

// emitSequenceStatusAlready outputs message when sequence is already at the requested status.
func emitSequenceStatusAlready(display *ui.UI, opts *statusOptions, seqName, status string) error {
	if opts.json {
		result := map[string]interface{}{
			"success":  true,
			"message":  "sequence already at requested status",
			"sequence": seqName,
			"status":   status,
		}
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		fmt.Printf("%s %s\n", ui.Info("Sequence already at status"), ui.GetStateStyle(status).Render(status))
		fmt.Printf("%s %s\n", ui.Label("Sequence"), ui.Value(seqName, ui.SequenceColor))
	}
	return nil
}

// emitSequenceStatusSuccess outputs success message after changing sequence status.
func emitSequenceStatusSuccess(display *ui.UI, opts *statusOptions, seqName, oldStatus, newStatus string) error {
	if opts.json {
		result := map[string]interface{}{
			"success":    true,
			"sequence":   seqName,
			"old_status": oldStatus,
			"new_status": newStatus,
		}
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		fmt.Println(ui.Success("✓ Sequence status updated"))
		fmt.Printf("%s %s\n", ui.Label("Sequence"), ui.Value(seqName, ui.SequenceColor))
		fmt.Printf("%s %s\n", ui.Label("From"), ui.GetStateStyle(oldStatus).Render(oldStatus))
		fmt.Printf("%s %s\n", ui.Label("To"), ui.GetStateStyle(newStatus).Render(newStatus))
	}
	return nil
}
