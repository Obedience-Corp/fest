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
	"github.com/Obedience-Corp/fest/internal/lifecycle"
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
			// Validate festival status using internal schema
			if !isValidFestivalStatus(newStatus) {
				validOptions := getValidFestivalStatuses()
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
	oldStatus, err := readGoalStatus(ctx, goalPath)
	if err != nil {
		return err
	}

	if string(oldStatus) == newStatus {
		return emitPhaseStatusAlready(display, opts, phaseName, newStatus)
	}

	if err := UpdateGoalFrontmatter(ctx, goalPath, frontmatter.Status(newStatus)); err != nil {
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

	if err := lifecycle.EnforcePreActive(ctx, festivalPath, lifecycle.EnforceOptions{
		TaskID: taskID,
		Reason: "fest status set",
	}); err != nil {
		return err
	}

	// Create progress manager with the lifecycle gate so mutations refuse
	// pre-active festivals as defense in depth.
	mgr, err := progress.NewManagerWithGate(ctx, festivalPath,
		lifecycle.NewGateWithReason(festivalPath, "fest status set"))
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

// resolveTaskID normalizes a task identifier to a festival-relative path.
func resolveTaskID(festivalPath, cwd, taskInput string) (string, error) {
	// If it's already a full path within the festival, extract relative part
	if strings.HasPrefix(taskInput, festivalPath) {
		return strings.TrimPrefix(taskInput, festivalPath+"/"), nil
	}

	// If it's a relative path starting with ./ or ../
	if strings.HasPrefix(taskInput, "./") || strings.HasPrefix(taskInput, "../") {
		absPath := filepath.Join(cwd, taskInput)
		if strings.HasPrefix(absPath, festivalPath) {
			return strings.TrimPrefix(absPath, festivalPath+"/"), nil
		}
		return "", errors.Validation("path is outside festival").
			WithField("path", taskInput).
			WithField("festival", festivalPath)
	}

	// If it looks like a phase/sequence/task path (e.g., 001/01/01_task.md)
	if strings.Contains(taskInput, "/") || strings.HasSuffix(taskInput, ".md") {
		// Verify it exists
		fullPath := filepath.Join(festivalPath, taskInput)
		if _, err := os.Stat(fullPath); err == nil {
			return taskInput, nil
		}
	}

	// Try to find in current directory context
	// If cwd is within festival, try appending task name
	if strings.HasPrefix(cwd, festivalPath) {
		relCwd := strings.TrimPrefix(cwd, festivalPath+"/")
		testPath := filepath.Join(relCwd, taskInput)
		fullPath := filepath.Join(festivalPath, testPath)
		if _, err := os.Stat(fullPath); err == nil {
			return testPath, nil
		}
	}

	// Finally, try searching for the task
	return findTaskByName(festivalPath, taskInput)
}

// findTaskByName searches for a task file by name within a festival.
func findTaskByName(festivalPath, taskName string) (string, error) {
	var matches []string

	err := filepath.Walk(festivalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}

		// Check if this matches the task name
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			if info.Name() == taskName || strings.Contains(info.Name(), taskName) {
				relPath := strings.TrimPrefix(path, festivalPath+"/")
				matches = append(matches, relPath)
			}
		}

		return nil
	})
	if err != nil {
		return "", errors.Wrap(err, "searching for task")
	}

	if len(matches) == 0 {
		return "", errors.NotFound("task").
			WithField("name", taskName).
			WithField("hint", "use full path like '001/01/01_task.md'")
	}

	if len(matches) > 1 {
		return "", errors.Validation("ambiguous task name").
			WithField("name", taskName).
			WithField("matches", strings.Join(matches, ", ")).
			WithField("hint", "use full path to disambiguate")
	}

	return matches[0], nil
}

// emitTaskStatusAlready outputs message when task is already at the requested status.
func emitTaskStatusAlready(display *ui.UI, opts *statusOptions, taskID, status string) error {
	if opts.json {
		result := map[string]any{
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
		result := map[string]any{
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
	oldStatus, err := readGoalStatus(ctx, goalPath)
	if err != nil {
		return err
	}

	if string(oldStatus) == newStatus {
		return emitPhaseStatusAlready(display, opts, phaseName, newStatus)
	}

	if err := UpdateGoalFrontmatter(ctx, goalPath, frontmatter.Status(newStatus)); err != nil {
		return err
	}

	return emitPhaseStatusSuccess(display, opts, phaseName, string(oldStatus), newStatus)
}

// resolvePhase finds a phase directory by name or number.
func resolvePhase(festivalPath, phaseInput string) (string, string, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", "", errors.IO("reading festival directory", err)
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip hidden and metadata directories
		if strings.HasPrefix(name, ".") {
			continue
		}
		// Check for phase match (by prefix number or full name)
		if strings.HasPrefix(name, phaseInput) || name == phaseInput {
			matches = append(matches, name)
		}
	}

	if len(matches) == 0 {
		return "", "", errors.NotFound("phase").
			WithField("input", phaseInput).
			WithField("hint", "use phase number like '001' or full name like '001_CRITICAL'")
	}

	if len(matches) > 1 {
		return "", "", errors.Validation("ambiguous phase").
			WithField("input", phaseInput).
			WithField("matches", strings.Join(matches, ", "))
	}

	return filepath.Join(festivalPath, matches[0]), matches[0], nil
}

// emitPhaseStatusAlready outputs message when phase is already at the requested status.
func emitPhaseStatusAlready(display *ui.UI, opts *statusOptions, phaseName, status string) error {
	if opts.json {
		result := map[string]any{
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
		result := map[string]any{
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
	oldStatus, err := readGoalStatus(ctx, goalPath)
	if err != nil {
		return err
	}

	if string(oldStatus) == newStatus {
		return emitSequenceStatusAlready(display, opts, seqName, newStatus)
	}

	if err := UpdateGoalFrontmatter(ctx, goalPath, frontmatter.Status(newStatus)); err != nil {
		return err
	}

	return emitSequenceStatusSuccess(display, opts, seqName, string(oldStatus), newStatus)
}

// resolveSequence finds a sequence directory by name or path.
func resolveSequence(festivalPath, cwd, seqInput string) (string, string, error) {
	// If input contains a slash, treat as phase/sequence path
	if strings.Contains(seqInput, "/") {
		parts := strings.SplitN(seqInput, "/", 2)
		phasePath, phaseName, err := resolvePhase(festivalPath, parts[0])
		if err != nil {
			return "", "", err
		}
		seqPath, seqName, err := findSequenceInPhase(phasePath, parts[1])
		if err != nil {
			return "", "", err
		}
		return seqPath, phaseName + "/" + seqName, nil
	}

	// Otherwise, search in current phase context or all phases
	// First check if we're in a phase directory
	if strings.HasPrefix(cwd, festivalPath) {
		relPath := strings.TrimPrefix(cwd, festivalPath+"/")
		parts := strings.Split(relPath, "/")
		if len(parts) >= 1 {
			// Try to find sequence in current phase
			phasePath := filepath.Join(festivalPath, parts[0])
			seqPath, seqName, err := findSequenceInPhase(phasePath, seqInput)
			if err == nil {
				return seqPath, parts[0] + "/" + seqName, nil
			}
		}
	}

	// Search all phases for the sequence
	return findSequenceGlobally(festivalPath, seqInput)
}

// findSequenceInPhase finds a sequence within a specific phase.
func findSequenceInPhase(phasePath, seqInput string) (string, string, error) {
	entries, err := os.ReadDir(phasePath)
	if err != nil {
		return "", "", errors.IO("reading phase directory", err)
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if strings.HasPrefix(name, seqInput) || name == seqInput {
			matches = append(matches, name)
		}
	}

	if len(matches) == 0 {
		return "", "", errors.NotFound("sequence in phase").
			WithField("input", seqInput)
	}

	if len(matches) > 1 {
		return "", "", errors.Validation("ambiguous sequence").
			WithField("input", seqInput).
			WithField("matches", strings.Join(matches, ", "))
	}

	return filepath.Join(phasePath, matches[0]), matches[0], nil
}

// findSequenceGlobally searches all phases for a sequence.
func findSequenceGlobally(festivalPath, seqInput string) (string, string, error) {
	phases, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", "", errors.IO("reading festival directory", err)
	}

	var matches []string
	for _, phase := range phases {
		if !phase.IsDir() || strings.HasPrefix(phase.Name(), ".") {
			continue
		}
		phasePath := filepath.Join(festivalPath, phase.Name())
		sequences, err := os.ReadDir(phasePath)
		if err != nil {
			continue
		}
		for _, seq := range sequences {
			if !seq.IsDir() || strings.HasPrefix(seq.Name(), ".") {
				continue
			}
			if strings.HasPrefix(seq.Name(), seqInput) || seq.Name() == seqInput {
				matches = append(matches, phase.Name()+"/"+seq.Name())
			}
		}
	}

	if len(matches) == 0 {
		return "", "", errors.NotFound("sequence").
			WithField("input", seqInput).
			WithField("hint", "use phase/sequence format like '001/01_api_design'")
	}

	if len(matches) > 1 {
		return "", "", errors.Validation("ambiguous sequence").
			WithField("input", seqInput).
			WithField("matches", strings.Join(matches, ", ")).
			WithField("hint", "use phase/sequence format to disambiguate")
	}

	parts := strings.SplitN(matches[0], "/", 2)
	return filepath.Join(festivalPath, parts[0], parts[1]), matches[0], nil
}

// emitSequenceStatusAlready outputs message when sequence is already at the requested status.
func emitSequenceStatusAlready(display *ui.UI, opts *statusOptions, seqName, status string) error {
	if opts.json {
		result := map[string]any{
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
		result := map[string]any{
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
